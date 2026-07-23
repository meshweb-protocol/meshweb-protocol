# MeshWeb Protocol V1 Official Release Process

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## 1. Overview
This document defines the formal Release Process and criteria required before MeshWeb Protocol V1 can be tagged as a production-ready `v1.0.0` release.

---

## 2. Release Gate Sequence

```text
Draft Specs ──► Frozen Standard ──► v1.0.0-rc1 (Release Candidate) ──► Validation & Fixes ──► v1.0.0 (Final Release)
```

The MeshWeb Technical Steering Committee SHALL tag a formal `v1.0.0` release ONLY after issuing `v1.0.0-rc1` and satisfying all 9 criteria:

- [x] **RFCs Frozen**: `RFC-0000` through `RFC-0007` are in `Frozen Standard` status.
- [x] **Governance Frozen**: `PROTOCOL_GOVERNANCE.md` and 10 Governance Rules are approved.
- [x] **Golden Vectors Frozen**: `/golden-vectors/` files pass byte-for-byte validation.
- [x] **Compliance Suite Specification Frozen**: `RFC-0006` and `CONFORMANCE_MATRIX.md` are frozen.
- [x] **Documentation Version Lock**: Specific immutable commit SHA hashes for all RFCs, Golden Vectors, and Conformance Matrices are locked to `v1.0.0`.
- [ ] **Reference SDK Certified**: `meshweb-sdk-go` passes Level 3 Compliance (`meshweb-compliance`).
- [ ] **Reference Daemon Certified**: `meshweb-node` passes Level 3 Compliance (`meshweb-compliance`).
- [ ] **Independent Implementation Certified**: Independent Python or Rust implementation passes Level 3 Compliance (`meshweb-compliance`).
- [ ] **Cross-Language Interoperability Verified**: Interoperability test matrix (`Go Node ◄──► Python/Rust Node`) achieves 100% pass rate.

---

## 3. First Technical Vertical Slice Benchmark
Before proceeding to applications or marketplace features, Phase 1 & 2 MUST pass the initial vertical slice benchmark:
```text
client.UploadFile("file.bin")
   └─► meshweb-node (Store Shards)
        └─► client.DownloadFile("out.bin")
             └─► SHA-256 Digest Match ──► PASS
```

---

## 4. Official Release Status Matrix

```text
Specification ........ COMPLETE
Governance ........... COMPLETE
Reference SDK ........ IN DEVELOPMENT (Phase 1)
Reference Node ....... IN DEVELOPMENT (Phase 2)
Compliance ........... PLANNED (Phase 3)
Interop .............. PENDING (Phase 4 & 5)
Protocol V1 Release .. NOT YET RELEASED (Pending v1.0.0-rc1 & Tag Criteria)
```

---

## 5. Issue Classification Categories
Issue tracking across all repositories MUST be categorized by subsystem tags:
- `RFC`: Specification or documentation bugs/clarifications.
- `SDK`: Reference Client SDK issues (`meshweb-sdk-go`).
- `NODE`: Reference Daemon issues (`meshweb-node`).
- `COMPLIANCE`: Audit harness issues (`meshweb-compliance`).
- `DRIVE`: Filesystem interface issues (`meshweb-drive`).
- `MARKETPLACE`: Economic and escrow features.
- `V2`: Proposals queued for Protocol V2.
