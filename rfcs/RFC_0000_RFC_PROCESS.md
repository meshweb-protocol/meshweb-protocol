# RFC-0000: MeshWeb RFC Process and Lifecycle Specification

**Status**: Frozen Standard  
**Category**: Process Specification  
**Author**: MeshWeb Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## 1. Overview
This document defines the formal Request for Comments (RFC) Process and Lifecycle for the MeshWeb Protocol ecosystem. All protocol standards, wire modifications, manifest updates, and governance rules MUST follow the lifecycle states defined herein.

---

## 2. The Formal RFC Lifecycle States

An RFC progresses through 7 normative lifecycle states:

```
[ Draft ] ──► [ Review ] ──► [ Candidate ] ──► [ Proposed Standard ] ──► [ Frozen Standard ]
                                                                                │
                                                                                ▼
                                                                        [ Superseded / Historic ]
```

### State Definitions
1. **Draft**: An initial proposal undergoing early design and drafting. Subject to frequent major revisions.
2. **Review**: Formally submitted to the Technical Steering Committee and community for peer review and feedback.
3. **Candidate**: Approved by TSC as a Release Candidate specification. Implementation & testing underway.
4. **Proposed Standard**: Implementation complete, passes all Compliance Suite requirements across reference nodes.
5. **Frozen Standard**: Formally locked protocol standard. MUST NOT be modified without version increment (`v1.1` or `v2.0`).
6. **Superseded**: Replaced by a newer RFC (e.g. RFC-0002 superseded by a future RFC-0008).
7. **Historic**: Retained for historical interest; no longer recommended for active implementation.

---

## 3. RFC Header Metadata Standard

Every MeshWeb RFC MUST contain the following header block:

```markdown
# RFC-XXXX: [Title]

**Status**: [ Draft | Review | Candidate | Proposed Standard | Frozen Standard | Superseded | Historic ]  
**Category**: [ Standards Track | Process | Information ]  
**Author**: [Author Name(s)]  
**Date**: [YYYY-MM-DD]  
**Supersedes**: [RFC-YYYY (optional)]  
**Superseded By**: [RFC-ZZZZ (optional)]  
```

---

## 4. RFC Modification Rules
- A **Frozen Standard** RFC SHALL NOT be edited directly, except to correct typographical errors.
- Any wire format change, frame addition, or protocol alteration MUST be submitted as a new RFC.
