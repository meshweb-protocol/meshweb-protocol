package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/meshweb/meshweb-protocol/erasure"
)

const (
	manifestV1      = "meshweb-manifest/1"
	manifestV2      = "meshweb-manifest/2"
	maxShardCount   = 256 // klauspost/reedsolomon supports at most 256 total shards.
	maxBlockSize    = 64 << 20
	maxFileIDLength = 192
	sha256HexLength = sha256.Size * 2
)

var fileIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,191}$`)

// ValidateFileID verifies that an identifier is safe to use as a protocol key
// and as a single directory name below a node's configured store.
func ValidateFileID(fileID string) error {
	if !fileIDPattern.MatchString(fileID) {
		return fmt.Errorf("invalid file id")
	}
	return nil
}

type FileManifest struct {
	Version      string   `json:"version"`
	FileID       string   `json:"file_id"`
	FileName     string   `json:"file_name"`
	FileSize     int64    `json:"file_size"`
	OriginalSize int64    `json:"original_size"`
	DataShards   int      `json:"data_shards"`
	ParityShards int      `json:"parity_shards"`
	MinShards    int      `json:"min_shards"`
	BlockSize    int      `json:"block_size"`
	Sha256       string   `json:"sha256"`
	ShardPaths   []string `json:"shard_paths"`
	CreatedAt    int64    `json:"created_at,omitempty"`
	ShardHashes  []string `json:"shard_hashes,omitempty"`
}

func NewManifest(fileID, fileName string, fileSize, originalSize int64, dataShards, parityShards, blockSize int, sha256sum string, shardPaths []string, createdAt int64, shardHashes []string) *FileManifest {
	return &FileManifest{
		Version:      manifestV2,
		FileID:       fileID,
		FileName:     fileName,
		FileSize:     fileSize,
		OriginalSize: originalSize,
		DataShards:   dataShards,
		ParityShards: parityShards,
		MinShards:    dataShards,
		BlockSize:    blockSize,
		Sha256:       sha256sum,
		ShardPaths:   shardPaths,
		CreatedAt:    createdAt,
		ShardHashes:  shardHashes,
	}
}

func LoadManifest(path string) (*FileManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m FileManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *FileManifest) Save(path string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (m *FileManifest) Validate() error {
	if m.Version != manifestV1 && m.Version != manifestV2 {
		return fmt.Errorf("unsupported manifest version %q", m.Version)
	}
	if err := ValidateFileID(m.FileID); err != nil {
		return err
	}
	if m.FileName == "" {
		return fmt.Errorf("missing file name")
	}
	if m.FileSize < 0 {
		return fmt.Errorf("invalid file size")
	}
	if m.OriginalSize < 0 || m.OriginalSize != m.FileSize {
		return fmt.Errorf("invalid original size")
	}
	if m.DataShards <= 0 || m.ParityShards <= 0 || m.DataShards+m.ParityShards > maxShardCount {
		return fmt.Errorf("invalid shard counts")
	}
	if m.MinShards != m.DataShards {
		return fmt.Errorf("invalid minimum shard count")
	}
	if m.BlockSize <= 0 || m.BlockSize > maxBlockSize {
		return fmt.Errorf("invalid block size")
	}
	if len(m.Sha256) != sha256HexLength {
		return fmt.Errorf("invalid file sha256")
	}
	if _, err := hex.DecodeString(m.Sha256); err != nil {
		return fmt.Errorf("invalid file sha256: %w", err)
	}
	if len(m.ShardPaths) != m.DataShards+m.ParityShards {
		return fmt.Errorf("shard count mismatch: got %d paths, expected %d", len(m.ShardPaths), m.DataShards+m.ParityShards)
	}
	for _, shardPath := range m.ShardPaths {
		if shardPath == "" || filepath.IsAbs(shardPath) || filepath.Base(shardPath) != shardPath || shardPath == "." || shardPath == ".." {
			return fmt.Errorf("invalid shard path %q", shardPath)
		}
	}
	if m.Version == manifestV2 {
		if m.CreatedAt <= 0 {
			return fmt.Errorf("missing or invalid created at timestamp")
		}
		if len(m.ShardHashes) != m.DataShards+m.ParityShards {
			return fmt.Errorf("shard hashes count mismatch: got %d hashes, expected %d", len(m.ShardHashes), m.DataShards+m.ParityShards)
		}
		for _, shardHash := range m.ShardHashes {
			if len(shardHash) != sha256HexLength {
				return fmt.Errorf("invalid shard sha256")
			}
			if _, err := hex.DecodeString(shardHash); err != nil {
				return fmt.Errorf("invalid shard sha256: %w", err)
			}
		}
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func CreateUploadManifest(inPath, outDir string, dataShards, parityShards, blockSize int) (*FileManifest, error) {
	fileInfo, err := os.Stat(inPath)
	if err != nil {
		return nil, err
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("input path must be a file")
	}

	sha256sum, err := hashFile(inPath)
	if err != nil {
		return nil, err
	}

	shardPaths, stats, err := erasure.EncodeFile(inPath, outDir, dataShards, parityShards, blockSize)
	if err != nil {
		return nil, err
	}

	relativePaths := make([]string, len(shardPaths))
	shardHashes := make([]string, len(shardPaths))
	for i, p := range shardPaths {
		rel, err := filepath.Rel(outDir, p)
		if err != nil {
			rel = p
		}
		relativePaths[i] = rel

		h, err := hashFile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to hash shard %s: %w", p, err)
		}
		shardHashes[i] = h
	}

	// A content-addressed ID remains stable across nodes and avoids deriving a
	// disk path from an arbitrary user filename.
	fileID := "sha256-" + sha256sum
	createdAt := time.Now().Unix()

	manifest := NewManifest(
		fileID,
		filepath.Base(inPath),
		fileInfo.Size(),
		fileInfo.Size(),
		dataShards,
		parityShards,
		blockSize,
		sha256sum,
		relativePaths,
		createdAt,
		shardHashes,
	)
	_ = stats
	return manifest, nil
}

func ResolveShardPaths(manifest *FileManifest, manifestDir string) []string {
	resolved := make([]string, len(manifest.ShardPaths))
	for i, p := range manifest.ShardPaths {
		if filepath.IsAbs(p) {
			resolved[i] = p
			continue
		}
		resolved[i] = filepath.Join(manifestDir, p)
	}
	return resolved
}

func ReconstructFromManifest(manifest *FileManifest, manifestPath, outPath string, presentIndices []int) (erasure.Stats, error) {
	if err := manifest.Validate(); err != nil {
		return erasure.Stats{}, err
	}
	if len(presentIndices) < manifest.MinShards {
		return erasure.Stats{}, fmt.Errorf("not enough shards provided: got %d, need %d", len(presentIndices), manifest.MinShards)
	}

	manifestDir := filepath.Dir(manifestPath)
	resolvedPaths := ResolveShardPaths(manifest, manifestDir)

	present := make(map[int]string)
	for _, idx := range presentIndices {
		if idx < 0 || idx >= len(resolvedPaths) {
			return erasure.Stats{}, fmt.Errorf("shard index %d out of range", idx)
		}
		present[idx] = resolvedPaths[idx]
	}

	stats, err := erasure.ReconstructFile(outPath, present, manifest.DataShards, manifest.ParityShards, manifest.BlockSize, manifest.OriginalSize)
	if err != nil {
		return stats, err
	}

	reconstructedSha, err := hashFile(outPath)
	if err != nil {
		return stats, err
	}
	if reconstructedSha != manifest.Sha256 {
		return stats, fmt.Errorf("reconstructed sha mismatch: %s != %s", reconstructedSha, manifest.Sha256)
	}

	return stats, nil
}
