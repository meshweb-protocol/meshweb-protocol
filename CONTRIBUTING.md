# Contributing to MeshWeb Protocol & Standards

**Status**: Standard Specification  
**Authority**: Technical Steering Committee (TSC)  
**Date**: 2026-07-23  

---

## 1. Governance Principles

MeshWeb is an open, specification-first, ownerless protocol platform. All contributions to the protocol, reference SDKs, nodes, or compliance tools MUST adhere to our core principles:

1. **Protocol-First**: The protocol specification is primary; code is merely an implementation.
2. **RFC-Centric**: Wire changes require formal RFC approval before code can be merged.
3. **Evidence-Based**: Every sprint and major PR MUST provide empirical verification evidence.
4. **Anti-Specification Drift**: Pull Requests modifying wire behavior MUST update written RFCs, Golden Test Vectors, Compliance Tests, and Code atomically.

---

## 2. Contribution Decision Tree

Use this decision tree to determine the required contribution path for your proposal or change:

```text
               What are you proposing or changing?
                               │
       ┌───────────────────────┼───────────────────────┐
       ▼                       ▼                       ▼
   Code Bug Fix /         Architectural           Wire Format /
 Optimization / Refactor    Design Decision       Protocol Specification
       │                       │                       │
       ▼                       ▼                       ▼
  Submit Pull Request     Author ADR in          Propose RFC in
   (Standard PR)        /docs/adr/             /rfcs/ (RFC-0000)
                                                       │
                                                       ▼
                                            Update Golden Vectors,
                                            Compliance Tests & Spec
                                                  Atomically
```

- **Bug Fix / Refactor**: Standard Pull Request passing all CI checks.
- **Architectural Choice**: Document in [`/docs/adr/`](file:///e:/MeshWeb/meshweb-protocol/docs/adr/) following standard template.
- **Wire / Protocol Change**: Follow [`RFC-0000`](file:///e:/MeshWeb/meshweb-protocol/rfcs/RFC_0000_RFC_PROCESS.md) proposal process. Requires TSC review, Golden Vector update, Compliance test addition, and atomic specification update.

---

## 3. Workflows

### 3.1 Proposing an RFC (`RFC-0000` Process)
1. Copy the standard RFC header template from [`RFC-0000`](file:///e:/MeshWeb/meshweb-protocol/rfcs/RFC_0000_RFC_PROCESS.md).
2. Submit your proposal as a `Draft` in the [`/rfcs/`](file:///e:/MeshWeb/meshweb-protocol/rfcs/) directory.
3. Present your RFC to the Technical Steering Committee (TSC) for review.
4. Upon approval, the RFC transitions through `Review` ──► `Candidate` ──► `Frozen Standard`.

### 3.2 Writing an Architecture Decision Record (ADR)
When making significant architectural choices (e.g. transport selection, interface design, framing decisions), author an ADR in [`/docs/adr/`](file:///e:/MeshWeb/meshweb-protocol/docs/adr/):
- Use the standard template: Context, Decision, Consequences, Alternatives Considered, Why Rejected.

### 3.3 Adding Machine-Readable Golden Test Vectors
When adding or updating wire payloads, add the raw JSON / binary files to [`/golden-vectors/`](file:///e:/MeshWeb/meshweb-protocol/golden-vectors/):
- Ensure test vectors pass byte-for-byte hash validation across all reference nodes.

### 3.4 Mandatory Compliance & Evidence Rule (Rule 11)
> **"Every new protocol feature MUST be accompanied by at least one corresponding Compliance Test, one Golden Vector (if wire behavior changes), and one Benchmark (if performance is impacted)."**

---

## 4. Pull Request (PR) Checklist

Before submitting a PR, verify:
- [ ] `go fmt ./...` passes with zero formatting errors.
- [ ] `go vet ./...` passes with zero static analysis warnings.
- [ ] `govulncheck ./...` passes with zero security vulnerabilities.
- [ ] `go test -v -race ./...` passes with zero data races.
- [ ] `go test -v -shuffle=on -count=10 ./...` passes all randomized runs.
- [ ] Goroutine leak check verified (`runtime.NumGoroutine()` delta <= 0).
- [ ] Written RFCs, Golden Vectors, and Conformance Matrix updated atomically if wire behavior changed.
