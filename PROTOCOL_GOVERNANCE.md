# MeshWeb Protocol Governance Specification

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-23  

---

## The Ultimate Governance Principle

> **"Success is measured by independent interoperability, not by lines of code or number of features."**

---

## MeshWeb Protocol V1 Constitution Freeze Invariant

> **"MeshWeb Protocol V1 Constitution is frozen. Future protocol evolution SHALL occur only through V2 or later protocol revisions, except for approved security, interoperability, or specification ambiguity corrections."**

---

## The 12 Fundamental Protocol Invariants & Rules

### Rule 1: Protocol-First Architecture
The MeshWeb Protocol is the primary product. Reference implementations, applications, libraries, SDKs, and tools exist solely to demonstrate and interact with the protocol.

### Rule 2: Reference Implementation & SDKs Are Not Normative
Neither reference Go code (`meshweb-protocol`, `meshweb-sdk-go`, `meshweb-node`) nor reference tooling are normative. Behavior in code that contradicts written RFCs is considered a bug in code, not a specification feature.

### Rule 3: RFCs Are Normative
Written Request for Comments (RFC) specifications (`RFC-0000` through `RFC-0007`+) define the normative protocol requirements. Implementations in any language MUST comply with written RFCs.

### Rule 4: Golden Test Vectors Are Normative
Machine-readable Golden Test Vectors in [`/golden-vectors/`](file:///e:/MeshWeb/meshweb-protocol/golden-vectors/) are authoritative wire format ground truth. If reference code produces bytes that deviate from golden vectors, reference code MUST be corrected.

### Rule 5: Compliance Suite Is Authoritative
The MeshWeb Compliance Test Suite (`RFC-0006`) is the authoritative benchmark for declaring compatibility. Any implementation in Go, Rust, Python, Java, or C++ that passes the suite is certified as MeshWeb Compatible.

### Rule 6: Wire Changes Require RFC Approval
No wire format change, frame addition, field modification, or error code alteration SHALL occur via code commit alone. All protocol wire changes MUST undergo formal RFC submission, review, and TSC approval.

### Rule 7: Backward Compatibility Is Mandatory
Backward compatibility MUST be preserved across all minor and patch versions within a major protocol release (`Protocol V1.x.x`). Breaking wire changes require a major version bump (`Protocol V2.0.0`).

### Rule 8: Multi-Repository Architecture
Each major component in the MeshWeb ecosystem SHALL be maintained as an independent repository to enforce strict layer separation:
- `meshweb-protocol/` (RFCs, Governance, Golden Vectors, Specifications)
- `meshweb-sdk-go/` (Reference Go Client SDK)
- `meshweb-node/` (Reference Storage Daemon)
- `meshweb-compliance/` (Automated Certification CLI)
- `meshweb-sdk-python/` (Clean-Room Python SDK)
- `meshweb-drive/` (Virtual M:\ Drive)

### Rule 9: Anti-Specification Drift Policy
Any Pull Request (PR) modifying wire behavior, manifest format, protocol IDs, error codes, or compliance rules MUST atomically update all four artifacts before merge approval:
1. Written RFC Specification
2. Machine-Readable Golden Vectors (`/golden-vectors/`)
3. Compliance Test Suite (`RFC-0006`)
4. Reference Implementation Code

### Rule 10: TSC Scope Limitation
The Technical Steering Committee (TSC) governs **protocols and specifications**, NOT internal implementation software architecture. Language-specific SDK design decisions (e.g. Go idioms vs Rust async structures) remain autonomous to language maintainers, provided they satisfy protocol wire specifications.

### Rule 11: Mandatory Compliance & Evidence Test Rule
Every new protocol feature or capability MUST be accompanied by at least one corresponding test case in the Compliance Test Suite (`RFC-0006`), at least one golden vector (if wire behavior changes), and a benchmark metric (if performance is impacted). A feature is NOT considered complete or mergeable until its automated compliance test is implemented and passing.

### Rule 12: Normative Language Policy (RFC 2119 / RFC 8174)
All normative MeshWeb RFC specifications SHALL strictly employ standardized RFC 2119 and RFC 8174 keyphrases—**MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**, **SHOULD**, **SHOULD NOT**, **RECOMMENDED**, **MAY**, and **OPTIONAL**—in uppercase to eliminate ambiguity across multi-language implementers.
