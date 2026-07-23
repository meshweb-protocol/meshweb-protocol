package node_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/node"
)

func TestStreamDeadlockRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Create 10 malicious peers that accept the stream but never write back.
	maliciousPeers := make([]peer.AddrInfo, 10)
	for i := 0; i < 10; i++ {
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()

		h.SetStreamHandler("/meshweb/storage/1.0.0", func(s network.Stream) {
			// Malicious behavior: read request but never respond, hanging the connection.
			var req struct {
				FileID string `json:"file_id"`
				Shard  int    `json:"shard"`
			}
			_ = json.NewDecoder(s).Decode(&req)
			// Wait forever...
			<-ctx.Done()
			s.Close()
		})
		maliciousPeers[i] = peer.AddrInfo{ID: h.ID(), Addrs: h.Addrs()}
	}

	// 2. Create the client node
	client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	for _, p := range maliciousPeers {
		if err := client.Connect(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	// 3. Run simulated retriever fetch loop
	sem := make(chan struct{}, 10) // Max 10 concurrent
	var wg sync.WaitGroup
	var timeoutCount int32

	startTime := time.Now()

	// We try to fetch 10 shards. All will hit the malicious peers.
	for shardIdx := 0; shardIdx < 10; shardIdx++ {
		wg.Add(1)
		go func(sIdx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Attempt to fetch from one malicious peer
			p := maliciousPeers[sIdx]

			err := node.WithRetry(ctx, 1, 1*time.Millisecond, func() error {
				sCtx, cnl := context.WithTimeout(ctx, 2*time.Second) // shorter timeout for test
				defer cnl()

				stream, err := client.NewStream(sCtx, p.ID, "/meshweb/storage/1.0.0")
				if err != nil {
					return err
				}
				defer stream.Close()

				// Critical part: ensure SetDeadline prevents the hang
				_ = stream.SetDeadline(time.Now().Add(2 * time.Second))

				req := struct {
					FileID string `json:"file_id"`
					Shard  int    `json:"shard"`
				}{FileID: "testfile", Shard: sIdx}

				if err := json.NewEncoder(stream).Encode(req); err != nil {
					return err
				}

				var resp struct {
					Data  string `json:"data,omitempty"`
					Error string `json:"error,omitempty"`
				}
				// This should timeout instead of hanging!
				if err := json.NewDecoder(stream).Decode(&resp); err != nil {
					return err
				}
				return nil
			})

			if err != nil {
				atomic.AddInt32(&timeoutCount, 1)
			}
		}(shardIdx)
	}

	wg.Wait()
	duration := time.Since(startTime)

	if timeoutCount != 10 {
		t.Fatalf("expected 10 timeouts, got %d", timeoutCount)
	}
	if duration > 10*time.Second {
		t.Fatalf("test took too long (%s), streams likely leaked/hung", duration)
	}

	// If we reach here, the semaphore recovered completely because wg.Wait() returned,
	// and the duration proves it didn't hang indefinitely.
}
