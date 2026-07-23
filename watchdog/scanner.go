package watchdog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/meshweb/meshweb-protocol/manifest"
)

type ScanResult struct {
	FileID          string
	HealthyShards   []int
	CorruptedShards []int
	MissingShards   []int
}

type IntegrityScanner interface {
	ScanLocalFile(fileID string) (*ScanResult, error)
}

type LocalScanner struct {
	storeDir      string
	quarantineDir string
}

func NewLocalScanner(storeDir, quarantineDir string) *LocalScanner {
	return &LocalScanner{
		storeDir:      storeDir,
		quarantineDir: quarantineDir,
	}
}

func (s *LocalScanner) ScanLocalFile(fileID string) (*ScanResult, error) {
	fileDir := filepath.Join(s.storeDir, fileID)
	manifestPath := filepath.Join(fileDir, "manifest.json")

	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest for %s: %w", fileID, err)
	}

	result := &ScanResult{
		FileID:          fileID,
		HealthyShards:   make([]int, 0),
		CorruptedShards: make([]int, 0),
		MissingShards:   make([]int, 0),
	}

	totalShards := m.DataShards + m.ParityShards
	for i := 0; i < totalShards; i++ {
		shardName := fmt.Sprintf("shard.%02d", i)
		shardPath := filepath.Join(fileDir, shardName)

		if _, err := os.Stat(shardPath); os.IsNotExist(err) {
			result.MissingShards = append(result.MissingShards, i)
			continue
		}

		hash, err := s.hashFile(shardPath)
		if err != nil {
			// If we can't read it, treat it as corrupted
			result.CorruptedShards = append(result.CorruptedShards, i)
			s.quarantine(fileID, shardName, shardPath, manifestPath)
			continue
		}

		if i < len(m.ShardHashes) && hash == m.ShardHashes[i] {
			result.HealthyShards = append(result.HealthyShards, i)
		} else {
			result.CorruptedShards = append(result.CorruptedShards, i)
			s.quarantine(fileID, shardName, shardPath, manifestPath)
		}
	}

	// Logging
	log.Printf("WATCHDOG\nScanning file: %s\nHealthy: %d\nMissing: %d\nCorrupted: %d\n",
		fileID, len(result.HealthyShards), len(result.MissingShards), len(result.CorruptedShards))

	if len(result.CorruptedShards) > 0 {
		log.Printf("Quarantined:\n")
		for _, idx := range result.CorruptedShards {
			log.Printf("Shard %d\n", idx)
		}
	}

	return result, nil
}

func (s *LocalScanner) hashFile(path string) (string, error) {
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

func (s *LocalScanner) quarantine(fileID, shardName, shardPath, manifestPath string) {
	qDir := filepath.Join(s.quarantineDir, fileID)
	if err := os.MkdirAll(qDir, 0o755); err != nil {
		log.Printf("failed to create quarantine dir: %v", err)
		return
	}

	qShardPath := filepath.Join(qDir, shardName)
	if err := os.Rename(shardPath, qShardPath); err != nil {
		log.Printf("failed to move shard %s to quarantine: %v", shardPath, err)
	}

	qManifestPath := filepath.Join(qDir, "manifest.json")
	if _, err := os.Stat(qManifestPath); os.IsNotExist(err) {
		s.copyFile(manifestPath, qManifestPath)
	}
}

func (s *LocalScanner) copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer out.Close()
	io.Copy(out, in)
}
