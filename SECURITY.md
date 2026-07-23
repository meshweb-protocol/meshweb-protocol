# MeshWeb Security Policy & Vulnerability Disclosure

**Status**: Standard Policy  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-23  

---

## 1. Supported Versions

| Component / Version | Security Support Status |
| :--- | :---: |
| **MeshWeb Protocol V1** (`v1.x.x`) | ✅ **Active Security Support** |
| **MeshWeb Protocol V0** (`v0.x.x`) | ❌ End of Life / Unsupported |

---

## 2. Responsible Vulnerability Disclosure

The MeshWeb Technical Steering Committee (TSC) welcomes security research and reports of potential vulnerabilities. We request that researchers follow responsible disclosure practices prior to public notification.

### Reporting Channel
- **Security Email**: `security@meshweb.org`
- **GPG Key Fingerprint**: `0xMESHWEB100`

---

## 3. Coordinated Security Disclosure Sequence

```text
Private Report ──► 48h Initial Response ──► Patch Development ──► Coordinated Release ──► Public Advisory / CVE
```

1. **Initial Response**: TSC security contacts acknowledge receipt within **48 hours**.
2. **Impact Assessment**: TSC assesses severity (Critical, High, Medium, Low) and reproduces issue.
3. **Private Embargo Patch**: Reference implementations (`meshweb-node`, `meshweb-sdk-go`) develop and test fixes in private branches.
4. **Coordinated Patch Release**: Patches are published simultaneously across all ecosystem repositories.
5. **Public Security Advisory**: Public CVE notice and advisory published **14 days** after patch release.
