package discovery

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	mh "github.com/multiformats/go-multihash"
)

// FileIDToDiscoveryCID converts a FileID string into a stable CID used for DHT provider discovery.
func FileIDToDiscoveryCID(fileID string) (cid.Cid, error) {
	hash := sha256.Sum256([]byte(fileID))
	mhHash, err := mh.Encode(hash[:], mh.SHA2_256)
	if err != nil {
		return cid.Cid{}, err
	}
	return cid.NewCidV1(cid.Raw, mhHash), nil
}

// NewDHTHost creates a libp2p host with a Kademlia DHT attached.
func NewDHTHost(ctx context.Context, listenAddrs []string, bootstrapPeers ...peer.AddrInfo) (host.Host, *dht.IpfsDHT, error) {
	var kad *dht.IpfsDHT
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(listenAddrs...),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			opts := []dht.Option{dht.Mode(dht.ModeServer)}
			if len(bootstrapPeers) > 0 {
				opts = append(opts, dht.BootstrapPeers(bootstrapPeers...))
			}
			var rerr error
			kad, rerr = dht.New(ctx, h, opts...)
			return kad, rerr
		}),
	)
	if err != nil {
		return nil, nil, err
	}
	if kad == nil {
		h.Close()
		return nil, nil, fmt.Errorf("failed to initialize DHT")
	}
	return h, kad, nil
}

// BootstrapDHT boots a DHT node and connects it to provided bootstrap peers.
func BootstrapDHT(ctx context.Context, h host.Host, kad *dht.IpfsDHT, bootstrapPeers []peer.AddrInfo) error {
	for _, p := range bootstrapPeers {
		if err := h.Connect(ctx, p); err != nil {
			return err
		}
	}
	if err := kad.Bootstrap(ctx); err != nil {
		return err
	}
	return nil
}

// AdvertiseFileID announces a file ID on the DHT.
func AdvertiseFileID(ctx context.Context, kad *dht.IpfsDHT, fileID string) error {
	key, err := FileIDToDiscoveryCID(fileID)
	if err != nil {
		return err
	}
	return kad.Provide(ctx, key, true)
}

// FindProviderPeers returns provider peer addresses for a given file ID.
func FindProviderPeers(ctx context.Context, kad *dht.IpfsDHT, fileID string, maxProviders int) ([]peer.AddrInfo, error) {
	key, err := FileIDToDiscoveryCID(fileID)
	if err != nil {
		return nil, err
	}

	providers := make([]peer.AddrInfo, 0, maxProviders)
	providerChan := kad.FindProvidersAsync(ctx, key, maxProviders)
	for p := range providerChan {
		providers = append(providers, peer.AddrInfo{ID: p.ID, Addrs: p.Addrs})
		if len(providers) >= maxProviders {
			break
		}
	}
	return providers, nil
}
