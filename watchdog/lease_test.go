package watchdog_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/watchdog"
)

func genPeerID() peer.ID {
	priv, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	id, _ := peer.IDFromPrivateKey(priv)
	return id
}

// MockStore implements LeaseStore for testing
type MockStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func NewMockStore() *MockStore {
	return &MockStore{data: make(map[string][]byte)}
}

func (m *MockStore) GetValue(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.data[key]
	if !ok {
		return nil, nil // Return empty bytes to simulate not found or empty
	}
	return val, nil
}

func (m *MockStore) PutValue(ctx context.Context, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func TestAcquireLease_EmptyDHT(t *testing.T) {
	store := NewMockStore()
	job := &watchdog.RepairJob{FileID: "file1", MissingShards: []int{0}}

	won, err := watchdog.AcquireLease(context.Background(), store, job, genPeerID(), "manifest/v2")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !won {
		t.Fatalf("expected to win lease on empty DHT")
	}
}

func TestAcquireLease_ExistingLease(t *testing.T) {
	store := NewMockStore()
	job := &watchdog.RepairJob{FileID: "file1", MissingShards: []int{0}}

	peerA := genPeerID()
	peerB := genPeerID()

	// peerA acquires first
	wonA, _ := watchdog.AcquireLease(context.Background(), store, job, peerA, "manifest/v2")
	if !wonA {
		t.Fatalf("peerA should win")
	}

	// peerB tries to acquire
	wonB, _ := watchdog.AcquireLease(context.Background(), store, job, peerB, "manifest/v2")
	if wonB {
		t.Fatalf("peerB should NOT win an existing alive lease")
	}
}

func TestAcquireLease_ExpiredLease(t *testing.T) {
	store := NewMockStore()
	job := &watchdog.RepairJob{FileID: "file1", MissingShards: []int{0}}

	peerA := genPeerID()
	peerB := genPeerID()

	// Create an expired lease manually
	oldLease := &watchdog.RepairLease{
		FileID:    "file1",
		Epoch:     1,
		Owner:     peerA,
		ExpiresAt: time.Now().Add(-10 * time.Minute), // Expired 10 mins ago
		CreatedAt: time.Now().Add(-20 * time.Minute),
	}
	b, _ := oldLease.Serialize()
	store.PutValue(context.Background(), "/meshweb/lease/file1", b)

	// peerB tries to acquire
	wonB, err := watchdog.AcquireLease(context.Background(), store, job, peerB, "manifest/v2")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !wonB {
		t.Fatalf("peerB should win because peerA lease is expired")
	}

	// Verify epoch incremented
	val, _ := store.GetValue(context.Background(), "/meshweb/lease/file1")
	l, _ := watchdog.DeserializeLease(val)
	if l.Epoch != 2 {
		t.Fatalf("expected epoch 2, got %d", l.Epoch)
	}
}

func TestAcquireLease_EpochRace(t *testing.T) {
	peerA := genPeerID()
	peerB := genPeerID()

	a := &watchdog.RepairLease{Owner: peerA, Epoch: 10}
	b := &watchdog.RepairLease{Owner: peerB, Epoch: 11}

	if watchdog.CompareLeases(a, b) >= 0 {
		t.Fatalf("expected B (epoch 11) to win over A (epoch 10)")
	}
}

func TestAcquireLease_TieBreak(t *testing.T) {
	now := time.Now()
	peerA := genPeerID()
	peerB := genPeerID()

	// Epoch tie, CreatedAt earlier wins
	a := &watchdog.RepairLease{Owner: peerA, Epoch: 10, CreatedAt: now.Add(-1 * time.Second)}
	b := &watchdog.RepairLease{Owner: peerB, Epoch: 10, CreatedAt: now}
	if watchdog.CompareLeases(a, b) <= 0 {
		t.Fatalf("expected A to win because it was created earlier")
	}

	// Make peerA < peerB lexicographically to test the PeerID tie-break
	if string(peerA) > string(peerB) {
		peerA, peerB = peerB, peerA
	}

	// Epoch and CreatedAt tie, PeerID lexical order wins (smaller is better)
	c := &watchdog.RepairLease{Owner: peerA, Epoch: 10, CreatedAt: now}
	d := &watchdog.RepairLease{Owner: peerB, Epoch: 10, CreatedAt: now}
	if watchdog.CompareLeases(c, d) <= 0 {
		t.Fatalf("expected A to win because peerA < peerB lexicographically")
	}
}

func TestAcquireLease_Stress(t *testing.T) {
	store := NewMockStore()
	job := &watchdog.RepairJob{FileID: "stress_file", MissingShards: []int{1}}

	var wg sync.WaitGroup
	winners := 0
	var winnersMu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			pid := genPeerID()

			// Introduce some jitter to simulate network
			time.Sleep(time.Duration(idx) * time.Millisecond)

			won, err := watchdog.AcquireLease(context.Background(), store, job, pid, "manifest/v2")
			if err == nil && won {
				winnersMu.Lock()
				winners++
				winnersMu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if winners != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", winners)
	}
}
