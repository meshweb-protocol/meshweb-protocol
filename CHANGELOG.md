# Changelog

All notable changes to the MeshWeb Protocol will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v1.0.0-rc1] - 2026-07-20

### Added
- **Manifest V2**: Implemented `meshweb-manifest/2` supporting Erasure Coding metadata (Data/Parity shards, ShardHashes).
- **Integrity Scanner**: Local watchdog functionality to verify SHA-256 hashes of on-disk shards and quarantine corrupted ones.
- **Health Protocol**: `/meshweb/health/1.0.0` endpoint for querying a peer's local shard bitmap.
- **Repair Queue & Aggregator**: Network state aggregation to identify missing global shards.
- **Soft Lease**: DHT-based `1.0.0` distributed mutex to elect a single repair node and prevent repair storms.
- **Repair Engine**: Segment-level reconstruction using Reed-Solomon without deep memory copies.
- **Chaos Tests**: Full suite of functional, corruption, partition, and randomized chaos tests to prove system resilience.
- **Architecture Freeze**: Complete documentation of Protocol V1 invariants and boundaries.
