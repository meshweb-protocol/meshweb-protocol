# ADR-0002: Reed-Solomon Erasure Coding Selection

**Status**: Accepted  
**Deciders**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## Context
Decentralized storage requires fault tolerance against node churn, network partitions, and provider offline states without requiring 100% full file replication overhead.

## Decision
The TSC selected **Galois Field `GF(2^8)` Reed-Solomon Erasure Coding** as the normative erasure scheme for MeshWeb Protocol V1.
- Standard default matrix configuration: `K=10` Data Shards, `N=20` Parity Shards (Total = 30 Shards).
- Minimum reconstruction threshold: Any `K` out of `N+K` shards (`MinShards = 10`).

## Consequences
- **Positive**: High fault tolerance (up to 66% node loss tolerance) with low storage expansion overhead (`3.0x`).
- **Negative**: Requires CPU Galois field multiplication during encoding and reconstruction.
