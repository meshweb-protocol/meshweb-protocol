package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meshweb/meshweb-protocol/manifest"
	"github.com/meshweb/meshweb-protocol/node"
	"github.com/meshweb/meshweb-protocol/retrieval"
)

// Segregated Client Interfaces following the SOLID Interface Segregation Principle

type ManifestClient interface {
	PushManifestToPeer(ctx context.Context, target peer.AddrInfo, fileID string, manifestData []byte) error
	FetchManifestFromPeer(ctx context.Context, target peer.AddrInfo, fileID string) ([]byte, error)
}

type StorageClient interface {
	PushShardToPeer(ctx context.Context, target peer.AddrInfo, fileID string, shard int, dataSize int64, dataReader io.Reader) error
}

type HostProvider interface {
	GetHost() host.Host
}

type Lifecycle interface {
	Stop() error
}

// NodeClient aggregates segregated client capabilities for storage node interactions.
type NodeClient interface {
	ManifestClient
	StorageClient
	HostProvider
	Lifecycle
}

// EmbeddedNodeClient implements NodeClient using an in-process node.Node instance.
type EmbeddedNodeClient struct {
	Node *node.Node
}

func (e *EmbeddedNodeClient) PushShardToPeer(ctx context.Context, target peer.AddrInfo, fileID string, shard int, dataSize int64, dataReader io.Reader) error {
	return e.Node.PushShardToPeer(ctx, target, fileID, shard, dataSize, dataReader)
}

func (e *EmbeddedNodeClient) PushManifestToPeer(ctx context.Context, target peer.AddrInfo, fileID string, manifestData []byte) error {
	return e.Node.PushManifestToPeer(ctx, target, fileID, manifestData)
}

func (e *EmbeddedNodeClient) FetchManifestFromPeer(ctx context.Context, target peer.AddrInfo, fileID string) ([]byte, error) {
	_ = e.Node.Host.Connect(ctx, target)
	stream, err := e.Node.Host.NewStream(ctx, target.ID, "/meshweb/manifest/1.0.0")
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	req := fmt.Sprintf("{\"file_id\":\"%s\"}\n", fileID)
	if _, err := stream.Write([]byte(req)); err != nil {
		return nil, err
	}
	stream.CloseWrite()

	return io.ReadAll(io.LimitReader(stream, 2<<20))
}

func (e *EmbeddedNodeClient) GetHost() host.Host {
	return e.Node.Host
}

func (e *EmbeddedNodeClient) Stop() error {
	if e.Node != nil {
		return e.Node.Stop()
	}
	return nil
}

type Config struct {
	BootPeers   []peer.AddrInfo
	StoreDir    string
	Concurrency int
}

type Client struct {
	nodeClient NodeClient
	cfg        Config
	storeDir   string
}

func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.StoreDir == "" {
		tmp, err := os.MkdirTemp("", "meshweb-client-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp store dir: %w", err)
		}
		cfg.StoreDir = tmp
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 5
	}

	var bootStrs []string
	for _, p := range cfg.BootPeers {
		for _, addr := range p.Addrs {
			bootStrs = append(bootStrs, fmt.Sprintf("%s/p2p/%s", addr, p.ID))
		}
	}

	nodeCfg := node.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		Bootstrap:   bootStrs,
		StoreDir:    cfg.StoreDir,
	}

	n, err := node.NewNode(ctx, nodeCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client node: %w", err)
	}

	if err := n.Start(); err != nil {
		return nil, fmt.Errorf("failed to start client node: %w", err)
	}

	embedded := &EmbeddedNodeClient{Node: n}

	return &Client{
		nodeClient: embedded,
		cfg:        cfg,
		storeDir:   cfg.StoreDir,
	}, nil
}

func (c *Client) Close() error {
	if c.nodeClient != nil {
		return c.nodeClient.Stop()
	}
	return nil
}

func (c *Client) UploadFile(ctx context.Context, inputPath string) (string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("failed to calculate sha256: %w", err)
	}
	fileHash := hex.EncodeToString(hasher.Sum(nil))

	shardsDir := filepath.Join(c.storeDir, "upload_shards_"+fileHash[:8])
	if err := os.MkdirAll(shardsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create shards dir: %w", err)
	}

	dataShards := 2
	parityShards := 2
	blockSize := int(1024 * 1024)
	if int(fi.Size()) < blockSize {
		blockSize = int(fi.Size())
		if blockSize <= 0 {
			blockSize = 1
		}
	}

	m, err := manifest.CreateUploadManifest(inputPath, shardsDir, dataShards, parityShards, blockSize)
	if err != nil {
		return "", fmt.Errorf("failed to create upload manifest: %w", err)
	}

	targetPeers := c.cfg.BootPeers
	if len(targetPeers) == 0 {
		return "", fmt.Errorf("no target storage peers available for upload")
	}

	manifestBytes, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest: %w", err)
	}

	for _, target := range targetPeers {
		_ = c.nodeClient.PushManifestToPeer(ctx, target, m.FileID, manifestBytes)

		for i, shardName := range m.ShardPaths {
			shardFile := filepath.Join(shardsDir, shardName)
			sData, err := os.ReadFile(shardFile)
			if err != nil {
				continue
			}
			sLen := int64(len(sData))
			_ = c.nodeClient.PushShardToPeer(ctx, target, m.FileID, i, sLen, bytes.NewReader(sData))
		}
	}

	return m.FileID, nil
}

func (c *Client) DownloadFile(ctx context.Context, fileID string, outputPath string) error {
	targetPeers := c.cfg.BootPeers
	if len(targetPeers) == 0 {
		return fmt.Errorf("no candidate peers provided")
	}

	var m *manifest.FileManifest
	for _, target := range targetPeers {
		sCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		manifestBytes, err := c.nodeClient.FetchManifestFromPeer(sCtx, target, fileID)
		cancel()
		if err == nil {
			var loaded manifest.FileManifest
			if json.Unmarshal(manifestBytes, &loaded) == nil {
				m = &loaded
				break
			}
		}
	}

	if m == nil {
		return fmt.Errorf("failed to retrieve manifest for file_id %s from any candidate peer", fileID)
	}

	downloadDir := filepath.Join(c.storeDir, "download_"+fileID)
	_ = os.MkdirAll(downloadDir, 0o755)

	_, _, err := retrieval.RunV1(ctx, c.nodeClient.GetHost(), m, targetPeers, c.cfg.Concurrency, downloadDir, outputPath)
	if err != nil {
		return fmt.Errorf("download retrieval failed: %w", err)
	}

	outF, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("reconstructed output file missing: %w", err)
	}
	defer outF.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, outF); err != nil {
		return fmt.Errorf("failed to hash reconstructed file: %w", err)
	}
	reconstructedHash := hex.EncodeToString(hasher.Sum(nil))

	if reconstructedHash != m.Sha256 {
		os.Remove(outputPath)
		return fmt.Errorf("reconstructed file hash mismatch: expected %s, got %s", m.Sha256, reconstructedHash)
	}

	return nil
}
