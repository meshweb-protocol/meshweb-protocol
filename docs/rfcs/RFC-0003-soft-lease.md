# RFC-0003: Soft Lease Arbitration

**Status:** Accepted & Implemented
**Author:** MeshWeb Core Team
**Created:** 2026-07-20

## Summary
Implements a distributed Soft Lease mechanism over the Kademlia DHT to elect a single repairing node when multiple nodes detect missing shards.

## Motivation
When a shard goes offline, all remaining nodes hosting the file will simultaneously detect the loss during their Watchdog scans. If all nodes attempt to reconstruct and push the missing shard simultaneously, the network will be flooded with redundant bandwidth ("Repair Storm"). A mechanism is needed to elect a single "Repair Leader".

## Specification
- Nodes compete by writing a `RepairLease` struct to a deterministic DHT key (`/meshweb/lease/<file_id>/<epoch>`).
- If a node reads back the key and finds its own `peer.ID` as the `Owner`, it has acquired the lease and proceeds with the repair.
- The `Epoch` prevents race conditions by incrementing strictly based on the number of currently healthy network shards.

## Drawbacks
DHT propagation delay may still result in split-brain scenarios if the latency exceeds the arbitration window, though this is mitigated by randomized backoffs before lease attempts.
