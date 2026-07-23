package placement

import "github.com/libp2p/go-libp2p/core/peer"

type Policy struct {
	ReplicaCount int
	Durability   string
	Priority     int
}

type Engine interface {
	SelectPeers(policy Policy, count int) ([]peer.ID, error)
}
