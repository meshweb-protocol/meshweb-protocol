package peerstate

import "github.com/libp2p/go-libp2p/core/peer"

type PeerHealth string

const (
	Healthy PeerHealth = "Healthy"
	Suspect PeerHealth = "Suspect"
	Offline PeerHealth = "Offline"
)

type State struct {
	ID        peer.ID
	Health    PeerHealth
	Available bool
}

type Manager interface {
	GetPeerState(id peer.ID) (State, error)
	GetAllHealthyPeers() []peer.ID
}
