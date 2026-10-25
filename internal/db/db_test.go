package db

import (
	"context"
	"errors"
	"testing"
)

func TestCreatePeerQuotaAndIPReuse(t *testing.T) {
	ctx := context.Background()
	store, err := Open(":memory:", 2) // Max 2 peers globally per user
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}
	defer store.Close()

	user, err := store.UpsertUser(ctx, "sub-1", "alice@example.com", "Alice", false)
	if err != nil {
		t.Fatalf("UpsertUser failed: %v", err)
	}
	// First user should be auto-promoted to admin
	if user.Role != "admin" {
		t.Fatalf("expected first user to be admin, got %s", user.Role)
	}

	net1, err := store.CreateNetwork(ctx, &WGNetwork{
		Name:                "Direct PPPoE",
		Description:         "10.10.0.0/24 via dual PPPoE",
		ROSInterface:        "wg-direct",
		ServerPublicKey:     "mTikDirectPPPoEServerPublicKey1111111111111=",
		SubnetCIDR:          "10.10.0.0/24",
		ServerIP:            "10.10.0.1",
		EndpointHost:        "vpn.example.com",
		EndpointPort:        51820,
		DNSServers:          "1.1.1.1",
		AllowedIPs:          "0.0.0.0/0",
		PersistentKeepalive: 25,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("CreateNetwork failed: %v", err)
	}

	mockProvision := func(_ *WGNetwork, allocatedIP string) (string, error) {
		return "*peer-" + allocatedIP, nil
	}

	// Peer 1 -> gets 10.10.0.2
	p1, _, err := store.CreatePeerWithIPAM(ctx, user.ID, net1.ID, "Laptop", "pub1", "psk1", "server", mockProvision)
	if err != nil {
		t.Fatalf("CreatePeer 1 failed: %v", err)
	}
	if p1.AllocatedIP != "10.10.0.2" {
		t.Fatalf("expected 10.10.0.2, got %s", p1.AllocatedIP)
	}

	// Peer 2 -> gets 10.10.0.3
	p2, _, err := store.CreatePeerWithIPAM(ctx, user.ID, net1.ID, "Phone", "pub2", "psk2", "server", mockProvision)
	if err != nil {
		t.Fatalf("CreatePeer 2 failed: %v", err)
	}
	if p2.AllocatedIP != "10.10.0.3" {
		t.Fatalf("expected 10.10.0.3, got %s", p2.AllocatedIP)
	}

	// Peer 3 -> should fail with ErrQuotaExceeded (global limit = 2)
	_, _, err = store.CreatePeerWithIPAM(ctx, user.ID, net1.ID, "Tablet", "pub3", "psk3", "server", mockProvision)
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}

	// Delete Peer 1 (10.10.0.2) -> frees both quota and IP 10.10.0.2
	if err := store.DeletePeer(ctx, p1.ID); err != nil {
		t.Fatalf("DeletePeer failed: %v", err)
	}

	// Create Peer 3 -> should now succeed and reuse 10.10.0.2!
	p3, _, err := store.CreatePeerWithIPAM(ctx, user.ID, net1.ID, "Tablet", "pub3", "psk3", "custom", mockProvision)
	if err != nil {
		t.Fatalf("CreatePeer 3 after delete failed: %v", err)
	}
	if p3.AllocatedIP != "10.10.0.2" {
		t.Fatalf("expected 10.10.0.2 to be reused, got %s", p3.AllocatedIP)
	}
}
