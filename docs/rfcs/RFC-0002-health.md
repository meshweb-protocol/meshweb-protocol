# RFC-0002: Health Protocol

**Status:** Accepted & Implemented
**Author:** MeshWeb Core Team
**Created:** 2026-07-20

## Summary
Introduces the `/meshweb/health/1.0.0` protocol to rapidly query a peer for the subset of shards it currently hosts for a specific `FileID`.

## Motivation
To determine network-wide health, a node needs to know exactly which shards are missing globally. Downloading full shards just to check existence is too expensive. A lightweight protocol returning a bitset is required.

## Specification
- **Request:** JSON payload containing `file_id`.
- **Response:** JSON payload containing `version`, `file_id`, and `bitmap` (base64 encoded bytes). The `i`-th bit in the bitmap is `1` if the peer has a healthy replica of shard `i`.

## Drawbacks
Bitmaps limit the max number of shards to the size of the bitset. For MeshWeb, standard file fragmentation is kept well within typical bitmap limits (e.g., 256 shards).
