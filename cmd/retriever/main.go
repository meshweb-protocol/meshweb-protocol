package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/discovery"
	"github.com/meshweb/meshweb-protocol/manifest"
	"github.com/meshweb/meshweb-protocol/retrieval"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	bootstrapStr := flag.String("bootstrap", "", "bootstrap node multiaddr")
	fileIDStr := flag.String("file-id", "", "FileID to retrieve")
	manifestB64Str := flag.String("manifest", "", "base64-encoded manifest JSON")
	outputPath := flag.String("out", "retrieved.bin", "output file path")
	concurrency := flag.Int("concurrency", 3, "number of concurrent shard downloads")
	useV2 := flag.Bool("use-v2", false, "use V2 Segment-Pipelined Retrieval (Phase 3B)")
	window := flag.Int("window", 4, "segment pipeline window size (V2 only)")
	storeDirStr := flag.String("store-dir", "", "directory to store checkpoints and shards")
	flag.Parse()

	if *bootstrapStr == "" {
		log.Fatal("--bootstrap is required")
	}
	if *fileIDStr == "" {
		log.Fatal("--file-id is required")
	}
	if *manifestB64Str == "" {
		log.Fatal("--manifest is required")
	}

	// parse manifest
	decoded, err := base64.StdEncoding.DecodeString(*manifestB64Str)
	if err != nil {
		log.Fatalf("failed to decode manifest: %v", err)
	}
	var m *manifest.FileManifest
	if err := json.Unmarshal(decoded, &m); err != nil {
		log.Fatalf("failed to unmarshal manifest: %v", err)
	}
	fmt.Printf("Manifest loaded: FileID=%s DataShards=%d ParityShards=%d MinShards=%d\n",
		m.FileID, m.DataShards, m.ParityShards, m.MinShards)

	// parse bootstrap peer
	bootstrapMaddr, err := multiaddr.NewMultiaddr(*bootstrapStr)
	if err != nil {
		log.Fatalf("invalid bootstrap multiaddr: %v", err)
	}
	bootstrapInfo, err := peer.AddrInfoFromP2pAddr(bootstrapMaddr)
	if err != nil {
		log.Fatalf("invalid bootstrap peer: %v", err)
	}
	fmt.Printf("Bootstrap peer: %s\n", bootstrapInfo.ID.String()[:8])

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	actualStoreDir := *storeDirStr
	if actualStoreDir == "" {
		actualStoreDir = filepath.Join(os.TempDir(), fmt.Sprintf("meshweb-client-%d", time.Now().UnixNano()))
	}

	// create client node (DHT-backed)
	clientCfg := struct {
		ListenAddrs []string
		Bootstrap   []string
		StoreDir    string
	}{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		Bootstrap:   []string{*bootstrapStr},
		StoreDir:    actualStoreDir,
	}
	os.MkdirAll(clientCfg.StoreDir, 0o755)
	if *storeDirStr == "" {
		defer os.RemoveAll(clientCfg.StoreDir)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if *storeDirStr == "" {
			os.RemoveAll(clientCfg.StoreDir)
		}
		os.Exit(1)
	}()

	// create client DHT host
	t0 := time.Now()
	h, kad, err := discovery.NewDHTHost(ctx, clientCfg.ListenAddrs, *bootstrapInfo)
	if err != nil {
		log.Fatalf("failed to create client host: %v", err)
	}
	defer h.Close()

	// bootstrap client DHT
	if err := discovery.BootstrapDHT(ctx, h, kad, []peer.AddrInfo{*bootstrapInfo}); err != nil {
		fmt.Printf("Warning: bootstrap failed (non-fatal): %v\n", err)
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Printf("[METRIC] DHT Bootstrap Time: %v\n", time.Since(t0))

	// discover providers via DHT
	t1 := time.Now()
	fmt.Printf("Discovering providers for FileID=%s...\n", *fileIDStr)
	discovered, err := discovery.FindProviderPeers(ctx, kad, *fileIDStr, m.DataShards+m.ParityShards)
	if err != nil {
		log.Fatalf("discovery failed: %v", err)
	}
	fmt.Printf("[METRIC] Provider Lookup Time: %v\n", time.Since(t1))
	fmt.Printf("Discovered %d providers\n", len(discovered))
	if len(discovered) == 0 {
		log.Fatal("no providers found")
	}

	var shardsUsed int
	var stats interface{}

	if *useV2 {
		used, v2Stats, err := retrieval.RunV2(ctx, h, m, discovered, *window, clientCfg.StoreDir, *outputPath)
		if err != nil {
			log.Fatalf("V2 Retrieval failed: %v", err)
		}
		shardsUsed = used
		stats = v2Stats
	} else {
		// Route to V1 Engine
		used, v1Stats, err := retrieval.RunV1(ctx, h, m, discovered, *concurrency, clientCfg.StoreDir, *outputPath)
		if err != nil {
			log.Fatalf("V1 Retrieval failed: %v", err)
		}
		shardsUsed = used
		stats = v1Stats
	}

	// read original for comparison
	origSha := m.Sha256
	reconstructedSha, err := hashFile(*outputPath)
	if err != nil {
		log.Fatalf("failed to hash reconstructed file: %v", err)
	}

	fmt.Printf("===== RECONSTRUCTION COMPLETE =====\n")
	fmt.Printf("Original SHA256:      %s\n", origSha)
	fmt.Printf("Reconstructed SHA256: %s\n", reconstructedSha)
	fmt.Printf("Match: %v\n", origSha == reconstructedSha)
	fmt.Printf("Shards used: %d\n", shardsUsed)
	fmt.Printf("Stats: %+v\n", stats)
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
