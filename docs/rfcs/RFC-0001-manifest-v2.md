# RFC-0001: Manifest V2

**Status:** Accepted & Implemented
**Author:** MeshWeb Core Team
**Created:** 2026-07-20

## Summary
Introduces `meshweb-manifest/2`, adding mandatory support for Erasure Coding metadata to enable segment-level repair and self-healing.

## Motivation
Manifest V1 only supported 1-to-1 file replication. To support the Self-Healing watchdog and minimize bandwidth during repairs, the network must agree on how a file is sharded (Reed-Solomon) and be able to cryptographically verify individual shards.

## Specification
The manifest must contain:
- `DataShards` & `ParityShards` (int)
- `BlockSize` (int)
- `ShardHashes` (array of hex strings)
- `CreatedAt` (unix timestamp)

## Drawbacks
Nodes running V1 will not be able to parse V2 manifests, requiring a hard version check during `/meshweb/manifest` syncs.
