<p align="center">
  <strong>MeshWeb Protocol</strong><br>
  <em>An Open Decentralized Storage Protocol Specification</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-V1%20Frozen-blue?style=flat-square" alt="Status: V1 Frozen">
  <img src="https://img.shields.io/badge/spec-RFC--0007-green?style=flat-square" alt="Spec: RFC-0007">
  <img src="https://img.shields.io/badge/license-MIT-yellow?style=flat-square" alt="License: MIT">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go" alt="Go">
</p>

---

> **⚠️ This repository is in Protocol & Governance Freeze.**
> Only critical security fixes, specification ambiguity resolutions, and interoperability defect fixes are accepted. See [SECURITY.md](SECURITY.md) for reporting vulnerabilities.

---

## What is MeshWeb?

MeshWeb is an **open protocol specification** for decentralized file storage over peer-to-peer networks. Files are split into shards, erasure-coded for redundancy, and distributed across independent storage nodes. Any developer can build a compatible implementation in any language using only the written specifications in this repository.

**This is not a product. This is a protocol.**

## Repository Structure

```
meshweb-protocol/
├── rfcs/                    # Protocol specifications (RFC-0000 .. RFC-0007)
├── golden-vectors/          # Machine-readable wire format test vectors
├── compliance/              # Compliance test suite library (Go)
├── cmd/meshweb-compliance/  # Compliance CLI auditor
├── docs/
│   ├── adr/                 # Architecture Decision Records (ADR-0001 .. ADR-0007)
│   └── rfcs/                # Extended RFC documents
│
├── client/                  # Reference Go client SDK
├── node/                    # Reference storage node engine
├── manifest/                # File manifest codec (V2 schema)
├── erasure/                 # Reed-Solomon GF(2^8) erasure coding
├── retrieval/               # Shard retrieval orchestration
├── watchdog/                # Health monitoring, repair, lease management
├── discovery/               # DHT-based peer discovery
│
├── PROTOCOL_GOVERNANCE.md   # 12 normative protocol rules
├── COMPLIANCE_LEVELS.md     # Certification levels 1-5 & test mapping
├── ROADMAP.md               # Execution roadmap & release gates
├── CONTRIBUTING.md          # Contributor guide & decision tree
└── SECURITY.md              # Vulnerability reporting policy
```

## Quick Start

### Run the Compliance Auditor

```bash
go run ./cmd/meshweb-compliance/main.go \
  -target 127.0.0.1:4001 \
  -level 4 \
  -out ./compliance-output
```

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0`  | All tests passed, certificate issued |
| `1`  | One or more compliance tests failed |
| `2`  | Configuration or argument error |
| `3`  | Network error (target unreachable) |
| `4`  | Internal harness error (not a compliance failure) |

### Run Reference Tests

```bash
go test ./...
```

## Specifications

| Document | Title |
|----------|-------|
| [RFC-0000](rfcs/RFC_0000_RFC_PROCESS.md) | RFC Process |
| [RFC-0001](docs/rfcs/RFC-0001-manifest-v2.md) | Manifest V2 Schema |
| [RFC-0002](docs/rfcs/RFC-0002-health.md) | Health & Availability Bitmask |
| [RFC-0003](docs/rfcs/RFC-0003-soft-lease.md) | Soft Lease Protocol |
| [RFC-0004](docs/rfcs/RFC-0004-marketplace-sdk.md) | Marketplace SDK |
| [RFC-0005](docs/rfcs/RFC-0005-marketplace-architecture.md) | Marketplace Architecture |
| [RFC-0006](docs/rfcs/RFC-0006-local-daemon-api.md) | Local Daemon API |
| [RFC-0007](docs/rfcs/RFC-0007-proof-of-storage.md) | Proof of Storage |

## Compliance Certification

MeshWeb defines five compliance levels. Any implementation that passes the compliance suite receives a machine-readable `certificate.json`.

| Level | Name | What it proves |
|-------|------|---------------|
| 1 | Wire Compatible | Correct frame format and manifest parsing |
| 2 | Vector Verified | Matches golden test vectors byte-for-byte |
| 3 | Functionally Complete | Storage, retrieval, and health protocols work |
| 4 | Production Hardened | Bounds checking, race safety, leak freedom |
| 5 | Interoperable Standard | Cross-language bi-directional exchange verified |

See [COMPLIANCE_LEVELS.md](COMPLIANCE_LEVELS.md) for the full test ID mapping.

## Governance

This protocol is governed by 12 normative rules defined in [PROTOCOL_GOVERNANCE.md](PROTOCOL_GOVERNANCE.md). Key principles:

- **RFCs are normative.** Code that contradicts a written RFC is a bug in code.
- **Golden vectors are authoritative.** If reference code deviates from golden vectors, reference code is corrected.
- **Wire changes require RFC approval.** No protocol change via code commit alone.
- **Constitution is frozen.** Future evolution occurs only through V2 or later revisions.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the decision tree:

- **Bug?** → Pull Request
- **Architecture change?** → ADR
- **Wire format change?** → RFC
- **Behavioral change?** → Compliance Test

## License

[MIT](LICENSE)
