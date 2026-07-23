package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/discovery"
	"github.com/meshweb/meshweb-protocol/erasure"
	"github.com/meshweb/meshweb-protocol/manifest"
	"github.com/multiformats/go-multiaddr"
)

var chunkPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 1024*1024)
		return &buf
	},
}

func hashBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

type SegmentResult struct {
	Idx  int
	Data []byte
	Hash string
}

type Checkpoint struct {
	Version      int    `json:"version"`
	FileID       string `json:"file_id"`
	LastSegment  int    `json:"last_segment"`
	BytesWritten int64  `json:"bytes_written"`
	UpdatedAt    int64  `json:"updated_at"`
}

func fetchChunk(ctx context.Context, h host.Host, p peer.AddrInfo, fileID string, sIdx, offset, length int) ([]byte, error) {
	sCtx, cnl := context.WithTimeout(ctx, 5*time.Second)
	defer cnl()

	stream, err := h.NewStream(sCtx, p.ID, "/meshweb/storage/2.0.0")
	if err != nil {
		return nil, err
	}
	defer stream.Close()

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

	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
		Length int    `json:"length"`
	}
	json.Unmarshal(line, &resp)
	if resp.Error != "" {
		return nil, fmt.Errorf("provider error: %s", resp.Error)
	}

	retLen := resp.Length

	bufPtr := chunkPool.Get().(*[]byte)
	buf := *bufPtr
	if cap(buf) < retLen {
		buf = make([]byte, retLen)
	}
	chunk := buf[:retLen]

	if _, err := io.ReadFull(reader, chunk); err != nil {
		chunkPool.Put(bufPtr)
		return nil, err
	}

	return chunk, nil
}

func main() {
	bootstrapStr := flag.String("bootstrap", "", "bootstrap multiaddr")
	windowSize := flag.Int("window", 1, "pipeline window size")
	flag.Parse()

	if *bootstrapStr == "" {
		log.Fatalf("--bootstrap required")
	}

	go func() {
		for {
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Printf("[RAM] Alloc: %v MB, Sys: %v MB, NumGC: %v\n", m.Alloc/1024/1024, m.Sys/1024/1024, m.NumGC)
			time.Sleep(2 * time.Second)
		}
	}()

	m := &manifest.FileManifest{
		FileID:       "test_1gb.bin-49bc20df15e412a6",
		FileSize:     1073741824,
		DataShards:   10,
		ParityShards: 20,
		BlockSize:    1024 * 1024,
	}

	bootstrapMaddr, _ := multiaddr.NewMultiaddr(*bootstrapStr)
	bootstrapInfo, _ := peer.AddrInfoFromP2pAddr(bootstrapMaddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h, kad, err := discovery.NewDHTHost(ctx, []string{"/ip4/127.0.0.1/tcp/0"}, *bootstrapInfo)
	if err != nil {
		log.Fatalf("Failed to create client host: %v", err)
	}
	defer h.Close()

	if err := discovery.BootstrapDHT(ctx, h, kad, []peer.AddrInfo{*bootstrapInfo}); err != nil {
		fmt.Printf("Warning: bootstrap failed: %v\n", err)
	}
	time.Sleep(500 * time.Millisecond)

	discovered, err := discovery.FindProviderPeers(ctx, kad, m.FileID, m.DataShards+m.ParityShards)
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}
	fmt.Printf("Discovered %d providers\n", len(discovered))

	shardSize := (m.FileSize + int64(m.DataShards) - 1) / int64(m.DataShards)
	totalSegments := int((shardSize + int64(m.BlockSize) - 1) / int64(m.BlockSize))

	fmt.Printf("Total Segments: %d, Window Size: %d\n", totalSegments, *windowSize)

	windowSem := make(chan struct{}, *windowSize)
	results := make(chan *SegmentResult, *windowSize)
	var activeSegments sync.WaitGroup
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	outPath := `e:\MeshWeb\out_V2_Pipeline.bin`
	ckptPath := `e:\MeshWeb\checkpoint.json`

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

	outFile, err := os.OpenFile(outPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("Failed to open out file: %v", err)
	}

	if startSegment > 0 {
		_, err = outFile.Seek(totalWritten, 0)
		if err != nil {
			log.Fatalf("Failed to seek out file: %v", err)
		}
	} else {
		outFile.Truncate(0)
	}

	go func() {
		defer writerWg.Done()
		defer outFile.Close()
		expectedSegment := startSegment
		buffer := make(map[int]*SegmentResult)

		for res := range results {
			buffer[res.Idx] = res

			for {
				if ready, ok := buffer[expectedSegment]; ok {
					fmt.Printf("Write Segment %d (Hash: %s)\n", ready.Idx, ready.Hash)
					n, err := outFile.Write(ready.Data)
					if err != nil {
						log.Fatalf("Write failed: %v", err)
					}
					totalWritten += int64(n)

					if expectedSegment%10 == 0 {
						ckpt := Checkpoint{
							Version:      1,
							FileID:       m.FileID,
							LastSegment:  expectedSegment,
							BytesWritten: totalWritten,
							UpdatedAt:    time.Now().Unix(),
						}
						b, _ := json.MarshalIndent(ckpt, "", "  ")
						os.WriteFile(ckptPath, b, 0644)
						fmt.Printf("Saved checkpoint at segment %d\n", expectedSegment)
					}

					delete(buffer, expectedSegment)
					expectedSegment++
				} else {
					break
				}
			}
		}

		if expectedSegment != totalSegments {
			log.Fatalf("DecodeBarrier failed! Expected %d segments, got %d", totalSegments, expectedSegment)
		}
	}()

	// Start Producer Loop
	for seg := startSegment; seg < totalSegments; seg++ {
		windowSem <- struct{}{}
		activeSegments.Add(1)

		go func(segIdx int) {
			defer activeSegments.Done()
			defer func() { <-windowSem }()

			fmt.Printf("Fetch Segment %d\n", segIdx)

			offset := int64(segIdx) * int64(m.BlockSize)
			length := int64(m.BlockSize)
			if offset+length > shardSize {
				length = shardSize - offset
			}

			shards := make([][]byte, m.DataShards+m.ParityShards)
			var got int32
			var wg sync.WaitGroup

			for i := 0; i < m.DataShards; i++ {
				wg.Add(1)
				go func(shardIdx int) {
					defer wg.Done()
					for _, p := range discovered {
						chunk, err := fetchChunk(ctx, h, p, m.FileID, shardIdx, int(offset), int(length))
						if err == nil {
							shards[shardIdx] = chunk
							atomic.AddInt32(&got, 1)
							break
						}
					}
				}(i)
			}
			wg.Wait()

			if got < int32(m.DataShards) {
				fmt.Printf("Segment %d: missing data shards, attempting parity...\n", segIdx)
				for i := m.DataShards; i < m.DataShards+m.ParityShards; i++ {
					if got >= int32(m.DataShards) {
						break
					}
					for _, p := range discovered {
						chunk, err := fetchChunk(ctx, h, p, m.FileID, i, int(offset), int(length))
						if err == nil {
							shards[i] = chunk
							atomic.AddInt32(&got, 1)
							break
						}
					}
				}
			}

			if got < int32(m.DataShards) {
				log.Fatalf("Segment %d: failed to fetch minimum shards (got %d)", segIdx, got)
			}

			fmt.Printf("Decode Segment %d\n", segIdx)
			stripeLen := int(length) * m.DataShards
			maxWrite := int(m.FileSize - int64(segIdx)*int64(m.BlockSize)*int64(m.DataShards))
			if stripeLen > maxWrite {
				stripeLen = maxWrite
			}

			var outBuf bytes.Buffer
			err := erasure.ReconstructSegmentData(m.DataShards, m.ParityShards, shards, &outBuf, stripeLen)
			if err != nil {
				log.Fatalf("Segment %d decode failed: %v", segIdx, err)
			}

			for _, chunk := range shards {
				if chunk != nil {
					if cap(chunk) >= 1024*1024 {
						c := chunk[:cap(chunk)]
						chunkPool.Put(&c)
					}
				}
			}

			segData := outBuf.Bytes()
			segHash := hashBytes(segData)

			results <- &SegmentResult{
				Idx:  segIdx,
				Data: segData,
				Hash: segHash,
			}
		}(seg)
	}

	go func() {
		activeSegments.Wait()
		close(results)
	}()

	writerWg.Wait()

	fmt.Printf("\n=== Final Validation ===\n")
	v2Hash := hashBytesFile(outPath)
	v1Hash := hashBytesFile(`e:\MeshWeb\out_Test1_Normal.bin`)

	fmt.Printf("V1 SHA256: %s\n", v1Hash)
	fmt.Printf("V2 SHA256: %s\n", v2Hash)

	if v1Hash == v2Hash {
		fmt.Printf("Result: PASS (Hashes match exactly)\n")
	} else {
		fmt.Printf("Result: FAIL\n")
	}
}

func hashBytesFile(path string) string {
	f, _ := os.Open(path)
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}
