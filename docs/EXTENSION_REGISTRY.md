# MeshWeb Protocol Extension and Reservation Registry

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## 1. Extension Lifecycle Statuses

Every proposal submitted to this registry SHALL have an explicit status:
- **Reserved**: Range or name reserved by TSC for future standards.
- **Draft**: Initial extension proposal under early consideration.
- **Experimental**: Implementation and testing active in non-production branches.
- **Approved**: Formally approved by TSC and integrated into Standards Track RFC.
- **Deprecated**: Extension phase-out; no longer recommended for new implementations.

---

## 2. Reserved Protocol Multicodec IDs

| Reserved Multicodec Prefix | Purpose | Lifecycle Status |
| :--- | :--- | :---: |
| `/meshweb/storage/2.x.x` | V2 Streaming Chunk Extensions | **Reserved** |
| `/meshweb/push/2.x.x` | V2 Chunk Upload Extensions | **Reserved** |
| `/meshweb/manifest/2.x.x` | V2 Extended Manifest Distribution | **Reserved** |
| `/meshweb/handshake/1.0.0` | Future Protocol Negotiation Handshake | **Draft** |
| `/meshweb/merkle/1.0.0` | Per-Chunk Merkle Inclusion Proofs | **Draft** |
| `/meshweb/settlement/1.0.0` | Escrow Voucher Settlement Protocol | **Draft** |

---

## 3. Reserved Error Code Ranges

| Reserved Range | Subsystem Category | Lifecycle Status |
| :---: | :--- | :---: |
| **1600 – 1699** | Settlement & Escrow Errors | **Reserved** |
| **1700 – 1799** | Cryptographic Handshake & Negotiation Errors | **Reserved** |
| **1800 – 1899** | Replication & Federation Errors | **Reserved** |
| **1900 – 1999** | Reserved for Experimental Extensions | **Experimental** |

---

## 4. Reserved Manifest Schema Fields
The following JSON field names are reserved in `FileManifest` to prevent naming collisions:
- `merkle_root` (string, reserved for V2 Merkle proofs)
- `encryption_algorithm` (string, reserved for V2 Vault encryption metadata)
- `signature_proof` (string, reserved for V2 provider identity signatures)
- `settlement_price` (string, reserved for V2 economic pricing)
