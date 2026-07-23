package watchdog_test

import (
	"reflect"
	"testing"

	"github.com/meshweb/meshweb-protocol/watchdog"
)

func encodeShards(shards []int) []byte {
	maxShard := -1
	for _, idx := range shards {
		if idx > maxShard {
			maxShard = idx
		}
	}
	if maxShard == -1 {
		return []byte{}
	}
	numBytes := (maxShard / 8) + 1
	bitmap := make([]byte, numBytes)
	for _, idx := range shards {
		bitmap[idx/8] |= (1 << (idx % 8))
	}
	return bitmap
}

func TestBuildNetworkState_Perfect(t *testing.T) {
	totalShards := 30
	var shards []int
	for i := 0; i < totalShards; i++ {
		shards = append(shards, i)
	}

	bm1 := encodeShards(shards)

	state := watchdog.BuildNetworkState("file123", totalShards, [][]byte{bm1})

	if len(state.MissingShards) != 0 {
		t.Fatalf("expected 0 missing, got %d", len(state.MissingShards))
	}
	if len(state.AvailableShards) != 30 {
		t.Fatalf("expected 30 available, got %d", len(state.AvailableShards))
	}

	job := watchdog.CreateRepairJob(state)
	if job != nil {
		t.Fatalf("expected nil repair job for perfect state, got %v", job)
	}
}

func TestBuildNetworkState_OneMissing(t *testing.T) {
	totalShards := 30
	var shards []int
	for i := 0; i < totalShards; i++ {
		if i != 7 { // missing shard 7
			shards = append(shards, i)
		}
	}

	bm1 := encodeShards(shards)

	state := watchdog.BuildNetworkState("file123", totalShards, [][]byte{bm1})

	if len(state.MissingShards) != 1 || state.MissingShards[0] != 7 {
		t.Fatalf("expected missing shard 7, got %v", state.MissingShards)
	}

	job := watchdog.CreateRepairJob(state)
	if job == nil {
		t.Fatalf("expected repair job, got nil")
	}
	if job.Priority != 1 {
		t.Fatalf("expected priority 1, got %d", job.Priority)
	}
}

func TestBuildNetworkState_MultiPeer(t *testing.T) {
	totalShards := 9
	bmA := encodeShards([]int{0, 1, 2})
	bmB := encodeShards([]int{3, 4, 5})
	bmC := encodeShards([]int{6, 7, 8})

	state := watchdog.BuildNetworkState("file123", totalShards, [][]byte{bmA, bmB, bmC})

	if len(state.MissingShards) != 0 {
		t.Fatalf("expected 0 missing, got %v", state.MissingShards)
	}
	if len(state.AvailableShards) != 9 {
		t.Fatalf("expected 9 available, got %v", state.AvailableShards)
	}
	expectedAvailable := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	if !reflect.DeepEqual(state.AvailableShards, expectedAvailable) {
		t.Fatalf("expected available %v, got %v", expectedAvailable, state.AvailableShards)
	}
}

func TestBuildNetworkState_DuplicateShards(t *testing.T) {
	totalShards := 4
	bmA := encodeShards([]int{1, 2, 3})
	bmB := encodeShards([]int{1, 2, 3})

	state := watchdog.BuildNetworkState("file123", totalShards, [][]byte{bmA, bmB})

	if state.ReplicaCount[1] != 2 {
		t.Errorf("expected 2 replicas for shard 1, got %d", state.ReplicaCount[1])
	}
	if state.ReplicaCount[2] != 2 {
		t.Errorf("expected 2 replicas for shard 2, got %d", state.ReplicaCount[2])
	}
	if state.ReplicaCount[3] != 2 {
		t.Errorf("expected 2 replicas for shard 3, got %d", state.ReplicaCount[3])
	}
	if state.ReplicaCount[0] != 0 {
		t.Errorf("expected 0 replicas for shard 0, got %d", state.ReplicaCount[0])
	}
}
