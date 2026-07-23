# ADR-0005: Decoupled Client SDK API Philosophy

**Status**: Accepted  
**Deciders**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## Context
Application developers building on MeshWeb should not be forced to handle low-level libp2p streams, Reed-Solomon matrices, base64 framing, or retry algorithms directly.

## Decision
The TSC established a strict separation between **Protocol Code** and **Client SDKs** (`SDK_API_CONTRACT.md`). SDKs expose clean, high-level, language-idiomatic methods (`UploadFile`, `DownloadFile`, `DeleteFile`, `GetManifest`, `QueryHealth`) while encapsulating all protocol wire details.

## Consequences
- **Positive**: Exceptional Developer Experience (DX); application code remains clean and decoupled from wire protocol revisions.
- **Negative**: Client SDKs MUST be maintained across multiple programming languages (Go, Python, Rust, Java).
