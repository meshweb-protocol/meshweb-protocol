# ADR-0003: libp2p Transport and Multicodec Stream Multiplexing

**Status**: Accepted  
**Deciders**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## Context
MeshWeb requires a robust, cross-language networking framework capable of NAT traversal, encrypted stream multiplexing, and multicodec protocol negotiation.

## Decision
The TSC selected **libp2p** with explicit multicodec protocol IDs (`/meshweb/storage/1.0.0`, `/meshweb/push/2.0.0`, `/meshweb/manifest/1.0.0`, `/meshweb/health/1.0.0`).

## Consequences
- **Positive**: Native cross-language support in Go, Rust, Python, C++, and JavaScript; built-in Noise/TLS encryption and Kademlia DHT routing.
- **Negative**: Adds libp2p dependency overhead to implementations.
