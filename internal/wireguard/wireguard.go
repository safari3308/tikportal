package wireguard

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// KeyPair represents a WireGuard Curve25519 key pair and an optional PresharedKey.
type KeyPair struct {
	PrivateKey   string `json:"private_key,omitempty"`
	PublicKey    string `json:"public_key"`
	PresharedKey string `json:"preshared_key,omitempty"`
}

// GenerateKeyPair generates a new Curve25519 PrivateKey, derives its PublicKey,
// and generates a random 32-byte PresharedKey encoded in standard Base64.
func GenerateKeyPair() (*KeyPair, error) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate x25519 key: %w", err)
	}

	pskBytes := make([]byte, 32)
	if _, err := rand.Read(pskBytes); err != nil {
		return nil, fmt.Errorf("generate preshared key: %w", err)
	}

	return &KeyPair{
		PrivateKey:   base64.StdEncoding.EncodeToString(priv.Bytes()),
		PublicKey:    base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()),
		PresharedKey: base64.StdEncoding.EncodeToString(pskBytes),
	}, nil
}

// GeneratePresharedKey generates a random 32-byte WireGuard preshared key in Base64.
func GeneratePresharedKey() (string, error) {
	pskBytes := make([]byte, 32)
	if _, err := rand.Read(pskBytes); err != nil {
		return "", fmt.Errorf("generate preshared key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pskBytes), nil
}

// ValidatePublicKey verifies that pubKey is a valid Base64-encoded 32-byte X25519 public key.
func ValidatePublicKey(pubKey string) error {
	trimmed := strings.TrimSpace(pubKey)
	if len(trimmed) != 44 {
		return errors.New("invalid WireGuard public key length (must be 44 base64 characters)")
	}
	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return fmt.Errorf("invalid base64 in public key: %w", err)
	}
	if len(raw) != 32 {
		return errors.New("decoded WireGuard public key must be 32 bytes")
	}
	if _, err := ecdh.X25519().NewPublicKey(raw); err != nil {
		return fmt.Errorf("invalid X25519 public key: %w", err)
	}
	return nil
}

// ClientConfigParams holds the parameters needed to render a WireGuard client .conf file.
type ClientConfigParams struct {
	PeerName            string
	NetworkName         string
	PrivateKey          string // Actual private key (Mode 1) or "<INSERT_YOUR_PRIVATE_KEY>" (Mode 2)
	AllocatedIP         string // e.g., "10.10.0.2"
	SubnetPrefixLen     int    // e.g., 32 or 24
	DNSServers          string // e.g., "1.1.1.1, 8.8.8.8"
	ServerPublicKey     string
	PresharedKey        string
	AllowedIPs          string // e.g., "0.0.0.0/0, ::/0"
	EndpointHost        string
	EndpointPort        int
	PersistentKeepalive int
}

// BuildClientConfig generates a standard WireGuard .conf file string.
func BuildClientConfig(p ClientConfigParams) string {
	privKey := strings.TrimSpace(p.PrivateKey)
	if privKey == "" {
		privKey = "<INSERT_YOUR_PRIVATE_KEY>"
	}
	prefixLen := p.SubnetPrefixLen
	if prefixLen <= 0 || prefixLen > 32 {
		prefixLen = 32
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# TikPortal WireGuard Configuration\n"))
	if p.NetworkName != "" {
		b.WriteString(fmt.Sprintf("# Network: %s\n", p.NetworkName))
	}
	if p.PeerName != "" {
		b.WriteString(fmt.Sprintf("# Device:  %s\n", p.PeerName))
	}
	b.WriteString("\n[Interface]\n")
	b.WriteString(fmt.Sprintf("PrivateKey = %s\n", privKey))
	b.WriteString(fmt.Sprintf("Address = %s/%d\n", p.AllocatedIP, prefixLen))
	if strings.TrimSpace(p.DNSServers) != "" {
		b.WriteString(fmt.Sprintf("DNS = %s\n", strings.TrimSpace(p.DNSServers)))
	}

	b.WriteString("\n[Peer]\n")
	b.WriteString(fmt.Sprintf("PublicKey = %s\n", strings.TrimSpace(p.ServerPublicKey)))
	if strings.TrimSpace(p.PresharedKey) != "" {
		b.WriteString(fmt.Sprintf("PresharedKey = %s\n", strings.TrimSpace(p.PresharedKey)))
	}
	allowed := strings.TrimSpace(p.AllowedIPs)
	if allowed == "" {
		allowed = "0.0.0.0/0"
	}
	b.WriteString(fmt.Sprintf("AllowedIPs = %s\n", allowed))
	b.WriteString(fmt.Sprintf("Endpoint = %s:%d\n", strings.TrimSpace(p.EndpointHost), p.EndpointPort))
	if p.PersistentKeepalive > 0 {
		b.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", p.PersistentKeepalive))
	}
	return b.String()
}
