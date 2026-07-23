package node_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/manifest"
	"github.com/meshweb/meshweb-protocol/node"
)

func TestFileIDIsolation(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()

	n1, _ := node.NewNode(context.Background(), node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, StoreDir: dir1})
	n2, _ := node.NewNode(context.Background(), node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, StoreDir: dir2})

	defer n1.Stop()
	defer n2.Stop()
	n1.Start()
	n2.Start()

	target := peer.AddrInfo{ID: n2.Host.ID(), Addrs: n2.Host.Addrs()}

	// Push shard 0 for file A from n1 to n2
	err := n1.PushShardToPeer(context.Background(), target, "fileA", 0, int64(len("dataA")), bytes.NewReader([]byte("dataA")))
	if err != nil {
		t.Fatal(err)
	}

	// Push shard 0 for file B from n1 to n2
	err = n1.PushShardToPeer(context.Background(), target, "fileB", 0, int64(len("dataB")), bytes.NewReader([]byte("dataB")))
	if err != nil {
		t.Fatal(err)
	}

	// Verify both exist on disk in isolated directories in dir2 (n2's store)
	pathA := filepath.Join(dir2, "fileA", "shard.00")
	pathB := filepath.Join(dir2, "fileB", "shard.00")

	dA, err := os.ReadFile(pathA)
	if err != nil || string(dA) != "dataA" {
		t.Fatal("fileA isolation failed")
	}

	dB, err := os.ReadFile(pathB)
	if err != nil || string(dB) != "dataB" {
		t.Fatal("fileB isolation failed")
	}
}

func TestRetryHelper(t *testing.T) {
	attempts := 0
	err := node.WithRetry(context.Background(), 3, 10*time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return context.DeadlineExceeded // simulate transient failure
		}
		return nil
	})

	if err != nil {
		t.Fatal("expected success after retries, got err:", err)
	}
	if attempts != 3 {
		t.Fatal("expected 3 attempts, got", attempts)
	}
}

func TestRestartRecoveryScan(t *testing.T) {
	tempDir := t.TempDir()

	// Seed some files manually to simulate a previous session
	os.MkdirAll(filepath.Join(tempDir, "file123"), 0o755)
	os.WriteFile(filepath.Join(tempDir, "file123", "shard.00"), []byte("data"), 0o644)

	// Malformed entries that should be skipped without panic
	os.WriteFile(filepath.Join(tempDir, "notadir.txt"), []byte("bad"), 0o644) // file instead of dir
	os.MkdirAll(filepath.Join(tempDir, "emptyDir"), 0o755)                    // empty dir

	cfg := node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, StoreDir: tempDir}
	n, err := node.NewNode(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer n.Stop()

	// Calling Start() will trigger go n.ScanAndAnnounce()
	err = n.Start()
	if err != nil {
		t.Fatal(err)
	}

	// Give it a moment to run the goroutine
	time.Sleep(100 * time.Millisecond)

	// If it didn't panic and reached here, the skip logic for malformed entries works.
}

func TestManifestPush(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()

	n1, _ := node.NewNode(context.Background(), node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, StoreDir: dir1})
	n2, _ := node.NewNode(context.Background(), node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, StoreDir: dir2})

	defer n1.Stop()
	defer n2.Stop()
	n1.Start()
	n2.Start()

	target := peer.AddrInfo{ID: n2.Host.ID(), Addrs: n2.Host.Addrs()}

	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	m := manifest.NewManifest("file123", "file.bin", 1, 1, 1, 1, 1, hash, []string{"shard.00", "shard.01"}, time.Now().Unix(), []string{hash, hash})
	manifestBytes, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	err = n1.PushManifestToPeer(context.Background(), target, "file123", manifestBytes)
	if err != nil {
		t.Fatalf("PushManifestToPeer failed: %v", err)
	}

	// Verify manifest.json exists on n2's disk
	path := filepath.Join(dir2, "file123", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read pushed manifest from disk: %v", err)
	}
	if string(data) != string(manifestBytes) {
		t.Fatalf("Pushed manifest content mismatch: got %s, expected %s", string(data), string(manifestBytes))
	}
}

func TestHealthProtocol(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()

	n1, _ := node.NewNode(context.Background(), node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, StoreDir: dir1})
	n2, _ := node.NewNode(context.Background(), node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, StoreDir: dir2})

	defer n1.Stop()
	defer n2.Stop()
	n1.Start()
	n2.Start()

	target := peer.AddrInfo{ID: n2.Host.ID(), Addrs: n2.Host.Addrs()}

	// Create a valid manifest and shards in n2's store. Health responses must
	// describe verified shards, not merely files that happen to exist.
	fileID := "file123"
	fileDir := filepath.Join(dir2, fileID)
	os.MkdirAll(fileDir, 0o755)
	shardData := []byte("data")
	digest := sha256.Sum256(shardData)
	hash := hex.EncodeToString(digest[:])
	paths := make([]string, 6)
	hashes := make([]string, 6)
	for i := range paths {
		paths[i] = fmt.Sprintf("shard.%02d", i)
		hashes[i] = hash
	}
	m := manifest.NewManifest(fileID, "file.bin", int64(len(shardData)), int64(len(shardData)), 3, 3, 1, hash, paths, time.Now().Unix(), hashes)
	manifestBytes, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fileDir, "manifest.json"), manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(fileDir, "shard.02"), shardData, 0o644)
	os.WriteFile(filepath.Join(fileDir, "shard.05"), shardData, 0o644)

	resp, err := n1.QueryHealth(context.Background(), target, fileID)
	if err != nil {
		t.Fatalf("QueryHealth failed: %v", err)
	}

	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
	if resp.FileID != fileID {
		t.Errorf("expected file_id %s, got %s", fileID, resp.FileID)
	}

	// We expect bits 2 and 5 to be set.
	// 2 -> 1<<2 = 4
	// 5 -> 1<<5 = 32
	// 4 + 32 = 36 = 0x24
	if len(resp.Bitmap) != 1 || resp.Bitmap[0] != 0x24 {
		t.Fatalf("expected bitmap [0x24], got %x", resp.Bitmap)
	}
}

func FuzzHealthBitmap(f *testing.F) {
	f.Add([]byte("eyJ2ZXJzaW9uIjoxLCJmaWxlX2lkIjoidGVzdCIsImJpdG1hcCI6IkpBPT0ifQ==\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var raw struct {
			Version int    `json:"version"`
			FileID  string `json:"file_id"`
			Bitmap  string `json:"bitmap"`
			Error   string `json:"error,omitempty"`
		}
		_ = json.Unmarshal(data, &raw)
	})
}
