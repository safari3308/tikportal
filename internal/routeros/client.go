package routeros

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WGInterface represents a WireGuard interface on MikroTik RouterOS v7.
type WGInterface struct {
	ID         string `json:".id"`
	Name       string `json:"name"`
	PublicKey  string `json:"public-key"`
	ListenPort int    `json:"listen-port"`
	MTU        int    `json:"mtu"`
	Running    bool   `json:"running"`
	Disabled   bool   `json:"disabled"`
	Comment    string `json:"comment,omitempty"`
}

// PeerInfo represents a WireGuard peer on MikroTik RouterOS v7.
type PeerInfo struct {
	ID             string `json:".id"`
	Interface      string `json:"interface"`
	Name           string `json:"name,omitempty"`
	PublicKey      string `json:"public-key"`
	PresharedKey   string `json:"preshared-key,omitempty"`
	AllowedAddress string `json:"allowed-address"`
	LastHandshake  string `json:"last-handshake,omitempty"`
	RxBytes        int64  `json:"rx"`
	TxBytes        int64  `json:"tx"`
	Disabled       bool   `json:"disabled"`
	Comment        string `json:"comment,omitempty"`
}

// AddPeerRequest holds parameters to create a peer on RouterOS.
type AddPeerRequest struct {
	Interface           string
	Name                string
	PublicKey           string
	PresharedKey        string
	AllowedAddress      string // e.g. "10.10.0.2/32"
	PersistentKeepalive int    // e.g. 25 (seconds)
	Comment             string
}

// Client defines the interface for interacting with MikroTik RouterOS v7.
type Client interface {
	IsMock() bool
	ListWireGuardInterfaces(ctx context.Context) ([]WGInterface, error)
	ListPeers(ctx context.Context, iface string) ([]PeerInfo, error)
	AddPeer(ctx context.Context, req AddPeerRequest) (string, error)
	UpdatePeerKeys(ctx context.Context, rosPeerID, publicKey, presharedKey string) error
	RemovePeer(ctx context.Context, rosPeerID string) error
}

// RESTClient implements Client against the RouterOS v7 HTTPS REST API (/rest/...).
type RESTClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

func NewRESTClient(baseURL, username, password string, insecureSkipVerify bool) *RESTClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify, // Common for RouterOS self-signed certs
		},
	}
	return &RESTClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout:   12 * time.Second,
			Transport: tr,
		},
	}
}

func (c *RESTClient) IsMock() bool { return false }

func (c *RESTClient) ListWireGuardInterfaces(ctx context.Context) ([]WGInterface, error) {
	var rawList []map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/rest/interface/wireguard", nil, &rawList); err != nil {
		return nil, err
	}

	out := make([]WGInterface, 0, len(rawList))
	for _, item := range rawList {
		listenPort, _ := strconv.Atoi(asString(item["listen-port"]))
		mtu, _ := strconv.Atoi(asString(item["mtu"]))
		out = append(out, WGInterface{
			ID:         asString(item[".id"]),
			Name:       asString(item["name"]),
			PublicKey:  asString(item["public-key"]),
			ListenPort: listenPort,
			MTU:        mtu,
			Running:    asBool(item["running"]),
			Disabled:   asBool(item["disabled"]),
			Comment:    asString(item["comment"]),
		})
	}
	return out, nil
}

func (c *RESTClient) ListPeers(ctx context.Context, iface string) ([]PeerInfo, error) {
	path := "/rest/interface/wireguard/peers"
	if strings.TrimSpace(iface) != "" {
		path += "?interface=" + strings.TrimSpace(iface)
	}
	var rawList []map[string]any
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &rawList); err != nil {
		return nil, err
	}

	out := make([]PeerInfo, 0, len(rawList))
	for _, item := range rawList {
		rx, _ := strconv.ParseInt(asString(item["rx"]), 10, 64)
		tx, _ := strconv.ParseInt(asString(item["tx"]), 10, 64)
		out = append(out, PeerInfo{
			ID:             asString(item[".id"]),
			Interface:      asString(item["interface"]),
			Name:           asString(item["name"]),
			PublicKey:      asString(item["public-key"]),
			PresharedKey:   asString(item["preshared-key"]),
			AllowedAddress: asString(item["allowed-address"]),
			LastHandshake:  asString(item["last-handshake"]),
			RxBytes:        rx,
			TxBytes:        tx,
			Disabled:       asBool(item["disabled"]),
			Comment:        asString(item["comment"]),
		})
	}
	return out, nil
}

func (c *RESTClient) AddPeer(ctx context.Context, req AddPeerRequest) (string, error) {
	payload := map[string]string{
		"interface":       req.Interface,
		"public-key":      req.PublicKey,
		"allowed-address": req.AllowedAddress,
	}
	if req.Name != "" {
		payload["name"] = req.Name
	}
	if req.PresharedKey != "" {
		payload["preshared-key"] = req.PresharedKey
	}
	if req.PersistentKeepalive > 0 {
		payload["persistent-keepalive"] = fmt.Sprintf("%ds", req.PersistentKeepalive)
	}
	if req.Comment != "" {
		payload["comment"] = req.Comment
	}

	var resp map[string]any
	// RouterOS v7 supports PUT /rest/interface/wireguard/peers or POST /rest/interface/wireguard/peers/add
	if err := c.doJSON(ctx, http.MethodPost, "/rest/interface/wireguard/peers/add", payload, &resp); err != nil {
		return "", err
	}
	if id := asString(resp["ret"]); id != "" {
		return id, nil
	}
	if id := asString(resp[".id"]); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("RouterOS did not return peer .id: %v", resp)
}

func (c *RESTClient) UpdatePeerKeys(ctx context.Context, rosPeerID, publicKey, presharedKey string) error {
	payload := map[string]string{
		"public-key": publicKey,
	}
	if presharedKey != "" {
		payload["preshared-key"] = presharedKey
	}
	path := "/rest/interface/wireguard/peers/" + strings.TrimPrefix(rosPeerID, "/")
	return c.doJSON(ctx, http.MethodPatch, path, payload, nil)
}

func (c *RESTClient) RemovePeer(ctx context.Context, rosPeerID string) error {
	if strings.TrimSpace(rosPeerID) == "" {
		return nil
	}
	path := "/rest/interface/wireguard/peers/" + strings.TrimPrefix(rosPeerID, "/")
	err := c.doJSON(ctx, http.MethodDelete, path, nil, nil)
	if err != nil && strings.Contains(err.Error(), "404") {
		// Already removed on RouterOS
		return nil
	}
	return err
}

func (c *RESTClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("RouterOS REST request %s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("RouterOS API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	if out != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("decode RouterOS response: %w", err)
		}
	}
	return nil
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case bool:
		return strconv.FormatBool(t)
	default:
		return ""
	}
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "yes"
	default:
		return false
	}
}

// MockClient simulates a MikroTik RouterOS v7 device with two WireGuard interfaces (wg-main and wg-guest)
// so TikPortal can be tested end-to-end without a physical router attached.
type MockClient struct {
	mu         sync.RWMutex
	nextID     int
	interfaces []WGInterface
	peers      map[string]PeerInfo
}

func NewMockClient() *MockClient {
	return &MockClient{
		nextID: 10,
		interfaces: []WGInterface{
			{
				ID:         "*1",
				Name:       "wg-main",
				PublicKey:  "mTikPortalMainWireGuardServerPublicKey11111=",
				ListenPort: 51820,
				MTU:        1420,
				Running:    true,
				Disabled:   false,
				Comment:    "TikPortal - Main VPN Network (10.10.0.0/24)",
			},
			{
				ID:         "*2",
				Name:       "wg-guest",
				PublicKey:  "mTikPortalGuestWireGuardServerPublicKey2222=",
				ListenPort: 51821,
				MTU:        1420,
				Running:    true,
				Disabled:   false,
				Comment:    "TikPortal - Isolated Guest VPN (10.20.0.0/24)",
			},
		},
		peers: make(map[string]PeerInfo),
	}
}

func (m *MockClient) IsMock() bool { return true }

func (m *MockClient) ListWireGuardInterfaces(_ context.Context) ([]WGInterface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]WGInterface, len(m.interfaces))
	copy(out, m.interfaces)
	return out, nil
}

func (m *MockClient) ListPeers(_ context.Context, iface string) ([]PeerInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []PeerInfo
	for _, p := range m.peers {
		if iface == "" || p.Interface == iface {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *MockClient) AddPeer(_ context.Context, req AddPeerRequest) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := fmt.Sprintf("*%X", m.nextID)
	m.peers[id] = PeerInfo{
		ID:             id,
		Interface:      req.Interface,
		Name:           req.Name,
		PublicKey:      req.PublicKey,
		PresharedKey:   req.PresharedKey,
		AllowedAddress: req.AllowedAddress,
		LastHandshake:  "42s",
		RxBytes:        148200,
		TxBytes:        924510,
		Disabled:       false,
		Comment:        req.Comment,
	}
	return id, nil
}

func (m *MockClient) UpdatePeerKeys(_ context.Context, rosPeerID, publicKey, presharedKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.peers[rosPeerID]
	if !ok {
		// In mock mode after restart, recreate peer entry gracefully
		p = PeerInfo{ID: rosPeerID, LastHandshake: "5s"}
	}
	p.PublicKey = publicKey
	p.PresharedKey = presharedKey
	p.LastHandshake = "1s"
	m.peers[rosPeerID] = p
	return nil
}

func (m *MockClient) RemovePeer(_ context.Context, rosPeerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.peers, rosPeerID)
	return nil
}
