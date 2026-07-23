package erasure_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/meshweb/meshweb-protocol/erasure"
	"github.com/meshweb/meshweb-protocol/manifest"
)

func hashBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func setupRepairTestEnv(t *testing.T, dataShards, parityShards, blockSize int, origData []byte) (string, *manifest.FileManifest, map[int]string, []string) {
	tmp := t.TempDir()
	origPath := filepath.Join(tmp, "original.bin")
	os.WriteFile(origPath, origData, 0o644)

	shardPaths, _, err := erasure.EncodeFile(origPath, tmp, dataShards, parityShards, blockSize)
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	shardHashes := make([]string, len(shardPaths))
	sourcePaths := make(map[int]string)

	for i, p := range shardPaths {
		data, _ := os.ReadFile(p)
		shardHashes[i] = hashBytes(data)
		sourcePaths[i] = p
	}

	m := &manifest.FileManifest{
		FileID:       "file_repair_test",
		OriginalSize: int64(len(origData)),
		DataShards:   dataShards,
		ParityShards: parityShards,
		BlockSize:    blockSize,
		ShardHashes:  shardHashes,
		CreatedAt:    time.Now().Unix(),
	}

	return tmp, m, sourcePaths, shardPaths
}

func TestRepairShards_SingleShard(t *testing.T) {
	origData := []byte("hello world this is a test for single shard repair. we need enough data.")
	_, m, sourcePaths, shardPaths := setupRepairTestEnv(t, 4, 2, 16, origData)

	// Missing shard 2
	missing := []int{2}
	delete(sourcePaths, 2)
	os.Remove(shardPaths[2])

	res, reconstructed, err := erasure.RepairShards(m.FileID, m.DataShards, m.ParityShards, m.BlockSize, m.OriginalSize, m.ShardHashes, missing, sourcePaths)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(res.Repaired) != 1 || res.Repaired[0] != 2 {
		t.Fatalf("expected repaired [2], got %v", res.Repaired)
	}

	hash := hashBytes(reconstructed[2])
	if hash != m.ShardHashes[2] {
		t.Fatalf("hash mismatch")
	}
}

func TestRepairShards_MultipleShards(t *testing.T) {
	origData := make([]byte, 1024)
	for i := range origData {
		origData[i] = byte(i % 256)
	}
	_, m, sourcePaths, shardPaths := setupRepairTestEnv(t, 10, 5, 256, origData)

	// Missing shards 1, 4, 7, 12, 14 (5 missing)
	missing := []int{1, 4, 7, 12, 14}
	for _, idx := range missing {
		delete(sourcePaths, idx)
		os.Remove(shardPaths[idx])
	}

	res, _, err := erasure.RepairShards(m.FileID, m.DataShards, m.ParityShards, m.BlockSize, m.OriginalSize, m.ShardHashes, missing, sourcePaths)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(res.Repaired) != 5 {
		t.Fatalf("expected 5 repaired, got %d", len(res.Repaired))
	}
}

func TestRepairShards_TooManyMissing(t *testing.T) {
	origData := make([]byte, 1024)
	_, m, sourcePaths, shardPaths := setupRepairTestEnv(t, 4, 2, 256, origData)

	// Missing 3 shards (can only tolerate 2)
	missing := []int{0, 1, 2}
	for _, idx := range missing {
		delete(sourcePaths, idx)
		os.Remove(shardPaths[idx])
	}

	_, _, err := erasure.RepairShards(m.FileID, m.DataShards, m.ParityShards, m.BlockSize, m.OriginalSize, m.ShardHashes, missing, sourcePaths)
	if err == nil {
		t.Fatalf("expected error for too many missing shards")
	}
}

func TestRepairShards_CorruptedSource(t *testing.T) {
	origData := make([]byte, 1024)
	_, m, sourcePaths, shardPaths := setupRepairTestEnv(t, 4, 2, 256, origData)

	// Missing 1
	missing := []int{1}
	delete(sourcePaths, 1)
	os.Remove(shardPaths[1])

	// Corrupt shard 0
	os.WriteFile(shardPaths[0], []byte("corrupted data................................"), 0o644)

	res, _, err := erasure.RepairShards(m.FileID, m.DataShards, m.ParityShards, m.BlockSize, m.OriginalSize, m.ShardHashes, missing, sourcePaths)
	if err == nil {
		t.Fatalf("expected error due to integrity verification failure (corrupt source leads to bad reconstruction)")
	}
	if len(res.Failed) == 0 {
		t.Fatalf("expected failed shards")
	}
}

func TestRepairShards_ByteForByte(t *testing.T) {
	tmp := t.TempDir()
	origData := make([]byte, 50000)
	for i := range origData {
		origData[i] = byte(i % 255)
	}

	_, m, sourcePaths, shardPaths := setupRepairTestEnv(t, 10, 4, 1024, origData)

	// Missing shards 0, 5, 11, 13
	missing := []int{0, 5, 11, 13}
	for _, idx := range missing {
		delete(sourcePaths, idx)
		os.Remove(shardPaths[idx])
	}

	// Repair
	_, reconstructed, err := erasure.RepairShards(m.FileID, m.DataShards, m.ParityShards, m.BlockSize, m.OriginalSize, m.ShardHashes, missing, sourcePaths)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Write reconstructed to disk (B.4.c Local Commit simulation)
	for idx, data := range reconstructed {
		os.WriteFile(shardPaths[idx], data, 0o644)
		sourcePaths[idx] = shardPaths[idx] // restore present map
	}

	// Reassemble
	reassembledPath := filepath.Join(tmp, "reassembled.bin")
	_, err = erasure.ReconstructFile(reassembledPath, sourcePaths, 10, 4, 1024, int64(len(origData)))
	if err != nil {
		t.Fatalf("failed to reconstruct whole file: %v", err)
	}

	reassembledData, _ := os.ReadFile(reassembledPath)

	hashOrig := hashBytes(origData)
	hashRe := hashBytes(reassembledData)

	if hashOrig != hashRe {
		t.Fatalf("byte-for-byte mismatch! Orig: %s, Re: %s", hashOrig, hashRe)
	}
}
