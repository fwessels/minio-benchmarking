package miniobenchmarking

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"math/rand"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/klauspost/reedsolomon"
	"github.com/minio/highwayhash"
)

type Uncached struct {
	data   []byte
	offset int64
}

func NewUncached(size int, randomize bool) (uncached *Uncached) {
	uncached = &Uncached{}
	uncached.data = make([]byte, size)
	if randomize {
		r := rand.New(rand.NewSource(123))
		r.Read(uncached.data)
	}
	return
}

func (uncached *Uncached) GetSlice(size int) []byte {
	sz := int64(size)
	n := int64(len(uncached.data))
	if sz <= 0 || sz > n { // bad input
		return nil
	}
	for {
		old := atomic.LoadInt64(&uncached.offset)

		// Try to allocate at the current offset.
		end := old + sz

		var start, newOff int64
		if end <= n {
			// Normal case: reserve [old, end)
			start = old
			newOff = end
		} else {
			// Wrap case: reserve [0, size)
			start = 0
			newOff = sz
		}

		// Attempt to commit the new offset (single linearizable update).
		if atomic.CompareAndSwapInt64(&uncached.offset, old, newOff) {
			return uncached.data[start : start+sz]
		}
		// else: someone else moved offset; retry
	}
}

var (
	input  *Uncached
	output *Uncached
	parity *Uncached
)

func init() {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		panic(err)
	}
	totalMem := si.Totalram * uint64(si.Unit)
	budget := int(float64(totalMem) * 0.05)
	const maxBudget = 5 * 1024 * 1024 * 1024
	if budget > maxBudget {
		budget = maxBudget
	}

	input = NewUncached(budget, true)
	output = NewUncached(budget, false)
	parity = NewUncached(budget, false)
	fmt.Println("Uncached input :", len(input.data)/1024/1024, "MB")
	fmt.Println("Uncached output:", len(output.data)/1024/1024, "MB")
	fmt.Println("Uncached parity:", len(parity.data)/1024/1024, "MB")
}

func benchmarkHighwayhash(b *testing.B, size int) {

	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")

	b.SetBytes(int64(size))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			data := input.GetSlice(size)
			highwayhash.Sum(data, key[:])
		}
	})
}

func BenchmarkHighwayhash(b *testing.B) {
	b.Run("1M", func(b *testing.B) {
		benchmarkHighwayhash(b, 1024*1024)
	})
	b.Run("5M", func(b *testing.B) {
		benchmarkHighwayhash(b, 5*1024*1024)
	})
	b.Run("10M", func(b *testing.B) {
		benchmarkHighwayhash(b, 10*1024*1024)
	})
	b.Run("25M", func(b *testing.B) {
		benchmarkHighwayhash(b, 25*1024*1024)
	})
	b.Run("50M", func(b *testing.B) {
		benchmarkHighwayhash(b, 50*1024*1024)
	})
	b.Run("100M", func(b *testing.B) {
		benchmarkHighwayhash(b, 100*1024*1024)
	})
}

func benchmarkRsParallel(b *testing.B, dataShards, parityShards, shardSize int) {

	r, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		b.Fatal(err)
	}

	// Create independent shards
	shardsCh := make(chan [][]byte, runtime.GOMAXPROCS(0))
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		shards := make([][]byte, dataShards+parityShards)
		shardsCh <- shards
	}

	b.SetBytes(int64(shardSize * dataShards))
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			shards := <-shardsCh
			for s := range shards {
				if s < dataShards {
					shards[s] = input.GetSlice(shardSize)
				} else {
					shards[s] = parity.GetSlice(shardSize)
				}
			}
			err = r.Encode(shards)
			if err != nil {
				b.Fatal(err)
			}
			shardsCh <- shards
		}
	})
}

func benchmarkReedSolomon(b *testing.B, dataShards, parityShards int) {
	b.Run(fmt.Sprintf("%dx%d_1M", dataShards, parityShards), func(b *testing.B) {
		benchmarkRsParallel(b, dataShards, parityShards, 1024*1024)
	})
	b.Run(fmt.Sprintf("%dx%d_5M", dataShards, parityShards), func(b *testing.B) {
		benchmarkRsParallel(b, dataShards, parityShards, 5*1024*1024)
	})
	b.Run(fmt.Sprintf("%dx%d_10M", dataShards, parityShards), func(b *testing.B) {
		benchmarkRsParallel(b, dataShards, parityShards, 10*1024*1024)
	})
	b.Run(fmt.Sprintf("%dx%d_25M", dataShards, parityShards), func(b *testing.B) {
		benchmarkRsParallel(b, dataShards, parityShards, 25*1024*1024)
	})
}

func BenchmarkReedsolomon(b *testing.B) {
	benchmarkReedSolomon(b, 8, 8)
	benchmarkReedSolomon(b, 12, 4)
}

func benchmarkAESGCM(b *testing.B, size int) {
	type State struct {
		key    [16]byte
		nonce  [12]byte
		ad     [13]byte
		aes    cipher.Block
		aesgcm cipher.AEAD
	}

	ch := make(chan State, runtime.GOMAXPROCS(0))
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		state := State{}
		state.aes, _ = aes.NewCipher(state.key[:])
		state.aesgcm, _ = cipher.NewGCM(state.aes)
		ch <- state
	}

	b.SetBytes(int64(size))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			state := <-ch
			data := input.GetSlice(size)
			out := output.GetSlice(size + state.aesgcm.Overhead())
			out = state.aesgcm.Seal(out[:0], state.nonce[:], data, state.ad[:])
			ch <- state
		}
	})
}

func BenchmarkAESGCM(b *testing.B) {
	b.Run("1M", func(b *testing.B) {
		benchmarkAESGCM(b, 1024*1024)
	})
	b.Run("5M", func(b *testing.B) {
		benchmarkAESGCM(b, 5*1024*1024)
	})
	b.Run("10M", func(b *testing.B) {
		benchmarkAESGCM(b, 10*1024*1024)
	})
	b.Run("25M", func(b *testing.B) {
		benchmarkAESGCM(b, 25*1024*1024)
	})
	b.Run("50M", func(b *testing.B) {
		benchmarkAESGCM(b, 50*1024*1024)
	})
}
