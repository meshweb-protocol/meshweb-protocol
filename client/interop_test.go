package client_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/meshweb/meshweb-protocol/manifest"
)

// TestINTEROP001_CleanRoomCrossLanguage validates Go <-> Python cross-language exchange
func TestINTEROP001_CleanRoomCrossLanguage(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	if err != nil {
		t.Skip("Python runtime not available for interop test")
	}

	tmpDir, err := os.MkdirTemp("", "meshweb-interop-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	payloadText := "MeshWeb Cross-Language Clean-Room Interoperability Payload 2026!"
	payloadBytes := []byte(payloadText)
	hasher := sha256.New()
	hasher.Write(payloadBytes)
	expectedSHA := hex.EncodeToString(hasher.Sum(nil))
	fileID := fmt.Sprintf("sha256-%s", expectedSHA)

	goOutputDir := filepath.Join(tmpDir, "go_output")
	if err := os.MkdirAll(goOutputDir, 0o755); err != nil {
		t.Fatalf("Failed to create go_output dir: %v", err)
	}

	// 1. Go Writes Manifest and Shards
	shardsDir := goOutputDir
	mid := len(payloadBytes) / 2
	shard0 := payloadBytes[:mid]
	shard1 := payloadBytes[mid:]

	m := manifest.FileManifest{
		Version:      "meshweb-manifest/2",
		FileID:       fileID,
		FileName:     "go_interop.txt",
		FileSize:     int64(len(payloadBytes)),
		DataShards:   2,
		ParityShards: 0,
		MinShards:    2,
		BlockSize:    1048576,
		Sha256:       expectedSHA,
		ShardPaths:   []string{"shard_0.bin", "shard_1.bin"},
	}

	mBytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goOutputDir, "manifest.json"), mBytes, 0o644); err != nil {
		t.Fatalf("Failed to write manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shardsDir, "shard_0.bin"), shard0, 0o644); err != nil {
		t.Fatalf("Failed to write shard_0.bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shardsDir, "shard_1.bin"), shard1, 0o644); err != nil {
		t.Fatalf("Failed to write shard_1.bin: %v", err)
	}

	// 2. Python Reads and Verifies Go Output
	scriptPath := filepath.Join("..", "..", "meshweb-sdk-python", "interop_harness.py")
	cmdRead := exec.Command(pythonPath, scriptPath, "read", goOutputDir, expectedSHA)
	output, err := cmdRead.CombinedOutput()
	if err != nil {
		t.Fatalf("Python failed to read Go manifest: %v | Output: %s", err, string(output))
	}
	t.Logf("[INTEROP DIRECTION 1 PASS] Go Upload -> Python Download: %s", string(output))

	// 3. Python Writes Manifest and Shards
	pyOutputDir := filepath.Join(tmpDir, "py_output")
	pyPayload := "Python Generated MeshWeb Payload for Go Verification!"
	pyHasher := sha256.New()
	pyHasher.Write([]byte(pyPayload))
	pyExpectedSHA := hex.EncodeToString(pyHasher.Sum(nil))
	pyFileID := fmt.Sprintf("sha256-%s", pyExpectedSHA)

	cmdWrite := exec.Command(pythonPath, scriptPath, "write", pyOutputDir, pyFileID, pyPayload)
	outputW, err := cmdWrite.CombinedOutput()
	if err != nil {
		t.Fatalf("Python failed to write manifest: %v | Output: %s", err, string(outputW))
	}

	// 4. Go Reads and Verifies Python Output
	pyManifestData, err := os.ReadFile(filepath.Join(pyOutputDir, "manifest.json"))
	if err != nil {
		t.Fatalf("Go failed to read Python manifest: %v", err)
	}
	var pyManifest manifest.FileManifest
	if err := json.Unmarshal(pyManifestData, &pyManifest); err != nil {
		t.Fatalf("Go failed to parse Python manifest JSON: %v", err)
	}

	s0Bytes, err := os.ReadFile(filepath.Join(pyOutputDir, pyManifest.ShardPaths[0]))
	if err != nil {
		t.Fatalf("Go failed to read Python shard 0: %v", err)
	}
	s1Bytes, err := os.ReadFile(filepath.Join(pyOutputDir, pyManifest.ShardPaths[1]))
	if err != nil {
		t.Fatalf("Go failed to read Python shard 1: %v", err)
	}

	reconstructedBytes := append(s0Bytes, s1Bytes...)
	reconstructedHasher := sha256.New()
	reconstructedHasher.Write(reconstructedBytes)
	reconstructedSHA := hex.EncodeToString(reconstructedHasher.Sum(nil))

	if reconstructedSHA != pyExpectedSHA {
		t.Fatalf("Go SHA256 mismatch for Python payload: expected %s, got %s", pyExpectedSHA, reconstructedSHA)
	}

	t.Logf("[INTEROP DIRECTION 2 PASS] Python Upload -> Go Download (SHA256 Match: %s)", reconstructedSHA)
	t.Logf("[INTEROP-001 PASS] 100%% Clean-Room Cross-Language Bi-Directional Exchange Verified!")
}
