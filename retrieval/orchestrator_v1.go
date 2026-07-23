package retrieval

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/erasure"
	"github.com/meshweb/meshweb-protocol/manifest"
)

const (
	maxShardSize   = int64(8 << 30)
	maxSegmentSize = int64(64 << 20)

	PenaltyTimeout        = 2
	PenaltyEOF            = 5
	PenaltyInvalidJSON    = 10
	PenaltyInvalidBase64  = 15
	PenaltyBounds         = 20
	PenaltyHashMismatch   = 50
	PenaltyForgedManifest = 100
)

type ProviderState struct {
	Info             peer.AddrInfo
	Score            int
	BlacklistedUntil time.Time
}

type Orchestrator struct {
	mu        sync.Mutex
	providers map[peer.ID]*ProviderState
}

func NewOrchestrator(infos []peer.AddrInfo) *Orchestrator {
	o := &Orchestrator{providers: make(map[peer.ID]*ProviderState)}
	for _, info := range infos {
		o.providers[info.ID] = &ProviderState{Info: info, Score: 0}
	}
	return o
}

func (o *Orchestrator) GetBestProvider() *peer.AddrInfo {
	o.mu.Lock()
	defer o.mu.Unlock()
	var best *ProviderState
	for _, p := range o.providers {
		if time.Now().Before(p.BlacklistedUntil) {
			continue
		}
		if best == nil || p.Score > best.Score {
			best = p
		}
	}
	if best == nil {
		return nil
	}
	return &best.Info
}

func (o *Orchestrator) ReportSuccess(id peer.ID) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if p, ok := o.providers[id]; ok {
		p.Score++
	}
}

func isResourceError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "provider error") || strings.Contains(err.Error(), "provider returned error")
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	errStr := err.Error()
	if strings.Contains(errStr, "stream reset") || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(errStr, "deadline exceeded") {
		return true
	}
	return false
}

func (o *Orchestrator) ReportError(id peer.ID, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	p, ok := o.providers[id]
	if !ok {
		return
	}

	errStr := strings.ToLower(err.Error())
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		p.Score -= PenaltyEOF
	} else if strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timeout") || isTransientError(err) {
		p.Score -= PenaltyTimeout
	} else if strings.Contains(errStr, "invalid json") {
		p.Score -= PenaltyInvalidJSON
		p.BlacklistedUntil = time.Now().Add(1 * time.Minute)
	} else if strings.Contains(errStr, "invalid base64") {
		p.Score -= PenaltyInvalidBase64
		p.BlacklistedUntil = time.Now().Add(2 * time.Minute)
	} else if strings.Contains(errStr, "invalid length") || strings.Contains(errStr, "bounds") || strings.Contains(errStr, "invalid chunk request") {
		p.Score -= PenaltyBounds
		p.BlacklistedUntil = time.Now().Add(5 * time.Minute)
	} else if strings.Contains(errStr, "verification failed") || strings.Contains(errStr, "hash mismatch") {
		p.Score -= PenaltyHashMismatch
		p.BlacklistedUntil = time.Now().Add(10 * time.Minute)
	} else {
		p.Score -= PenaltyForgedManifest
		p.BlacklistedUntil = time.Now().Add(1 * time.Hour)
	}
}

func RunV1(ctx context.Context, h host.Host, m *manifest.FileManifest, discovered []peer.AddrInfo, concurrency int, storeDir, outputPath string) (int, erasure.Stats, error) {
	t2 := time.Now()
	orch := NewOrchestrator(discovered)
	var lastProgress atomic.Value
	lastProgress.Store(time.Now())

	ctxCancel, cancelFetch := context.WithCancel(ctx)
	defer cancelFetch()

	go func() {
		for {
			select {
			case <-ctxCancel.Done():
				return
			case <-time.After(1 * time.Second):
				last := lastProgress.Load().(time.Time)
				if time.Since(last) > 15*time.Second {
					fmt.Printf("Progress timeout: no shards fetched for 15s. Aborting.\n")
					cancelFetch()
					return
				}
			}
		}
	}()

	var mu sync.Mutex
	verifiedUniqueShards := make(map[int]string)
	var successCount int32

	type fetchTask struct {
		shardIdx int
		attempts int
	}

	taskQueue := make(chan fetchTask, m.DataShards+m.ParityShards)
	for i := 0; i < m.DataShards+m.ParityShards; i++ {
		taskQueue <- fetchTask{shardIdx: i, attempts: 0}
	}

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctxCancel.Done():
					return
				case task := <-taskQueue:
					if atomic.LoadInt32(&successCount) >= int32(m.MinShards) {
						return
					}
					if task.attempts >= 5 { // max 5 attempts per shard
						continue
					}

					sIdx := task.shardIdx

					providerInfo := orch.GetBestProvider()
					if providerInfo == nil {
						cancelFetch()
						return
					}

					var foundData []byte
					err := func() error {
						sCtx, cnl := context.WithTimeout(ctxCancel, 10*time.Second)
						defer cnl()

						supportsV2, suppErr := h.Peerstore().SupportsProtocols(providerInfo.ID, "/meshweb/storage/2.0.0")
						useV2 := suppErr == nil && len(supportsV2) > 0

						var stream network.Stream
						var streamErr error
						if useV2 {
							stream, streamErr = h.NewStream(sCtx, providerInfo.ID, "/meshweb/storage/2.0.0")
						} else {
							stream, streamErr = h.NewStream(sCtx, providerInfo.ID, "/meshweb/storage/1.0.0")
						}
						if streamErr != nil {
							return streamErr
						}
						defer stream.Close()
						_ = stream.SetDeadline(time.Now().Add(30 * time.Second))

						if useV2 {
							req := struct {
								FileID string `json:"file_id"`
								Shard  int    `json:"shard"`
								Offset int64  `json:"offset"`
								Length int64  `json:"length"`
							}{FileID: m.FileID, Shard: sIdx, Offset: 0, Length: 0}
							reqBytes, _ := json.Marshal(req)
							stream.Write(append(reqBytes, '\n'))

							reader := bufio.NewReader(stream)
							line, err := reader.ReadBytes('\n')
							if err != nil {
								return err
							}
							var resp chunkResponse
							if err := json.Unmarshal(line, &resp); err != nil {
								return fmt.Errorf("invalid json: %w", err)
							}
							if resp.Error != "" {
								return fmt.Errorf("provider error: %s", resp.Error)
							}

							if resp.Length <= 0 || resp.Length > maxShardSize {
								return fmt.Errorf("provider returned invalid length")
							}

							length := resp.Length
							shardPath := filepath.Join(storeDir, fmt.Sprintf("shard.%02d", sIdx))
							f, err := os.OpenFile(shardPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
							if err != nil {
								return err
							}
							defer f.Close()

							written, err := io.CopyN(f, reader, length)
							if err != nil || written != length {
								return fmt.Errorf("failed to read full chunk: %v", err)
							}

							// Cryptographic validation
							f.Seek(0, 0)
							hash := sha256.New()
							io.Copy(hash, f)
							f.Close()
							computedHash := hex.EncodeToString(hash.Sum(nil))
							if computedHash != m.ShardHashes[sIdx] {
								os.Remove(shardPath) // Clean up partial shard on verification failure
								return fmt.Errorf("cryptographic verification failed for v2 stream")
							}

							foundData = []byte(shardPath) // placeholder to indicate success
							return nil
						} else {
							req := struct {
								FileID string `json:"file_id"`
								Shard  int    `json:"shard"`
							}{FileID: m.FileID, Shard: sIdx}

							if err := json.NewEncoder(stream).Encode(req); err != nil {
								return err
							}

							var resp struct {
								Data  string `json:"data,omitempty"`
								Error string `json:"error,omitempty"`
							}
							if err := json.NewDecoder(stream).Decode(&resp); err != nil {
								return fmt.Errorf("invalid json: %w", err)
							}

							if resp.Error != "" {
								return fmt.Errorf("provider returned error: %s", resp.Error)
							}
							if resp.Data == "" {
								return fmt.Errorf("provider returned empty data")
							}

							shardData, err := base64.StdEncoding.DecodeString(resp.Data)
							if err != nil {
								return fmt.Errorf("invalid base64: %w", err)
							}

							if int64(len(shardData)) <= 0 || int64(len(shardData)) > maxShardSize {
								return fmt.Errorf("provider returned invalid length")
							}

							computedHash := sha256.Sum256(shardData)
							if hex.EncodeToString(computedHash[:]) != m.ShardHashes[sIdx] {
								return fmt.Errorf("cryptographic verification failed for v1")
							}

							foundData = shardData
							return nil
						}
					}()

					if err == nil {
						mu.Lock()
						if _, exists := verifiedUniqueShards[sIdx]; !exists {
							shardPath := filepath.Join(storeDir, fmt.Sprintf("shard.%02d", sIdx))
							isV1 := string(foundData) != shardPath
							if isV1 {
								if writeErr := os.WriteFile(shardPath, foundData, 0o644); writeErr != nil {
									mu.Unlock()
									orch.ReportError(providerInfo.ID, writeErr)
									os.Remove(shardPath)
									go func(t fetchTask) {
										t.attempts++
										select {
										case taskQueue <- t:
										case <-ctxCancel.Done():
										}
									}(task)
									continue
								}
								verifiedUniqueShards[sIdx] = shardPath
							} else {
								verifiedUniqueShards[sIdx] = string(foundData)
							}
							atomic.AddInt32(&successCount, 1)
							lastProgress.Store(time.Now())
							orch.ReportSuccess(providerInfo.ID)
						}
						mu.Unlock()

						if atomic.LoadInt32(&successCount) >= int32(m.MinShards) {
							cancelFetch()
							return
						}
					} else {
						orch.ReportError(providerInfo.ID, err)
						go func(t fetchTask) {
							t.attempts++
							select {
							case taskQueue <- t:
							case <-ctxCancel.Done():
							}
						}(task)
					}
				}
			}
		}()
	}

	wg.Wait()

	if len(verifiedUniqueShards) < m.MinShards {
		return 0, erasure.Stats{}, fmt.Errorf("not enough shards fetched: got %d need %d", len(verifiedUniqueShards), m.MinShards)
	}
	fmt.Printf("[METRIC] Fetch Time: %v\n", time.Since(t2))
	fmt.Printf("Got %d shards, need %d\n", len(verifiedUniqueShards), m.MinShards)

	t3 := time.Now()

	fmt.Printf("Reconstructing to %s...\n", outputPath)
	tempManifest := filepath.Join(storeDir, "manifest.json")
	manifestData, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(tempManifest, manifestData, 0o644); err != nil {
		return 0, erasure.Stats{}, fmt.Errorf("failed to write temp manifest: %v", err)
	}

	presentIndices := make([]int, 0, len(verifiedUniqueShards))
	for idx := range verifiedUniqueShards {
		presentIndices = append(presentIndices, idx)
	}

	stats, err := manifest.ReconstructFromManifest(m, tempManifest, outputPath, presentIndices)
	if err != nil {
		return 0, erasure.Stats{}, fmt.Errorf("reconstruction failed: %v", err)
	}
	fmt.Printf("[METRIC] RS Reconstruction Time: %v\n", time.Since(t3))

	return len(verifiedUniqueShards), stats, nil
}
