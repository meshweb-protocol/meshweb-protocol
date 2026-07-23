package erasure_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/klauspost/reedsolomon"
	"github.com/meshweb/meshweb-protocol/erasure"
)

func TestReconstructSegment(t *testing.T) {
	dataShards := 10
	parityShards := 20
	totalShards := dataShards + parityShards
	blockSize := 1024 * 1024 // 1MB
	stripeLen := dataShards * blockSize

	// 1. Generate random original segment
	origSegment := make([]byte, stripeLen)
	rand.Read(origSegment)

	// Hash the original segment
	h1 := sha256.New()
	h1.Write(origSegment)
	origHash := hex.EncodeToString(h1.Sum(nil))

	// 2. Encode into shards
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}
	shards, err := enc.Split(origSegment)
	if err != nil {
		t.Fatalf("failed to split: %v", err)
	}
	if err := enc.Encode(shards); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	// 3. Simulate missing shards (Keep exactly 10 shards)
	testShards := make([][]byte, totalShards)
	// We'll keep the first 5 data shards and the last 5 parity shards
	for i := 0; i < 5; i++ {
		testShards[i] = append([]byte(nil), shards[i]...)
	}
	for i := totalShards - 5; i < totalShards; i++ {
		testShards[i] = append([]byte(nil), shards[i]...)
	}

	// 4. Reconstruct
	var outBuf bytes.Buffer
	err = erasure.ReconstructSegment(dataShards, parityShards, testShards, &outBuf, stripeLen)
	if err != nil {
		t.Fatalf("ReconstructSegment failed: %v", err)
	}

	// 5. Compare hash
	h2 := sha256.New()
	h2.Write(outBuf.Bytes())
	reconHash := hex.EncodeToString(h2.Sum(nil))

	if origHash != reconHash {
		t.Fatalf("Hash mismatch!\nExpected: %s\nGot:      %s", origHash, reconHash)
	}
}
