package node

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/meshweb/meshweb-protocol/discovery"
	"github.com/meshweb/meshweb-protocol/manifest"
	"github.com/meshweb/meshweb-protocol/watchdog"
)

const (
	storageProtocolID   = "/meshweb/storage/1.0.0"
	pushProtocolID      = "/meshweb/push/1.0.0"
	storageProtocolIDv2 = "/meshweb/storage/2.0.0"
	pushProtocolIDv2    = "/meshweb/push/2.0.0"
	manifestProtocolID  = "/meshweb/manifest/1.0.0"
	healthProtocolID    = "/meshweb/health/1.0.0"
	maxShardIndex       = 255
	maxShardSize        = int64(8 << 30)
	maxManifestSize     = 1 << 20
)

type Config struct {
	ListenAddrs []string
	Bootstrap   []string
	StoreDir    string
}

type Node struct {
	Host   host.Host
	Ctx    context.Context
	Cancel context.CancelFunc
	DHT    *dht.IpfsDHT
	Store  string
}

type pushRequest struct {
	FileID string `json:"file_id"`
	Shard  int    `json:"shard"`
	Data   string `json:"data"`
}

func validateShardRequest(fileID string, shard int) error {
	if err := manifest.ValidateFileID(fileID); err != nil {
		return err
	}
	if shard < 0 || shard > maxShardIndex {
		return fmt.Errorf("invalid shard index")
	}
	return nil
}

func NewNode(ctx context.Context, cfg Config) (*Node, error) {
	ctx, cancel := context.WithCancel(ctx)

	// parse bootstrap peers
	var bootPeers []peer.AddrInfo
	for _, s := range cfg.Bootstrap {
		if s == "" {
			continue
		}
		maddr, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("invalid bootstrap multiaddr %q: %w", s, err)
		}
		ai, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("invalid bootstrap peer addr %q: %w", s, err)
		}
		bootPeers = append(bootPeers, *ai)
	}

	// create host + DHT using helper
	listen := cfg.ListenAddrs
	if len(listen) == 0 {
		listen = []string{"/ip4/0.0.0.0/tcp/0"}
	}
	h, kad, err := discovery.NewDHTHost(ctx, listen, bootPeers...)
	if err != nil {
		cancel()
		return nil, err
	}

	n := &Node{
		Host:   h,
		Ctx:    ctx,
		Cancel: cancel,
		DHT:    kad,
		Store:  cfg.StoreDir,
	}

	// ensure store dir
	if n.Store == "" {
		n.Store = filepath.Join(".", "store")
	}
	if err := os.MkdirAll(n.Store, 0o755); err != nil {
		n.Stop()
		return nil, err
	}

	// storage handler
	h.SetStreamHandler(storageProtocolID, func(s network.Stream) {
		defer s.Close()
		_ = s.SetDeadline(time.Now().Add(600 * time.Second))
		var req struct {
			FileID string `json:"file_id"`
			Shard  int    `json:"shard"`
		}
		if err := json.NewDecoder(s).Decode(&req); err != nil {
			return
		}
		if err := validateShardRequest(req.FileID, req.Shard); err != nil {
			_ = json.NewEncoder(s).Encode(map[string]string{"error": err.Error()})
			return
		}
		shardPath := filepath.Join(n.Store, req.FileID, fmt.Sprintf("shard.%02d", req.Shard))
		data, err := os.ReadFile(shardPath)
		var resp struct {
			Data  string `json:"data,omitempty"`
			Error string `json:"error,omitempty"`
		}
		if err != nil {
			resp.Error = "not found"
			_ = json.NewEncoder(s).Encode(resp)
			return
		}
		resp.Data = base64.StdEncoding.EncodeToString(data)
		_ = json.NewEncoder(s).Encode(resp)
	})

	// push handler (accept shard uploads)
	h.SetStreamHandler(pushProtocolID, func(s network.Stream) {
		defer s.Close()
		_ = s.SetDeadline(time.Now().Add(600 * time.Second))
		var req pushRequest
		if err := json.NewDecoder(s).Decode(&req); err != nil {
			return
		}
		if err := validateShardRequest(req.FileID, req.Shard); err != nil {
			_ = json.NewEncoder(s).Encode(map[string]string{"error": err.Error()})
			return
		}
		// write shard
		shardDir := filepath.Join(n.Store, req.FileID)
		if err := os.MkdirAll(shardDir, 0o755); err != nil {
			_ = json.NewEncoder(s).Encode(map[string]string{"error": "failed to create dir"})
			return
		}
		shardPath := filepath.Join(shardDir, fmt.Sprintf("shard.%02d", req.Shard))
		decoded, err := base64.StdEncoding.DecodeString(req.Data)
		if err == nil {
			_ = os.WriteFile(shardPath, decoded, 0o644)
			// advertise on DHT
			_ = discovery.AdvertiseFileID(n.Ctx, kad, req.FileID)
			_ = json.NewEncoder(s).Encode(map[string]string{"status": "ok"})
			return
		}
		_ = json.NewEncoder(s).Encode(map[string]string{"error": "bad data"})
	})

	// storage handler v2
	h.SetStreamHandler(storageProtocolIDv2, func(s network.Stream) {
		defer s.Close()
		_ = s.SetDeadline(time.Now().Add(600 * time.Second))
		reader := bufio.NewReader(s)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var req struct {
			FileID string `json:"file_id"`
			Shard  int    `json:"shard"`
			Offset int64  `json:"offset"`
			Length int64  `json:"length"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			return
		}
		if err := validateShardRequest(req.FileID, req.Shard); err != nil || req.Offset < 0 || req.Length < 0 {
			resp, _ := json.Marshal(map[string]string{"error": "invalid storage request"})
			s.Write(append(resp, '\n'))
			return
		}

		shardPath := filepath.Join(n.Store, req.FileID, fmt.Sprintf("shard.%02d", req.Shard))
		f, err := os.Open(shardPath)
		if err != nil {
			resp, _ := json.Marshal(map[string]string{"error": "not found"})
			s.Write(append(resp, '\n'))
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			return
		}
		totalSize := info.Size()
		if req.Offset > totalSize {
			resp, _ := json.Marshal(map[string]string{"error": "offset beyond shard"})
			s.Write(append(resp, '\n'))
			return
		}

		var length = req.Length
		if length == 0 || length > totalSize-req.Offset {
			length = totalSize - req.Offset
		}

		_, err = f.Seek(req.Offset, io.SeekStart)
		if err != nil {
			return
		}

		var resp struct {
			Status         string `json:"status"`
			Error          string `json:"error,omitempty"`
			FileID         string `json:"file_id"`
			Shard          int    `json:"shard"`
			Offset         int64  `json:"offset"`
			Length         int64  `json:"length"`
			TotalShardSize int64  `json:"total_shard_size"`
			ChunkHash      string `json:"chunk_hash,omitempty"`
			ShardHash      string `json:"shard_hash,omitempty"`
		}
		resp.Status = "ok"
		resp.FileID = req.FileID
		resp.Shard = req.Shard
		resp.Offset = req.Offset
		resp.Length = length
		resp.TotalShardSize = totalSize

		respBytes, _ := json.Marshal(resp)
		s.Write(append(respBytes, '\n'))

		io.CopyN(s, f, length)
	})

	// push handler v2
	h.SetStreamHandler(pushProtocolIDv2, func(s network.Stream) {
		defer s.Close()
		_ = s.SetDeadline(time.Now().Add(600 * time.Second))
		reader := bufio.NewReader(s)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var req struct {
			FileID         string `json:"file_id"`
			Shard          int    `json:"shard"`
			Offset         int64  `json:"offset"`
			Length         int64  `json:"length"`
			TotalShardSize int64  `json:"total_shard_size"`
			ChunkHash      string `json:"chunk_hash,omitempty"`
			ShardHash      string `json:"shard_hash,omitempty"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			return
		}
		if err := validateShardRequest(req.FileID, req.Shard); err != nil || req.Offset < 0 || req.Length <= 0 || req.TotalShardSize <= 0 || req.TotalShardSize > maxShardSize || req.Offset > req.TotalShardSize || req.Length > req.TotalShardSize-req.Offset {
			resp, _ := json.Marshal(map[string]string{"error": "invalid push request"})
			s.Write(append(resp, '\n'))
			return
		}

		shardDir := filepath.Join(n.Store, req.FileID)
		if err := os.MkdirAll(shardDir, 0o755); err != nil {
			resp, _ := json.Marshal(map[string]string{"error": "failed to create dir"})
			s.Write(append(resp, '\n'))
			return
		}
		shardPath := filepath.Join(shardDir, fmt.Sprintf("shard.%02d", req.Shard))

		flags := os.O_CREATE | os.O_WRONLY
		if req.Offset == 0 {
			flags |= os.O_TRUNC
		}
		f, err := os.OpenFile(shardPath, flags, 0o644)
		if err != nil {
			resp, _ := json.Marshal(map[string]string{"error": "failed to open file"})
			s.Write(append(resp, '\n'))
			return
		}
		defer f.Close()

		if req.Offset > 0 {
			f.Seek(req.Offset, io.SeekStart)
		}

		written, err := io.CopyN(f, reader, req.Length)

		var resp struct {
			Status        string `json:"status"`
			Error         string `json:"error,omitempty"`
			ReceivedBytes int64  `json:"received_bytes"`
		}
		resp.Status = "ok"
		resp.ReceivedBytes = written
		if err != nil {
			resp.Error = err.Error()
		}
		respBytes, _ := json.Marshal(resp)
		s.Write(append(respBytes, '\n'))

		if err == nil && written == req.Length && req.Offset+written == req.TotalShardSize {
			_ = discovery.AdvertiseFileID(n.Ctx, kad, req.FileID)
		}
	})

	// manifest handler
	h.SetStreamHandler(manifestProtocolID, func(s network.Stream) {
		defer s.Close()
		_ = s.SetDeadline(time.Now().Add(30 * time.Second))
		reader := bufio.NewReader(s)

		// Read FileID line
		fileIDBytes, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var req struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal(fileIDBytes, &req); err != nil || manifest.ValidateFileID(req.FileID) != nil {
			return
		}
		manifestDir := filepath.Join(n.Store, req.FileID)
		manifestPath := filepath.Join(manifestDir, "manifest.json")

		// Check if payload bytes follow. If stream is at EOF/empty payload, serve stored manifest.
		manifestData, err := io.ReadAll(io.LimitReader(reader, maxManifestSize+1))
		if err != nil || len(manifestData) == 0 {
			mData, readErr := os.ReadFile(manifestPath)
			if readErr != nil {
				resp, _ := json.Marshal(map[string]string{"error": "manifest not found"})
				s.Write(append(resp, '\n'))
				return
			}
			s.Write(mData)
			return
		}

		var receivedManifest manifest.FileManifest
		if err := json.Unmarshal(manifestData, &receivedManifest); err != nil || receivedManifest.Validate() != nil || receivedManifest.FileID != req.FileID {
			resp, _ := json.Marshal(map[string]string{"error": "invalid manifest"})
			s.Write(append(resp, '\n'))
			return
		}

		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			resp, _ := json.Marshal(map[string]string{"error": "failed to create dir"})
			s.Write(append(resp, '\n'))
			return
		}

		f, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			resp, _ := json.Marshal(map[string]string{"error": "failed to open file"})
			s.Write(append(resp, '\n'))
			return
		}
		defer f.Close()

		written, err := f.Write(manifestData)
		var resp struct {
			Status        string `json:"status"`
			Error         string `json:"error,omitempty"`
			ReceivedBytes int    `json:"received_bytes"`
		}
		resp.Status = "ok"
		resp.ReceivedBytes = written
		if err != nil {
			resp.Error = err.Error()
		}
		respBytes, _ := json.Marshal(resp)
		s.Write(append(respBytes, '\n'))
	})

	// health handler
	h.SetStreamHandler(healthProtocolID, func(s network.Stream) {
		defer s.Close()
		_ = s.SetDeadline(time.Now().Add(10 * time.Second))
		reader := bufio.NewReader(s)

		reqBytes, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var req struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal(reqBytes, &req); err != nil || manifest.ValidateFileID(req.FileID) != nil {
			return
		}

		var resp struct {
			Version int    `json:"version"`
			FileID  string `json:"file_id"`
			Bitmap  string `json:"bitmap,omitempty"`
			Error   string `json:"error,omitempty"`
		}
		resp.Version = 1
		resp.FileID = req.FileID

		scanner := watchdog.NewLocalScanner(n.Store, filepath.Join(n.Store, "quarantine"))
		scan, err := scanner.ScanLocalFile(req.FileID)
		if err != nil {
			resp.Error = err.Error()
		} else {
			bitmap := watchdog.BitmapFromShardIndices(len(scan.HealthyShards)+len(scan.MissingShards)+len(scan.CorruptedShards), scan.HealthyShards)
			resp.Bitmap = base64.StdEncoding.EncodeToString(bitmap)
		}
		respBytes, _ := json.Marshal(resp)
		s.Write(append(respBytes, '\n'))
	})

	// Bootstrap DHT (connect to boot peers)
	if err := discovery.BootstrapDHT(ctx, h, kad, bootPeers); err != nil {
		// non-fatal: continue
	}

	return n, nil
}

func (n *Node) Start() error {
	// Re-announce happens after NewNode bootstraps DHT
	go n.ScanAndAnnounce()
	return nil
}

func (n *Node) ScanAndAnnounce() {
	// Wait for DHT convergence before announcing
	for i := 0; i < 60; i++ {
		if n.DHT.RoutingTable().Size() > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	entries, err := os.ReadDir(n.Store)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fileID := entry.Name()
		if len(fileID) == 0 {
			continue
		}

		// Skip malformed/empty directories without panicking
		shards, err := os.ReadDir(filepath.Join(n.Store, fileID))
		if err != nil || len(shards) == 0 {
			continue
		}

		_ = discovery.AdvertiseFileID(n.Ctx, n.DHT, fileID)
	}
}

func (n *Node) Stop() error {
	n.Cancel()
	return n.Host.Close()
}

func (n *Node) PushShardToPeer(ctx context.Context, target peer.AddrInfo, fileID string, shard int, dataSize int64, dataReader io.Reader) error {
	if err := validateShardRequest(fileID, shard); err != nil {
		return err
	}
	if dataSize <= 0 || dataSize > maxShardSize {
		return fmt.Errorf("invalid shard size")
	}
	dialCtx := network.WithForceDirectDial(ctx, "bypass backoff")
	if err := n.Host.Connect(dialCtx, target); err != nil {
		return err
	}

	supportsV2, err := n.Host.Peerstore().SupportsProtocols(target.ID, pushProtocolIDv2)
	useV2 := err == nil && len(supportsV2) > 0

	var stream network.Stream
	var streamErr error
	if useV2 {
		stream, streamErr = n.Host.NewStream(ctx, target.ID, pushProtocolIDv2)
	} else {
		stream, streamErr = n.Host.NewStream(ctx, target.ID, pushProtocolID)
	}
	if streamErr != nil {
		return streamErr
	}
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(600 * time.Second))

	if useV2 {
		req := struct {
			FileID         string `json:"file_id"`
			Shard          int    `json:"shard"`
			Offset         int64  `json:"offset"`
			Length         int64  `json:"length"`
			TotalShardSize int64  `json:"total_shard_size"`
		}{
			FileID:         fileID,
			Shard:          shard,
			Offset:         0,
			Length:         dataSize,
			TotalShardSize: dataSize,
		}
		reqBytes, _ := json.Marshal(req)
		stream.Write(append(reqBytes, '\n'))

		buf := make([]byte, 256*1024)
		if _, err := io.CopyBuffer(stream, dataReader, buf); err != nil {
			return err
		}

		reader := bufio.NewReader(stream)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}
		var resp struct {
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}
		if err := json.Unmarshal(line, &resp); err != nil {
			return fmt.Errorf("invalid json: %w", err)
		}
		if resp.Error != "" {
			return errors.New(resp.Error)
		}
		return nil
	} else {
		data, err := io.ReadAll(dataReader)
		if err != nil {
			return err
		}
		req := pushRequest{FileID: fileID, Shard: shard, Data: base64.StdEncoding.EncodeToString(data)}
		if err := json.NewEncoder(stream).Encode(req); err != nil {
			return err
		}
		var resp map[string]string
		if err := json.NewDecoder(stream).Decode(&resp); err != nil {
			return err
		}
		if errStr, ok := resp["error"]; ok && errStr != "" {
			return errors.New(errStr)
		}
		return nil
	}
}

func (n *Node) PushManifestToPeer(ctx context.Context, target peer.AddrInfo, fileID string, manifestData []byte) error {
	if err := manifest.ValidateFileID(fileID); err != nil {
		return err
	}
	if len(manifestData) == 0 || len(manifestData) > maxManifestSize {
		return fmt.Errorf("invalid manifest payload size")
	}
	var parsed manifest.FileManifest
	if err := json.Unmarshal(manifestData, &parsed); err != nil || parsed.Validate() != nil || parsed.FileID != fileID {
		return fmt.Errorf("invalid manifest")
	}
	dialCtx := network.WithForceDirectDial(ctx, "bypass backoff")
	if err := n.Host.Connect(dialCtx, target); err != nil {
		return err
	}

	stream, err := n.Host.NewStream(ctx, target.ID, manifestProtocolID)
	if err != nil {
		return err
	}
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(60 * time.Second))

	req := map[string]string{
		"file_id": fileID,
	}
	reqBytes, _ := json.Marshal(req)
	stream.Write(append(reqBytes, '\n'))

	stream.Write(manifestData)
	stream.CloseWrite()

	reader := bufio.NewReader(stream)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return err
	}

	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

type HealthResponse struct {
	Version int    `json:"version"`
	FileID  string `json:"file_id"`
	Bitmap  []byte `json:"-"` // We will decode from base64
}

func (n *Node) QueryHealth(ctx context.Context, target peer.AddrInfo, fileID string) (*HealthResponse, error) {
	if err := manifest.ValidateFileID(fileID); err != nil {
		return nil, err
	}
	dialCtx := network.WithForceDirectDial(ctx, "bypass backoff")
	if err := n.Host.Connect(dialCtx, target); err != nil {
		return nil, err
	}

	stream, err := n.Host.NewStream(ctx, target.ID, healthProtocolID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(15 * time.Second))

	req := map[string]string{
		"file_id": fileID,
	}
	reqBytes, _ := json.Marshal(req)
	stream.Write(append(reqBytes, '\n'))

	reader := bufio.NewReader(stream)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var raw struct {
		Version int    `json:"version"`
		FileID  string `json:"file_id"`
		Bitmap  string `json:"bitmap"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	if raw.Error != "" {
		return nil, errors.New(raw.Error)
	}

	resp := &HealthResponse{
		Version: raw.Version,
		FileID:  raw.FileID,
	}
	if raw.Bitmap != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw.Bitmap)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 bitmap: %w", err)
		}
		resp.Bitmap = decoded
	}

	return resp, nil
}
