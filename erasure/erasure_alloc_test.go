package erasure

import (
	"testing"

	"github.com/klauspost/reedsolomon"
)

func BenchmarkSplitOldPath(b *testing.B) {
	dataShards := 10
	parityShards := 20
	blockSize := 1024 * 1024
	stripeSize := dataShards * blockSize

	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		b.Fatal(err)
	}

	buf := make([]byte, stripeSize) // len=10MB, cap=10MB

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		shards, err := enc.Split(buf)
		if err != nil {
			b.Fatal(err)
		}
		if err := enc.Encode(shards); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSplitNewPath(b *testing.B) {
	dataShards := 10
	parityShards := 20
	blockSize := 1024 * 1024
	stripeSize := dataShards * blockSize
	totalSize := (dataShards + parityShards) * blockSize

	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		b.Fatal(err)
	}

	buf := make([]byte, stripeSize, totalSize) // len=10MB, cap=30MB

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		shards, err := enc.Split(buf)
		if err != nil {
			b.Fatal(err)
		}
		if err := enc.Encode(shards); err != nil {
			b.Fatal(err)
		}
	}
}
