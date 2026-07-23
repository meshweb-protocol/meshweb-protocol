# MeshWeb Client SDK API Contract Specification

**Status**: Standard Specification  
**Category**: Developer Experience (DX) Contract  
**Version**: 1.0.0  
**Target Languages**: Go, Python, Rust, Java, C++, TypeScript  
**Date**: 2026-07-22  

---

## 1. Overview
This document defines the normative Developer Experience (DX) API contract for all MeshWeb Client SDKs. The Client SDK encapsulates protocol-layer libp2p wire framing, erasure coding, hash verification, and retry orchestration, exposing a clean, high-level, language-idiomatic interface to application developers.

---

## 2. Normative SDK Interface Contract

All MeshWeb Client SDKs MUST expose equivalent high-level methods:

### 2.1 `UploadFile(path string, options UploadOptions) -> (fileID string, err error)`
- **Purpose**: Encodes local file into Reed-Solomon shards, generates FileManifest V2, pushes manifest and shards to available target nodes, advertises on DHT.
- **Options**:
  - `DataShards` (int, default: 10)
  - `ParityShards` (int, default: 20)
  - `BlockSize` (int, default: 1MB)

### 2.2 `DownloadFile(fileID string, outputPath string, options DownloadOptions) -> err`
- **Purpose**: Fetches manifest, queries available providers, downloads minimum required shards (`min_shards`), verifies cryptographic hashes, reconstructs original file via Reed-Solomon, writes to `outputPath`. Removes temp files on error.

### 2.3 `DeleteFile(fileID string) -> err`
- **Purpose**: Issues localized purge request to storage nodes holding shards of `fileID`.

### 2.4 `GetManifest(fileID string) -> (manifest FileManifest, err error)`
- **Purpose**: Fetches and cryptographically verifies the `FileManifest` for `fileID`.

### 2.5 `QueryHealth(fileID string) -> (health FileHealthStatus, err error)`
- **Purpose**: Queries providers for shard availability bitmasks (`/meshweb/health/1.0.0`) and returns health summary (healthy, missing, corrupted shard counts).

### 2.6 `SearchProviders(fileID string) -> (providers []PeerInfo, err error)`
- **Purpose**: Queries DHT/Registry for active nodes advertising shards for `fileID`.

---

## 3. Compliance Machine-Readable Certificate Schema (`certificate.json`)

When `meshweb-compliance run` completes, it MUST produce a standardized machine-readable certification artifact:

```json
{
  "implementation": "meshweb-sdk-python",
  "version": "1.0.0",
  "protocol": "1.0",
  "level": 3,
  "level_name": "Fully Protocol Compliant",
  "total_tests": 100,
  "passed_tests": 100,
  "failed_tests": 0,
  "timestamp": 1700000000,
  "signature": "certified_by_meshweb_compliance_v1"
}
```

---

## 4. SDK Versioning & Deprecation Policy

1. **Semantic Versioning**: All SDKs MUST follow Semantic Versioning (`MAJOR.MINOR.PATCH`).
2. **Minor Backward Compatibility**: All `1.x.x` minor releases MUST remain 100% backward compatible for application developers.
3. **Deprecation Window**: Any method marked for deprecation MUST be retained and emit deprecation warnings for at least **one minor version cycle** before removal in a major SDK bump (`2.0.0`).
