package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/canhnm/tikportal/internal/config"
	"github.com/canhnm/tikportal/internal/db"
	"github.com/canhnm/tikportal/internal/ipam"
	"github.com/canhnm/tikportal/internal/routeros"
	"github.com/canhnm/tikportal/internal/wireguard"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrBadInput  = errors.New("invalid input")
)

type PeerProvisionResult struct {
	Peer               *db.WGPeer `json:"peer"`
	ConfigText         string     `json:"config_text"`
	QRSupported        bool       `json:"qr_supported"`
	PrivateKeyIncluded bool       `json:"private_key_included"`
}

type VPNService struct {
	cfg   *config.Config
	store *db.Store
	ros   routeros.Client
}

func NewVPNService(cfg *config.Config, store *db.Store, ros routeros.Client) *VPNService {
	return &VPNService{
		cfg:   cfg,
		store: store,
		ros:   ros,
	}
}

func (s *VPNService) SeedMockNetworks(ctx context.Context) {
	nets, err := s.store.ListNetworks(ctx, false)
	if err != nil || len(nets) > 0 {
		return
	}
	_, _ = s.store.CreateNetwork(ctx, &db.WGNetwork{
		Name:                "Main VPN Network (10.10.0.0/24)",
		Description:         "Primary WireGuard VPN tunnel",
		ROSInterface:        "wg-main",
		ServerPublicKey:     "mTikPortalMainWireGuardServerPublicKey11111=",
		SubnetCIDR:          "10.10.0.0/24",
		ServerIP:            "10.10.0.1",
		EndpointHost:        "vpn.example.com",
		EndpointPort:        51820,
		DNSServers:          "10.10.0.1",
		AllowedIPs:          "0.0.0.0/0",
		PersistentKeepalive: 25,
		Enabled:             true,
	})
	_, _ = s.store.CreateNetwork(ctx, &db.WGNetwork{
		Name:                "Guest / Isolated VPN (10.20.0.0/24)",
		Description:         "Isolated internet-only WireGuard VPN tunnel",
		ROSInterface:        "wg-guest",
		ServerPublicKey:     "mTikPortalGuestWireGuardServerPublicKey2222=",
		SubnetCIDR:          "10.20.0.0/24",
		ServerIP:            "10.20.0.1",
		EndpointHost:        "vpn.example.com",
		EndpointPort:        51821,
		DNSServers:          "10.20.0.1",
		AllowedIPs:          "0.0.0.0/0",
		PersistentKeepalive: 25,
		Enabled:             true,
	})
}

func (s *VPNService) ProvisionPeer(
	ctx context.Context,
	user *db.User,
	networkID int64,
	rawPeerName string,
	rawCustomPubKey string,
) (*PeerProvisionResult, error) {
	peerName := strings.TrimSpace(rawPeerName)
	if peerName == "" {
		return nil, fmt.Errorf("%w: Connection name is required (e.g. 'MacBook Pro' or 'iPhone')", ErrBadInput)
	}
	if networkID <= 0 {
		return nil, fmt.Errorf("%w: Please select a WireGuard network", ErrBadInput)
	}

	privKey, pubKey, psk, keyMode, qrAllowed, err := resolveKeyPair(rawCustomPubKey)
	if err != nil {
		return nil, err
	}

	peer, netCfg, err := s.store.CreatePeerWithIPAM(
		ctx,
		user.ID,
		networkID,
		peerName,
		pubKey,
		psk,
		keyMode,
		func(network *db.WGNetwork, allocatedIP string) (string, error) {
			rosName := sanitizeROSPeerName(user.Email, peerName, allocatedIP)
			comment := fmt.Sprintf("tikportal:user=%s:device=%s", user.Email, peerName)
			return s.ros.AddPeer(ctx, routeros.AddPeerRequest{
				Interface:           network.ROSInterface,
				Name:                rosName,
				PublicKey:           pubKey,
				PresharedKey:        psk,
				AllowedAddress:      allocatedIP + "/32",
				PersistentKeepalive: network.PersistentKeepalive,
				Comment:             comment,
			})
		},
	)
	if err != nil {
		return nil, err
	}

	cfgText := BuildPeerConfigText(peer, netCfg, privKey)
	return &PeerProvisionResult{
		Peer:               peer,
		ConfigText:         cfgText,
		QRSupported:        qrAllowed,
		PrivateKeyIncluded: qrAllowed,
	}, nil
}

func (s *VPNService) RegeneratePeerKeys(
	ctx context.Context,
	actor *db.User,
	peerID int64,
	rawCustomPubKey string,
) (*PeerProvisionResult, error) {
	existing, err := s.store.GetPeerByID(ctx, peerID)
	if err != nil {
		return nil, err
	}
	if existing.UserID != actor.ID && actor.Role != "admin" {
		return nil, fmt.Errorf("%w: You do not have permission to modify this peer", ErrForbidden)
	}

	privKey, pubKey, psk, keyMode, qrAllowed, err := resolveKeyPair(rawCustomPubKey)
	if err != nil {
		return nil, err
	}

	netCfg, err := s.store.GetNetworkByID(ctx, existing.NetworkID)
	if err != nil {
		return nil, err
	}

	rosPeerID := existing.ROSPeerID
	if err := s.ros.UpdatePeerKeys(ctx, rosPeerID, pubKey, psk); err != nil {
		// If the peer was manually removed on RouterOS, re-add it with the same IP
		rosName := sanitizeROSPeerName(existing.UserEmail, existing.Name, existing.AllocatedIP)
		comment := fmt.Sprintf("tikportal:user=%s:device=%s", existing.UserEmail, existing.Name)
		newROSPeerID, addErr := s.ros.AddPeer(ctx, routeros.AddPeerRequest{
			Interface:           netCfg.ROSInterface,
			Name:                rosName,
			PublicKey:           pubKey,
			PresharedKey:        psk,
			AllowedAddress:      existing.AllocatedIP + "/32",
			PersistentKeepalive: netCfg.PersistentKeepalive,
			Comment:             comment,
		})
		if addErr != nil {
			return nil, fmt.Errorf("failed to update peer on RouterOS: %w", err)
		}
		rosPeerID = newROSPeerID
	}

	updated, err := s.store.UpdatePeerKeys(ctx, existing.ID, pubKey, psk, keyMode, rosPeerID)
	if err != nil {
		return nil, err
	}

	cfgText := BuildPeerConfigText(updated, netCfg, privKey)
	return &PeerProvisionResult{
		Peer:               updated,
		ConfigText:         cfgText,
		QRSupported:        qrAllowed,
		PrivateKeyIncluded: qrAllowed,
	}, nil
}

func (s *VPNService) GetPeerConfigTemplate(
	ctx context.Context,
	actor *db.User,
	peerID int64,
) (*db.WGPeer, string, error) {
	peer, err := s.store.GetPeerByID(ctx, peerID)
	if err != nil {
		return nil, "", err
	}
	if peer.UserID != actor.ID && actor.Role != "admin" {
		return nil, "", fmt.Errorf("%w: Forbidden", ErrForbidden)
	}
	netCfg, err := s.store.GetNetworkByID(ctx, peer.NetworkID)
	if err != nil {
		return nil, "", err
	}
	return peer, BuildPeerConfigText(peer, netCfg, ""), nil
}

func (s *VPNService) DeletePeer(ctx context.Context, actor *db.User, peerID int64) (string, error) {
	peer, err := s.store.GetPeerByID(ctx, peerID)
	if err != nil {
		return "", err
	}
	if peer.UserID != actor.ID && actor.Role != "admin" {
		return "", fmt.Errorf("%w: You do not have permission to delete this peer", ErrForbidden)
	}
	if err := s.ros.RemovePeer(ctx, peer.ROSPeerID); err != nil {
		return "", fmt.Errorf("failed to remove peer from RouterOS: %w", err)
	}
	if err := s.store.DeletePeer(ctx, peer.ID); err != nil {
		return "", err
	}
	return peer.AllocatedIP, nil
}

func (s *VPNService) EnrichPeersWithRouterOSStats(ctx context.Context, peers []db.WGPeer) {
	if len(peers) == 0 {
		return
	}
	rosPeers, err := s.ros.ListPeers(ctx, "")
	if err != nil {
		log.Printf("warn: failed to fetch live RouterOS peer stats: %v", err)
		return
	}
	byID := make(map[string]routeros.PeerInfo, len(rosPeers))
	byPub := make(map[string]routeros.PeerInfo, len(rosPeers))
	for _, rp := range rosPeers {
		byID[rp.ID] = rp
		byPub[rp.PublicKey] = rp
	}

	for i := range peers {
		rp, ok := byID[peers[i].ROSPeerID]
		if !ok {
			rp, ok = byPub[peers[i].PublicKey]
		}
		if ok {
			peers[i].LastHandshake = rp.LastHandshake
			peers[i].RxBytes = rp.RxBytes
			peers[i].TxBytes = rp.TxBytes
			peers[i].Online = rp.LastHandshake != "" && rp.LastHandshake != "never"
		}
	}
}

func (s *VPNService) ListAllowedROSInterfaces(ctx context.Context) ([]routeros.WGInterface, error) {
	ifaces, err := s.ros.ListWireGuardInterfaces(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]routeros.WGInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		if s.cfg.IsInterfaceAllowed(iface.Name, iface.Comment) {
			filtered = append(filtered, iface)
		}
	}
	return filtered, nil
}

func (s *VPNService) CreateNetwork(ctx context.Context, n *db.WGNetwork) (*db.WGNetwork, error) {
	if err := ValidateNetworkInput(n); err != nil {
		return nil, err
	}
	if err := s.verifyAllowedROSInterface(ctx, n.ROSInterface); err != nil {
		return nil, err
	}
	return s.store.CreateNetwork(ctx, n)
}

func (s *VPNService) UpdateNetwork(ctx context.Context, id int64, n *db.WGNetwork) (*db.WGNetwork, error) {
	if err := ValidateNetworkInput(n); err != nil {
		return nil, err
	}
	if err := s.verifyAllowedROSInterface(ctx, n.ROSInterface); err != nil {
		return nil, err
	}
	return s.store.UpdateNetwork(ctx, id, n)
}

func (s *VPNService) DeleteNetwork(ctx context.Context, id int64) error {
	peers, _ := s.store.ListAllPeers(ctx)
	for _, p := range peers {
		if p.NetworkID == id {
			_ = s.ros.RemovePeer(ctx, p.ROSPeerID)
		}
	}
	return s.store.DeleteNetwork(ctx, id)
}

func (s *VPNService) verifyAllowedROSInterface(ctx context.Context, rosInterfaceName string) error {
	ifaces, err := s.ros.ListWireGuardInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify interface on RouterOS: %w", err)
	}
	for _, iface := range ifaces {
		if strings.EqualFold(iface.Name, strings.TrimSpace(rosInterfaceName)) {
			if !s.cfg.IsInterfaceAllowed(iface.Name, iface.Comment) {
				return fmt.Errorf(
					"RouterOS interface %q is not permitted by TikPortal filter (ROUTEROS_INTERFACE_TAG=%q)",
					rosInterfaceName, s.cfg.RouterOSInterfaceTag,
				)
			}
			return nil
		}
	}
	return fmt.Errorf("RouterOS WireGuard interface %q does not exist on the router", rosInterfaceName)
}

func resolveKeyPair(rawCustomPubKey string) (privKey, pubKey, psk, keyMode string, qrAllowed bool, err error) {
	customPub := strings.TrimSpace(rawCustomPubKey)
	if customPub == "" {
		kp, genErr := wireguard.GenerateKeyPair()
		if genErr != nil {
			return "", "", "", "", false, fmt.Errorf("failed to generate WireGuard keys: %w", genErr)
		}
		return kp.PrivateKey, kp.PublicKey, kp.PresharedKey, "server", true, nil
	}

	if valErr := wireguard.ValidatePublicKey(customPub); valErr != nil {
		return "", "", "", "", false, fmt.Errorf("%w: %v", ErrBadInput, valErr)
	}
	generatedPSK, pskErr := wireguard.GeneratePresharedKey()
	if pskErr != nil {
		return "", "", "", "", false, pskErr
	}
	return "", customPub, generatedPSK, "custom", false, nil
}

func BuildPeerConfigText(peer *db.WGPeer, netCfg *db.WGNetwork, privateKey string) string {
	return wireguard.BuildClientConfig(wireguard.ClientConfigParams{
		PeerName:            peer.Name,
		NetworkName:         netCfg.Name,
		PrivateKey:          privateKey,
		AllocatedIP:         peer.AllocatedIP,
		SubnetPrefixLen:     32,
		DNSServers:          netCfg.DNSServers,
		ServerPublicKey:     netCfg.ServerPublicKey,
		PresharedKey:        peer.PresharedKey,
		AllowedIPs:          netCfg.AllowedIPs,
		EndpointHost:        netCfg.EndpointHost,
		EndpointPort:        netCfg.EndpointPort,
		PersistentKeepalive: netCfg.PersistentKeepalive,
	})
}

func ValidateNetworkInput(n *db.WGNetwork) error {
	n.Name = strings.TrimSpace(n.Name)
	n.ROSInterface = strings.TrimSpace(n.ROSInterface)
	n.ServerPublicKey = strings.TrimSpace(n.ServerPublicKey)
	n.EndpointHost = strings.TrimSpace(n.EndpointHost)
	if n.Name == "" {
		return errors.New("network name is required")
	}
	if n.ROSInterface == "" {
		return errors.New("RouterOS WireGuard interface name is required (e.g. wg-direct)")
	}
	if n.ServerPublicKey == "" {
		return errors.New("server public key is required")
	}
	if n.EndpointHost == "" || n.EndpointPort <= 0 || n.EndpointPort > 65535 {
		return errors.New("valid public endpoint host and UDP port (1-65535) are required")
	}
	if strings.TrimSpace(n.DNSServers) == "" {
		n.DNSServers = strings.TrimSpace(n.ServerIP)
	}
	if strings.TrimSpace(n.AllowedIPs) == "" {
		n.AllowedIPs = "0.0.0.0/0"
	}
	if n.PersistentKeepalive <= 0 {
		n.PersistentKeepalive = 25
	}
	_, err := ipam.ValidateSubnet(n.SubnetCIDR, n.ServerIP)
	return err
}

func sanitizeROSPeerName(email, deviceName, ip string) string {
	userPart := strings.Split(email, "@")[0]
	clean := func(s string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(s) {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				b.WriteRune(r)
			} else if r == ' ' || r == '_' || r == '.' {
				b.WriteRune('-')
			}
		}
		return strings.Trim(b.String(), "-")
	}
	return fmt.Sprintf("tp-%s-%s-%s", clean(userPart), clean(deviceName), strings.ReplaceAll(ip, ".", "-"))
}
