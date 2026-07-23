package manifest

import (
	crand "crypto/rand"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func genRandomFile(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bufSize := int64(1 << 20)
	buf := make([]byte, bufSize)
	var written int64
	for written < size {
		toWrite := bufSize
		if remaining := size - written; remaining < toWrite {
			toWrite = remaining
		}
		if _, err := crand.Read(buf[:toWrite]); err != nil {
			return err
		}
		if _, err := f.Write(buf[:toWrite]); err != nil {
			return err
		}
		written += toWrite
	}
	return nil
}

func TestManifestPipelineEndToEnd(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "testfile.bin")
	size := int64(100 << 20)
	if err := genRandomFile(inPath, size); err != nil {
		t.Fatalf("failed to generate file: %v", err)
	}

	outDir := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	manifest, err := CreateUploadManifest(inPath, outDir, 10, 20, 1<<20)
	if err != nil {
		t.Fatalf("CreateUploadManifest failed: %v", err)
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		t.Fatalf("saving manifest failed: %v", err)
	}

	loaded, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest failed: %v", err)
	}
	if loaded.FileID != manifest.FileID {
		t.Fatalf("loaded manifest file id mismatch")
	}

	totalShards := loaded.DataShards + loaded.ParityShards
	indices := rand.Perm(totalShards)[:loaded.MinShards]

	outPath := filepath.Join(tmp, "reconstructed.bin")
	stats, err := ReconstructFromManifest(loaded, manifestPath, outPath, indices)
	if err != nil {
		t.Fatalf("reconstruct from manifest failed: %v", err)
	}

	if fi, err := os.Stat(outPath); err != nil {
		t.Fatalf("reconstructed file stat failed: %v", err)
	} else if fi.Size() != size {
		t.Fatalf("reconstructed file size mismatch: got %d expected %d", fi.Size(), size)
	}

	t.Logf("E2E manifest pipeline success: decode duration=%v maxAlloc=%d", stats.DecodeDuration, stats.MaxAllocBytes)
}

func TestManifestBackwardCompatibility(t *testing.T) {
	tmp := t.TempDir()
	fileHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shardHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	v1JSON := `{
		"version": "meshweb-manifest/1",
		"file_id": "test_v1",
		"file_name": "test.bin",
		"file_size": 100,
		"original_size": 100,
		"data_shards": 2,
		"parity_shards": 2,
		"min_shards": 2,
		"block_size": 1024,
		"sha256": "` + fileHash + `",
		"shard_paths": ["s0", "s1", "s2", "s3"]
	}`

	v2JSON := `{
		"version": "meshweb-manifest/2",
		"file_id": "test_v2",
		"file_name": "test2.bin",
		"file_size": 100,
		"original_size": 100,
		"data_shards": 2,
		"parity_shards": 2,
		"min_shards": 2,
		"block_size": 1024,
		"sha256": "` + fileHash + `",
		"shard_paths": ["s0", "s1", "s2", "s3"],
		"created_at": 1700000000,
		"shard_hashes": ["` + shardHash + `", "` + shardHash + `", "` + shardHash + `", "` + shardHash + `"]
	}`

	v1Path := filepath.Join(tmp, "v1.json")
	os.WriteFile(v1Path, []byte(v1JSON), 0o644)

	v2Path := filepath.Join(tmp, "v2.json")
	os.WriteFile(v2Path, []byte(v2JSON), 0o644)

	loaded1, err := LoadManifest(v1Path)
	if err != nil {
		t.Fatalf("Failed to load V1 manifest: %v", err)
	}
	if loaded1.Version != "meshweb-manifest/1" {
		t.Errorf("Expected V1 version, got %s", loaded1.Version)
	}

	loaded2, err := LoadManifest(v2Path)
	if err != nil {
		t.Fatalf("Failed to load V2 manifest: %v", err)
	}
	if loaded2.Version != "meshweb-manifest/2" {
		t.Errorf("Expected V2 version, got %s", loaded2.Version)
	}
	if loaded2.CreatedAt != 1700000000 {
		t.Errorf("Expected CreatedAt 1700000000, got %d", loaded2.CreatedAt)
	}
}

func TestManifestVersionFail(t *testing.T) {
	tmp := t.TempDir()

	v99JSON := `{
		"version": "meshweb-manifest/99",
		"file_id": "test_v99",
		"file_name": "test.bin",
		"file_size": 100,
		"original_size": 100,
		"data_shards": 2,
		"parity_shards": 2,
		"min_shards": 2,
		"block_size": 1024,
		"sha256": "dummy",
		"shard_paths": ["s0", "s1", "s2", "s3"]
	}`

	v99Path := filepath.Join(tmp, "v99.json")
	os.WriteFile(v99Path, []byte(v99JSON), 0o644)

	_, err := LoadManifest(v99Path)
	if err == nil {
		t.Fatalf("Expected failure when loading V99 manifest, but it succeeded")
	}
}

func TestManifestRejectsUnsafePathsAndHashes(t *testing.T) {
	validHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	base := FileManifest{
		Version: manifestV2, FileID: "safe-file-id", FileName: "file.bin",
		FileSize: 1, OriginalSize: 1, DataShards: 1, ParityShards: 1,
		MinShards: 1, BlockSize: 1, Sha256: validHash, CreatedAt: 1,
		ShardPaths: []string{"shard.00", "shard.01"}, ShardHashes: []string{validHash, validHash},
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*FileManifest)
	}{
		{"file id traversal", func(m *FileManifest) { m.FileID = "../escape" }},
		{"absolute shard path", func(m *FileManifest) { m.ShardPaths[0] = filepath.Join(t.TempDir(), "shard.00") }},
		{"relative shard traversal", func(m *FileManifest) { m.ShardPaths[0] = "../shard.00" }},
		{"invalid file hash", func(m *FileManifest) { m.Sha256 = "not-a-hash" }},
		{"invalid shard hash", func(m *FileManifest) { m.ShardHashes[0] = "not-a-hash" }},
		{"invalid quorum", func(m *FileManifest) { m.MinShards = 2 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			m.ShardPaths = append([]string(nil), base.ShardPaths...)
			m.ShardHashes = append([]string(nil), base.ShardHashes...)
			tc.mutate(&m)
			if err := m.Validate(); err == nil {
				t.Fatal("unsafe manifest was accepted")
			}
		})
	}
}

func TestShardHashesMismatch(t *testing.T) {
	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "testfile.bin")
	if err := genRandomFile(inPath, 1024); err != nil {
		t.Fatalf("failed to generate file: %v", err)
	}

	outDir := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	manifest, err := CreateUploadManifest(inPath, outDir, 2, 2, 1024)
	if err != nil {
		t.Fatalf("CreateUploadManifest failed: %v", err)
	}

	// Corrupt shard 0
	shard0Path := filepath.Join(outDir, manifest.ShardPaths[0])
	os.WriteFile(shard0Path, []byte("corrupted_data"), 0o644)

	// Re-hash and check
	h, err := hashFile(shard0Path)
	if err != nil {
		t.Fatalf("failed to hash shard: %v", err)
	}

	if h == manifest.ShardHashes[0] {
		t.Fatalf("Hash matches even after corruption!")
	}
}

func FuzzManifest(f *testing.F) {
	f.Add([]byte(`{"version":"meshweb-manifest/1","file_id":"test","file_name":"test.bin","file_size":100,"original_size":100,"data_shards":2,"parity_shards":2,"min_shards":2,"block_size":1024,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","shard_paths":["s0","s1","s2","s3"]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var m FileManifest
		if err := json.Unmarshal(data, &m); err == nil {
			_ = m.Validate()
		}
	})
}
