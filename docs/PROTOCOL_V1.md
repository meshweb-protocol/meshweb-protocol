# Protocol V1 Freeze

This document defines the frozen specifications for the MeshWeb Protocol V1 (Storage & Self-Healing Phase).

## Version Table

| Interface / Message | Version Identifier | Status | Notes |
| :--- | :--- | :--- | :--- |
| **Manifest** | `meshweb-manifest/2` | **Stable** | JSON representation of Erasure Coded files |
| **Health Protocol** | `/meshweb/health/1.0.0` | **Stable** | Query peer for shard bitmaps |
| **Push Protocol** | `/meshweb/push/1.0.0` | **Stable** | Stream shard data to target peer |
| **Storage Protocol** | `/meshweb/storage/2.0.0` | **Stable** | Read chunked shard data from peer |
| **Lease Store** | `1.0.0` | **Experimental** | Distributed Mutex over Kademlia DHT |

---

## 1. Data Structures

### Manifest V2
```json
{
  "Version": "meshweb-manifest/2",
  "FileID": "string",
  "FileName": "string",
  "FileSize": 1048576,
  "OriginalSize": 1048576,
  "DataShards": 20,
  "ParityShards": 10,
  "MinShards": 20,
  "BlockSize": 1024,
  "ShardHashes": ["hex-string", "..."],
  "ShardPaths": ["shard.00", "..."],
  "CreatedAt": 1721021434
}
```

### HealthRequest
```json
{
  "file_id": "string"
}
```

### HealthResponse
```json
{
  "version": 1,
  "file_id": "string",
  "bitmap": "base64-encoded bytes"
}
```

### RepairLease
```json
{
  "FileID": "string",
  "Epoch": 15,
  "Owner": "peer-id-string",
  "IntentHash": "hex-string",
  "ExpiresAt": "2026-07-20T12:00:00Z",
  "CreatedAt": "2026-07-20T11:59:00Z"
}
```

---

## 2. API Endpoints

### `/meshweb/health/1.0.0`
- **Direction:** Node -> Peer
- **Transport:** libp2p Stream
- **Behavior:** Returns a bitset denoting which shards the peer currently hosts for `file_id`. Quarantined/corrupt shards are NOT included in this bitset.

### `/meshweb/push/1.0.0`
- **Direction:** Node -> Peer
- **Transport:** libp2p Stream
- **Behavior:** Accepts a base64 encoded shard. Upon successful disk write, the peer publishes the availability to the DHT.

### `/meshweb/storage/2.0.0`
- **Direction:** Retriever -> Peer
- **Transport:** libp2p Stream
- **Behavior:** Streams the raw binary contents of a requested shard in 64KB blocks.

## 3. Protocol Rules

- **Manifest Immutability**: Once a file is uploaded, its manifest cannot be altered. Changes require a new `FileID`.
- **Quarantine Principle**: If a local disk scan detects a hash mismatch for a shard, it is immediately quarantined and effectively marked as "missing" for all network queries.
- **Repair Determinism**: A Repair pipeline cannot reconstruct data without possessing exactly `MinShards` (or `DataShards`).
