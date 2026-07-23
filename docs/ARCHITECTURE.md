# MeshWeb Architecture

This document outlines the core architecture of the MeshWeb Protocol (V1).

## 1. Component Architecture Flow

The self-healing pipeline relies on a strict, decoupled flow of operations:

```mermaid
flowchart TD
    A[Manifest V2] --> B[Integrity Scanner]
    B -->|Generates Local Bitmap| C[Health Protocol]
    C -->|Queries Remote Peers| D[Network State Aggregator]
    D -->|Identifies Missing Shards| E[Repair Queue Job]
    E -->|Competes on DHT| F[Soft Lease Store]
    F -->|If Won| G[Segment-Level Repair Engine]
    G -->|Reconstructs| H[Distribution Push]
```

## 2. Layer Diagram

MeshWeb is designed with strict separation of concerns, ensuring that high-level economic or application requirements do not leak into the storage foundations.

```mermaid
flowchart BT
    A[Storage Layer] --> B[Transport Layer]
    B --> C[Protocol Layer]
    C --> D[SDK Layer]
    D --> E[Application / Marketplace Layer]

    style A fill:#003366,stroke:#333,stroke-width:2px,color:#fff
    style B fill:#005599,stroke:#333,stroke-width:2px,color:#fff
    style C fill:#0077CC,stroke:#333,stroke-width:2px,color:#fff
    style D fill:#3399FF,stroke:#333,stroke-width:2px,color:#fff
    style E fill:#66B2FF,stroke:#333,stroke-width:2px,color:#fff
```
- **Storage Layer**: Erasure coding (`klauspost/reedsolomon`), local disk I/O, hash verification.
- **Transport Layer**: libp2p (Streams, DHT, Gossip).
- **Protocol Layer**: Watchdog Daemon, Repair Pipeline, Health Endpoints, Manifest Specs.
- **SDK Layer**: Golang libraries wrapping the protocol logic.
- **Application Layer**: MeshWeb GUI, Storage Economy, Tokens, User accounts.

## 3. State Machine (File / Shard Lifecycle)

```mermaid
stateDiagram-v2
    [*] --> Healthy
    
    Healthy --> Missing : Node goes offline / Bit-rot
    
    Missing --> RepairQueued : Watchdog Aggregator detects shortage
    
    RepairQueued --> LeaseAcquired : DHT arbitration won
    RepairQueued --> Missing : Lease lost (another node repairs)
    
    LeaseAcquired --> Repairing : ReconstructSome execution
    
    Repairing --> Verified : SHA256 integrity check passes
    Repairing --> RepairQueued : Reconstruction fails (retry)
    
    Verified --> Distributed : Shard pushed to peers
    
    Distributed --> Healthy : Network replica count restored
```

## 4. Future Repository Layout (Post V1)

> Repository restructuring is intentionally deferred until after Protocol V1 freeze to minimize churn and preserve git history during stabilization.

Once the protocol is absolutely stable, the repository will migrate to the following structure:
```text
meshweb/
    protocol/    # Core V1 specs and engines (erasure, watchdog)
    daemon/      # Headless node runner 
    sdk/         # Go SDK for client apps
    cli/         # Command line interface
    examples/    # Sample code
    docs/        # Architecture & Specs
```
