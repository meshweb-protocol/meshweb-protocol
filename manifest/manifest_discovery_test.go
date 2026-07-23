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
)

func TestManifestDiscoveryUnderChurn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "input_discovery.bin")
	fileSize := int64(4 << 20) // 4MB
	if err := genRandomFile(inputFile, fileSize); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	shardDir := filepath.Join(tmp, "shards_discovery")
	if err := os.MkdirAll(shardDir, 0o755); err != nil {
		t.Fatalf("failed to create shard dir: %v", err)
	}

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

	bootstrapHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("failed to create bootstrap host: %v", err)
	}
	defer bootstrapHost.Close()

	discovery.NewRegistry(bootstrapHost)
	bootstrapInfo := peer.AddrInfo{ID: bootstrapHost.ID(), Addrs: bootstrapHost.Addrs()}

	providerCount := 15
	providers := make([]*providerNode, 0, providerCount)
	providerShards := make(map[string][]int)
	providerByID := make(map[string]*providerNode)

	for i := 0; i < providerCount; i++ {
		providerDir := filepath.Join(tmp, fmt.Sprintf("discovery-provider-%d", i))
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

		providerClient := discovery.NewRegistry(h)
		if err := providerClient.Announce(ctx, bootstrapInfo, manifest.FileID, peer.AddrInfo{ID: h.ID(), Addrs: h.Addrs()}, shardIndices); err != nil {
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

	discovered, err := discovery.NewRegistry(clientHost).FindProviders(ctx, bootstrapInfo, manifest.FileID)
	if err != nil {
		t.Fatalf("failed to discover providers for file id: %v", err)
	}
	if len(discovered) < manifest.MinShards/2 {
		t.Fatalf("expected at least %d provider candidates, got %d", manifest.MinShards/2, len(discovered))
	}

	offlineIndexes := []int{2, 7}
	offlineIDs := make(map[string]bool)
	for _, idx := range offlineIndexes {
		providers[idx].Host.Close()
		offlineIDs[providers[idx].Host.ID().String()] = true
	}

	time.Sleep(500 * time.Millisecond)
	discovered, err = discovery.NewRegistry(clientHost).FindProviders(ctx, bootstrapInfo, manifest.FileID)
	if err != nil {
		t.Fatalf("failed to rediscover providers after churn: %v", err)
	}

	selectedShards := make([]int, 0, manifest.MinShards)
	for _, p := range discovered {
		if offlineIDs[p.ID.String()] {
			continue
		}
		if _, ok := providerShards[p.ID.String()]; !ok {
			continue
		}
		if err := clientHost.Connect(ctx, p); err != nil {
			continue
		}
		for _, shardIdx := range providerShards[p.ID.String()] {
			if len(selectedShards) >= manifest.MinShards {
				break
			}
			selectedShards = append(selectedShards, shardIdx)
		}
		if len(selectedShards) >= manifest.MinShards {
			break
		}
	}

	if len(selectedShards) < manifest.MinShards {
		t.Fatalf("not enough shards selected after churn: got %d need %d", len(selectedShards), manifest.MinShards)
	}

	fetchDir := filepath.Join(tmp, "fetch_discovery")
	if err := os.MkdirAll(fetchDir, 0o755); err != nil {
		t.Fatalf("failed to create fetch dir: %v", err)
	}

	for _, shardIdx := range selectedShards {
		var fetched bool
		for _, p := range discovered {
			if offlineIDs[p.ID.String()] {
				continue
			}
			providerNode, ok := providerByID[p.ID.String()]
			if !ok {
				continue
			}
			found := false
			for _, candidate := range providerShards[p.ID.String()] {
				if candidate == shardIdx {
					found = true
					break
				}
			}
			if !found {
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
			t.Fatalf("failed to fetch shard %d from discovered providers", shardIdx)
		}
	}

	downloadManifest := *manifest
	downloadManifest.ShardPaths = make([]string, len(manifest.ShardPaths))
	for i := range downloadManifest.ShardPaths {
		downloadManifest.ShardPaths[i] = fmt.Sprintf("shard.%02d", i)
	}
	downloadManifestPath := filepath.Join(fetchDir, "manifest.json")
	if err := downloadManifest.Save(downloadManifestPath); err != nil {
		t.Fatalf("failed to save download manifest: %v", err)
	}

	restoredPath := filepath.Join(tmp, "restored_discovery.bin")
	if _, err := ReconstructFromManifest(&downloadManifest, downloadManifestPath, restoredPath, selectedShards); err != nil {
		t.Fatalf("reconstruction failed after discovery churn: %v", err)
	}
}
