package miniobenchmarking

import (
	"encoding/hex"
	"github.com/minio/highwayhash"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
)

func benchmarkHwhParallel(b *testing.B, size, c int) {

	restore := runtime.GOMAXPROCS(c)

	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")

	rng := rand.New(rand.NewSource(0xabadc0cac01a))
	data := make([][]byte, c)
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

func BenchmarkHighwayhashSingle(b *testing.B) {
	b.Run("1M", func(b *testing.B) {
		benchmarkHwhParallel(b, 1*1024*1024, 1)
	})
	b.Run("5M", func(b *testing.B) {
		benchmarkHwhParallel(b, 5*1024*1024, 1)
	})
	b.Run("10M", func(b *testing.B) {
		benchmarkHwhParallel(b, 10*1024*1024, 1)
	})
	b.Run("25M", func(b *testing.B) {
		benchmarkHwhParallel(b, 25*1024*1024, 1)
	})
	b.Run("50M", func(b *testing.B) {
		benchmarkHwhParallel(b, 50*1024*1024, 1)
	})
	b.Run("100M", func(b *testing.B) {
		benchmarkHwhParallel(b, 100*1024*1024, 1)
	})
}

func BenchmarkHighwayhashParallel(b *testing.B) {
	b.Run("1M", func(b *testing.B) {
		benchmarkHwhParallel(b, 1*1024*1024, runtime.GOMAXPROCS(0))
	})
	b.Run("5M", func(b *testing.B) {
		benchmarkHwhParallel(b, 5*1024*1024, runtime.GOMAXPROCS(0))
	})
	b.Run("10M", func(b *testing.B) {
		benchmarkHwhParallel(b, 10*1024*1024, runtime.GOMAXPROCS(0))
	})
	b.Run("25M", func(b *testing.B) {
		benchmarkHwhParallel(b, 25*1024*1024, runtime.GOMAXPROCS(0))
	})
	b.Run("50M", func(b *testing.B) {
		benchmarkHwhParallel(b, 50*1024*1024, runtime.GOMAXPROCS(0))
	})
	b.Run("100M", func(b *testing.B) {
		benchmarkHwhParallel(b, 100*1024*1024, runtime.GOMAXPROCS(0))
	})
}

