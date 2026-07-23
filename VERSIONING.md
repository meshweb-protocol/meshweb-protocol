# MeshWeb Ecosystem Versioning Policy

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-23  

---

## 1. Overview
MeshWeb enforces strict Semantic Versioning (`MAJOR.MINOR.PATCH`) with explicit decoupling between **Protocol Specifications**, **Reference SDKs**, **Storage Daemons**, and **Compliance Tools**.

---

## 2. Decoupled Component Versioning Matrix

```text
Protocol Specification (e.g. Protocol 1.0.0)
       │
       ├──────► Reference SDK (e.g. meshweb-sdk-go v1.8.4)
       ├──────► Storage Daemon (e.g. meshweb-node v1.7.2)
       └──────► Compliance Suite (e.g. meshweb-compliance v1.0.0)
```

| Component | Repository | Versioning Scheme | Backward Compatibility Rules |
| :--- | :--- | :--- | :--- |
| **Protocol Specification** | `meshweb-protocol` | `Major.Minor.Patch` | Breaking wire changes require `Major` bump (`V2.0.0`). Minor releases (`V1.1.0`) are strictly backward-compatible. |
| **Reference SDK** | `meshweb-sdk-go` | `Major.Minor.Patch` | Independent release cycle adhering to [`SDK_API_CONTRACT.md`](file:///e:/MeshWeb/meshweb-protocol/SDK_API_CONTRACT.md). Minor releases (`1.x.0`) preserve public API compatibility. |
| **Storage Daemon** | `meshweb-node` | `Major.Minor.Patch` | Independent release cycle. Internal refactoring and performance fixes bump `Patch` or `Minor` without impacting protocol compliance. |
| **Compliance CLI** | `meshweb-compliance` | `Major.Minor.Patch` | Locked to the supported Protocol Major Version (`v1.x.x`). |

---

## 3. Version Compatibility Headers
libp2p multicodec protocol IDs encode the supported Major protocol version (`/meshweb/storage/1.0.0`, `/meshweb/manifest/1.0.0`). Minor specification revisions MUST maintain wire compatibility with existing `1.0.0` multicodecs.
