package miniobenchmarking

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"github.com/klauspost/reedsolomon"
	"github.com/minio/highwayhash"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
)

func benchmarkHighwayhash(b *testing.B, size int) {

	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")

	rng := rand.New(rand.NewSource(0xabadc0cac01a))
	data := make([][]byte, runtime.GOMAXPROCS(0))
	for i := range data {
		data[i] = make([]byte, size)
		rng.Read(data[i])
	}

	b.SetBytes(int64(size))
	b.ResetTimer()

	counter := uint64(0)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			index := atomic.AddUint64(&counter, 1)
			highwayhash.Sum(data[int(index)%len(data)], key[:])
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

	fillRandom := func(p []byte) {
		for i := 0; i < len(p); i += 7 {
			val := rand.Int63()
			for j := 0; i+j < len(p) && j < 7; j++ {
				p[i+j] = byte(val)
				val >>= 8
			}
		}
	}

	r, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		b.Fatal(err)
	}

	// Create independent shards
	shardsCh := make(chan [][]byte, runtime.GOMAXPROCS(0))
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		rand.Seed(int64(i))
		shards := make([][]byte, dataShards+parityShards)
		for s := range shards {
			shards[s] = make([]byte, shardSize)
		}
		for s := 0; s < dataShards; s++ {
			fillRandom(shards[s])
		}
		shardsCh <- shards
	}

	b.SetBytes(int64(shardSize * dataShards))
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			shards := <-shardsCh
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

	data := make([][]byte, runtime.GOMAXPROCS(0))

	rng := rand.New(rand.NewSource(0xabadc0cac01a))
	for i := range data {
		data[i] = make([]byte, size)
		rng.Read(data[i])
	}

	keys := make([][16]byte, len(data))
	nonces := make([][12]byte, len(data))
	ads := make([][13]byte, len(data))
	aeses := make([]cipher.Block, len(data))
	aesgcms := make([]cipher.AEAD, len(data))
	outs := make([][]byte, len(data))

	for i, k := range keys {
		aeses[i], _ = aes.NewCipher(k[:])
		aesgcms[i], _ = cipher.NewGCM(aeses[i])
		outs[i] = []byte{}
	}

	b.SetBytes(int64(size))
	b.ResetTimer()

	counter := uint64(0)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			index := int(atomic.AddUint64(&counter, 1)) % len(data)
			outs[index] = aesgcms[index].Seal(outs[index][:0], nonces[index][:], data[index], ads[index][:])
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
