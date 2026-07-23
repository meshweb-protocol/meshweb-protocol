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

The reference Go implementation is available as a guide, but implementers who wish to perform a strict clean-room exercise may choose not to consult it. Neither approach is considered more authoritative than the other; protocol compliance is determined by the written specification and compliance suite.

## Possible Outcomes

We welcome all of the following outcomes:

* Successful interoperable implementations
* Reports of specification ambiguities
* Compliance suite issues or feature gaps
* Golden Test Vector inconsistencies
* Interoperability edge cases
* Documentation and developer experience improvements

Bug reports, ambiguity reports, and edge cases are just as valuable to the protocol's evolution as fully working implementations.

## Resources & Links

* **Repository**: [https://github.com/meshweb-protocol/meshweb-protocol](https://github.com/meshweb-protocol/meshweb-protocol)
* **Getting Started Guide**: [docs/GETTING_STARTED.md](GETTING_STARTED.md)
* **Compliance Auditor CLI**: `cmd/meshweb-compliance/`

We welcome questions, implementation reports, interoperability results, bug reports, specification ambiguity reports, issues, and pull requests!
