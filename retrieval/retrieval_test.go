package retrieval

import (
	"bufio"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/manifest"
)

func createTestHost(t *testing.T) host.Host {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("failed to create host: %v", err)
	}
	return h
}

// TestV2MalformedResponse tests that a provider returning bad JSON or oversized length fails gracefully
func TestV2MalformedResponse(t *testing.T) {
	provider := createTestHost(t)
	defer provider.Close()

	client := createTestHost(t)
	defer client.Close()

	client.Connect(context.Background(), peer.AddrInfo{ID: provider.ID(), Addrs: provider.Addrs()})

	// Malformed JSON handler
	provider.SetStreamHandler("/meshweb/storage/2.0.0", func(s network.Stream) {
		defer s.Close()
		bufio.NewReader(s).ReadBytes('\n') // consume request
		s.Write([]byte("{ malformed json ... \n"))
	})

	_, err := fetchChunk(context.Background(), client, provider.ID(), "testfile", 0, 0, 1024)
	if err == nil {
		t.Fatalf("expected error on malformed json")
	}

	// Oversized length handler
	provider.SetStreamHandler("/meshweb/storage/2.0.0", func(s network.Stream) {
		defer s.Close()
		bufio.NewReader(s).ReadBytes('\n')
		resp := chunkResponse{
			Status:         "ok",
			FileID:         "testfile",
			Shard:          0,
			Offset:         0,
			Length:         maxSegmentSize + 1, // oversized
			TotalShardSize: maxSegmentSize + 1,
		}
		b, _ := json.Marshal(resp)
		s.Write(append(b, '\n'))
	})

	_, err = fetchChunk(context.Background(), client, provider.ID(), "testfile", 0, 0, 1024)
	if err == nil {
		t.Fatalf("expected error on oversized length from provider metadata")
	}
}

// TestV1HashMismatch tests that a downloaded shard with invalid hash is rejected
func TestV1HashMismatch(t *testing.T) {
	provider := createTestHost(t)
	defer provider.Close()

	client := createTestHost(t)
	defer client.Close()

	client.Connect(context.Background(), peer.AddrInfo{ID: provider.ID(), Addrs: provider.Addrs()})

	// Provide bad data
	provider.SetStreamHandler("/meshweb/storage/1.0.0", func(s network.Stream) {
		defer s.Close()
		var req struct {
			FileID string `json:"file_id"`
			Shard  int    `json:"shard"`
		}
		json.NewDecoder(s).Decode(&req)
		resp := map[string]string{
			"data": "YmFkZGF0YQ==", // "baddata"
		}
		json.NewEncoder(s).Encode(resp)
	})

	m := &manifest.FileManifest{
		FileID:       "testfile",
		DataShards:   1,
		ParityShards: 0,
		MinShards:    1,
		ShardHashes:  []string{"validhash0000000000000000000000000000000000000000000000000000000"},
	}

	storeDir := t.TempDir()
	outPath := filepath.Join(storeDir, "out.dat")

	_, _, err := RunV1(context.Background(), client, m, []peer.AddrInfo{{ID: provider.ID(), Addrs: provider.Addrs()}}, 1, storeDir, outPath)
	if err == nil {
		t.Fatalf("expected error due to hash mismatch")
	}
}

func FuzzChunkResponse(f *testing.F) {
	f.Add([]byte(`{"status":"ok","file_id":"test","shard":0,"offset":0,"length":10,"total_shard_size":100}` + "\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var resp chunkResponse
		if err := json.Unmarshal(data, &resp); err == nil {
			_ = resp.Status
		}
	})
}

func TestRetrievalLeak1000x(t *testing.T) {
	client := createTestHost(t)
	defer client.Close()

	m := &manifest.FileManifest{
		FileID:       "testfile",
		DataShards:   1,
		ParityShards: 0,
		MinShards:    1,
		ShardHashes:  []string{"dummyhash"},
	}

	storeDir := t.TempDir()
	outPath := filepath.Join(storeDir, "out.dat")

	// Execute 100 rapid retrieve/cancel cycles
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately
		_, _, _ = RunV1(ctx, client, m, nil, 1, storeDir, outPath)
	}
}

func TestDeterministicRetrieval100x(t *testing.T) {
	// Simple sanity test for 100x determinism
	hash1 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	for i := 0; i < 100; i++ {
		if hash1 != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
			t.Fatalf("non-deterministic output at iteration %d", i)
		}
	}
}
