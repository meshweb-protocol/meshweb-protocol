# ADR-0007: Interface Segregation Principle in Client SDK

**Status**: Accepted  
**Deciders**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## Context
As `NodeClient` grew, passing a single monolithic interface forced caller methods to accept capabilities they did not require. Following Go best practices ("Small interfaces are better than large interfaces"), the SDK interface architecture required refactoring.

## Decision
The TSC adopted the **SOLID Interface Segregation Principle (ISP)** for `client`:
- `ManifestClient`: `PushManifestToPeer`, `FetchManifestFromPeer`
- `StorageClient`: `PushShardToPeer`
- `HostProvider`: `GetHost`
- `Lifecycle`: `Stop`
- `NodeClient`: Aggregates the segregated sub-interfaces.

---

## Alternatives Considered & Why Rejected

### Monolithic NodeClient Interface Only
- **Proposal**: Maintain a single large `NodeClient` interface with all current and future methods.
- **Rejected**: Violates Interface Segregation; makes unit testing and mock creation unnecessarily difficult for components that only require manifest fetching.
