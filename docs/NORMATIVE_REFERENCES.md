# MeshWeb Protocol Normative vs Informative References

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## 1. Overview
This document categorizes all documents, artifacts, and tools in the MeshWeb Protocol repository into **Normative** (binding standard requirements) and **Informative** (explanatory, guidance, and application) references.

---

## 2. Normative References (Binding Standards)
Implementations in any programming language (Go, Rust, Python, C++, Java, Zig) MUST strictly comply with all Normative References:

- **[`RFC-0001: MeshWeb Protocol Constitution`](file:///e:/MeshWeb/meshweb-protocol/rfcs/RFC_INDEX.md)**: Core Protocol Invariants & Rules.
- **[`RFC-0002: MeshWeb Wire Protocol V1`](file:///e:/MeshWeb/meshweb-protocol/rfcs/RFC_INDEX.md)**: Multicodecs, Framing & Wire Serialization.
- **[`RFC-0003: MeshWeb Manifest Format V2`](file:///e:/MeshWeb/meshweb-protocol/rfcs/RFC_INDEX.md)**: FileManifest Schema V2 & Validation Rules.
- **[`RFC-0004: MeshWeb Error Registry`](file:///e:/MeshWeb/meshweb-protocol/rfcs/RFC_INDEX.md)**: Standardized Error Codes 1000–1599.
- **[`RFC-0005: MeshWeb Discovery Protocol`](file:///e:/MeshWeb/meshweb-protocol/rfcs/RFC_INDEX.md)**: DHT & Registry Discovery Wire Schemas.
- **[`RFC-0006: MeshWeb Compliance Suite`](file:///e:/MeshWeb/meshweb-protocol/rfcs/RFC_INDEX.md)**: Conformance Levels 1, 2, and 3.
- **[`RFC-0007: MeshWeb Golden Test Vectors`](file:///e:/MeshWeb/meshweb-protocol/rfcs/RFC_INDEX.md)**: Ground Truth Test Payloads.
- **[`/golden-vectors/`](file:///e:/MeshWeb/meshweb-protocol/golden-vectors/)**: Machine-Readable Test Vector Files.
- **[`PROTOCOL_GOVERNANCE.md`](file:///e:/MeshWeb/meshweb-protocol/PROTOCOL_GOVERNANCE.md)**: The 8 Governance Rules & TSC Policies.

---

## 3. Informative References (Guidance & Non-Binding)
Informative references provide guidance, developer experience contracts, or application ideas. They do NOT dictate protocol wire compatibility:

- **[`SDK_API_CONTRACT.md`](file:///e:/MeshWeb/meshweb-protocol/SDK_API_CONTRACT.md)**: Recommended Developer Experience (DX) interface for Client SDKs.
- **[`ROADMAP.md`](file:///e:/MeshWeb/meshweb-protocol/ROADMAP.md)**: Ecosystem execution milestones & Phase 1–7 plans.
- **[`EXTENSION_REGISTRY.md`](file:///e:/MeshWeb/meshweb-protocol/EXTENSION_REGISTRY.md)**: Reserved V2 Protocol IDs and extension ranges.
- **[`SECURITY_CONSIDERATIONS.md`](file:///e:/MeshWeb/meshweb-protocol/SECURITY_CONSIDERATIONS.md)**: Threat modeling & security guidelines.
- **[`CONFORMANCE_MATRIX.md`](file:///e:/MeshWeb/meshweb-protocol/CONFORMANCE_MATRIX.md)**: Requirement-to-test mapping matrix.
- **Reference Code Examples**: Non-normative Go reference implementations (`cmd/meshwebd`, `node`, `retrieval`).
