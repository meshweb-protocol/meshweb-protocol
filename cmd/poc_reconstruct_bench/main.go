package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/meshweb/meshweb-protocol/erasure"
)

func main() {
	useDataOnly := flag.Bool("data-only", false, "Use ReconstructData instead of Reconstruct")
	flag.Parse()

	dataShards := 10
	parityShards := 20
	blockSize := 1024 * 1024 // 1MB
	stripeSize := dataShards * blockSize
	totalSegments := 1000
	windowSize := 4

	// Generate random 10MB chunk
	orig := make([]byte, stripeSize)
	rand.Read(orig)

	// We split it into 10 shards.
	// We don't even need to use reedsolomon.Split because we can just slice it manually for the test.
	baseShards := make([][]byte, dataShards+parityShards)
	for i := 0; i < dataShards; i++ {
		baseShards[i] = orig[i*blockSize : (i+1)*blockSize]
	}

	fmt.Printf("Starting benchmark. Total Segments: %d, Window: %d, DataOnly: %v\n", totalSegments, windowSize, *useDataOnly)

	var activeSegments sync.WaitGroup
	windowSem := make(chan struct{}, windowSize)

	start := time.Now()

	go func() {
		for {
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Printf("[RAM] Alloc: %v MB, Sys: %v MB, NumGC: %v\n", m.Alloc/1024/1024, m.Sys/1024/1024, m.NumGC)
			time.Sleep(1 * time.Second)
		}
	}()

	for seg := 0; seg < totalSegments; seg++ {
		windowSem <- struct{}{}
		activeSegments.Add(1)

		go func(segIdx int) {
			defer activeSegments.Done()
			defer func() { <-windowSem }()

			// Create a copy of the base shards so each goroutine modifies its own array slice headers.
			// Reconstruct modifies the slice itself by allocating new arrays for nil entries!
			shards := make([][]byte, dataShards+parityShards)
			copy(shards, baseShards)

			var outBuf bytes.Buffer
			var err error

			if *useDataOnly {
				err = erasure.ReconstructSegmentData(dataShards, parityShards, shards, &outBuf, stripeSize)
			} else {
				err = erasure.ReconstructSegment(dataShards, parityShards, shards, &outBuf, stripeSize)
			}

			if err != nil {
				log.Fatalf("Decode failed: %v", err)
			}

			// dummy write to discard
			io.Copy(io.Discard, &outBuf)

		}(seg)
	}

	activeSegments.Wait()
	duration := time.Since(start)

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Time taken: %v\n", duration)
	fmt.Printf("Total Alloc: %v MB\n", finalMem.TotalAlloc/1024/1024)
	fmt.Printf("Final Sys RAM: %v MB\n", finalMem.Sys/1024/1024)
	fmt.Printf("Total GC Cycles: %v\n", finalMem.NumGC)
}
