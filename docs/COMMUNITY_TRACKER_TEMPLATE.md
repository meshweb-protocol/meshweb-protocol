# External Implementations & Interoperability Tracker Template

*Use this template as the pinned issue/discussion post on GitHub Discussions.*

---

## 🚀 External Implementations

| Language | Status | Compliance | Interop | Repository / Contact |
|----------|--------|------------|---------|----------------------|
| Go (Reference) | Maintained | Level 5 | ✅ PASS | Official (`/client`, `/node`) |
| Python (Clean-Room) | Verified | Level 5 | ✅ PASS | `meshweb-sdk-python` |
| Rust | Planned / In Progress | — | — | [Open Issue / PR] |
| Java | Planned | — | — | [Open Issue / PR] |
| C# | Planned | — | — | [Open Issue / PR] |
| Zig | Planned | — | — | [Open Issue / PR] |

---

## ❓ Specification Ambiguities & Issue Tracker

| Issue # | Document & Section | Topic | Status |
|---------|-------------------|-------|--------|
| #1 | RFC-0003 §4 | Soft Lease Expiration Edge Case | Resolved |
| #2 | RFC-0002 §7 | Bitmap Allocation | Under Discussion |

---

## 🔄 Interoperability Matrix

| Source (Uploader) | Target (Downloader) | SHA-256 Match | Status |
|-------------------|---------------------|---------------|--------|
| Go | Python | ✅ Yes | PASS |
| Python | Go | ✅ Yes | PASS |
| Go | Rust | ⏳ Pending | Pending |
| Rust | Go | ⏳ Pending | Pending |
| Rust | Python | ⏳ Pending | Pending |

---

## 📜 Compliance Certification Log

| Implementation | Version | Target Level | Certified Date | Certificate |
|----------------|---------|--------------|----------------|-------------|
| Go (Reference) | v1.0.0-rc1 | Level 5 | 2026-07-23 | `certificate.json` |
| Python (Clean-Room) | v0.1.0 | Level 5 | 2026-07-23 | `certificate.json` |
| Rust | — | — | — | — |
