# ADR-0004: FileManifest Schema V2 Design

**Status**: Accepted  
**Deciders**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## Context
Files stored in MeshWeb are chunked into erasure shards. Nodes require a single, cryptographically verifiable metadata object to orchestrate retrieval without central servers.

## Decision
The TSC specified `FileManifest V2` (`version: "meshweb-manifest/2"`), containing file metadata, Reed-Solomon matrix parameters (`data_shards`, `parity_shards`, `min_shards`), file SHA-256 digest, and relative shard basenames (`ShardPaths`).

## Consequences
- **Positive**: Self-contained metadata; path sanitization (`filepath.Base`) prevents directory traversal attacks.
- **Negative**: Manifest MUST be retrieved and cryptographically verified before shard retrieval can commence.
