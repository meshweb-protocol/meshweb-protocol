# MeshWeb Protocol V1 – External Implementation Invitation

**We're looking for independent protocol implementers.**

MeshWeb Protocol V1 is an open storage protocol specification accompanied by:

* RFC specifications
* Golden Test Vectors
* Automated compliance suite
* Reference Go implementation (non-normative)

We invite developers to build an **independent SDK or node implementation** in any language, including:

* Rust
* Python
* Java
* C#
* Zig
* C++
* or any other language

The goal is to determine whether the written specification alone (or alongside the reference implementation) is sufficient to produce an interoperable implementation.

## Implementation Methodology

We encourage participants to implement the protocol using:

* RFC specifications
* Golden Test Vectors
* `meshweb-compliance`

The reference Go implementation is available as a guide, but implementers who wish to perform a strict clean-room exercise may choose not to consult it. Both approaches provide immense value for validating protocol interoperability.

## Success Criteria

A successful implementation should:

* correctly exchange protocol messages;
* pass the published compliance suite;
* generate a valid compliance certificate (`certificate.json`);
* interoperate with existing MeshWeb implementations.

Bug reports, specification ambiguity reports, and interoperability edge cases are just as valuable as complete implementations.

## Resources & Links

* **Repository**: [https://github.com/meshweb-protocol/meshweb-protocol](https://github.com/meshweb-protocol/meshweb-protocol)
* **Getting Started Guide**: [docs/GETTING_STARTED.md](GETTING_STARTED.md)
* **Compliance Auditor CLI**: `cmd/meshweb-compliance/`

We welcome feedback, issues, and pull requests!
