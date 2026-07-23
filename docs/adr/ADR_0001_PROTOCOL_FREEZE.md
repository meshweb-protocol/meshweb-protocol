# ADR-0001: Protocol V1 Freeze Policy and Evidence Sprints

**Status**: Accepted  
**Deciders**: Technical Steering Committee (TSC)  
**Date**: 2026-07-22  

---

## Context
As the core wire protocol, framing, manifest schema, and error codes matured, there was a risk of entering a "Feature Trap" where endless minor additions would cause specification drift and destabilize reference implementations.

## Decision
The Technical Steering Committee decided to:
1. Declare **MeshWeb Protocol V1 Feature Complete & Frozen**.
2. Transition from "Feature Sprints" to **"Evidence Sprints"**, where every sprint goal is defined by empirical proof rather than new functionality.
3. Establish a mandatory **Engineering Freeze Period** between core hardening and compliance certification.

## Consequences
- **Positive**: Wire protocol remains 100% stable; independent implementers can rely on frozen specs without targeting a moving target.
- **Negative**: New protocol proposals MUST be deferred to future Protocol V2 Backlog.
