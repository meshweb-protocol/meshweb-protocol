package retrieval

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/erasure"
	"github.com/meshweb/meshweb-protocol/manifest"
)

var chunkPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 1024*1024)
		return &buf
	},
}

type Checkpoint struct {
	Version      int    `json:"version"`
	FileID       string `json:"file_id"`
	LastSegment  int    `json:"last_segment"`
	BytesWritten int64  `json:"bytes_written"`
	UpdatedAt    int64  `json:"updated_at"`
}

type SegmentResult struct {
	Idx  int
	Data []byte
}

type chunkResponse struct {
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	FileID         string `json:"file_id"`
	Shard          int    `json:"shard"`
	Offset         int64  `json:"offset"`
	Length         int64  `json:"length"`
	TotalShardSize int64  `json:"total_shard_size"`
}

func fetchChunk(ctx context.Context, h host.Host, p peer.ID, fileID string, sIdx, offset, length int) ([]byte, error) {
	if length <= 0 || int64(length) > maxSegmentSize || offset < 0 || sIdx < 0 {
		return nil, fmt.Errorf("invalid chunk request")
	}
	sCtx, cnl := context.WithTimeout(ctx, 10*time.Second)
	defer cnl()

	stream, err := h.NewStream(sCtx, p, "/meshweb/storage/2.0.0")
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(30 * time.Second))

	req := map[string]interface{}{
		"file_id": fileID,
		"shard":   sIdx,
		"offset":  offset,
		"length":  length,
	}
	reqBytes, _ := json.Marshal(req)
	stream.Write(append(reqBytes, '\n'))

	reader := bufio.NewReader(stream)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var resp chunkResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("invalid provider response: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("provider error: %s", resp.Error)
	}
	if resp.Status != "ok" || resp.FileID != fileID || resp.Shard != sIdx || resp.Offset != int64(offset) || resp.Length != int64(length) || resp.TotalShardSize < int64(offset)+resp.Length {
		return nil, fmt.Errorf("invalid provider response metadata")
	}

	bufPtr := chunkPool.Get().(*[]byte)
	buf := *bufPtr
	if cap(buf) < length {
		buf = make([]byte, length)
	}
	chunk := buf[:length]

	if _, err := io.ReadFull(reader, chunk); err != nil {
		chunkPool.Put(bufPtr)
		return nil, err
	}

	return chunk, nil
}

func verifyShardData(data []byte, expectedHash string) bool {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]) == expectedHash
}

func RunV2(ctx context.Context, h host.Host, m *manifest.FileManifest, discovered []peer.AddrInfo, windowSize int, storeDir, outputPath string) (int, erasure.Stats, error) {
	if err := m.Validate(); err != nil {
		return 0, erasure.Stats{}, fmt.Errorf("invalid manifest: %w", err)
	}
	if windowSize <= 0 {
		return 0, erasure.Stats{}, fmt.Errorf("window size must be positive")
	}
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
				if time.Since(last) > 30*time.Second {
					fmt.Printf("Progress timeout: no chunks fetched for 30s. Aborting.\n")
					cancelFetch()
					return
				}
			}
		}
	}()

	shardSize := (m.FileSize + int64(m.DataShards) - 1) / int64(m.DataShards)
	totalSegments := int((shardSize + int64(m.BlockSize) - 1) / int64(m.BlockSize))

	windowSem := make(chan struct{}, windowSize)
	results := make(chan *SegmentResult, windowSize)
	var activeSegments sync.WaitGroup
	var writerWg sync.WaitGroup

	writerWg.Add(1)
	ckptPath := filepath.Join(storeDir, m.FileID+".checkpoint")

	startSegment := 0
	var totalWritten int64 = 0

	// Check if checkpoint exists
	if b, err := os.ReadFile(ckptPath); err == nil {
		var ckpt Checkpoint
		if err := json.Unmarshal(b, &ckpt); err == nil {
			if ckpt.FileID == m.FileID && ckpt.LastSegment >= 0 {
				startSegment = ckpt.LastSegment + 1
				totalWritten = ckpt.BytesWritten
				fmt.Printf("Loaded checkpoint! Resuming from segment %d (Bytes: %d)\n", startSegment, totalWritten)
			}
		}
	}

	outFile, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return 0, erasure.Stats{}, fmt.Errorf("failed to open out file: %v", err)
	}

	if startSegment > 0 {
		_, err = outFile.Seek(totalWritten, 0)
		if err != nil {
			outFile.Close()
			return 0, erasure.Stats{}, fmt.Errorf("failed to seek out file: %v", err)
		}
	} else {
		outFile.Truncate(0)
	}

	var writeErr atomic.Value

	go func() {
		defer writerWg.Done()
		defer outFile.Close()
		expectedSegment := startSegment
		buffer := make(map[int]*SegmentResult)

		for res := range results {
			buffer[res.Idx] = res

			for {
				if ready, ok := buffer[expectedSegment]; ok {
					n, err := outFile.Write(ready.Data)
					if err != nil {
						writeErr.Store(err)
						cancelFetch()
						return
					}
					totalWritten += int64(n)

					if expectedSegment%10 == 0 || expectedSegment == totalSegments-1 {
						ckpt := Checkpoint{
							Version:      1,
							FileID:       m.FileID,
							LastSegment:  expectedSegment,
							BytesWritten: totalWritten,
							UpdatedAt:    time.Now().Unix(),
						}
						b, _ := json.MarshalIndent(ckpt, "", "  ")
						os.WriteFile(ckptPath, b, 0644)
					}

					delete(buffer, expectedSegment)
					expectedSegment++
				} else {
					break
				}
			}
		}

		if expectedSegment != totalSegments {
			if writeErr.Load() == nil {
				writeErr.Store(fmt.Errorf("DecodeBarrier failed! Expected %d segments, got %d", totalSegments, expectedSegment))
			}
		}
	}()

	var fetchedShards int32

	for seg := startSegment; seg < totalSegments; seg++ {
		select {
		case windowSem <- struct{}{}:
		case <-ctxCancel.Done():
			break
		}

		if ctxCancel.Err() != nil {
			break
		}

		activeSegments.Add(1)

		go func(segIdx int) {
			defer activeSegments.Done()
			defer func() { <-windowSem }()

			offset := int64(segIdx) * int64(m.BlockSize)
			length := int64(m.BlockSize)
			if offset+length > shardSize {
				length = shardSize - offset
			}

			type fetchTask struct {
				shardIdx int
				attempts int
			}

			taskQueue := make(chan fetchTask, m.DataShards+m.ParityShards)
			for i := 0; i < m.DataShards+m.ParityShards; i++ {
				taskQueue <- fetchTask{shardIdx: i, attempts: 0}
			}

			shards := make([][]byte, m.DataShards+m.ParityShards)
			var got int32
			var fetchWg sync.WaitGroup

			for i := 0; i < m.DataShards; i++ {
				fetchWg.Add(1)
				go func() {
					defer fetchWg.Done()
					for {
						if atomic.LoadInt32(&got) >= int32(m.DataShards) {
							return
						}

						var task fetchTask
						select {
						case task = <-taskQueue:
						case <-ctxCancel.Done():
							return
						default:
							return
						}

						if task.attempts >= 5 {
							continue // drop this shard, let RS reconstruction try with others
						}
						sIdx := task.shardIdx

						bestProvider := orch.GetBestProvider()
						if bestProvider == nil {
							time.Sleep(500 * time.Millisecond)
							taskQueue <- task
							continue
						}

						chunk, err := fetchChunk(ctxCancel, h, bestProvider.ID, m.FileID, sIdx, int(offset), int(length))
						if err == nil {
							shards[sIdx] = chunk
							newGot := atomic.AddInt32(&got, 1)
							orch.ReportSuccess(bestProvider.ID)
							lastProgress.Store(time.Now())
							atomic.AddInt32(&fetchedShards, 1)
							if newGot >= int32(m.DataShards) {
								return
							}
						} else {
							orch.ReportError(bestProvider.ID, err)
							go func(t fetchTask) {
								t.attempts++
								select {
								case taskQueue <- t:
								case <-ctxCancel.Done():
								}
							}(task)
						}
					}
				}()
			}
			fetchWg.Wait()

			if got < int32(m.DataShards) {
				if ctxCancel.Err() == nil {
					writeErr.Store(fmt.Errorf("Segment %d: failed to fetch minimum shards", segIdx))
					cancelFetch()
				}
				return
			}

			stripeLen := int(length) * m.DataShards
			maxWrite := int(m.FileSize - int64(segIdx)*int64(m.BlockSize)*int64(m.DataShards))
			if stripeLen > maxWrite {
				stripeLen = maxWrite
			}

			var outBuf bytes.Buffer
			err := erasure.ReconstructSegmentData(m.DataShards, m.ParityShards, shards, &outBuf, stripeLen)
			if err != nil {
				writeErr.Store(fmt.Errorf("Segment %d decode failed: %v", segIdx, err))
				cancelFetch()
				return
			}

			for _, chunk := range shards {
				if chunk != nil {
					if cap(chunk) >= 1024*1024 {
						c := chunk[:cap(chunk)]
						chunkPool.Put(&c)
					}
				}
			}

			select {
			case results <- &SegmentResult{Idx: segIdx, Data: outBuf.Bytes()}:
			case <-ctxCancel.Done():
			}
		}(seg)
	}

	go func() {
		activeSegments.Wait()
		close(results)
	}()

	writerWg.Wait()

	if err := writeErr.Load(); err != nil {
		return 0, erasure.Stats{}, err.(error)
	}

	// Final File Verification for V2
	f, err := os.Open(outputPath)
	if err == nil {
		hash := sha256.New()
		io.Copy(hash, f)
		f.Close()
		computedHash := hex.EncodeToString(hash.Sum(nil))
		if computedHash != m.Sha256 {
			os.Remove(outputPath)
			return 0, erasure.Stats{}, fmt.Errorf("cryptographic verification failed: reconstructed file hash mismatch")
		}
	} else {
		return 0, erasure.Stats{}, fmt.Errorf("failed to open reconstructed file for verification: %v", err)
	}

	fmt.Printf("[METRIC] Fetch+Decode+Write Time: %v\n", time.Since(t2))

	// Stats
	stats := erasure.Stats{
		ShardSize: int64(shardSize),
	}

	return int(fetchedShards), stats, nil
}
