# MeshWeb Protocol Deprecation & Compatibility Policy

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-23  

---

## 1. Overview
This policy governs how features, API methods, error codes, and protocol frames are deprecated, transitioned, and eventually retired within the MeshWeb ecosystem.

---

## 2. Deprecation Lifecycle States

A protocol feature or SDK method progresses through 4 deprecation stages:

```text
[ Active / Recommended ] ──► [ Deprecated (Warning) ] ──► [ Obsolete (Must Not Use) ] ──► [ Removed in Major Bump ]
```

1. **Active / Recommended**: Standard, fully supported feature.
2. **Deprecated (Warning)**: Marked for retirement. Emits deprecation warnings in logs/SDKs. MUST remain functional for at least **one full minor release cycle**.
3. **Obsolete (Must Not Use)**: Retained for backward compatibility only. MUST NOT be used in new implementations.
4. **Removed**: Permanently removed in a major protocol bump (`Protocol V2.0.0`).

---

## 3. Invariants & Rules

- **V1 Non-Removal Invariant**: No normative wire frame, header field, or error code in `Protocol V1` SHALL be removed during the lifetime of Major Version 1 (`Protocol V1.x.x`).
- **SDK Deprecation Notice**: Any SDK function marked `@deprecated` MUST specify the replacement function and remain supported for at least 1 minor version bump (`v1.x.0` to `v1.y.0`).
