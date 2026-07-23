package watchdog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/erasure"
	"github.com/meshweb/meshweb-protocol/manifest"
)

type PeerHealthResponse struct {
	Version int
	FileID  string
	Bitmap  []byte
}

// HealthClient abstracts the network calls to query health status of peers.
type HealthClient interface {
	QueryHealth(ctx context.Context, target peer.ID, fileID string) (*PeerHealthResponse, error)
	PushShard(ctx context.Context, target peer.ID, fileID string, shardIdx int, data []byte) error
}

// PeerResolver abstracts the view of the network (useful for chaos partition tests).
type PeerResolver interface {
	VisiblePeers(ctx context.Context, fileID string) ([]peer.ID, error)
}

// RepairPipeline orchestrates the entire repair flow sequentially without timers or loops.
type RepairPipeline struct {
	StoreDir     string
	Self         peer.ID
	LeaseStore   LeaseStore
	HealthClient HealthClient
	Resolver     PeerResolver
}

// RunOnce executes a single repair pass for the given fileID.
func (p *RepairPipeline) RunOnce(ctx context.Context, fileID string) error {
	// 1. Scan Local Storage
	scanner := NewLocalScanner(p.StoreDir, filepath.Join(p.StoreDir, "quarantine"))
	scanRes, err := scanner.ScanLocalFile(fileID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No such file here, nothing to repair
		}
		return fmt.Errorf("local scan failed: %w", err)
	}

	manifestPath := filepath.Join(p.StoreDir, fileID, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	peers, err := p.Resolver.VisiblePeers(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to resolve peers: %w", err)
	}

	// 2. Query Health and Build Network State
	var bitmaps [][]byte

	// Add self locally
	localBitmap := BitmapFromShardIndices(m.DataShards+m.ParityShards, scanRes.HealthyShards)
	bitmaps = append(bitmaps, localBitmap)

	// Add remote peers
	for _, pid := range peers {
		if pid == p.Self {
			continue
		}
		resp, err := p.HealthClient.QueryHealth(ctx, pid, fileID)
		if err == nil && len(resp.Bitmap) > 0 {
			bitmaps = append(bitmaps, resp.Bitmap)
		}
	}

	state := BuildNetworkState(fileID, m.DataShards+m.ParityShards, bitmaps)

	// If no missing shards, we are done!
	if len(state.MissingShards) == 0 {
		return nil
	}

	// 3. Create Repair Job
	job := CreateRepairJob(state)

	// 4. Acquire Lease
	won, err := AcquireLease(ctx, p.LeaseStore, job, p.Self, m.Version)
	if err != nil {
		return fmt.Errorf("lease acquisition failed: %w", err)
	}
	if !won {
		return nil // Another node is repairing it
	}

	// 5. Repair Shards (Reconstruction)
	// Build source paths map for available local shards
	sourcePaths := make(map[int]string)
	for _, idx := range scanRes.HealthyShards {
		sourcePaths[idx] = filepath.Join(p.StoreDir, fileID, fmt.Sprintf("shard.%02d", idx))
	}

	res, reconstructed, err := erasure.RepairShards(
		m.FileID, m.DataShards, m.ParityShards, m.BlockSize, m.OriginalSize, m.ShardHashes,
		state.MissingShards, sourcePaths,
	)
	if err != nil {
		return fmt.Errorf("reconstruction failed: %w", err)
	}

	// 6. Local Commit & Distribute
	for _, idx := range res.Repaired {
		data := reconstructed[idx]

		// Write to local disk if we don't have it
		localPath := filepath.Join(p.StoreDir, fileID, fmt.Sprintf("shard.%02d", idx))
		_ = os.WriteFile(localPath, data, 0o644)

		// Distribute to peers that might need it
		// For simplicity in V1, just push it to visible peers that don't have it
		for _, pid := range peers {
			if pid == p.Self {
				continue
			}
			// PushShard
			_ = p.HealthClient.PushShard(ctx, pid, fileID, idx, data)
		}
	}

	return nil
}
