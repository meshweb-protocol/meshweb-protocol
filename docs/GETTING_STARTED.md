# Getting Started with MeshWeb Protocol

```
Read RFCs
     │
     ▼
Study Golden Vectors
     │
     ▼
Implement (Any Language)
     │
     ▼
Run meshweb-compliance
     │
     ▼
Level 5 PASS → certificate.json
```

## Who is this for?

- Developers who want to **implement a MeshWeb node or SDK** in any language.
- Contributors who want to **understand the protocol** before making changes.
- Auditors who want to **evaluate the specification** for completeness.

---

## Learning Path

Follow these steps in order. Each step builds on the previous one.

### Step 1: Understand What MeshWeb Is

Read the top-level [README](../README.md).

Key takeaway: MeshWeb is a **protocol specification**, not a software product. The Go code in this repository is a reference implementation — one possible way to implement the protocol.

### Step 2: Read the Architecture Overview

Read [ARCHITECTURE.md](ARCHITECTURE.md) to understand how the layers fit together:

```
Client SDK  →  uploads/downloads files
Node Engine →  stores shards, runs repair, answers health queries
Erasure     →  Reed-Solomon GF(2^8) encoding/decoding
Manifest    →  JSON metadata describing how a file is split
Discovery   →  DHT-based peer routing via libp2p
Watchdog    →  health monitoring, availability bitmasks, lease management
```

### Step 3: Read the Protocol Specification

Start with the wire-level specification: [WIRE_SPECIFICATION.md](WIRE_SPECIFICATION.md)

Then read the RFCs in order:

| Order | RFC | What you'll learn |
|-------|-----|-------------------|
| 1 | [RFC-0000](../rfcs/RFC_0000_RFC_PROCESS.md) | How the RFC process works |
| 2 | [RFC-0001](rfcs/RFC-0001-manifest-v2.md) | The Manifest V2 JSON schema — the core data structure |
| 3 | [RFC-0002](rfcs/RFC-0002-health.md) | How nodes report shard availability |
| 4 | [RFC-0003](rfcs/RFC-0003-soft-lease.md) | How repair ownership is coordinated |
| 5 | [RFC-0004](rfcs/RFC-0004-marketplace-sdk.md) | Marketplace client interface |
| 6 | [RFC-0005](rfcs/RFC-0005-marketplace-architecture.md) | Marketplace architecture |
| 7 | [RFC-0006](rfcs/RFC-0006-local-daemon-api.md) | Local daemon API |
| 8 | [RFC-0007](rfcs/RFC-0007-proof-of-storage.md) | How storage proofs work |

### Step 4: Study the Golden Vectors

The [`golden-vectors/`](../golden-vectors/) directory contains machine-readable test data:

- `manifest_v2.json` — a valid Manifest V2 document
- `chunk_request_v2.json` — a valid shard request frame
- `chunk_response_v2.json` — a valid shard response frame
- `payload.sha256` — expected SHA-256 digest

A compliant implementation produces and accepts data that matches these vectors byte-for-byte. See the RFCs for the normative requirements.

### Step 5: Understand Compliance Levels

Read [COMPLIANCE_LEVELS.md](../COMPLIANCE_LEVELS.md) to learn the five certification levels:

| Level | Name | Summary |
|-------|------|---------|
| 1 | Wire Compatible | Parse and produce correct frames |
| 2 | Vector Verified | Match golden vectors exactly |
| 3 | Functionally Complete | Store, retrieve, and health-check |
| 4 | Production Hardened | Bounds, races, leaks |
| 5 | Interoperable Standard | Cross-language exchange works |

### Step 6: Build Your Implementation

Using only the RFCs and golden vectors (no reference Go code), write your implementation. This is called **clean-room development**.

### Step 7: Validate with the Compliance Suite

Run the compliance auditor against your node:

```bash
go run ./cmd/meshweb-compliance/main.go \
  -target <your-node-address>:4001 \
  -level 5 \
  -out ./results
```

If all tests pass, a `certificate.json` file is generated automatically.

---

## Architecture Decision Records

For context on why the protocol is designed the way it is, read the ADRs:

| ADR | Decision |
|-----|----------|
| [ADR-0001](adr/ADR_0001_PROTOCOL_FREEZE.md) | Why the protocol is frozen at V1 |
| [ADR-0002](adr/ADR_0002_REED_SOLOMON.md) | Why Reed-Solomon over other erasure schemes |
| [ADR-0003](adr/ADR_0003_LIBP2P_TRANSPORT.md) | Why libp2p for transport |
| [ADR-0004](adr/ADR_0004_MANIFEST_V2_DESIGN.md) | Manifest V2 design rationale |
| [ADR-0005](adr/ADR_0005_SDK_PHILOSOPHY.md) | SDK design philosophy |
| [ADR-0006](adr/ADR_0006_DAEMON_COMMUNICATION.md) | Daemon communication model |
| [ADR-0007](adr/ADR_0007_INTERFACE_SEGREGATION.md) | Interface segregation in Go SDK |

## Questions?

If something in the specification is unclear, [open an issue](https://github.com/meshweb-protocol/meshweb-protocol/issues). Specification ambiguities are one of the few things we fix during the V1 freeze.
