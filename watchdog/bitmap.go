package watchdog

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type BitmapProvider interface {
	BuildBitmap(fileID string) ([]byte, error)
}

type LocalBitmapBuilder struct {
	storeDir string
}

func NewLocalBitmapBuilder(storeDir string) *LocalBitmapBuilder {
	return &LocalBitmapBuilder{storeDir: storeDir}
}

func (b *LocalBitmapBuilder) BuildBitmap(fileID string) ([]byte, error) {
	fileDir := filepath.Join(b.storeDir, fileID)
	entries, err := os.ReadDir(fileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte{}, nil // Empty file, returns empty bitmap
		}
		return nil, err
	}

	maxShard := -1
	var shards []int

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "shard.") {
			idxStr := strings.TrimPrefix(entry.Name(), "shard.")
			idx, err := strconv.Atoi(idxStr)
			if err == nil {
				shards = append(shards, idx)
				if idx > maxShard {
					maxShard = idx
				}
			}
		}
	}

	if maxShard == -1 {
		return []byte{}, nil
	}

	// Calculate number of bytes needed
	numBytes := (maxShard / 8) + 1
	bitmap := make([]byte, numBytes)

	for _, idx := range shards {
		byteIndex := idx / 8
		bitIndex := idx % 8
		bitmap[byteIndex] |= (1 << bitIndex)
	}

	return bitmap, nil
}

// DecodeBitmap is a helper for extracting the shard indices from a bitmap
func DecodeBitmap(bitmap []byte) []int {
	var shards []int
	for byteIndex, b := range bitmap {
		for bitIndex := 0; bitIndex < 8; bitIndex++ {
			if (b & (1 << bitIndex)) != 0 {
				shards = append(shards, byteIndex*8+bitIndex)
			}
		}
	}
	return shards
}

// BitmapFromShardIndices encodes verified shard indices into a fixed-size
// bitmap. Callers must only pass shards that have already passed integrity
// verification; file presence alone is not a health signal.
func BitmapFromShardIndices(totalShards int, indices []int) []byte {
	if totalShards <= 0 {
		return []byte{}
	}
	bitmap := make([]byte, (totalShards+7)/8)
	for _, idx := range indices {
		if idx < 0 || idx >= totalShards {
			continue
		}
		bitmap[idx/8] |= 1 << (idx % 8)
	}
	return bitmap
}
