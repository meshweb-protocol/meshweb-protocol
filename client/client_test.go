package client_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/client"
	"github.com/meshweb/meshweb-protocol/node"
)

func TestSprint1VerticalSlice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeStore := t.TempDir()
	nodeCfg := node.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		StoreDir:    nodeStore,
	}

	storageNode, err := node.NewNode(ctx, nodeCfg)
	if err != nil {
		t.Fatalf("failed to create storage node: %v", err)
	}
	if err := storageNode.Start(); err != nil {
		t.Fatalf("failed to start storage node: %v", err)
	}
	defer storageNode.Stop()

	targetPeer := peer.AddrInfo{
		ID:    storageNode.Host.ID(),
		Addrs: storageNode.Host.Addrs(),
	}

	clientStore := t.TempDir()
	clientCfg := client.Config{
		BootPeers:   []peer.AddrInfo{targetPeer},
		StoreDir:    clientStore,
		Concurrency: 2,
	}

	c, err := client.NewClient(ctx, clientCfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer c.Close()

	// 1. Generate 1MB random input file
	inputFile := filepath.Join(t.TempDir(), "hello.bin")
	payload := make([]byte, 1024*1024)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("failed to generate random data: %v", err)
	}
	if err := os.WriteFile(inputFile, payload, 0o644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	hasher := sha256.New()
	hasher.Write(payload)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	// 2. Upload
	fileID, err := c.UploadFile(ctx, inputFile)
	if err != nil {
		t.Fatalf("client.UploadFile failed: %v", err)
	}
	if fileID == "" {
		t.Fatalf("expected non-empty fileID from UploadFile")
	}

	// 3. Download
	outputFile := filepath.Join(t.TempDir(), "out.bin")
	if err := c.DownloadFile(ctx, fileID, outputFile); err != nil {
		t.Fatalf("client.DownloadFile failed: %v", err)
	}

	// 4. Verify SHA256 Match
	downloadedData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	outHasher := sha256.New()
	outHasher.Write(downloadedData)
	actualHash := hex.EncodeToString(outHasher.Sum(nil))

	if actualHash != expectedHash {
		t.Fatalf("Vertical slice SHA256 mismatch! Expected %s, got %s", expectedHash, actualHash)
	}

	t.Logf("[SPRINT 1 PASS] Single-Node Vertical Slice (FileID: %s, Bytes: %d)", fileID, len(downloadedData))
}

func TestMultiNodeDistributedSlice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const numNodes = 3
	var storageNodes []*node.Node
	var targetPeers []peer.AddrInfo

	for i := 0; i < numNodes; i++ {
		nodeStore := t.TempDir()
		nodeCfg := node.Config{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
			StoreDir:    nodeStore,
		}
		sNode, err := node.NewNode(ctx, nodeCfg)
		if err != nil {
			t.Fatalf("failed to create node %d: %v", i, err)
		}
		if err := sNode.Start(); err != nil {
			t.Fatalf("failed to start node %d: %v", i, err)
		}
		defer sNode.Stop()

		storageNodes = append(storageNodes, sNode)
		targetPeers = append(targetPeers, peer.AddrInfo{
			ID:    sNode.Host.ID(),
			Addrs: sNode.Host.Addrs(),
		})

		t.Logf("[NODE %d VERIFIED] PeerID: %s, StoreDir: %s, Addr: %v", i+1, sNode.Host.ID(), nodeStore, sNode.Host.Addrs())
	}

	clientStore := t.TempDir()
	clientCfg := client.Config{
		BootPeers:   targetPeers,
		StoreDir:    clientStore,
		Concurrency: 3,
	}

	c, err := client.NewClient(ctx, clientCfg)
	if err != nil {
		t.Fatalf("failed to create multi-node client: %v", err)
	}
	defer c.Close()

	inputFile := filepath.Join(t.TempDir(), "multi_node_test.bin")
	payload := make([]byte, 512*1024)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("failed to generate payload: %v", err)
	}
	os.WriteFile(inputFile, payload, 0o644)

	hasher := sha256.New()
	hasher.Write(payload)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	fileID, err := c.UploadFile(ctx, inputFile)
	if err != nil {
		t.Fatalf("multi-node UploadFile failed: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), "multi_node_out.bin")
	if err := c.DownloadFile(ctx, fileID, outputFile); err != nil {
		t.Fatalf("multi-node DownloadFile failed: %v", err)
	}

	downloadedData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read downloaded data: %v", err)
	}

	outHasher := sha256.New()
	outHasher.Write(downloadedData)
	actualHash := hex.EncodeToString(outHasher.Sum(nil))

	if actualHash != expectedHash {
		t.Fatalf("Multi-node SHA256 mismatch! Expected %s, got %s", expectedHash, actualHash)
	}

	t.Logf("[DISTRIBUTED PASS] 3-Node Cluster Distributed Upload & Retrieval (FileID: %s, Bytes: %d)", fileID, len(downloadedData))
}

func TestNodeFailureRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var storageNodes []*node.Node
	var targetPeers []peer.AddrInfo

	for i := 0; i < 3; i++ {
		nodeStore := t.TempDir()
		nodeCfg := node.Config{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
			StoreDir:    nodeStore,
		}
		sNode, err := node.NewNode(ctx, nodeCfg)
		if err != nil {
			t.Fatalf("failed to create node %d: %v", i, err)
		}
		if err := sNode.Start(); err != nil {
			t.Fatalf("failed to start node %d: %v", i, err)
		}

		storageNodes = append(storageNodes, sNode)
		targetPeers = append(targetPeers, peer.AddrInfo{
			ID:    sNode.Host.ID(),
			Addrs: sNode.Host.Addrs(),
		})
	}

	clientStore := t.TempDir()
	clientCfg := client.Config{
		BootPeers:   targetPeers,
		StoreDir:    clientStore,
		Concurrency: 3,
	}

	c, err := client.NewClient(ctx, clientCfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	inputFile := filepath.Join(t.TempDir(), "fault_test.bin")
	payload := make([]byte, 256*1024)
	rand.Read(payload)
	os.WriteFile(inputFile, payload, 0o644)

	hasher := sha256.New()
	hasher.Write(payload)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	fileID, err := c.UploadFile(ctx, inputFile)
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	t.Logf("[FAULT INJECTION] Stopping Node 2 (PeerID: %s)...", storageNodes[1].Host.ID())
	storageNodes[1].Stop()

	defer storageNodes[0].Stop()
	defer storageNodes[2].Stop()
	defer c.Close()

	outputFile := filepath.Join(t.TempDir(), "fault_recovered.bin")
	if err := c.DownloadFile(ctx, fileID, outputFile); err != nil {
		t.Fatalf("DownloadFile failed after Node 2 was killed: %v", err)
	}

	downloadedData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read recovered output: %v", err)
	}

	outHasher := sha256.New()
	outHasher.Write(downloadedData)
	actualHash := hex.EncodeToString(outHasher.Sum(nil))

	if actualHash != expectedHash {
		t.Fatalf("Fault recovery SHA256 mismatch! Expected %s, got %s", expectedHash, actualHash)
	}

	t.Logf("[FAULT RECOVERY PASS] Successfully retrieved & reconstructed file after Node 2 crash! (SHA256 Match: %s)", actualHash)
}

func TestConcurrentUploadDownloadRaceAndLeak(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const numNodes = 3
	var storageNodes []*node.Node
	var targetPeers []peer.AddrInfo

	for i := 0; i < numNodes; i++ {
		nodeStore := t.TempDir()
		nodeCfg := node.Config{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
			StoreDir:    nodeStore,
		}
		sNode, err := node.NewNode(ctx, nodeCfg)
		if err != nil {
			t.Fatalf("failed to create node %d: %v", i, err)
		}
		if err := sNode.Start(); err != nil {
			t.Fatalf("failed to start node %d: %v", i, err)
		}

		storageNodes = append(storageNodes, sNode)
		targetPeers = append(targetPeers, peer.AddrInfo{
			ID:    sNode.Host.ID(),
			Addrs: sNode.Host.Addrs(),
		})
	}

	clientStore := t.TempDir()
	clientCfg := client.Config{
		BootPeers:   targetPeers,
		StoreDir:    clientStore,
		Concurrency: 4,
	}

	c, err := client.NewClient(ctx, clientCfg)
	if err != nil {
		t.Fatalf("failed to create multi-node client: %v", err)
	}

	const concurrencyLevel = 5
	var wg sync.WaitGroup

	for worker := 0; worker < concurrencyLevel; worker++ {
		wg.Add(1)
		go func(wId int) {
			defer wg.Done()

			inFile := filepath.Join(t.TempDir(), fmt.Sprintf("concurrent_in_%d.bin", wId))
			data := make([]byte, 128*1024)
			rand.Read(data)
			os.WriteFile(inFile, data, 0o644)

			h := sha256.New()
			h.Write(data)
			exp := hex.EncodeToString(h.Sum(nil))

			fId, uErr := c.UploadFile(ctx, inFile)
			if uErr != nil {
				t.Errorf("worker %d upload error: %v", wId, uErr)
				return
			}

			outFile := filepath.Join(t.TempDir(), fmt.Sprintf("concurrent_out_%d.bin", wId))
			if dErr := c.DownloadFile(ctx, fId, outFile); dErr != nil {
				t.Errorf("worker %d download error: %v", wId, dErr)
				return
			}

			outData, _ := os.ReadFile(outFile)
			h2 := sha256.New()
			h2.Write(outData)
			got := hex.EncodeToString(h2.Sum(nil))

			if got != exp {
				t.Errorf("worker %d SHA256 mismatch! exp %s got %s", wId, exp, got)
			}
		}(worker)
	}

	wg.Wait()

	// Explicitly stop all nodes and client to clean up background goroutines
	c.Close()
	for _, s := range storageNodes {
		s.Stop()
	}

	// Give libp2p background routines 300ms to exit cleanly
	time.Sleep(300 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	delta := finalGoroutines - initialGoroutines

	t.Logf("[CONCURRENCY PASS] 5 Parallel Workers Completed Successfully. Initial Goroutines: %d, Final: %d (Delta: %d)", initialGoroutines, finalGoroutines, delta)

	if delta > 5 {
		t.Fatalf("Goroutine leak detected! Initial: %d, Final: %d, Delta: %d", initialGoroutines, finalGoroutines, delta)
	}
}
