package manifest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/meshweb/meshweb-protocol/discovery"
	"github.com/meshweb/meshweb-protocol/internal/testutil"
)

// This test is a focused comparison harness between Registry discovery and DHT discovery.
// It runs the same upload/download workflow for both discovery modes and logs:
// - initial lookup latency
// - lookup latency after churn
// - whether reconstruction succeeded
// Keep this harness minimal: it must not add lifecycle/repair/lease logic.

func TestDiscoveryComparison(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "input_compare.bin")
	fileSize := int64(2 << 20) // 2MB to keep test quick
	if err := genRandomFile(inputFile, fileSize); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	shardDir := filepath.Join(tmp, "shards_compare")
	if err := os.MkdirAll(shardDir, 0o755); err != nil {
		t.Fatalf("failed to create shard dir: %v", err)
	}

	// Use same RS parameters as other tests to keep parity
	manifest, err := CreateUploadManifest(inputFile, shardDir, 10, 20, 1<<20)
	if err != nil {
		t.Fatalf("CreateUploadManifest failed: %v", err)
	}

	manifestPath := filepath.Join(shardDir, "manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	resolvedShardPaths := ResolveShardPaths(manifest, filepath.Dir(manifestPath))
	if len(resolvedShardPaths) != manifest.DataShards+manifest.ParityShards {
		t.Fatalf("resolved shard count mismatch")
	}

	providerCount := 12

	// --- Registry mode ---
	t.Log("=== Registry discovery run ===")
	regResult := runRegistryScenario(ctx, t, tmp, manifest, resolvedShardPaths, providerCount)
	t.Logf("Registry: initialLookup=%s afterChurnLookup=%s success=%v", regResult.initialLookup, regResult.afterChurnLookup, regResult.success)

	// --- DHT mode ---
	t.Log("=== DHT discovery run ===")
	dhtResult := runDHTScenario(ctx, t, tmp, manifest, resolvedShardPaths, providerCount)
	t.Logf("DHT: initialLookup=%s afterChurnLookup=%s success=%v", dhtResult.initialLookup, dhtResult.afterChurnLookup, dhtResult.success)

	// Simple conclusion logged for human review (automated recommendation not made here)
	t.Log("Comparison complete. Review latencies and success flags above for recommendation.")
}

type scenarioResult struct {
	initialLookup    time.Duration
	afterChurnLookup time.Duration
	success          bool
}

func runRegistryScenario(ctx context.Context, t *testing.T, tmp string, manifest *FileManifest, resolvedShardPaths []string, providerCount int) scenarioResult {
	bootstrapHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("failed to create bootstrap host: %v", err)
	}
	defer bootstrapHost.Close()
	// install registry handler on bootstrap node
	_ = discovery.NewRegistry(bootstrapHost)
	bootstrapInfo := peer.AddrInfo{ID: bootstrapHost.ID(), Addrs: bootstrapHost.Addrs()}

	providers := make([]*providerNode, 0, providerCount)
	providerShards := make(map[string][]int)
	providerByID := make(map[string]*providerNode)

	for i := 0; i < providerCount; i++ {
		providerDir := filepath.Join(tmp, fmt.Sprintf("reg-provider-%d", i))
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		if err != nil {
			t.Fatalf("failed to create provider host: %v", err)
		}
		defer h.Close()

		pn := startProviderWithHost(t, manifest.FileID, providerDir, h)
		providers = append(providers, pn)
		providerByID[pn.Host.ID().String()] = pn

		firstShard := i * 2
		shardIndices := []int{firstShard, firstShard + 1}
		for _, shardIdx := range shardIndices {
			if shardIdx >= len(resolvedShardPaths) {
				break
			}
			shardBytes, err := os.ReadFile(resolvedShardPaths[shardIdx])
			if err != nil {
				t.Fatalf("failed to read shard %d: %v", shardIdx, err)
			}
			if err := os.WriteFile(filepath.Join(pn.StoreDir, fmt.Sprintf("shard.%02d", shardIdx)), shardBytes, 0o644); err != nil {
				t.Fatalf("failed to store shard %d on provider: %v", shardIdx, err)
			}
			providerShards[pn.Host.ID().String()] = append(providerShards[pn.Host.ID().String()], shardIdx)
		}

		if err := h.Connect(ctx, bootstrapInfo); err != nil {
			t.Fatalf("provider failed to connect to bootstrap: %v", err)
		}
		if err := bootstrapHost.Connect(ctx, peer.AddrInfo{ID: h.ID(), Addrs: h.Addrs()}); err != nil {
			t.Fatalf("bootstrap failed to connect to provider: %v", err)
		}

		// Announce to registry via provider's registry client
		providerClient := discovery.NewRegistry(h)
		if err := providerClient.Announce(ctx, bootstrapInfo, manifest.FileID, peer.AddrInfo{ID: h.ID(), Addrs: h.Addrs()}, providerShards[pn.Host.ID().String()]); err != nil {
			t.Fatalf("provider failed to announce file id: %v", err)
		}
	}

	clientHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("failed to create client host: %v", err)
	}
	defer clientHost.Close()
	if err := clientHost.Connect(ctx, bootstrapInfo); err != nil {
		t.Fatalf("client failed to connect to bootstrap: %v", err)
	}

	clientRegistry := discovery.NewRegistry(clientHost)

	// initial lookup
	start := time.Now()
	discovered, err := clientRegistry.FindProviders(ctx, bootstrapInfo, manifest.FileID)
	lookup := time.Since(start)
	if err != nil {
		t.Fatalf("registry find providers failed: %v", err)
	}
	if len(discovered) == 0 {
		t.Fatalf("registry returned zero providers")
	}

	// Simulate churn: take two providers offline
	offline := []int{1, 5}
	offlineIDs := map[string]bool{}
	for _, idx := range offline {
		providers[idx].Host.Close()
		offlineIDs[providers[idx].Host.ID().String()] = true
	}
	time.Sleep(300 * time.Millisecond)

	// lookup after churn
	start2 := time.Now()
	discovered2, err := clientRegistry.FindProviders(ctx, bootstrapInfo, manifest.FileID)
	lookup2 := time.Since(start2)
	if err != nil {
		t.Fatalf("registry find after churn failed: %v", err)
	}

	// attempt to fetch enough shards
	selectedShards := selectShardsFromDiscovered(discovered2, providerShards, offlineIDs, manifest.MinShards)
	if len(selectedShards) < manifest.MinShards {
		return scenarioResult{initialLookup: lookup, afterChurnLookup: lookup2, success: false}
	}

	fetchDir := filepath.Join(tmp, "fetch_registry")
	if err := os.MkdirAll(fetchDir, 0o755); err != nil {
		t.Fatalf("failed to create fetch dir: %v", err)
	}

	for _, shardIdx := range selectedShards {
		// find provider that has this shard
		found := false
		for _, p := range discovered2 {
			if offlineIDs[p.ID.String()] {
				continue
			}
			providerNode, ok := providerByID[p.ID.String()]
			if !ok {
				continue
			}
			if !containsInt(providerShards[p.ID.String()], shardIdx) {
				continue
			}
			data, err := fetchShard(ctx, clientHost, providerNode, shardIdx)
			if err != nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(fetchDir, fmt.Sprintf("shard.%02d", shardIdx)), data, 0o644); err != nil {
				t.Fatalf("failed to write shard %d: %v", shardIdx, err)
			}
			found = true
			break
		}
		if !found {
			return scenarioResult{initialLookup: lookup, afterChurnLookup: lookup2, success: false}
		}
	}

	// attempt reconstruction
	downloadManifest := *manifest
	downloadManifest.ShardPaths = make([]string, len(manifest.ShardPaths))
	for i := range downloadManifest.ShardPaths {
		downloadManifest.ShardPaths[i] = fmt.Sprintf("shard.%02d", i)
	}
	manifestPath := filepath.Join(fetchDir, "manifest.json")
	if err := downloadManifest.Save(manifestPath); err != nil {
		t.Fatalf("failed to save download manifest: %v", err)
	}
	restoredPath := filepath.Join(tmp, "restored_registry.bin")
	if _, err := ReconstructFromManifest(&downloadManifest, manifestPath, restoredPath, selectedShards); err != nil {
		return scenarioResult{initialLookup: lookup, afterChurnLookup: lookup2, success: false}
	}
	return scenarioResult{initialLookup: lookup, afterChurnLookup: lookup2, success: true}
}

func runDHTScenario(ctx context.Context, t *testing.T, tmp string, manifest *FileManifest, resolvedShardPaths []string, providerCount int) scenarioResult {
	// Create DHT bootstrap node
	bootHost, bootDHT, err := discovery.NewDHTHost(ctx, []string{"/ip4/127.0.0.1/tcp/0"})
	if err != nil {
		t.Fatalf("failed to create bootstrap DHT host: %v", err)
	}
	defer bootHost.Close()
	if err := discovery.BootstrapDHT(ctx, bootHost, bootDHT, nil); err != nil {
		t.Fatalf("failed to bootstrap DHT bootstrap: %v", err)
	}

	providers := make([]*providerNode, 0, providerCount)
	providerShards := make(map[string][]int)
	providerByID := make(map[string]*providerNode)
	// providerDHTs not needed for this benchmark

	for i := 0; i < providerCount; i++ {
		providerDir := filepath.Join(tmp, fmt.Sprintf("dht-provider-%d", i))
		h, dhtNode, err := discovery.NewDHTHost(ctx, []string{"/ip4/127.0.0.1/tcp/0"}, peer.AddrInfo{ID: bootHost.ID(), Addrs: bootHost.Addrs()})
		if err != nil {
			t.Fatalf("failed to create provider DHT host: %v", err)
		}
		defer h.Close()
		if err := h.Connect(ctx, peer.AddrInfo{ID: bootHost.ID(), Addrs: bootHost.Addrs()}); err != nil {
			t.Fatalf("provider failed to connect to bootstrap: %v", err)
		}
		if err := discovery.BootstrapDHT(ctx, h, dhtNode, []peer.AddrInfo{{ID: bootHost.ID(), Addrs: bootHost.Addrs()}}); err != nil {
			t.Fatalf("failed to bootstrap provider DHT node: %v", err)
		}
		if err := testutil.WaitForRoutingReady(ctx, dhtNode); err != nil {
			t.Fatalf("provider DHT failed to become ready: %v", err)
		}

		pn := startProviderWithHost(t, manifest.FileID, providerDir, h)
		providers = append(providers, pn)
		providerByID[pn.Host.ID().String()] = pn

		firstShard := i * 2
		shardIndices := []int{firstShard, firstShard + 1}
		for _, shardIdx := range shardIndices {
			if shardIdx >= len(resolvedShardPaths) {
				break
			}
			shardBytes, err := os.ReadFile(resolvedShardPaths[shardIdx])
			if err != nil {
				t.Fatalf("failed to read shard %d: %v", shardIdx, err)
			}
			if err := os.WriteFile(filepath.Join(pn.StoreDir, fmt.Sprintf("shard.%02d", shardIdx)), shardBytes, 0o644); err != nil {
				t.Fatalf("failed to store shard %d on provider: %v", shardIdx, err)
			}
			providerShards[pn.Host.ID().String()] = append(providerShards[pn.Host.ID().String()], shardIdx)
		}

		// Advertise on the DHT
		if err := discovery.AdvertiseFileID(ctx, dhtNode, manifest.FileID); err != nil {
			t.Fatalf("provider failed to advertise file id: %v", err)
		}
	}

	// client DHT host
	clientHost, clientDHT, err := discovery.NewDHTHost(ctx, []string{"/ip4/127.0.0.1/tcp/0"}, peer.AddrInfo{ID: bootHost.ID(), Addrs: bootHost.Addrs()})
	if err != nil {
		t.Fatalf("failed to create client DHT host: %v", err)
	}
	defer clientHost.Close()
	if err := clientHost.Connect(ctx, peer.AddrInfo{ID: bootHost.ID(), Addrs: bootHost.Addrs()}); err != nil {
		t.Fatalf("client failed to connect to bootstrap: %v", err)
	}
	if err := discovery.BootstrapDHT(ctx, clientHost, clientDHT, []peer.AddrInfo{{ID: bootHost.ID(), Addrs: bootHost.Addrs()}}); err != nil {
		t.Fatalf("failed to bootstrap client DHT node: %v", err)
	}
	if err := testutil.WaitForRoutingReady(ctx, clientDHT); err != nil {
		t.Fatalf("client DHT failed to become ready: %v", err)
	}

	// initial lookup (with a small retry loop to allow DHT propagation)
	var discovered []peer.AddrInfo
	var lookup time.Duration
	for attempt := 0; attempt < 5; attempt++ {
		start := time.Now()
		discovered, _ = discovery.FindProviderPeers(ctx, clientDHT, manifest.FileID, providerCount)
		lookup = time.Since(start)
		if len(discovered) > 0 {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if len(discovered) == 0 {
		t.Fatalf("dht returned zero providers after retries")
	}

	// simulate churn
	offline := []int{2, 6}
	offlineIDs := map[string]bool{}
	for _, idx := range offline {
		providers[idx].Host.Close()
		offlineIDs[providers[idx].Host.ID().String()] = true
	}
	time.Sleep(500 * time.Millisecond)

	// lookup after churn
	var discovered2 []peer.AddrInfo
	var lookup2 time.Duration
	for attempt := 0; attempt < 5; attempt++ {
		start := time.Now()
		discovered2, _ = discovery.FindProviderPeers(ctx, clientDHT, manifest.FileID, providerCount)
		lookup2 = time.Since(start)
		if len(discovered2) >= manifest.MinShards/2 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	if len(discovered2) == 0 {
		return scenarioResult{initialLookup: lookup, afterChurnLookup: lookup2, success: false}
	}

	// attempt to select shards
	selectedShards := selectShardsFromDiscovered(discovered2, providerShards, offlineIDs, manifest.MinShards)
	if len(selectedShards) < manifest.MinShards {
		return scenarioResult{initialLookup: lookup, afterChurnLookup: lookup2, success: false}
	}

	// fetch selected shards
	fetchDir := filepath.Join(tmp, "fetch_dht")
	if err := os.MkdirAll(fetchDir, 0o755); err != nil {
		t.Fatalf("failed to create fetch dir: %v", err)
	}

	for _, shardIdx := range selectedShards {
		var fetched bool
		for _, p := range discovered2 {
			if offlineIDs[p.ID.String()] {
				continue
			}
			providerNode, ok := providerByID[p.ID.String()]
			if !ok {
				continue
			}
			if !containsInt(providerShards[p.ID.String()], shardIdx) {
				continue
			}
			data, err := fetchShard(ctx, clientHost, providerNode, shardIdx)
			if err != nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(fetchDir, fmt.Sprintf("shard.%02d", shardIdx)), data, 0o644); err != nil {
				t.Fatalf("failed to write shard %d: %v", shardIdx, err)
			}
			fetched = true
			break
		}
		if !fetched {
			return scenarioResult{initialLookup: lookup, afterChurnLookup: lookup2, success: false}
		}
	}

	downloadManifest := *manifest
	downloadManifest.ShardPaths = make([]string, len(manifest.ShardPaths))
	for i := range downloadManifest.ShardPaths {
		downloadManifest.ShardPaths[i] = fmt.Sprintf("shard.%02d", i)
	}
	manifestPath := filepath.Join(fetchDir, "manifest.json")
	if err := downloadManifest.Save(manifestPath); err != nil {
		t.Fatalf("failed to save download manifest: %v", err)
	}
	restoredPath := filepath.Join(tmp, "restored_dht.bin")
	if _, err := ReconstructFromManifest(&downloadManifest, manifestPath, restoredPath, selectedShards); err != nil {
		return scenarioResult{initialLookup: lookup, afterChurnLookup: lookup2, success: false}
	}
	return scenarioResult{initialLookup: lookup, afterChurnLookup: lookup2, success: true}
}

// selectShardsFromDiscovered picks up to needed shards from discovered providers ignoring offline IDs
func selectShardsFromDiscovered(discovered []peer.AddrInfo, providerShards map[string][]int, offlineIDs map[string]bool, needed int) []int {
	selected := make([]int, 0, needed)
	for _, p := range discovered {
		if offlineIDs[p.ID.String()] {
			continue
		}
		if shards, ok := providerShards[p.ID.String()]; ok {
			for _, s := range shards {
				if len(selected) >= needed {
					break
				}
				if !containsInt(selected, s) {
					selected = append(selected, s)
				}
			}
		}
		if len(selected) >= needed {
			break
		}
	}
	return selected
}
