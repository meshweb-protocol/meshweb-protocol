package erasure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/klauspost/reedsolomon"
)

type RepairResult struct {
	FileID   string
	Repaired []int
	Failed   []int
	Duration time.Duration
}

// RepairShards reconstructs the missing shards in memory, verifies their SHA256 hashes against the manifest,
// and returns the reconstructed bytes. It does not write to disk.
func RepairShards(fileID string, dataShards, parityShards, blockSize int, origSize int64, shardHashes []string, missingIndices []int, sourcePaths map[int]string) (*RepairResult, map[int][]byte, error) {
	start := time.Now()
	res := &RepairResult{
		FileID:   fileID,
		Repaired: make([]int, 0),
		Failed:   make([]int, 0),
	}

	total := dataShards + parityShards
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return res, nil, err
	}

	// Calculate shard size based on original size
	stripeSize := int64(dataShards) * int64(blockSize)
	numStripes := (origSize + stripeSize - 1) / stripeSize
	shardSize := numStripes * int64(blockSize)

	// Prepare output buffers in memory for missing shards
	outBufs := make(map[int]*bytes.Buffer)
	for _, idx := range missingIndices {
		outBufs[idx] = bytes.NewBuffer(make([]byte, 0, shardSize))
	}

	// Open available source shards
	files := make([]*os.File, total)
	for i := 0; i < total; i++ {
		if p, ok := sourcePaths[i]; ok {
			f, err := os.Open(p)
			if err != nil {
				return res, nil, err
			}
			files[i] = f
			defer f.Close()
		}
	}

	remaining := origSize
	for remaining > 0 {
		stripeLen := stripeSize
		if remaining < stripeLen {
			stripeLen = remaining
		}

		readSize := int(blockSize)
		if remaining < stripeSize {
			readSize = int((remaining + int64(dataShards) - 1) / int64(dataShards))
		}

		shards := make([][]byte, total)
		for i := 0; i < total; i++ {
			if files[i] == nil {
				shards[i] = nil
				continue
			}

			buf := make([]byte, readSize)
			n, err := io.ReadFull(files[i], buf)
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				buf = buf[:n]
			} else if err != nil {
				return res, nil, err
			}

			// reedsolomon.ReconstructSome requires all non-nil shards to have the same capacity/length
			// If buf is somehow shorter (shouldn't happen for valid shards), pad it
			if len(buf) < readSize {
				padded := make([]byte, readSize)
				copy(padded, buf)
				buf = padded
			}
			shards[i] = buf
		}

		needReconstruct := make([]bool, total)
		for _, idx := range missingIndices {
			needReconstruct[idx] = true
			shards[idx] = make([]byte, 0, readSize) // Zero-length, but with capacity
		}

		if err := enc.ReconstructSome(shards, needReconstruct); err != nil {
			return res, nil, err
		}

		for _, idx := range missingIndices {
			outBufs[idx].Write(shards[idx])
		}

		remaining -= stripeLen
	}

	reconstructed := make(map[int][]byte)
	// Verification
	for _, idx := range missingIndices {
		data := outBufs[idx].Bytes()

		// Some padding might exceed the exact shardSize if we padded the last block,
		// but actually shard generation in EncodeFile flushes the whole padded shard.
		// Let's verify hash
		h := sha256.New()
		h.Write(data)
		hashStr := hex.EncodeToString(h.Sum(nil))

		if idx < len(shardHashes) && hashStr == shardHashes[idx] {
			res.Repaired = append(res.Repaired, idx)
			reconstructed[idx] = data
		} else {
			res.Failed = append(res.Failed, idx)
		}
	}

	res.Duration = time.Since(start)

	if len(res.Failed) > 0 {
		return res, nil, fmt.Errorf("integrity verification failed for shards: %v", res.Failed)
	}

	return res, reconstructed, nil
}
