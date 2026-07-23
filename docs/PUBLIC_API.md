# Public API Lock

This document tracks the stability of the public APIs exposed by the MeshWeb Protocol V1. 
These guarantees apply to both network protocol endpoints (libp2p streams) and the Go SDK exposed to the Application Layer.

## API Status Definitions
- **Stable**: The API is frozen. No breaking changes are allowed. Additions must be backward compatible.
- **Experimental**: The API is functioning and tested, but may undergo backward-incompatible changes before reaching Stable status.
- **Internal**: The API is strictly for node-to-node internal machinery. Do not rely on this in the SDK.

---

## Network Protocols (libp2p)

| Endpoint | Status | Description |
| :--- | :--- | :--- |
| `/meshweb/health/1.0.0` | **Stable** | Queries a peer for a shard bitmap representing healthy fragments. |
| `/meshweb/push/1.0.0` | **Stable** | Pushes a single shard to a remote peer. |
| `/meshweb/storage/2.0.0` | **Stable** | Retrieves a specific chunked shard from a peer. |
| `DHT Lease Mutex 1.0.0` | **Experimental** | Uses standard DHT `PutValue`/`GetValue` for soft leases. Subject to change based on BFT requirements. |

---

## SDK / Application Interfaces

### Manifest
| Interface | Status | Notes |
| :--- | :--- | :--- |
| `manifest.FileManifest` | **Stable** | The `meshweb-manifest/2` struct is fixed. |
| `manifest.LoadManifest(path)` | **Stable** | |

### Erasure & Repair
| Interface | Status | Notes |
| :--- | :--- | :--- |
| `erasure.EncodeFile(...)` | **Stable** | |
| `erasure.ReconstructFile(...)` | **Stable** | |
| `erasure.RepairShards(...)` | **Stable** | |

### Watchdog Pipeline
| Interface | Status | Notes |
| :--- | :--- | :--- |
| `watchdog.RepairPipeline.RunOnce(...)` | **Experimental** | The orchestrator logic might expand to include pricing constraints in V2. |
| `watchdog.LocalScanner` | **Internal** | |
| `watchdog.NetworkStateAggregator` | **Internal** | |
