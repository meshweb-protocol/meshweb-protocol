# MeshWeb Protocol Conformance Matrix

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## 1. Overview
This document maps normative protocol requirements across `RFC-0001` through `RFC-0007` to their corresponding automated test cases in the **MeshWeb Compliance Test Suite** (`RFC-0006`).

---

## 2. Requirement-to-Test Mapping Matrix

| Requirement | Normative Specification Source | Compliance Test ID | Required Level |
| :--- | :--- | :---: | :---: |
| **Multicodec Framing** | RFC-0002 §2.1 | `WIRE-001` | Level 1 |
| **Newline Delimiter (`\n`)** | RFC-0002 §2.2 | `WIRE-002` | Level 1 |
| **Base64 Payload Decoding** | RFC-0002 §3.1 | `WIRE-003` | Level 1 |
| **Raw Binary Stream Decoding** | RFC-0002 §3.3 | `WIRE-004` | Level 1 |
| **Golden Vector Match** | RFC-0007 §2-4 | `VECTOR-001` | Level 1 |
| **8 Explicit Bounds Invariants** | RFC-0002 §4.2 | `BOUNDS-001` | Level 2 |
| **Memory Allocation Sequence** | RFC-0002 §4.1 | `BOUNDS-002` | Level 2 |
| **FileManifest Validation** | RFC-0003 §3 | `MANIFEST-001` | Level 2 |
| **SHA-256 Digest Verification** | RFC-0002 §4.3 | `CRYPTO-001` | Level 2 |
| **Partial File Cleanup on Error** | RFC-0002 §4.3 | `CLEANUP-001` | Level 2 |
| **Health Availability Bitmask** | RFC-0005 §3.6 | `HEALTH-001` | Level 2 |
| **Reputation Scoring & Blacklist** | RFC-0002 §5 | `REPUTATION-001` | Level 3 |
| **Candidate Exhaustion Exit** | RFC-0002 §4.4 | `ROBUST-001` | Level 3 |
| **Idempotent Context Cancellation** | RFC-0002 §4.5 | `LEAK-001` | Level 3 |
| **Dual-Mode 100x Determinism** | RFC-0002 §4.6 | `DETERMINISM-001` | Level 3 |
