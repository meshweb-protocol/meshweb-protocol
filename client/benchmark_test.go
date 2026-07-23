package client_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/client"
	"github.com/meshweb/meshweb-protocol/node"
)

func BenchmarkPipeline1MB(b *testing.B) {
	runPipelineBenchmark(b, 1*1024*1024)
}

func BenchmarkPipeline10MB(b *testing.B) {
	runPipelineBenchmark(b, 10*1024*1024)
}

func runPipelineBenchmark(b *testing.B, payloadSize int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const numNodes = 3
	var storageNodes []*node.Node
	var targetPeers []peer.AddrInfo

	for i := 0; i < numNodes; i++ {
		nodeStore := b.TempDir()
		nodeCfg := node.Config{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
			StoreDir:    nodeStore,
		}
		sNode, err := node.NewNode(ctx, nodeCfg)
		if err != nil {
			b.Fatalf("failed to create node %d: %v", i, err)
		}
		if err := sNode.Start(); err != nil {
			b.Fatalf("failed to start node %d: %v", i, err)
		}
		defer sNode.Stop()

		storageNodes = append(storageNodes, sNode)
		targetPeers = append(targetPeers, peer.AddrInfo{
			ID:    sNode.Host.ID(),
			Addrs: sNode.Host.Addrs(),
		})
	}

	clientStore := b.TempDir()
	clientCfg := client.Config{
		BootPeers:   targetPeers,
		StoreDir:    clientStore,
		Concurrency: 4,
	}

	c, err := client.NewClient(ctx, clientCfg)
	if err != nil {
		b.Fatalf("failed to create client: %v", err)
	}
	defer c.Close()

	payload := make([]byte, payloadSize)
	rand.Read(payload)

	hasher := sha256.New()
	hasher.Write(payload)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		inFile := filepath.Join(b.TempDir(), "bench_in.bin")
		os.WriteFile(inFile, payload, 0o644)

		t0 := time.Now()
		fId, err := c.UploadFile(ctx, inFile)
		if err != nil {
			b.Fatalf("upload failed: %v", err)
		}
		uploadDuration := time.Since(t0)

		t1 := time.Now()
		outFile := filepath.Join(b.TempDir(), "bench_out.bin")
		if err := c.DownloadFile(ctx, fId, outFile); err != nil {
			b.Fatalf("download failed: %v", err)
		}
		downloadDuration := time.Since(t1)

		outData, _ := os.ReadFile(outFile)
		h := sha256.New()
		h.Write(outData)
		if hex.EncodeToString(h.Sum(nil)) != expectedHash {
			b.Fatalf("benchmark SHA256 mismatch!")
		}

		b.Logf("[BENCHMARK] Payload: %d Bytes | Upload: %v | Download: %v", payloadSize, uploadDuration, downloadDuration)
	}
}
