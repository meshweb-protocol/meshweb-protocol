# MeshWeb Protocol Security Considerations and Threat Modeling

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## 1. Threat Model & Security Boundaries

MeshWeb is designed under an **Adversarial Network Model**. Nodes MUST assume that candidate storage providers, DHT peers, and incoming network streams may be malicious, compromised, or faulty.

### Security Boundaries
- **Encryption Scope**: Encryption of file contents MUST occur on the Buyer/User device before upload. Storage nodes never receive encryption keys.
- **Protocol Scope**: Reputation, economic marketplace, and discovery policies improve peer selection but MUST NOT alter protocol verification rules. A malicious node with 100% reputation MUST be subjected to identical cryptographic verification as a node with 0% reputation.

---

## 2. Specific Attack Vectors and Mitigations

### 2.1 Memory Exhaustion & Unbounded Allocation Attacks
- **Threat**: Malicious peer sends frame specifying `length = 4GB` to trigger OOM panic on receiving node.
- **Mitigation**: **Memory Allocation Processing Sequence Invariant** (`RFC-0002 §4`). Implementations MUST NOT allocate payload buffers until header bounds (`length <= 64MB`, `offset >= 0`, `offset + length <= shardSize`) pass validation.

### 2.2 Malformed Frame Attacks
- **Threat**: Peer sends invalid JSON, broken UTF-8, or unexpected types (`map[string]interface{}` panic exploits).
- **Mitigation**: **Zero Dynamic Parsing Invariant** (`RFC-0002 §1`). Strictly typed unmarshaling. Unmarshaling failures return `ERR_INVALID_HEADER` (1001) and trigger score penalty (-10) + temporary blacklist window.

### 2.3 Corrupted & Poisoned Payload Attacks
- **Threat**: Malicious provider serves modified or corrupted shard data during retrieval.
- **Mitigation**: **Cryptographic Verification & Partial Cleanup Invariant** (`RFC-0002 §3`). Data MUST NOT be persisted to disk or exposed to higher layers until SHA-256 matches `Manifest.ShardHashes` or `Manifest.Sha256`. Corrupted temp files MUST be deleted immediately. Peer receives score penalty (-50) and 10-minute temporary blacklist.

### 2.4 Denial of Service (DoS) & Slowloris Stream Attacks
- **Threat**: Peer opens libp2p streams and holds them open indefinitely without transmitting bytes.
- **Mitigation**: Implementations MUST enforce normative read/write deadlines (10s header deadline, 600s transfer deadline, 30s idle deadline). Unresponsive streams are closed with `ERR_TIMEOUT` (1302).

### 2.5 Replay & Forged Manifest Attacks
- **Threat**: Attacker crafts fake manifest pointing to unauthorized files or directory traversal paths (`../escape`).
- **Mitigation**: **Manifest Invariants** (`RFC-0003`). `FileID` and `ShardPaths` MUST pass strict path sanitization (`filepath.Base`) to prevent directory traversal escapes. Invalid manifests return `ERR_INVALID_MANIFEST` (1102) with -100 score penalty and 1-hour blacklist.

### 2.6 Cryptographic Digest Assumptions
- MeshWeb Protocol V1 relies on **SHA-256** for cryptographic digest verification. In the event of a theoretical SHA-256 collision, Manifest V2 supports dual hashing capability for future transition.

---

## 3. Coordinated Vulnerability Disclosure & CVE Policy

Security vulnerabilities MUST be disclosed responsibly following standard procedures:
1. **Private Disclosure**: Security researchers submit report to TSC security contact (`security@meshweb.org`).
2. **Audit & Verification**: TSC validates report within 48 hours and assigns tracking ID.
3. **Patch Development**: Reference implementations develop and test patch under private embargo.
4. **Coordinated Release**: Reference nodes and SDKs release patched versions simultaneously.
5. **Public Advisory & CVE**: Security advisory and CVE report published 14 days after release.
