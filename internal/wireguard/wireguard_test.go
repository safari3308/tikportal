package wireguard

import (
	"strings"
	"testing"
)

func TestGenerateAndValidateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}
	if kp.PrivateKey == "" || kp.PublicKey == "" || kp.PresharedKey == "" {
		t.Fatalf("expected non-empty keys, got %+v", kp)
	}
	if kp.PrivateKey == kp.PublicKey {
		t.Fatalf("private key and public key must differ")
	}
	if err := ValidatePublicKey(kp.PublicKey); err != nil {
		t.Fatalf("ValidatePublicKey() failed on generated key: %v", err)
	}
	if err := ValidatePublicKey("invalid-key"); err == nil {
		t.Fatalf("expected ValidatePublicKey() to reject invalid key")
	}
}

func TestBuildClientConfig(t *testing.T) {
	cfg := BuildClientConfig(ClientConfigParams{
		PeerName:            "MacBook Pro",
		NetworkName:         "Direct PPPoE",
		PrivateKey:          "", // Mode 2 template
		AllocatedIP:         "10.10.0.2",
		SubnetPrefixLen:     24,
		DNSServers:          "1.1.1.1",
		ServerPublicKey:     "server-pub-key=",
		PresharedKey:        "psk-key=",
		AllowedIPs:          "0.0.0.0/0",
		EndpointHost:        "vpn.example.com",
		EndpointPort:        51820,
		PersistentKeepalive: 25,
	})

	if !strings.Contains(cfg, "PrivateKey = <INSERT_YOUR_PRIVATE_KEY>") {
		t.Errorf("expected placeholder private key in Mode 2 config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "Address = 10.10.0.2/24") {
		t.Errorf("expected Address = 10.10.0.2/24, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "Endpoint = vpn.example.com:51820") {
		t.Errorf("expected Endpoint, got:\n%s", cfg)
	}
}
