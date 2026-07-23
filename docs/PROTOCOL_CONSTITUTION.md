# MeshWeb Protocol Constitution

The Constitution defines the philosophy and invariant principles guiding the design of the MeshWeb Protocol. Every architecture decision, pull request, and design specification must be weighed against these core principles.

## 1. Protocol-First
The core of MeshWeb is its protocol, not its implementations. Reference clients (e.g., the MeshWeb Vault app) exist to demonstrate the protocol, but product requirements must never dictate protocol changes. If a client needs a specialized feature, it must be built on top of the protocol, not baked into it.

## 2. Storage-First
Data durability and integrity are paramount. We must guarantee that files are stored securely and redundantly before we introduce complex mechanisms for indexing, discovery, or tokenomics. The system must work flawlessly as a dumb, distributed hard drive before it tries to be a smart marketplace.

## 3. Ownerless Protocol
The network has no central authority, no master node, and no privileged keys. Anyone can run a node. All coordination happens through decentralized structures (DHT, Gossip, Soft Leases).

## 4. Product Independent
The protocol provides raw infrastructure (blocks, manifests, health endpoints). It does not know about "folders", "user accounts", or "web apps". Those concepts belong in the SDK or Application layers.

## 5. Marketplace Excluded (For V1)
Marketplaces, bidding, dynamic pricing, and resource allocation are separated from the core storage logic. The storage protocol assumes nodes *want* to store and repair data. Economic incentives sit on a distinct layer above this.

## 6. Payment Logic Excluded
Proof-of-Storage determines whether storage obligations were fulfilled. The Marketplace determines whether payment should be released based on these proofs. Payment logic must never be allowed to weaken or alter storage verification requirements.

## 7. Open Specification
Everything from manifest JSON formats to DHT routing keys is openly documented and strictly versioned. A developer should be able to build a compliant node in any language using only the specification.

## 8. Backward Compatibility Preferred
Once an endpoint or schema (like `/meshweb/manifest/1.0.0` or `meshweb-manifest/2`) is marked Stable, it must never be broken. New features require new versions. Older versions must be supported until explicitly deprecated through a rigorous sunset process.

## 9. Minimal Trusted Assumptions
- We do not trust network latency.
- We do not trust node uptimes.
- We do not trust the integrity of data on disk (bit-rot happens).
Everything is verified cryptographically (SHA256 hashes for every shard).

## 10. Self-Healing by Default
The protocol assumes continuous decay. Nodes will go offline, disks will corrupt, and shards will be lost. The system does not wait for a user to request their file; it actively monitors health and reconstructs data automatically in the background using an orchestrating Watchdog and Erasure Coding.
