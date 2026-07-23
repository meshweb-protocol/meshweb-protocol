# RFC-0006: Local Daemon API

**Status:** Proposed (Sprint C)
**Author:** MeshWeb Core Team
**Created:** 2026-07-20

## Summary
Defines the local API boundary exposed by the MeshWeb Protocol Daemon strictly for consumption by the local SDK/Application layer.

## Motivation
To strictly separate the Protocol (V1) from the Marketplace (Sprint C), the daemon cannot run pricing or user account logic. Instead, the Application SDK orchestrates economic events and issues commands to the daemon through a well-defined API boundary, acting as a headless controller for the storage engine.

## Specification (Proposed Endpoints)

The API will be exposed locally (e.g., via gRPC on `localhost:50051` or REST on `localhost:8080`).

### 1. Storage Control
- `POST /storage/upload`: Accept local files, generate Manifest V2, chunk, and erasure-encode. (SDK dictates which authorized `peer.ID`s receive the push).
- `GET /storage/status/{fileID}`: Fetch local scan health (Healthy/Missing shards based on local database).
- `POST /storage/verify`: Execute an ad-hoc integrity scan and hash-check for a given file.

### 2. Node Status
- `GET /node/health`: Returns overall daemon uptime, DHT connection count, and libp2p network status.
- `GET /node/capacity`: Returns raw disk storage used/available.

### 3. P2P Authorization (Crucial Boundary)
- `POST /auth/grant`: SDK tells the daemon "You are authorized to accept PUSH requests from or send PUSH requests to this list of `AuthorizedPeers` under this `AuthorizationID`." The daemon relies strictly on this list to accept or reject incoming shard data. It does not know what a "deal", "price", or "voucher" is.

## Drawbacks
Requires the daemon to maintain a state machine of "Authorized Sessions" to prevent unauthorized nodes from filling up local disk space. This authorization table must be perfectly synced with the SDK's state.
