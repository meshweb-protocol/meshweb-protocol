# MeshWeb Protocol V1 Wire Specification

**Status**: Standard Specification (Canonical V1 Wire Spec)  
**Version**: 1.0.0  
**Date**: 2026-07-22  
**Target Implementations**: Go, Rust, C++, Python, Java, Zig  
**Reference Compliance**: Zero Reference Code Dependencies Required  

---

## 1. Scope and Architectural Overview

This document defines the formal, self-contained wire specification for **MeshWeb Protocol V1**. Any software implementation complying with this specification SHALL be fully interoperable with any other compliant MeshWeb node regardless of programming language, operating system, or storage engine.

MeshWeb is a decentralized, peer-to-peer storage protocol providing erasure-coded data placement, cryptographic verification, pipelined retrieval, and autonomous health monitoring over libp2p streams.

### Normative Language Keywords
The key words **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**, **SHOULD**, **SHOULD NOT**, **RECOMMENDED**, **MAY**, and **OPTIONAL** in this document are to be interpreted as described in [RFC 2119](https://www.ietf.org/rfc/rfc2119.txt).

---

## 2. Transport Sublayer and Stream Framing

### 2.1 Transport Layer
- **Transport Sublayer**: All MeshWeb Protocol interactions MUST take place over **libp2p** streams (using TCP/IP or QUIC underlying transports).
- **Security & Encryption**: Streams MUST be encrypted via Noise or TLS 1.3 as negotiated by libp2p.
- **Protocol Multicodec Standard**: Streams are established using libp2p multicodec protocol negotiation.

### 2.2 Message Framing & Binary Payload Stream Semantics
- **Serialization Format**: UTF-8 encoded JavaScript Object Notation (JSON) as specified in [RFC 8259].
- **Frame Delimiter**: Every JSON message frame MUST be terminated by a single ASCII newline character (`\n`, ASCII code `0x0A`).
- **Binary Data Transfer Semantics**:
  - **V1 Protocols**: Binary payloads MUST be encoded as standard RFC 4648 Base64 strings inside JSON payload fields.
  - **V2 Protocols**: Binary payloads MUST be streamed directly as raw binary bytes immediately following the ASCII newline (`\n`) of the JSON header response/request frame.
  - **Binary Stream Termination**: Upon transmitting exactly `length` raw payload bytes, the sender SHALL NOT transmit additional data on that stream for the current request. For single-frame streams, the sender SHOULD execute `CloseWrite()` immediately after payload transmission.

### 2.3 Wire Compatibility & Forward Extensibility Policy
- **Unknown Fields**: Implementations MUST safely ignore unknown optional top-level JSON fields during unmarshaling to maintain forward compatibility.
- **Required Fields**: If any required structural field is missing or invalid, the receiving node MUST reject the message frame immediately with `ERR_INVALID_HEADER` (code 1001).
- **Spec Immutability**: No message field additions, removals, or type modifications SHALL occur without a formal Protocol Major Version bump.

### 2.4 Independent Versioning
- **Protocol Version** (`Protocol V1.0`) and **Manifest Schema Version** (`meshweb-manifest/2`) develop independently. Protocol V1 implementations MUST support reading both `meshweb-manifest/1` and `meshweb-manifest/2` manifests.

### 2.5 Timeout Semantics
- **Deadline Policies**: Implementations SHOULD enforce reasonable stream read/write deadlines:
  - Header Read Timeout: RECOMMENDED 10 seconds.
  - Shard Transfer Timeout: RECOMMENDED 600 seconds per stream.
  - Health/Discovery Query Timeout: RECOMMENDED 10 seconds.
  - Idle Stream Timeout: RECOMMENDED 30 seconds.

---

## 3. Protocol Multicodec Identifiers and Formal Message Schemas

MeshWeb Protocol V1 defines 7 normative protocol multicodec identifiers:

| Protocol ID | Purpose | Transfer Mode |
| :--- | :--- | :--- |
| `/meshweb/storage/1.0.0` | V1 Shard Retrieval | JSON + Base64 |
| `/meshweb/push/1.0.0` | V1 Shard Upload | JSON + Base64 |
| `/meshweb/storage/2.0.0` | V2 Streaming Chunk Retrieval | JSON Header + Raw Stream |
| `/meshweb/push/2.0.0` | V2 Streaming Chunk Upload | JSON Header + Raw Stream |
| `/meshweb/manifest/1.0.0` | Manifest Exchange & Push | JSON Header + Raw Manifest Payload |
| `/meshweb/health/1.0.0` | Health & Shard Availability Query | JSON Frame |
| `/meshweb/discovery/1.0.0` | Node Discovery & Registry Query | JSON Frame |

---

### 3.1 V1 Shard Retrieval (`/meshweb/storage/1.0.0`)

#### Request Frame Schema
- `file_id` (string, REQUIRED): Hex-encoded file identifier. Pattern: `^[a-zA-Z0-9_\-\.]{1,128}$`.
- `shard` (integer, REQUIRED): 0-indexed shard index. Range: `0 <= shard < total_shards`.

```json
{
  "file_id": "testfile_49bc20df15e412a6",
  "shard": 0
}
```

#### Response Frame Schema
- **Success Case**:
  ```json
  {
    "data": "SGVsbG8gTWVzaFdlYiBTdG9yYWdlIFNoYXJk"
  }
  ```
  - `data` (string, REQUIRED): Standard Base64 encoded payload.
- **Error Case**:
  ```json
  {
    "error": "not found",
    "code": 1005
  }
  ```

---

### 3.2 V1 Shard Upload (`/meshweb/push/1.0.0`)

#### Request Frame Schema
- `file_id` (string, REQUIRED): `^[a-zA-Z0-9_\-\.]{1,128}$`.
- `shard` (integer, REQUIRED): `0 <= shard < total_shards`.
- `data` (string, REQUIRED): RFC 4648 Base64 string. Max size: `MaxShardSize` (8GB Base64 string).

```json
{
  "file_id": "testfile_49bc20df15e412a6",
  "shard": 0,
  "data": "SGVsbG8gTWVzaFdlYiBTdG9yYWdlIFNoYXJk"
}
```

#### Response Frame Schema
- **Success**: `{"status": "ok"}`
- **Error**: `{"error": "bad data", "code": 1006}`

---

### 3.3 V2 Streaming Chunk Retrieval (`/meshweb/storage/2.0.0`)

#### Request Frame Schema
- `file_id` (string, REQUIRED): `^[a-zA-Z0-9_\-\.]{1,128}$`.
- `shard` (integer, REQUIRED): `0 <= shard < total_shards`.
- `offset` (int64, REQUIRED): `offset >= 0`.
- `length` (int64, REQUIRED): `0 < length <= 67108864` (64MB max segment size).

```json
{
  "file_id": "testfile_49bc20df15e412a6",
  "shard": 0,
  "offset": 0,
  "length": 1048576
}
```

#### Response Header Frame Schema
```json
{
  "status": "ok",
  "file_id": "testfile_49bc20df15e412a6",
  "shard": 0,
  "offset": 0,
  "length": 1048576,
  "total_shard_size": 10485760
}
```
Immediately following the `\n` character of the response header JSON frame, the provider SHALL stream exactly `length` bytes of raw binary payload.

---

### 3.4 V2 Streaming Chunk Upload (`/meshweb/push/2.0.0`)

#### Request Header Frame Schema
```json
{
  "file_id": "testfile_49bc20df15e412a6",
  "shard": 0,
  "offset": 0,
  "length": 1048576,
  "total_shard_size": 10485760
}
```
Immediately following the `\n` character of the request header JSON frame, the client SHALL stream exactly `length` bytes of raw binary payload.

#### Response Frame Schema
```json
{
  "status": "ok",
  "received_bytes": 1048576
}
```

---

### 3.5 Manifest Distribution (`/meshweb/manifest/1.0.0`)

#### Protocol Sequence
1. Client sends newline-terminated JSON header:
   ```json
   {
     "file_id": "testfile_49bc20df15e412a6"
   }
   ```
2. Client streams raw manifest JSON payload (max 2MB) and calls `CloseWrite()`.
3. Provider validates manifest JSON and returns response frame:
   ```json
   {
     "status": "ok",
     "received_bytes": 1420
   }
   ```

#### Canonical FileManifest Schema (V2)
```json
{
  "version": "meshweb-manifest/2",
  "file_id": "testfile_49bc20df15e412a6",
  "file_name": "example.bin",
  "file_size": 104857600,
  "original_size": 104857600,
  "data_shards": 10,
  "parity_shards": 20,
  "min_shards": 10,
  "block_size": 1048576,
  "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "shard_paths": ["shard.00", "shard.01", "shard.02"],
  "shard_hashes": [
    "5891ba4044d3d52b21f7c327d738090d4b97a7a03470b170943d3270b24dc4b8",
    "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"
  ],
  "created_at": 1700000000
}
```

---

### 3.6 Health & Availability Query (`/meshweb/health/1.0.0`)

#### Request Frame Schema
```json
{
  "file_id": "testfile_49bc20df15e412a6"
}
```

#### Response Frame Schema
```json
{
  "version": 1,
  "file_id": "testfile_49bc20df15e412a6",
  "bitmap": "JA=="
}
```
- `bitmap` (string, REQUIRED): Base64-encoded byte array representing a shard availability bitmask. Bit `i` set to `1` indicates shard `i` is stored locally and passed cryptographic verification.

---

### 3.7 Discovery Protocol (`/meshweb/discovery/1.0.0`)

#### Request Frame Schema (Client → Registry)
```json
{
  "action": "find_providers",
  "file_id": "testfile_49bc20df15e412a6"
}
```

#### Response Frame Schema (Registry → Client)
```json
{
  "status": "ok",
  "file_id": "testfile_49bc20df15e412a6",
  "providers": [
    "/ip4/192.168.1.50/tcp/4001/p2p/12D3KooW..."
  ]
}
```

---

## 4. Standard Error Code Registry

The following table defines the normative MeshWeb Protocol V1 Error Registry:

| Code | Symbol | Standard Description | Retryable |
| :---: | :--- | :--- | :---: |
| **1001** | `ERR_INVALID_HEADER` | Missing or malformed structural header field | No |
| **1002** | `ERR_INVALID_LENGTH` | Length bounds or integer overflow violation | No |
| **1003** | `ERR_HASH_MISMATCH` | Cryptographic SHA-256 digest verification failed | Yes (Different Peer) |
| **1004** | `ERR_TIMEOUT` | Stream deadline or execution timeout exceeded | Yes |
| **1005** | `ERR_NOT_FOUND` | Requested file or shard index does not exist | Yes (Different Peer) |
| **1006** | `ERR_INVALID_BASE64` | Base64 string decoding failure | No |
| **1007** | `ERR_BOUNDS_VIOLATION` | Requested offset exceeds total shard size | No |

---

## 5. Processing Sequence and Invariants

### 5.1 Memory Allocation Processing Sequence
An implementation **MUST NOT** allocate memory for network payloads until all header invariants applicable to that payload have passed validation.
The enforced processing sequence MUST be:
```
Receive Frame Header 
→ Validate Header Invariants 
→ Allocate Bounded Buffer 
→ Read Payload Data 
→ Validate Payload Length 
→ Verify Cryptographic SHA-256 Digest 
→ Persist Payload
```

### 5.2 The 8 Explicit Bounds Invariants
Every payload read MUST explicitly satisfy all 8 invariants:
1. `offset >= 0`
2. `length > 0`
3. `offset <= shardSize`
4. `offset + length <= shardSize`
5. `length <= shardSize - offset` (int64 overflow prevention)
6. `shardSize <= MaxShardSize` (8,589,934,592 bytes [8GB ceiling])
7. `declaredLen <= MaxSegmentSize` (67,108,864 bytes [64MB ceiling])
8. `readLen == declaredLen` / `decodedLen == declaredLen`

### 5.3 Persistence & Partial Cleanup Invariant
- A shard **SHALL NOT** be persisted to disk, announced to the DHT, counted as retrieved, or exposed to higher protocol layers until cryptographic verification has completed successfully.
- A failed verification or cancelled retrieval operation **MUST** remove any partially written temporary files (`.tmp` or partial shards) from disk before returning.

### 5.4 Protocol Cancellation Invariant
When a retrieval `context.Context` is cancelled:
1. All worker threads/goroutines MUST terminate immediately.
2. All underlying network streams MUST be closed without delay.
3. No shard write operation SHALL continue after cancellation.
4. No retry task SHALL be re-enqueued onto the task queue.
5. **Idempotence**: Cancellation SHALL be idempotent. Invoking `cancel()` multiple times MUST NOT cause panics, memory leaks, or double-close panics.

---

## 6. Reputation & Blacklist Policy (RECOMMENDED Defaults)

Implementations MUST track peer performance to prioritize reliable providers. The following penalty scores and temporary blacklist durations are RECOMMENDED:

| Event Type | Score Penalty | Blacklist Duration | Action Description |
| :--- | :---: | :---: | :--- |
| Timeout / Conn Drop | -2 | None | Reduce peer selection priority |
| EOF Before Length | -5 | None | Lower priority, re-enqueue task |
| Invalid JSON / Malformed | -10 | 1 Minute | Temporary blacklist window |
| Invalid Base64 Encoding | -15 | 2 Minutes | Temporary blacklist window |
| Bounds Violation | -20 | 5 Minutes | Temporary blacklist window |
| Hash Mismatch | -50 | 10 Minutes | Temporary blacklist window, report error |
| Replay / Forged Manifest | -100 | 1 Hour | Blacklist peer for 1 hour |

- **Decay Rule**: Blacklists are temporary (1m to 1h), allowing recovered peers to re-enter after time decay while instantly blocking active attack loops.

---

## 7. Protocol Compliance Levels

Implementations MAY declare compliance according to the following 3 formal levels:

- **Level 1: Wire Compatible**: Implementation parses and formats all 7 protocol multicodec frames, newline framing, Base64/Raw streams, and error codes correctly.
- **Level 2: Storage Compatible**: Level 1 + implements Reed-Solomon erasure reconstruction, 8 bounds invariants, SHA-256 verification, and health bitmasks.
- **Level 3: Fully Protocol Compliant**: Level 2 + implements reputation scoring, candidate exhaustion early exit, 100x deterministic outputs, and idempotent cancellation.

---

## 8. Golden Test Vectors

### 8.1 Test Vector 1: Standard FileManifest (V2)
```json
{
  "version": "meshweb-manifest/2",
  "file_id": "test_v2_vector_01",
  "file_name": "sample.bin",
  "file_size": 1048576,
  "original_size": 1048576,
  "data_shards": 2,
  "parity_shards": 2,
  "min_shards": 2,
  "block_size": 524288,
  "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "shard_paths": ["shard.00", "shard.01", "shard.02", "shard.03"],
  "shard_hashes": [
    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  ],
  "created_at": 1700000000
}
```

### 8.2 Test Vector 2: V2 Storage Request & Response Header
**Request Frame Bytes (hex)**:
`7b2266696c655f6964223a22746573745f76325f766563746f725f3031222c227368617264223a302c226f6666736574223a302c226c656e677468223a3532343238387d0a`

**Response Header Frame Bytes (hex)**:
`7b22737461747573223a226f6b222c2266696c655f6964223a22746573745f76325f766563746f725f3031222c227368617264223a302c226f6666736574223a302c226c656e677468223a3532343238382c22746f74616c5f73686172645f73697a65223a3532343238387d0a`
