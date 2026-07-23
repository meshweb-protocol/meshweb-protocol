# Placement Module

This module serves as the "Brain of Storage Allocation".
It determines which chunk goes to which peer based on health, availability, and geographic diversity.
It accepts a `Policy` (e.g. `ReplicaCount = 10`) and outputs `TargetPeers`.
