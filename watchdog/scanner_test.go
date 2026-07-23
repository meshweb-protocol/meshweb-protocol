package watchdog_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/meshweb/meshweb-protocol/manifest"
	"github.com/meshweb/meshweb-protocol/watchdog"
)

func hashBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func setupTestEnvironment(t *testing.T) (string, string, string, *manifest.FileManifest) {
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	qDir := filepath.Join(tmp, "quarantine")
	os.MkdirAll(storeDir, 0o755)
	os.MkdirAll(qDir, 0o755)

	fileID := "file_123"
	fileDir := filepath.Join(storeDir, fileID)
	os.MkdirAll(fileDir, 0o755)

	// Create 5 shards
	shardHashes := make([]string, 5)
	fileHasher := sha256.New()
	for i := 0; i < 5; i++ {
		data := []byte(fmt.Sprintf("shard_data_%d", i))
		shardPath := filepath.Join(fileDir, fmt.Sprintf("shard.%02d", i))
		os.WriteFile(shardPath, data, 0o644)
		shardHashes[i] = hashBytes(data)
		fileHasher.Write(data)
	}

	m := &manifest.FileManifest{
		Version:      "meshweb-manifest/2",
		FileID:       fileID,
		FileName:     "test.bin",
		FileSize:     100,
		OriginalSize: 100,
		DataShards:   3,
		ParityShards: 2,
		MinShards:    3,
		BlockSize:    1024,
		Sha256:       hex.EncodeToString(fileHasher.Sum(nil)),
		ShardPaths:   []string{"shard.00", "shard.01", "shard.02", "shard.03", "shard.04"},
		CreatedAt:    time.Now().Unix(),
		ShardHashes:  shardHashes,
	}

	manifestPath := filepath.Join(fileDir, "manifest.json")
	mBytes, _ := json.Marshal(m)
	os.WriteFile(manifestPath, mBytes, 0o644)

	return storeDir, qDir, fileID, m
}

func TestLocalScanner_Healthy(t *testing.T) {
	storeDir, qDir, fileID, _ := setupTestEnvironment(t)
	scanner := watchdog.NewLocalScanner(storeDir, qDir)

	result, err := scanner.ScanLocalFile(fileID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.HealthyShards) != 5 {
		t.Errorf("expected 5 healthy shards, got %d", len(result.HealthyShards))
	}
	if len(result.MissingShards) != 0 {
		t.Errorf("expected 0 missing, got %d", len(result.MissingShards))
	}
	if len(result.CorruptedShards) != 0 {
		t.Errorf("expected 0 corrupted, got %d", len(result.CorruptedShards))
	}
}

func TestLocalScanner_Missing(t *testing.T) {
	storeDir, qDir, fileID, _ := setupTestEnvironment(t)

	// Delete shard 2
	os.Remove(filepath.Join(storeDir, fileID, "shard.02"))

	scanner := watchdog.NewLocalScanner(storeDir, qDir)
	result, err := scanner.ScanLocalFile(fileID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.MissingShards) != 1 || result.MissingShards[0] != 2 {
		t.Errorf("expected missing shard 2, got %v", result.MissingShards)
	}
	if len(result.HealthyShards) != 4 {
		t.Errorf("expected 4 healthy, got %d", len(result.HealthyShards))
	}
}

func TestLocalScanner_Corrupted(t *testing.T) {
	storeDir, qDir, fileID, _ := setupTestEnvironment(t)

	// Corrupt shard 1
	os.WriteFile(filepath.Join(storeDir, fileID, "shard.01"), []byte("corrupted_data"), 0o644)

	scanner := watchdog.NewLocalScanner(storeDir, qDir)
	result, err := scanner.ScanLocalFile(fileID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.CorruptedShards) != 1 || result.CorruptedShards[0] != 1 {
		t.Errorf("expected corrupted shard 1, got %v", result.CorruptedShards)
	}
	if len(result.HealthyShards) != 4 {
		t.Errorf("expected 4 healthy, got %d", len(result.HealthyShards))
	}

	// Verify it was moved to quarantine
	qShardPath := filepath.Join(qDir, fileID, "shard.01")
	if _, err := os.Stat(qShardPath); os.IsNotExist(err) {
		t.Errorf("corrupted shard not moved to quarantine")
	}

	// Verify manifest was copied
	qManifestPath := filepath.Join(qDir, fileID, "manifest.json")
	if _, err := os.Stat(qManifestPath); os.IsNotExist(err) {
		t.Errorf("manifest not copied to quarantine")
	}

	// Verify shard was removed from store
	storeShardPath := filepath.Join(storeDir, fileID, "shard.01")
	if _, err := os.Stat(storeShardPath); !os.IsNotExist(err) {
		t.Errorf("corrupted shard still exists in store dir")
	}
}

func TestLocalScanner_ManifestMissing(t *testing.T) {
	storeDir, qDir, fileID, _ := setupTestEnvironment(t)

	// Delete manifest
	os.Remove(filepath.Join(storeDir, fileID, "manifest.json"))

	scanner := watchdog.NewLocalScanner(storeDir, qDir)
	result, err := scanner.ScanLocalFile(fileID)

	if err == nil {
		t.Fatalf("expected error for missing manifest")
	}
	if result != nil {
		t.Fatalf("expected nil result")
	}
}
