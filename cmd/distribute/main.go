package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/meshweb/meshweb-protocol/manifest"
)

func main() {
	inPath := `e:\MeshWeb\test_1gb.bin`
	outDir := `e:\MeshWeb\temp_shards`

	os.RemoveAll(outDir)
	os.MkdirAll(outDir, 0755)

	fmt.Println("Encoding test_1gb.bin...")
	man, err := manifest.CreateUploadManifest(inPath, outDir, 10, 20, 1024*1024)
	if err != nil {
		log.Fatalf("Encode error: %v", err)
	}
	fmt.Printf("FileID: %s\n", man.FileID)

	nodes := []string{`e:\MeshWeb\node1`, `e:\MeshWeb\node2`, `e:\MeshWeb\node3`, `e:\MeshWeb\node4`}

	for _, n := range nodes {
		storeDir := filepath.Join(n, "store", man.FileID)
		os.MkdirAll(storeDir, 0755)
	}

	for i := 0; i < 30; i++ {
		shardName := fmt.Sprintf("shard.%02d", i)
		src := filepath.Join(outDir, shardName)

		var targetNode string
		if i < 10 {
			targetNode = nodes[0]
		} else if i < 20 {
			targetNode = nodes[1]
		} else if i < 28 {
			targetNode = nodes[2]
		} else {
			targetNode = nodes[3] // Shards 28, 29 go to node4 for test 4
		}

		dst := filepath.Join(targetNode, "store", man.FileID, shardName)

		data, err := os.ReadFile(src)
		if err != nil {
			log.Fatalf("Read error for %s: %v", src, err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			log.Fatalf("Write error for %s: %v", dst, err)
		}
	}

	fmt.Println("Distribution complete.")
}
