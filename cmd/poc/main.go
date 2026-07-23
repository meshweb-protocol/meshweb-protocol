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
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/discovery"
	"github.com/meshweb/meshweb-protocol/erasure"
	"github.com/meshweb/meshweb-protocol/manifest"
	"github.com/multiformats/go-multiaddr"
)

func hashBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	outPath := `e:\MeshWeb\out_Test1_Normal.bin`
	fV1, err := os.Open(outPath)
	if err != nil {
		log.Fatalf("Failed to open V1 output: %v", err)
	}
	v1Data := make([]byte, 10*1024*1024)
	if _, err := io.ReadFull(fV1, v1Data); err != nil {
		log.Fatalf("Failed to read 10MB from V1 output: %v", err)
	}
	fV1.Close()
	hashV1 := hashBytes(v1Data)
	fmt.Printf("=== Test 1 (V1 Hash) ===\n")
	fmt.Printf("Segment 0 Hash (V1): %s\n\n", hashV1)

	m := &manifest.FileManifest{
		FileID:       "test_1gb.bin-49bc20df15e412a6",
		DataShards:   10,
		ParityShards: 20,
		BlockSize:    1024 * 1024,
	}

	bootstrapStr := flag.String("bootstrap", "", "bootstrap multiaddr")
	flag.Parse()
	if *bootstrapStr == "" {
		log.Fatalf("--bootstrap required")
	}

	bootstrapMaddr, err := multiaddr.NewMultiaddr(*bootstrapStr)
	if err != nil {
		log.Fatalf("Invalid bootstrap address: %v", err)
	}
	bootstrapInfo, err := peer.AddrInfoFromP2pAddr(bootstrapMaddr)
	if err != nil {
		log.Fatalf("Invalid bootstrap peer info: %v", err)
	}

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

	fetchChunk := func(sIdx int) ([]byte, error) {
		for _, p := range discovered {
			sCtx, cnl := context.WithTimeout(ctx, 5*time.Second)
			stream, err := h.NewStream(sCtx, p.ID, "/meshweb/storage/2.0.0")
			if err != nil {
				cnl()
				continue
			}
			req := map[string]interface{}{
				"file_id": m.FileID,
				"shard":   sIdx,
				"offset":  0,
				"length":  m.BlockSize,
			}
			reqBytes, _ := json.Marshal(req)
			stream.Write(append(reqBytes, '\n'))

			reader := bufio.NewReader(stream)
			line, err := reader.ReadBytes('\n')
			if err != nil {
				stream.Close()
				cnl()
				continue
			}
			var resp struct {
				Status string `json:"status"`
				Error  string `json:"error,omitempty"`
				Length int64  `json:"length"`
			}
			json.Unmarshal(line, &resp)
			if resp.Error != "" {
				stream.Close()
				cnl()
				continue
			}
			length := resp.Length
			chunk := make([]byte, length)
			io.ReadFull(reader, chunk)
			stream.Close()
			cnl()
			return chunk, nil
		}
		return nil, fmt.Errorf("failed to fetch chunk %d", sIdx)
	}

	fmt.Printf("\n=== Test 2 (V2 Hash) ===\n")
	shards := make([][]byte, m.DataShards+m.ParityShards)
	for i := 0; i < m.DataShards; i++ {
		chunk, err := fetchChunk(i)
		if err != nil {
			log.Fatalf("Failed to fetch chunk %d: %v", i, err)
		}
		shards[i] = chunk
		fmt.Printf("Fetched chunk %d\n", i)
	}

	var outBuf bytes.Buffer
	err = erasure.ReconstructSegment(m.DataShards, m.ParityShards, shards, &outBuf, m.DataShards*m.BlockSize)
	if err != nil {
		log.Fatalf("ReconstructSegment failed: %v", err)
	}
	hashV2 := hashBytes(outBuf.Bytes())
	fmt.Printf("Segment 0 Hash (V2): %s\n", hashV2)
	if hashV1 == hashV2 {
		fmt.Printf("Result: PASS (Hashes match!)\n")
	} else {
		fmt.Printf("Result: FAIL\n")
	}

	fmt.Printf("\n=== Test 3 (Parity Recovery) ===\n")
	shards[3] = nil
	shards[7] = nil
	fmt.Printf("Dropped chunks 3 and 7\n")

	chunk10, _ := fetchChunk(10)
	shards[10] = chunk10
	fmt.Printf("Fetched parity chunk 10\n")

	chunk11, _ := fetchChunk(11)
	shards[11] = chunk11
	fmt.Printf("Fetched parity chunk 11\n")

	var outBufParity bytes.Buffer
	err = erasure.ReconstructSegment(m.DataShards, m.ParityShards, shards, &outBufParity, m.DataShards*m.BlockSize)
	if err != nil {
		log.Fatalf("ReconstructSegment with parity failed: %v", err)
	}
	hashV2Parity := hashBytes(outBufParity.Bytes())
	fmt.Printf("Segment 0 Hash (Parity): %s\n", hashV2Parity)

	if hashV1 == hashV2Parity {
		fmt.Printf("Result: PASS (Parity recovery works! Hashes match!)\n")
	} else {
		fmt.Printf("Result: FAIL\n")
	}
}
