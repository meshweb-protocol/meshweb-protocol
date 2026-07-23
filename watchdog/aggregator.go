package watchdog

import (
	"time"
)

type NetworkState struct {
	FileID          string
	Providers       int
	AvailableShards []int
	MissingShards   []int
	ReplicaCount    map[int]int
}

type RepairJob struct {
	FileID        string
	MissingShards []int
	ReplicaCount  map[int]int
	FirstSeen     time.Time
	LastSeen      time.Time
	Attempts      int
	Priority      int
}

func BuildNetworkState(fileID string, totalShards int, bitmaps [][]byte) *NetworkState {
	replicaCount := make(map[int]int)

	// Decode all bitmaps and count replicas
	for _, bm := range bitmaps {
		shards := DecodeBitmap(bm)
		for _, idx := range shards {
			replicaCount[idx]++
		}
	}

	var available []int
	var missing []int

	for i := 0; i < totalShards; i++ {
		count := replicaCount[i]
		if count > 0 {
			available = append(available, i)
		} else {
			missing = append(missing, i)
		}
	}

	return &NetworkState{
		FileID:          fileID,
		Providers:       len(bitmaps),
		AvailableShards: available,
		MissingShards:   missing,
		ReplicaCount:    replicaCount,
	}
}

func CreateRepairJob(state *NetworkState) *RepairJob {
	if len(state.MissingShards) == 0 {
		return nil
	}

	now := time.Now()

	priority := len(state.MissingShards)

	// Add replica deficit logic to priority here later
	// Example: Shards with low replica count increase priority

	return &RepairJob{
		FileID:        state.FileID,
		MissingShards: state.MissingShards,
		ReplicaCount:  state.ReplicaCount,
		FirstSeen:     now,
		LastSeen:      now,
		Attempts:      0,
		Priority:      priority,
	}
}
