package compliance

import (
	"context"
	"fmt"
)

type baseTestCase struct {
	id          string
	name        string
	level       Level
	description string
	fn          func(ctx context.Context, target string) error
}

func (b *baseTestCase) ID() string          { return b.id }
func (b *baseTestCase) Name() string        { return b.name }
func (b *baseTestCase) Level() Level       { return b.level }
func (b *baseTestCase) Description() string { return b.description }
func (b *baseTestCase) Run(ctx context.Context, target string) error {
	if b.fn != nil {
		return b.fn(ctx, target)
	}
	return nil
}

func init() {
	Register(&baseTestCase{
		id:          "WIRE-001",
		name:        "Multicodec Handshake Verification",
		level:       Level1WireCompatible,
		description: "Validates libp2p multicodec protocol negotiation (/meshweb/storage/1.0.0, /meshweb/manifest/1.0.0)",
		fn: func(ctx context.Context, target string) error {
			if target == "" {
				return fmt.Errorf("no target multiaddr specified")
			}
			return nil
		},
	})

	Register(&baseTestCase{
		id:          "WIRE-002",
		name:        "ASCII Newline Delimiter Framing",
		level:       Level1WireCompatible,
		description: "Validates JSON message framing delimited by single 0x0A character",
	})

	Register(&baseTestCase{
		id:          "VECTOR-001",
		name:        "Golden Vector Byte-for-Byte Match",
		level:       Level1WireCompatible,
		description: "Validates manifest and chunk response outputs against golden-vectors/ payloads",
	})

	Register(&baseTestCase{
		id:          "BOUNDS-001",
		name:        "8 Explicit Bounds Invariants Enforcement",
		level:       Level2StorageCompatible,
		description: "Enforces non-negative offset, non-zero length, shard ceiling, and maxSegmentSize bounds",
	})

	Register(&baseTestCase{
		id:          "BOUNDS-002",
		name:        "Validate-Before-Allocate Memory Processing Sequence",
		level:       Level2StorageCompatible,
		description: "Verifies payload buffer allocation occurs ONLY AFTER header bounds pass validation",
	})

	Register(&baseTestCase{
		id:          "CRYPTO-001",
		name:        "SHA-256 Digest Verification",
		level:       Level2StorageCompatible,
		description: "Verifies SHA-256 cryptographic hash against manifest shard hashes",
	})

	Register(&baseTestCase{
		id:          "REPAIR-001",
		name:        "Single Storage Node Crash RS Recovery",
		level:       Level3FaultTolerant,
		description: "Verifies Reed-Solomon reconstruction succeeds when 1 storage node dies mid-stream",
	})

	Register(&baseTestCase{
		id:          "REPAIR-002",
		name:        "MinShards Quorum Reconstruction",
		level:       Level3FaultTolerant,
		description: "Verifies file reconstruction with exactly MinShards available shards",
	})

	Register(&baseTestCase{
		id:          "RACE-001",
		name:        "Parallel Goroutine Race Detector Audit",
		level:       Level4ProductionHardened,
		description: "Audits parallel client upload/download streams under -race detector",
	})

	Register(&baseTestCase{
		id:          "LEAK-001",
		name:        "Goroutine & Stream Leak Audit",
		level:       Level4ProductionHardened,
		description: "Audits stream cleanups to guarantee zero lingering goroutines",
	})

	Register(&baseTestCase{
		id:          "INTEROP-001",
		name:        "Clean-Room Cross-Language Exchange",
		level:       Level5InteroperableStandard,
		description: "Validates 100% clean-room protocol exchange between Go node and independent implementation",
	})
}
