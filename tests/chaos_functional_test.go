package tests

import (
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/erasure"
	"github.com/meshweb/meshweb-protocol/manifest"
	"github.com/meshweb/meshweb-protocol/watchdog"
)

func hashBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func randomPeerID() peer.ID {
	priv, _, _ := crypto.GenerateEd25519Key(crand.Reader)
	pid, _ := peer.IDFromPrivateKey(priv)
	return pid
}

// Helper to setup a network with a file fully distributed
func setupChaosNetwork(t *testing.T, numNodes int, fileID string, origData []byte, dataShards, parityShards int) (*TestNetwork, []peer.ID) {
	net := NewTestNetwork()
	var peers []peer.ID

	for i := 0; i < numNodes; i++ {
		id := randomPeerID()
		net.AddNode(t, string(id))
		peers = append(peers, id)
	}

	// Create Original File and Encode
	tmp := t.TempDir()
	origPath := filepath.Join(tmp, "original.bin")
	os.WriteFile(origPath, origData, 0o644)

	shardPaths, _, err := erasure.EncodeFile(origPath, tmp, dataShards, parityShards, 1024)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	shardHashes := make([]string, len(shardPaths))
	relShardPaths := make([]string, len(shardPaths))
	for i, p := range shardPaths {
		data, _ := os.ReadFile(p)
		shardHashes[i] = hashBytes(data)
		relShardPaths[i] = filepath.Base(p)
	}

	m := &manifest.FileManifest{
		Version:      "meshweb-manifest/2",
		FileID:       fileID,
		FileName:     "test.bin",
		FileSize:     int64(len(origData)),
		OriginalSize: int64(len(origData)),
		DataShards:   dataShards,
		ParityShards: parityShards,
		MinShards:    dataShards,
		BlockSize:    1024,
		Sha256:       hashBytes(origData),
		ShardHashes:  shardHashes,
		ShardPaths:   relShardPaths,
		CreatedAt:    time.Now().Unix(),
	}

	// Distribute manifest and ALL shards to ALL nodes initially
	for _, pid := range peers {
		node := net.nodes[pid]
		fileDir := filepath.Join(node.StoreDir, fileID)
		os.MkdirAll(fileDir, 0o755)

		err := m.Save(filepath.Join(fileDir, "manifest.json"))
		if err != nil {
			t.Fatalf("failed to save manifest: %v", err)
		}
		for i, p := range shardPaths {
			dest := filepath.Join(fileDir, fmt.Sprintf("shard.%02d", i))
			copyFile(p, dest)
		}
	}

	return net, peers
}

func copyFile(src, dst string) {
	in, _ := os.Open(src)
	defer in.Close()
	out, _ := os.Create(dst)
	defer out.Close()
	io.Copy(out, in)
}

func deleteRandomShards(t *testing.T, net *TestNetwork, peers []peer.ID, fileID string, count int, seed int64) {
	r := rand.New(rand.NewSource(seed))
	deleted := 0
	for deleted < count {
		pid := peers[r.Intn(len(peers))]
		node := net.nodes[pid]
		shardIdx := r.Intn(30) // Assuming max 30 shards

		path := filepath.Join(node.StoreDir, fileID, fmt.Sprintf("shard.%02d", shardIdx))
		if _, err := os.Stat(path); err == nil {
			os.Remove(path)
			deleted++
		}
	}
}

func countMissingAcrossNetwork(net *TestNetwork, peers []peer.ID, fileID string, totalShards int) int {
	var bitmaps [][]byte
	for _, pid := range peers {
		b := watchdog.NewLocalBitmapBuilder(net.nodes[pid].StoreDir)
		bm, _ := b.BuildBitmap(fileID)
		if bm != nil {
			bitmaps = append(bitmaps, bm)
		}
	}
	state := watchdog.BuildNetworkState(fileID, totalShards, bitmaps)
	return len(state.MissingShards)
}

func TestChaos_Functional(t *testing.T) {
	origData := make([]byte, 1024*1024)
	crand.Read(origData)

	fileID := "file_func"
	net, peers := setupChaosNetwork(t, 10, fileID, origData, 15, 15) // 30 total shards, can tolerate 15 missing

	scenarios := []int{5, 10, 15} // missing shards

	for _, missing := range scenarios {
		t.Run(fmt.Sprintf("%d_missing", missing), func(t *testing.T) {
			// To ensure exactly `missing` are globally missing, delete them from ALL nodes
			for i := 0; i < missing; i++ {
				for _, pid := range peers {
					os.Remove(filepath.Join(net.nodes[pid].StoreDir, fileID, fmt.Sprintf("shard.%02d", i)))
				}
			}

			// Run Pipeline on ONE node (e.g. node_0)
			node0 := net.nodes[peers[0]]
			err := node0.Pipeline.RunOnce(context.Background(), fileID)
			if err != nil {
				t.Fatalf("pipeline error: %v", err)
			}

			// Verify 0 missing
			missingAfter := countMissingAcrossNetwork(net, peers, fileID, 30)
			if missingAfter != 0 {
				t.Fatalf("expected 0 missing after repair, got %d", missingAfter)
			}
		})
	}
}

func TestChaos_CorruptedSource(t *testing.T) {
	origData := make([]byte, 1024*10)
	fileID := "file_corrupt"
	net, peers := setupChaosNetwork(t, 5, fileID, origData, 10, 5)

	// Make shard 0 globally missing
	for _, pid := range peers {
		os.Remove(filepath.Join(net.nodes[pid].StoreDir, fileID, "shard.00"))
	}

	// Corrupt shard 1 on all nodes
	for _, pid := range peers {
		path := filepath.Join(net.nodes[pid].StoreDir, fileID, "shard.01")
		os.WriteFile(path, []byte("garbage data!!!"), 0o644)
	}

	// Try repair
	err := net.nodes[peers[0]].Pipeline.RunOnce(context.Background(), fileID)
	if err != nil {
		t.Fatalf("expected repair to succeed despite corrupted source: %v", err)
	}

	missing := countMissingAcrossNetwork(net, peers, fileID, 15)
	if missing != 0 {
		t.Fatalf("expected 0 missing after repair, got %d", missing)
	}
}

func TestChaos_NetworkPartition(t *testing.T) {
	origData := make([]byte, 1024*10)
	fileID := "file_partition"
	net, peers := setupChaosNetwork(t, 10, fileID, origData, 10, 5)

	// Partition 5|5
	group1 := peers[:5]
	group2 := peers[5:]
	for _, p := range group1 {
		net.SetPartition(p, group1)
	}
	for _, p := range group2 {
		net.SetPartition(p, group2)
	}

	// Delete shard 0 from group1 entirely
	for _, pid := range group1 {
		os.Remove(filepath.Join(net.nodes[pid].StoreDir, fileID, "shard.00"))
	}
	// Delete shard 1 from group2 entirely
	for _, pid := range group2 {
		os.Remove(filepath.Join(net.nodes[pid].StoreDir, fileID, "shard.01"))
	}

	// Group1 tries to repair
	err := net.nodes[group1[0]].Pipeline.RunOnce(context.Background(), fileID)
	// It might succeed if group1 has enough shards for shard 0 (it has 14 shards available, needs 10 - yes it can!)
	// Wait, group 1 has 5 nodes. Each node has all shards EXCEPT shard 0.
	// So group 1 HAS shards 1..14. It CAN repair shard 0 locally!
	if err != nil {
		t.Fatalf("group 1 should be able to repair locally: %v", err)
	}

	// Heal partition
	net.ClearPartitions()

	// Verify consistency
	err = net.nodes[group2[0]].Pipeline.RunOnce(context.Background(), fileID)
	if err != nil {
		t.Fatalf("group 2 repair after heal failed: %v", err)
	}
}

func TestChaos_Randomized(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Logf("Randomized Chaos Seed: %d", seed)
	r := rand.New(rand.NewSource(seed))

	origData := make([]byte, 1024*10)
	fileID := "file_random"
	net, peers := setupChaosNetwork(t, 10, fileID, origData, 15, 15)

	for i := 0; i < 20; i++ {
		// Random deletions: 1 to 5 shards across the network
		deleteRandomShards(t, net, peers, fileID, 1+r.Intn(5), r.Int63())

		// Random node triggers repair
		pIdx := r.Intn(len(peers))
		node := net.nodes[peers[pIdx]]

		_ = node.Pipeline.RunOnce(context.Background(), fileID)
	}

	// Final heal
	err := net.nodes[peers[0]].Pipeline.RunOnce(context.Background(), fileID)
	if err != nil {
		t.Fatalf("final heal failed: %v", err)
	}

	missing := countMissingAcrossNetwork(net, peers, fileID, 30)
	if missing > 0 {
		t.Fatalf("randomized test failed, %d missing shards remaining", missing)
	}
}
