package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/watchdog"
)

// MemoryLeaseStore implements a thread-safe in-memory DHT for leases
type MemoryLeaseStore struct {
	mu     sync.Mutex
	values map[string][]byte
}

func NewMemoryLeaseStore() *MemoryLeaseStore {
	return &MemoryLeaseStore{values: make(map[string][]byte)}
}

func (s *MemoryLeaseStore) GetValue(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, ok := s.values[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return val, nil
}

func (s *MemoryLeaseStore) PutValue(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
	return nil
}

// TestNetwork represents the simulated network of nodes
type TestNetwork struct {
	mu         sync.Mutex
	nodes      map[peer.ID]*TestNode
	leaseStore *MemoryLeaseStore
	partitions map[peer.ID]map[peer.ID]bool
}

type TestNode struct {
	ID       peer.ID
	StoreDir string
	Pipeline *watchdog.RepairPipeline
}

func NewTestNetwork() *TestNetwork {
	return &TestNetwork{
		nodes:      make(map[peer.ID]*TestNode),
		leaseStore: NewMemoryLeaseStore(),
		partitions: make(map[peer.ID]map[peer.ID]bool),
	}
}

func (net *TestNetwork) AddNode(t *testing.T, id string) *TestNode {
	pid := peer.ID(id)
	storeDir := t.TempDir()

	node := &TestNode{
		ID:       pid,
		StoreDir: storeDir,
	}

	node.Pipeline = &watchdog.RepairPipeline{
		StoreDir:     storeDir,
		Self:         pid,
		LeaseStore:   net.leaseStore,
		HealthClient: &SimulatedHealthClient{net: net},
		Resolver:     &SimulatedResolver{net: net, self: pid},
	}

	net.mu.Lock()
	net.nodes[pid] = node
	net.mu.Unlock()

	return node
}

// SetPartition defines which peers are visible to 'from'
func (net *TestNetwork) SetPartition(from peer.ID, visible []peer.ID) {
	net.mu.Lock()
	defer net.mu.Unlock()
	vMap := make(map[peer.ID]bool)
	for _, p := range visible {
		vMap[p] = true
	}
	net.partitions[from] = vMap
}

func (net *TestNetwork) ClearPartitions() {
	net.mu.Lock()
	defer net.mu.Unlock()
	net.partitions = make(map[peer.ID]map[peer.ID]bool)
}

// SimulatedResolver implements PeerResolver using TestNetwork
type SimulatedResolver struct {
	net  *TestNetwork
	self peer.ID
}

func (r *SimulatedResolver) VisiblePeers(ctx context.Context, fileID string) ([]peer.ID, error) {
	r.net.mu.Lock()
	defer r.net.mu.Unlock()

	var visible []peer.ID
	vMap, hasPartition := r.net.partitions[r.self]

	for pid := range r.net.nodes {
		if hasPartition {
			if vMap[pid] {
				visible = append(visible, pid)
			}
		} else {
			visible = append(visible, pid)
		}
	}
	return visible, nil
}

// SimulatedHealthClient implements HealthClient using TestNetwork direct file access
type SimulatedHealthClient struct {
	net *TestNetwork
}

func (c *SimulatedHealthClient) QueryHealth(ctx context.Context, target peer.ID, fileID string) (*watchdog.PeerHealthResponse, error) {
	// Simulate small network delay
	time.Sleep(1 * time.Millisecond)

	c.net.mu.Lock()
	targetNode, ok := c.net.nodes[target]
	c.net.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("node offline")
	}

	builder := watchdog.NewLocalBitmapBuilder(targetNode.StoreDir)
	bm, err := builder.BuildBitmap(fileID)
	if err != nil {
		return nil, err
	}

	return &watchdog.PeerHealthResponse{
		Version: 1,
		FileID:  fileID,
		Bitmap:  bm,
	}, nil
}

func (c *SimulatedHealthClient) PushShard(ctx context.Context, target peer.ID, fileID string, shardIdx int, data []byte) error {
	c.net.mu.Lock()
	targetNode, ok := c.net.nodes[target]
	c.net.mu.Unlock()

	if !ok {
		return fmt.Errorf("node offline")
	}

	// Simulate writing the shard directly
	shardDir := filepath.Join(targetNode.StoreDir, fileID)
	os.MkdirAll(shardDir, 0o755)
	shardPath := filepath.Join(shardDir, fmt.Sprintf("shard.%02d", shardIdx))
	return os.WriteFile(shardPath, data, 0o644)
}
