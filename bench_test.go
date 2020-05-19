package miniobenchmarking

import (
	"encoding/hex"
	"fmt"
	"github.com/klauspost/reedsolomon"
	"github.com/minio/highwayhash"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
)

func benchmarkHwhParallel(b *testing.B, size, concurrency int) {

	restore := runtime.GOMAXPROCS(concurrency)

	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")

	rng := rand.New(rand.NewSource(0xabadc0cac01a))
	data := make([][]byte, concurrency)
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

	runtime.GOMAXPROCS(restore)
}

func benchmarkHighwayhash(b *testing.B, concurrency int) {
	b.Run("1M", func(b *testing.B) {
		benchmarkHwhParallel(b, 1*1024*1024, concurrency)
	})
	b.Run("5M", func(b *testing.B) {
		benchmarkHwhParallel(b, 5*1024*1024, concurrency)
	})
	b.Run("10M", func(b *testing.B) {
		benchmarkHwhParallel(b, 10*1024*1024, concurrency)
	})
	b.Run("25M", func(b *testing.B) {
		benchmarkHwhParallel(b, 25*1024*1024, concurrency)
	})
	b.Run("50M", func(b *testing.B) {
		benchmarkHwhParallel(b, 50*1024*1024, concurrency)
	})
	b.Run("100M", func(b *testing.B) {
		benchmarkHwhParallel(b, 100*1024*1024, concurrency)
	})
}

func BenchmarkHighwayhash(b *testing.B) {
	b.Run("Single", func(b *testing.B) {
		benchmarkHighwayhash(b, 1)
	})
	b.Run("Parallel", func(b *testing.B) {
		benchmarkHighwayhash(b, runtime.GOMAXPROCS(0))
	})

}

func benchmarkRsParallel(b *testing.B, dataShards, parityShards, shardSize, concurrency int) {

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

	restore := runtime.GOMAXPROCS(concurrency)

	// Create independent shards
	shardsCh := make(chan [][]byte, concurrency)
	for i := 0; i < concurrency; i++ {
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

	runtime.GOMAXPROCS(restore)
}

func benchmarkReedSolomon(b *testing.B, dataShards, parityShards, concurrency int) {
	b.Run(fmt.Sprintf("%dx%d_1M", dataShards, parityShards), func(b *testing.B) {
		benchmarkRsParallel(b, dataShards, parityShards, 1024*1024, concurrency)
	})
	b.Run(fmt.Sprintf("%dx%d_5M", dataShards, parityShards), func(b *testing.B) {
		benchmarkRsParallel(b, dataShards, parityShards, 5*1024*1024, concurrency)
	})
	b.Run(fmt.Sprintf("%dx%d_10M", dataShards, parityShards), func(b *testing.B) {
		benchmarkRsParallel(b, dataShards, parityShards, 10*1024*1024, concurrency)
	})
	b.Run(fmt.Sprintf("%dx%d_25M", dataShards, parityShards), func(b *testing.B) {
		benchmarkRsParallel(b, dataShards, parityShards, 25*1024*1024, concurrency)
	})
}

func BenchmarkReedsolomon(b *testing.B) {
	b.Run("Single", func(b *testing.B) {
		benchmarkReedSolomon(b, 8, 8, 1)
		benchmarkReedSolomon(b, 12, 4, 1)
	})
	b.Run("Parallel", func(b *testing.B) {
		benchmarkReedSolomon(b, 8, 8, runtime.GOMAXPROCS(0))
		benchmarkReedSolomon(b, 12, 4, runtime.GOMAXPROCS(0))
	})
}
