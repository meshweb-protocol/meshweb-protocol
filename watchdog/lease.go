package watchdog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type RepairLease struct {
	FileID     string    `json:"file_id"`
	Epoch      uint64    `json:"epoch"`
	Owner      peer.ID   `json:"owner"`
	IntentHash []byte    `json:"intent_hash"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (l *RepairLease) Serialize() ([]byte, error) {
	return json.Marshal(l)
}

func DeserializeLease(data []byte) (*RepairLease, error) {
	var l RepairLease
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func CalculateIntentHash(fileID string, missingShards []int, manifestVersion string) []byte {
	h := sha256.New()
	h.Write([]byte(fileID))
	for _, s := range missingShards {
		// Just simple serialization for hash
		h.Write([]byte{byte(s >> 24), byte(s >> 16), byte(s >> 8), byte(s)})
	}
	h.Write([]byte(manifestVersion))
	return h.Sum(nil)
}

// CompareLeases returns > 0 if A wins, < 0 if B wins, 0 if equal.
// Tie-break: Epoch -> Earlier CreatedAt -> Lexical PeerID
func CompareLeases(a, b *RepairLease) int {
	if a.Epoch > b.Epoch {
		return 1
	}
	if a.Epoch < b.Epoch {
		return -1
	}

	// Tie-break by CreatedAt (Earlier wins)
	if a.CreatedAt.Before(b.CreatedAt) {
		return 1
	}
	if a.CreatedAt.After(b.CreatedAt) {
		return -1
	}

	// Tie-break by PeerID (Lexical order, smaller wins)
	if a.Owner < b.Owner {
		return 1
	}
	if a.Owner > b.Owner {
		return -1
	}

	return 0
}

type LeaseStore interface {
	GetValue(ctx context.Context, key string) ([]byte, error)
	PutValue(ctx context.Context, key string, value []byte) error
}

func AcquireLease(ctx context.Context, store LeaseStore, job *RepairJob, self peer.ID, manifestVersion string) (bool, error) {
	intentHash := CalculateIntentHash(job.FileID, job.MissingShards, manifestVersion)
	key := "/meshweb/lease/" + job.FileID

	now := time.Now()
	var newEpoch uint64 = 1

	existingBytes, err := store.GetValue(ctx, key)
	if err == nil && len(existingBytes) > 0 {
		existingLease, err := DeserializeLease(existingBytes)
		if err == nil {
			// Check if existing lease is still valid
			if existingLease.ExpiresAt.After(now) {
				// Alive lease
				// Is it us?
				if existingLease.Owner == self {
					// We already own it, renew it
					newEpoch = existingLease.Epoch + 1
				} else {
					// Someone else owns it and it's not expired. We lost.
					return false, nil
				}
			} else {
				// Expired lease, we can take over
				newEpoch = existingLease.Epoch + 1
			}
		}
	}

	newLease := &RepairLease{
		FileID:     job.FileID,
		Epoch:      newEpoch,
		Owner:      self,
		IntentHash: intentHash,
		ExpiresAt:  now.Add(10 * time.Minute),
		CreatedAt:  now,
	}

	leaseBytes, err := newLease.Serialize()
	if err != nil {
		return false, err
	}

	// Put our lease
	if err := store.PutValue(ctx, key, leaseBytes); err != nil {
		return false, err
	}

	// Read back to arbitrate
	// Give the DHT a tiny bit of time to propagate before reading back (simulating race)
	time.Sleep(100 * time.Millisecond)

	finalBytes, err := store.GetValue(ctx, key)
	if err != nil {
		// If we can't read it back, assume lost for safety
		return false, err
	}

	finalLease, err := DeserializeLease(finalBytes)
	if err != nil {
		return false, err
	}

	winner := CompareLeases(newLease, finalLease)
	if winner >= 0 {
		// We won or we are exactly the final lease
		return true, nil
	}

	return false, nil
}
