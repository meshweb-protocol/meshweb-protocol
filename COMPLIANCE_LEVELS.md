# MeshWeb Protocol Compliance Certification Levels & CLI Contract

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-23  

---

## 1. Allowed Enum Values for `verification_method`

| Enum Value | Verification Meaning & Scope |
| :--- | :--- |
| `developer_attestation` | Official attestation by the implementation team that zero reference source code was inspected. |
| `independent_audit` | Certified via formal audit by an independent 3rd-party security or standards auditor. |
| `reproducible_review` | Certified via automated clean-room build pipeline reproducibility review. |

---

## 2. Machine-Readable Certification Output Schema (`certificate.json`)

```json
{
  "certificate_version": 1,
  "certificate_id": "cert_mw_v1_948f20b411a0",
  "protocol_version": "1.0.0",
  "compliance_profile": "meshweb-v1",
  "profile_version": 1,
  "spec_revision": "RFC-0007",
  "implementation": "meshweb-sdk-python",
  "implementation_version": "1.0.0",
  "runner_version": "meshweb-compliance/v1.0.0",
  "vector_set": "golden-vectors/v1.0.0",
  "compliance_level": 5,
  "level_name": "Interoperable Standard",
  "reference_independent": true,
  "verification_method": "developer_attestation",
  "total_tests": 100,
  "passed_tests": 100,
  "failed_tests": 0,
  "issued_at": 1700000000,
  "signature": "certified_by_meshweb_tsc_v1_ecdsa_sha256"
}
```
