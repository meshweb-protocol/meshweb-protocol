package discovery

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
)

const DiscoveryProtocolID = protocol.ID("/meshweb/discovery/1.0.0")

type ProviderInfo struct {
	ID    string   `json:"id"`
	Addrs []string `json:"addrs"`
}

type announceRequest struct {
	Action       string       `json:"action"`
	FileID       string       `json:"file_id"`
	Provider     ProviderInfo `json:"provider"`
	ShardIndices []int        `json:"shard_indices,omitempty"`
}

type discoveryRequest struct {
	Action string `json:"action"`
	FileID string `json:"file_id"`
}

type discoveryResponse struct {
	Providers []ProviderInfo `json:"providers,omitempty"`
	Error     string         `json:"error,omitempty"`
}

type providerRecord struct {
	Info         ProviderInfo
	ShardIndices []int
	LastSeen     time.Time
}

type Registry struct {
	host    host.Host
	mu      sync.Mutex
	records map[string]map[string]providerRecord
	ttl     time.Duration
}

func NewRegistry(h host.Host) *Registry {
	r := &Registry{
		host:    h,
		records: make(map[string]map[string]providerRecord),
		ttl:     5 * time.Minute,
	}
	h.SetStreamHandler(DiscoveryProtocolID, r.handleStream)
	return r
}

func (r *Registry) handleStream(s network.Stream) {
	defer s.Close()
	reader := bufio.NewReader(s)
	decoder := json.NewDecoder(reader)

	var req announceRequest
	if err := decoder.Decode(&req); err != nil {
		return
	}

	switch req.Action {
	case "announce":
		r.handleAnnounce(s, req)
	case "find":
		r.handleFind(s, req.FileID)
	default:
		_ = json.NewEncoder(s).Encode(discoveryResponse{Error: "unknown action"})
	}
}

func (r *Registry) handleAnnounce(s network.Stream, req announceRequest) {
	if req.FileID == "" || req.Provider.ID == "" {
		_ = json.NewEncoder(s).Encode(discoveryResponse{Error: "invalid announce payload"})
		return
	}

	addrs := make([]string, 0, len(req.Provider.Addrs))
	for _, addr := range req.Provider.Addrs {
		if _, err := ma.NewMultiaddr(addr); err == nil {
			addrs = append(addrs, addr)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.records[req.FileID]; !ok {
		r.records[req.FileID] = make(map[string]providerRecord)
	}
	r.records[req.FileID][req.Provider.ID] = providerRecord{
		Info: ProviderInfo{
			ID:    req.Provider.ID,
			Addrs: addrs,
		},
		ShardIndices: req.ShardIndices,
		LastSeen:     time.Now(),
	}
	_ = json.NewEncoder(s).Encode(discoveryResponse{})
}

func (r *Registry) handleFind(s network.Stream, fileID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked()

	providers := make([]ProviderInfo, 0)
	if records, ok := r.records[fileID]; ok {
		for _, rec := range records {
			providers = append(providers, rec.Info)
		}
	}

	_ = json.NewEncoder(s).Encode(discoveryResponse{Providers: providers})
}

func (r *Registry) pruneLocked() {
	now := time.Now()
	for fileID, providers := range r.records {
		for pid, rec := range providers {
			if now.Sub(rec.LastSeen) > r.ttl {
				delete(providers, pid)
			}
		}
		if len(providers) == 0 {
			delete(r.records, fileID)
		}
	}
}

func (r *Registry) Announce(ctx context.Context, registryInfo peer.AddrInfo, fileID string, provider peer.AddrInfo, shardIndices []int) error {
	stream, err := r.host.NewStream(ctx, registryInfo.ID, DiscoveryProtocolID)
	if err != nil {
		return err
	}
	defer stream.Close()

	providerInfo := ProviderInfo{ID: provider.ID.String(), Addrs: make([]string, 0, len(provider.Addrs))}
	for _, addr := range provider.Addrs {
		providerInfo.Addrs = append(providerInfo.Addrs, addr.String())
	}

	req := announceRequest{
		Action:       "announce",
		FileID:       fileID,
		Provider:     providerInfo,
		ShardIndices: shardIndices,
	}
	if err := json.NewEncoder(stream).Encode(req); err != nil {
		return err
	}

	var resp discoveryResponse
	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func (r *Registry) FindProviders(ctx context.Context, registryInfo peer.AddrInfo, fileID string) ([]peer.AddrInfo, error) {
	stream, err := r.host.NewStream(ctx, registryInfo.ID, DiscoveryProtocolID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	req := discoveryRequest{Action: "find", FileID: fileID}
	if err := json.NewEncoder(stream).Encode(req); err != nil {
		return nil, err
	}

	var resp discoveryResponse
	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	providers := make([]peer.AddrInfo, 0, len(resp.Providers))
	for _, provider := range resp.Providers {
		addrs := make([]ma.Multiaddr, 0, len(provider.Addrs))
		for _, raw := range provider.Addrs {
			if maddr, err := ma.NewMultiaddr(raw); err == nil {
				addrs = append(addrs, maddr)
			}
		}
		pid, err := peer.Decode(provider.ID)
		if err != nil {
			continue
		}
		providers = append(providers, peer.AddrInfo{ID: pid, Addrs: addrs})
	}
	return providers, nil
}
