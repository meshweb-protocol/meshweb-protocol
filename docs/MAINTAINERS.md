# MeshWeb Protocol Maintainers & Governance Structure

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## 1. Governance Overview
Although the MeshWeb Protocol is **ownerless** (no single entity owns the protocol), administrative coordination and specification maintenance are managed by the **MeshWeb Technical Steering Committee (TSC)**.

---

## 2. Technical Steering Committee Responsibilities

The TSC is responsible for:
1. **RFC Review & Approval**: Managing the `RFC-0000` lifecycle from `Draft` to `Frozen Standard`.
2. **Compliance Certification**: Overseeing official `certificate.json` validations from `meshweb-compliance`.
3. **Coordinated Security Disclosure**: Handling private security advisories and CVE disclosures.
4. **Release Gate Approval**: Approving `v1.0.0` release criteria tags.
5. **Extension Registration**: Managing range reservations in `EXTENSION_REGISTRY.md`.

---

## 3. Coordinated Security Disclosure Policy
Vulnerabilities MUST be reported privately to the TSC security team before public disclosure.
- **Reporting Channel**: `security@meshweb.org` (GPG Key ID: `0xMESHWEB100`)
- **Disclosure Sequence**:
  ```text
  Private Report ──► Vulnerability Audit ──► Patch Development ──► Coordinated Release ──► Public Advisory / CVE
  ```
- **Embargo Window**: Standard 90-day security embargo window prior to public disclosure.
