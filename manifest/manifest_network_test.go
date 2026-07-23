package manifest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const storageProtocolID = protocol.ID("/meshweb/storage/1.0.0")

type chunkRequest struct {
	FileID string `json:"file_id"`
	Shard  int    `json:"shard"`
}

type chunkResponse struct {
	FileID string `json:"file_id"`
	Shard  int    `json:"shard"`
	Data   string `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
}

type providerNode struct {
	Host     host.Host
	StoreDir string
	FileID   string
	Delay    time.Duration
}

func newTestHost(ctx context.Context) (host.Host, error) {
	return libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
}

func startProviderWithDelay(t *testing.T, ctx context.Context, fileID, storeDir string, delay time.Duration) *providerNode {
	h, err := newTestHost(ctx)
	if err != nil {
		t.Fatalf("failed to create provider host: %v", err)
	}

	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("failed to create provider storage dir: %v", err)
	}

	h.SetStreamHandler(storageProtocolID, func(s network.Stream) {
		defer s.Close()
		if delay > 0 {
			time.Sleep(delay)
		}

		var req chunkRequest
		if err := json.NewDecoder(s).Decode(&req); err != nil {
			return
		}
		if req.FileID != fileID {
			_ = json.NewEncoder(s).Encode(chunkResponse{
				FileID: req.FileID,
				Shard:  req.Shard,
				Error:  "file id mismatch",
			})
			return
		}

		shardPath := filepath.Join(storeDir, fmt.Sprintf("shard.%02d", req.Shard))
		data, err := os.ReadFile(shardPath)
		if err != nil {
			_ = json.NewEncoder(s).Encode(chunkResponse{
				FileID: req.FileID,
				Shard:  req.Shard,
				Error:  err.Error(),
			})
			return
		}

		_ = json.NewEncoder(s).Encode(chunkResponse{
			FileID: req.FileID,
			Shard:  req.Shard,
			Data:   base64.StdEncoding.EncodeToString(data),
		})
	})

	return &providerNode{Host: h, StoreDir: storeDir, FileID: fileID, Delay: delay}
}

func startProvider(t *testing.T, ctx context.Context, fileID, storeDir string) *providerNode {
	return startProviderWithDelay(t, ctx, fileID, storeDir, 0)
}

func startProviderWithHost(t *testing.T, fileID, storeDir string, h host.Host) *providerNode {
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("failed to create provider storage dir: %v", err)
	}

	h.SetStreamHandler(storageProtocolID, func(s network.Stream) {
		defer s.Close()

		var req chunkRequest
		if err := json.NewDecoder(s).Decode(&req); err != nil {
			return
		}
		if req.FileID != fileID {
			_ = json.NewEncoder(s).Encode(chunkResponse{
				FileID: req.FileID,
				Shard:  req.Shard,
				Error:  "file id mismatch",
			})
			return
		}

		shardPath := filepath.Join(storeDir, fmt.Sprintf("shard.%02d", req.Shard))
		data, err := os.ReadFile(shardPath)
		if err != nil {
			_ = json.NewEncoder(s).Encode(chunkResponse{
				FileID: req.FileID,
				Shard:  req.Shard,
				Error:  err.Error(),
			})
			return
		}

		_ = json.NewEncoder(s).Encode(chunkResponse{
			FileID: req.FileID,
			Shard:  req.Shard,
			Data:   base64.StdEncoding.EncodeToString(data),
		})
	})

	return &providerNode{Host: h, StoreDir: storeDir, FileID: fileID}
}

func connectProviders(t *testing.T, ctx context.Context, client host.Host, providers []*providerNode) {
	for _, provider := range providers {
		if provider.Host == nil {
			continue
		}
		if err := client.Connect(ctx, peer.AddrInfo{ID: provider.Host.ID(), Addrs: provider.Host.Addrs()}); err != nil {
			t.Fatalf("failed to connect to provider %s: %v", provider.Host.ID().String(), err)
		}
	}
}

func containsInt(list []int, value int) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func fetchShard(ctx context.Context, client host.Host, provider *providerNode, shard int) ([]byte, error) {
	stream, err := client.NewStream(ctx, provider.Host.ID(), storageProtocolID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	req := chunkRequest{
		FileID: provider.FileID,
		Shard:  shard,
	}
	if err := json.NewEncoder(stream).Encode(req); err != nil {
		return nil, err
	}

	var resp chunkResponse
	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("provider error: %s", resp.Error)
	}
	return base64.StdEncoding.DecodeString(resp.Data)
}

func TestManifestNetworkReconstruction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "input.bin")
	fileSize := int64(4 << 20) // 4MB
	if err := genRandomFile(inputFile, fileSize); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	originalSha, err := hashFile(inputFile)
	if err != nil {
		t.Fatalf("failed to compute original sha256: %v", err)
	}

	shardDir := filepath.Join(tmp, "shards")
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
		t.Fatalf("resolved shard path count mismatch")
	}

	numProviders := 6
	providers := make([]*providerNode, 0, numProviders)
	for i := 0; i < numProviders; i++ {
		providerDir := filepath.Join(tmp, fmt.Sprintf("provider-%d", i))
		providers = append(providers, startProvider(t, ctx, manifest.FileID, providerDir))
		defer providers[i].Host.Close()
	}

	for shardIdx, shardPath := range resolvedShardPaths {
		destProvider := providers[shardIdx%numProviders]
		shardBytes, err := os.ReadFile(shardPath)
		if err != nil {
			t.Fatalf("failed to read shard %d: %v", shardIdx, err)
		}
		if err := os.WriteFile(filepath.Join(destProvider.StoreDir, fmt.Sprintf("shard.%02d", shardIdx)), shardBytes, 0o644); err != nil {
			t.Fatalf("failed to copy shard %d to provider: %v", shardIdx, err)
		}
	}

	clientHost, err := newTestHost(ctx)
	if err != nil {
		t.Fatalf("failed to create client host: %v", err)
	}
	defer clientHost.Close()

	connectProviders(t, ctx, clientHost, providers)
	time.Sleep(200 * time.Millisecond)

	selectedShards := []int{0, 1, 2, 3, 4, 10, 11, 12, 13, 14}
	fetchDir := filepath.Join(tmp, "fetch")
	if err := os.MkdirAll(fetchDir, 0o755); err != nil {
		t.Fatalf("failed to create fetch dir: %v", err)
	}

	for _, shardIdx := range selectedShards {
		provider := providers[shardIdx%numProviders]
		data, err := fetchShard(ctx, clientHost, provider, shardIdx)
		if err != nil {
			t.Fatalf("failed to fetch shard %d from provider %s: %v", shardIdx, provider.Host.ID().String(), err)
		}
		if err := os.WriteFile(filepath.Join(fetchDir, fmt.Sprintf("shard.%02d", shardIdx)), data, 0o644); err != nil {
			t.Fatalf("failed to write fetched shard %d: %v", shardIdx, err)
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

	restoredPath := filepath.Join(tmp, "restored.bin")
	stats, err := ReconstructFromManifest(&downloadManifest, downloadManifestPath, restoredPath, selectedShards)
	if err != nil {
		t.Fatalf("failed to reconstruct from network shards: %v", err)
	}

	restoredSha, err := hashFile(restoredPath)
	if err != nil {
		t.Fatalf("failed to hash restored file: %v", err)
	}
	if restoredSha != originalSha {
		t.Fatalf("restored sha mismatch: got %s expected %s", restoredSha, originalSha)
	}

	t.Logf("Network manifest reconstruction succeeded: %d shards fetched from %d providers, decode duration=%v, max alloc=%d", len(selectedShards), numProviders, stats.DecodeDuration, stats.MaxAllocBytes)
}

func TestManifestNetworkMixedShardReconstruction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "input_mixed.bin")
	fileSize := int64(4 << 20) // 4MB
	if err := genRandomFile(inputFile, fileSize); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	shardDir := filepath.Join(tmp, "shards_mixed")
	if err := os.MkdirAll(shardDir, 0o755); err != nil {
		t.Fatalf("failed to create shard dir: %v", err)
	}

	manifest, err := CreateUploadManifest(inputFile, shardDir, 10, 20, 1<<20)
	if err != nil {
		t.Fatalf("CreateUploadManifest failed: %v", err)
	}
	manifestPath := filepath.Join(tmp, "shards_mixed", "manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	resolvedShardPaths := ResolveShardPaths(manifest, filepath.Dir(manifestPath))
	numProviders := 6
	providers := make([]*providerNode, 0, numProviders)
	for i := 0; i < numProviders; i++ {
		providerDir := filepath.Join(tmp, fmt.Sprintf("mixed-provider-%d", i))
		providers = append(providers, startProvider(t, ctx, manifest.FileID, providerDir))
		defer providers[i].Host.Close()
	}
	for shardIdx, shardPath := range resolvedShardPaths {
		destProvider := providers[shardIdx%numProviders]
		shardBytes, err := os.ReadFile(shardPath)
		if err != nil {
			t.Fatalf("failed to read shard %d: %v", shardIdx, err)
		}
		if err := os.WriteFile(filepath.Join(destProvider.StoreDir, fmt.Sprintf("shard.%02d", shardIdx)), shardBytes, 0o644); err != nil {
			t.Fatalf("failed to copy shard %d: %v", shardIdx, err)
		}
	}

	clientHost, err := newTestHost(ctx)
	if err != nil {
		t.Fatalf("failed to create client host: %v", err)
	}
	defer clientHost.Close()
	connectProviders(t, ctx, clientHost, providers)
	time.Sleep(200 * time.Millisecond)

	// 5 data shards and 5 parity shards.
	selectedShards := []int{1, 3, 6, 9, 7, 12, 17, 22, 25, 28}
	fetchDir := filepath.Join(tmp, "fetch_mixed")
	if err := os.MkdirAll(fetchDir, 0o755); err != nil {
		t.Fatalf("failed to create fetch dir: %v", err)
	}
	for _, shardIdx := range selectedShards {
		provider := providers[shardIdx%numProviders]
		data, err := fetchShard(ctx, clientHost, provider, shardIdx)
		if err != nil {
			t.Fatalf("failed to fetch shard %d: %v", shardIdx, err)
		}
		if err := os.WriteFile(filepath.Join(fetchDir, fmt.Sprintf("shard.%02d", shardIdx)), data, 0o644); err != nil {
			t.Fatalf("failed to write shard %d: %v", shardIdx, err)
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

	restoredPath := filepath.Join(tmp, "restored_mixed.bin")
	if _, err := ReconstructFromManifest(&downloadManifest, downloadManifestPath, restoredPath, selectedShards); err != nil {
		t.Fatalf("mixed shard reconstruction failed: %v", err)
	}
}

func TestManifestNetworkProviderLoss(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "input_provider_loss.bin")
	fileSize := int64(4 << 20) // 4MB
	if err := genRandomFile(inputFile, fileSize); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	shardDir := filepath.Join(tmp, "shards_loss")
	if err := os.MkdirAll(shardDir, 0o755); err != nil {
		t.Fatalf("failed to create shard dir: %v", err)
	}

	manifest, err := CreateUploadManifest(inputFile, shardDir, 10, 20, 1<<20)
	if err != nil {
		t.Fatalf("CreateUploadManifest failed: %v", err)
	}
	manifestPath := filepath.Join(tmp, "shards_loss", "manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	resolvedShardPaths := ResolveShardPaths(manifest, filepath.Dir(manifestPath))
	numProviders := 8
	providers := make([]*providerNode, 0, numProviders)
	for i := 0; i < numProviders; i++ {
		providerDir := filepath.Join(tmp, fmt.Sprintf("loss-provider-%d", i))
		providers = append(providers, startProvider(t, ctx, manifest.FileID, providerDir))
	}

	for shardIdx, shardPath := range resolvedShardPaths {
		destProvider := providers[shardIdx%numProviders]
		shardBytes, err := os.ReadFile(shardPath)
		if err != nil {
			t.Fatalf("failed to read shard %d: %v", shardIdx, err)
		}
		if err := os.WriteFile(filepath.Join(destProvider.StoreDir, fmt.Sprintf("shard.%02d", shardIdx)), shardBytes, 0o644); err != nil {
			t.Fatalf("failed to copy shard %d: %v", shardIdx, err)
		}
	}

	offlineProviders := []int{1, 5}
	onlineProviders := make([]*providerNode, 0, numProviders-len(offlineProviders))
	for idx, provider := range providers {
		if containsInt(offlineProviders, idx) {
			provider.Host.Close()
			continue
		}
		onlineProviders = append(onlineProviders, provider)
		defer provider.Host.Close()
	}

	clientHost, err := newTestHost(ctx)
	if err != nil {
		t.Fatalf("failed to create client host: %v", err)
	}
	defer clientHost.Close()
	connectProviders(t, ctx, clientHost, onlineProviders)
	time.Sleep(200 * time.Millisecond)

	var selectedShards []int
	for shardIdx := 0; shardIdx < manifest.DataShards+manifest.ParityShards && len(selectedShards) < manifest.MinShards; shardIdx++ {
		if containsInt(offlineProviders, shardIdx%numProviders) {
			continue
		}
		selectedShards = append(selectedShards, shardIdx)
	}

	if len(selectedShards) < manifest.MinShards {
		t.Fatalf("not enough available shards after provider loss")
	}

	fetchDir := filepath.Join(tmp, "fetch_loss")
	if err := os.MkdirAll(fetchDir, 0o755); err != nil {
		t.Fatalf("failed to create fetch dir: %v", err)
	}
	for _, shardIdx := range selectedShards {
		provider := providers[shardIdx%numProviders]
		data, err := fetchShard(ctx, clientHost, provider, shardIdx)
		if err != nil {
			t.Fatalf("failed to fetch shard %d after provider loss: %v", shardIdx, err)
		}
		if err := os.WriteFile(filepath.Join(fetchDir, fmt.Sprintf("shard.%02d", shardIdx)), data, 0o644); err != nil {
			t.Fatalf("failed to write shard %d: %v", shardIdx, err)
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

	restoredPath := filepath.Join(tmp, "restored_loss.bin")
	if _, err := ReconstructFromManifest(&downloadManifest, downloadManifestPath, restoredPath, selectedShards); err != nil {
		t.Fatalf("reconstruction failed after provider loss: %v", err)
	}
}

func TestManifestNetworkSlowProviders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "input_slow.bin")
	fileSize := int64(4 << 20) // 4MB
	if err := genRandomFile(inputFile, fileSize); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	shardDir := filepath.Join(tmp, "shards_slow")
	if err := os.MkdirAll(shardDir, 0o755); err != nil {
		t.Fatalf("failed to create shard dir: %v", err)
	}

	manifest, err := CreateUploadManifest(inputFile, shardDir, 10, 20, 1<<20)
	if err != nil {
		t.Fatalf("CreateUploadManifest failed: %v", err)
	}
	manifestPath := filepath.Join(tmp, "shards_slow", "manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	resolvedShardPaths := ResolveShardPaths(manifest, filepath.Dir(manifestPath))
	numProviders := 6
	providers := make([]*providerNode, 0, numProviders)
	for i := 0; i < numProviders; i++ {
		delay := time.Duration(0)
		if i%2 == 0 {
			delay = 250 * time.Millisecond
		}
		providerDir := filepath.Join(tmp, fmt.Sprintf("slow-provider-%d", i))
		providers = append(providers, startProviderWithDelay(t, ctx, manifest.FileID, providerDir, delay))
		defer providers[i].Host.Close()
	}

	for shardIdx, shardPath := range resolvedShardPaths {
		destProvider := providers[shardIdx%numProviders]
		shardBytes, err := os.ReadFile(shardPath)
		if err != nil {
			t.Fatalf("failed to read shard %d: %v", shardIdx, err)
		}
		if err := os.WriteFile(filepath.Join(destProvider.StoreDir, fmt.Sprintf("shard.%02d", shardIdx)), shardBytes, 0o644); err != nil {
			t.Fatalf("failed to copy shard %d: %v", shardIdx, err)
		}
	}

	clientHost, err := newTestHost(ctx)
	if err != nil {
		t.Fatalf("failed to create client host: %v", err)
	}
	defer clientHost.Close()
	connectProviders(t, ctx, clientHost, providers)
	time.Sleep(200 * time.Millisecond)

	selectedShards := []int{0, 5, 8, 11, 14, 17, 20, 23, 26, 29}
	fetchDir := filepath.Join(tmp, "fetch_slow")
	if err := os.MkdirAll(fetchDir, 0o755); err != nil {
		t.Fatalf("failed to create fetch dir: %v", err)
	}
	for _, shardIdx := range selectedShards {
		provider := providers[shardIdx%numProviders]
		data, err := fetchShard(ctx, clientHost, provider, shardIdx)
		if err != nil {
			t.Fatalf("failed to fetch shard %d from slow provider set: %v", shardIdx, err)
		}
		if err := os.WriteFile(filepath.Join(fetchDir, fmt.Sprintf("shard.%02d", shardIdx)), data, 0o644); err != nil {
			t.Fatalf("failed to write shard %d: %v", shardIdx, err)
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

	restoredPath := filepath.Join(tmp, "restored_slow.bin")
	if _, err := ReconstructFromManifest(&downloadManifest, downloadManifestPath, restoredPath, selectedShards); err != nil {
		t.Fatalf("reconstruction failed with slow providers: %v", err)
	}
}
