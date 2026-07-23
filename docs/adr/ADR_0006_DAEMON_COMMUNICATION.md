# ADR-0006: Client SDK Daemon Communication Model

**Status**: Accepted  
**Deciders**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## Context
Client SDKs (`meshweb-sdk-go`, `meshweb-sdk-python`, etc.) must interface with storage nodes and DHT peers. There are two primary architectural patterns:
1. **Embedded Node Mode**: The Client SDK instantiates an in-process libp2p node (`node.NewNode`).
2. **External Process Daemon Mode**: The Client SDK communicates with a standalone background daemon process (`meshweb-node.exe`) over IPC (Unix Domain Sockets / Named Pipes) or RPC.

## Decision
The Technical Steering Committee decided to adopt a **Dual-Mode Dependency Inversion Abstraction**:
- **Interface Contract**: All Client SDKs SHALL depend on an abstract `NodeClient` interface (`PushShardToPeer`, `PushManifestToPeer`, `FetchManifestFromPeer`, `GetHost`, `Stop`).
- **EmbeddedNodeClient**: Implements `NodeClient` for in-process local testing, zero-dependency CLI tools, and rapid integration tests.
- **DaemonNodeClient**: Implements `NodeClient` over IPC/RPC to communicate with a background `meshweb-node` process without instantiating redundant P2P hosts per application process.

---

## Alternatives Considered & Why Rejected

### 1. Embedded Node Only
- **Proposal**: Force every application importing `meshweb-sdk-go` to run an in-process libp2p host.
- **Rejected**: In multi-process applications or desktop GUI wrappers, spawning 10 independent libp2p hosts causes severe port exhaustion, memory duplication, and uncoordinated DHT routing.

### 2. External Daemon Only
- **Proposal**: Require an external background daemon (`meshweb-node.exe`) to be pre-installed and running before any SDK call can function.
- **Rejected**: Breaks zero-dependency developer experience (DX), complicates unit/integration testing in CI pipelines, and burdens simple CLI tools.
