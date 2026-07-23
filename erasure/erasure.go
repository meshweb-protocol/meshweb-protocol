package erasure

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/klauspost/reedsolomon"
)

type Stats struct {
	EncodeDuration time.Duration
	DecodeDuration time.Duration
	MaxAllocBytes  uint64
	ShardSize      int64
}

// EncodeFile streams `inPath` into `dataShards+parityShards` shard files placed in outDir.
// It returns the list of shard file paths and stats.
func EncodeFile(inPath, outDir string, dataShards, parityShards int, blockSize int) ([]string, Stats, error) {
	var stats Stats

	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, stats, err
	}

	inF, err := os.Open(inPath)
	if err != nil {
		return nil, stats, err
	}
	defer inF.Close()

	// Prepare shard files
	totalShards := dataShards + parityShards

	shardPaths := make([]string, totalShards)
	shardFiles := make([]*os.File, totalShards)
	shardWriters := make([]*bufio.Writer, totalShards)
	for i := 0; i < totalShards; i++ {
		p := filepath.Join(outDir, fmt.Sprintf("shard.%02d", i))
		shardPaths[i] = p
		f, err := os.Create(p)
		if err != nil {
			return nil, stats, err
		}
		shardFiles[i] = f
		shardWriters[i] = bufio.NewWriterSize(f, 4*1024*1024)
		defer f.Close()
	}

	// Sampling goroutine for memory
	var maxAlloc uint64
	stop := make(chan struct{})
	go func() {
		var ms runtime.MemStats
		for {
			select {
			case <-stop:
				return
			default:
				runtime.ReadMemStats(&ms)
				a := ms.Alloc
				for {
					cur := atomic.LoadUint64(&maxAlloc)
					if a <= cur || atomic.CompareAndSwapUint64(&maxAlloc, cur, a) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	start := time.Now()

	stripeSize := int64(dataShards) * int64(blockSize)
	totalSize := int64(totalShards) * int64(blockSize)
	buf := make([]byte, stripeSize, totalSize)
	for {
		n, err := io.ReadFull(inF, buf)
		if err == io.EOF {
			break
		}
		if err == io.ErrUnexpectedEOF {
			buf = buf[:n]
		} else if err != nil {
			close(stop)
			stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)
			return nil, stats, err
		}

		shards, err := enc.Split(buf)
		if err != nil {
			close(stop)
			stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)
			return nil, stats, err
		}
		if err := enc.Encode(shards); err != nil {
			close(stop)
			stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)
			return nil, stats, err
		}

		for i := 0; i < totalShards; i++ {
			if _, err := shardWriters[i].Write(shards[i]); err != nil {
				close(stop)
				stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)
				return nil, stats, err
			}
		}

		if n < len(buf) {
			break
		}
	}

	for i := 0; i < totalShards; i++ {
		if err := shardWriters[i].Flush(); err != nil {
			close(stop)
			stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)
			return nil, stats, err
		}
	}

	stats.EncodeDuration = time.Since(start)
	close(stop)
	stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)

	fi, err := shardFiles[0].Stat()
	if err == nil {
		stats.ShardSize = fi.Size()
	}

	return shardPaths, stats, nil
}

// ReconstructFile attempts to rebuild the original file from provided shard files.
// `present` is a map of shard index -> file path for shards that exist. Missing shards should be omitted.
func ReconstructFile(outPath string, present map[int]string, dataShards, parityShards int, blockSize int, origSize int64) (Stats, error) {
	var stats Stats
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return stats, err
	}

	total := dataShards + parityShards

	// open files (nil for missing)
	files := make([]*os.File, total)
	for i := 0; i < total; i++ {
		if p, ok := present[i]; ok {
			f, err := os.Open(p)
			if err != nil {
				return stats, err
			}
			files[i] = f
			defer f.Close()
		}
	}

	outF, err := os.Create(outPath)
	if err != nil {
		return stats, err
	}
	defer outF.Close()

	// memory sampler
	var maxAlloc uint64
	stop := make(chan struct{})
	go func() {
		var ms runtime.MemStats
		for {
			select {
			case <-stop:
				return
			default:
				runtime.ReadMemStats(&ms)
				a := ms.Alloc
				for {
					cur := atomic.LoadUint64(&maxAlloc)
					if a <= cur || atomic.CompareAndSwapUint64(&maxAlloc, cur, a) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	stripeSize := int64(dataShards) * int64(blockSize)
	remaining := origSize
	startRe := time.Now()
	for remaining > 0 {
		stripeLen := stripeSize
		if remaining < stripeLen {
			stripeLen = remaining
		}

		shards := make([][]byte, total)
		for i := 0; i < total; i++ {
			if files[i] == nil {
				shards[i] = nil
				continue
			}

			buf := make([]byte, blockSize)
			n, err := io.ReadFull(files[i], buf)
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				buf = buf[:n]
			} else if err != nil {
				close(stop)
				stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)
				return stats, err
			}
			shards[i] = buf
		}

		if err := enc.Reconstruct(shards); err != nil {
			close(stop)
			stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)
			return stats, err
		}

		if err := enc.Join(outF, shards, int(stripeLen)); err != nil {
			close(stop)
			stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)
			return stats, err
		}

		remaining -= stripeLen
	}
	stats.DecodeDuration = time.Since(startRe)
	close(stop)
	stats.MaxAllocBytes = atomic.LoadUint64(&maxAlloc)

	return stats, nil
}

// FileSHA256 computes SHA256 hex of a file.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ReconstructSegment reconstructs a single segment (stripe) from available chunk buffers in memory.
// `shards` is a slice of `dataShards + parityShards` byte slices. Missing chunks should be `nil`.
// The function decodes missing chunks in-place and writes the reconstructed data chunks to `out`.
// `stripeLen` specifies how many bytes of the reconstructed data should be written to `out`.
func ReconstructSegment(dataShards, parityShards int, shards [][]byte, out io.Writer, stripeLen int) error {
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return err
	}

	if err := enc.Reconstruct(shards); err != nil {
		return err
	}

	return enc.Join(out, shards, stripeLen)
}

// ReconstructSegmentData reconstructs ONLY the data shards of a single segment from available chunk buffers.
// It ignores missing parity shards, avoiding the overhead of reconstructing them.
func ReconstructSegmentData(dataShards, parityShards int, shards [][]byte, out io.Writer, stripeLen int) error {
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return err
	}

	if err := enc.ReconstructData(shards); err != nil {
		return err
	}

	return enc.Join(out, shards, stripeLen)
}
