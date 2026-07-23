package watchdog_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/meshweb/meshweb-protocol/watchdog"
)

func TestBitmapEmpty(t *testing.T) {
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	os.MkdirAll(storeDir, 0o755)

	builder := watchdog.NewLocalBitmapBuilder(storeDir)

	// File does not exist
	bitmap, err := builder.BuildBitmap("nonexistent")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(bitmap) != 0 {
		t.Fatalf("expected empty bitmap, got length %d", len(bitmap))
	}
}

func TestBitmapFull(t *testing.T) {
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	fileID := "file123"
	fileDir := filepath.Join(storeDir, fileID)
	os.MkdirAll(fileDir, 0o755)

	// Create 30 shards
	for i := 0; i < 30; i++ {
		os.WriteFile(filepath.Join(fileDir, fmt.Sprintf("shard.%02d", i)), []byte("data"), 0o644)
	}

	builder := watchdog.NewLocalBitmapBuilder(storeDir)
	bitmap, err := builder.BuildBitmap(fileID)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// 30 shards -> bytes: 30 / 8 = 3. maxShard = 29. so 4 bytes needed.
	// bits 0..29 are 1. bit 30,31 are 0.
	if len(bitmap) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(bitmap))
	}

	expected := []byte{0xFF, 0xFF, 0xFF, 0x3F}
	if !bytes.Equal(bitmap, expected) {
		t.Fatalf("expected %x, got %x", expected, bitmap)
	}

	decoded := watchdog.DecodeBitmap(bitmap)
	if len(decoded) != 30 {
		t.Fatalf("expected 30 decoded, got %d", len(decoded))
	}
}

func TestBitmapPartial(t *testing.T) {
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	fileID := "file123"
	fileDir := filepath.Join(storeDir, fileID)
	os.MkdirAll(fileDir, 0o755)

	// Create shards 0, 2, 5, 9
	shards := []int{0, 2, 5, 9}
	for _, idx := range shards {
		os.WriteFile(filepath.Join(fileDir, fmt.Sprintf("shard.%02d", idx)), []byte("data"), 0o644)
	}

	builder := watchdog.NewLocalBitmapBuilder(storeDir)
	bitmap, err := builder.BuildBitmap(fileID)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	decoded := watchdog.DecodeBitmap(bitmap)

	if !reflect.DeepEqual(shards, decoded) {
		t.Fatalf("encode/decode mismatch. expected %v, got %v", shards, decoded)
	}
}
