package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/canhnm/tikportal/internal/auth"
	"github.com/canhnm/tikportal/internal/config"
	"github.com/canhnm/tikportal/internal/db"
	"github.com/canhnm/tikportal/internal/ipam"
	"github.com/canhnm/tikportal/internal/routeros"
	"github.com/canhnm/tikportal/internal/wireguard"
)

type contextKey string

const userContextKey contextKey = "tikportal_user"

type Server struct {
	cfg      *config.Config
	store    *db.Store
	ros      routeros.Client
	authMgr  *auth.Manager
	frontend fs.FS
}

func NewServer(cfg *config.Config, store *db.Store, ros routeros.Client, authMgr *auth.Manager, frontend fs.FS) *Server {
	s := &Server{
		cfg:      cfg,
		store:    store,
		ros:      ros,
		authMgr:  authMgr,
		frontend: frontend,
	}
	if ros.IsMock() {
		s.seedMockNetworks(context.Background())
	}
	return s
}

func (s *Server) seedMockNetworks(ctx context.Context) {
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

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Auth & Status endpoints
	mux.HandleFunc("GET /api/auth/me", s.handleAuthMe)
	mux.HandleFunc("GET /api/auth/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /api/auth/callback", s.handleOIDCCallback)
	mux.HandleFunc("POST /api/auth/dev-login", s.handleDevLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// User endpoints
	mux.Handle("GET /api/networks", s.requireAuth(http.HandlerFunc(s.handleListNetworks)))
	mux.Handle("GET /api/peers", s.requireAuth(http.HandlerFunc(s.handleListMyPeers)))
	mux.Handle("POST /api/peers", s.requireAuth(http.HandlerFunc(s.handleCreatePeer)))
	mux.Handle("POST /api/peers/{id}/regenerate", s.requireAuth(http.HandlerFunc(s.handleRegeneratePeer)))
	mux.Handle("GET /api/peers/{id}/config", s.requireAuth(http.HandlerFunc(s.handleDownloadConfigTemplate)))
	mux.Handle("DELETE /api/peers/{id}", s.requireAuth(http.HandlerFunc(s.handleDeletePeer)))

	// Admin endpoints
	mux.Handle("GET /api/admin/routeros/interfaces", s.requireAdmin(http.HandlerFunc(s.handleAdminROSInterfaces)))
	mux.Handle("GET /api/admin/networks", s.requireAdmin(http.HandlerFunc(s.handleAdminListNetworks)))
	mux.Handle("POST /api/admin/networks", s.requireAdmin(http.HandlerFunc(s.handleAdminCreateNetwork)))
	mux.Handle("PUT /api/admin/networks/{id}", s.requireAdmin(http.HandlerFunc(s.handleAdminUpdateNetwork)))
	mux.Handle("DELETE /api/admin/networks/{id}", s.requireAdmin(http.HandlerFunc(s.handleAdminDeleteNetwork)))
	mux.Handle("GET /api/admin/users", s.requireAdmin(http.HandlerFunc(s.handleAdminListUsers)))
	mux.Handle("PUT /api/admin/users/{id}", s.requireAdmin(http.HandlerFunc(s.handleAdminUpdateUser)))
	mux.Handle("GET /api/admin/peers", s.requireAdmin(http.HandlerFunc(s.handleAdminListAllPeers)))
	mux.Handle("GET /api/admin/settings", s.requireAdmin(http.HandlerFunc(s.handleAdminGetSettings)))
	mux.Handle("PUT /api/admin/settings", s.requireAdmin(http.HandlerFunc(s.handleAdminUpdateSettings)))

	// Embedded React SPA handler for all non-API routes
	mux.Handle("/", s.spaHandler())

	return mux
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := s.authMgr.ParseSessionRequest(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		user, err := s.store.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			s.authMgr.ClearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "User session no longer valid")
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)
		if user == nil || user.Role != "admin" {
			writeError(w, http.StatusForbidden, "Admin privileges required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func currentUser(r *http.Request) *db.User {
	u, _ := r.Context().Value(userContextKey).(*db.User)
	return u
}

// Auth Handlers

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	var user *db.User
	if claims, err := s.authMgr.ParseSessionRequest(r); err == nil {
		user, _ = s.store.GetUserByID(r.Context(), claims.UserID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated":    user != nil,
		"user":             user,
		"oidc_enabled":     s.authMgr.OIDCEnabled(),
		"dev_auth_enabled": s.cfg.DevAuthEnabled,
		"routeros_mock":    s.ros.IsMock(),
		"routeros_url":     s.cfg.RouterOSURL,
	})
}

func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if !s.authMgr.OIDCEnabled() {
		writeError(w, http.StatusBadRequest, "OIDC is not configured on this server")
		return
	}
	authURL, err := s.authMgr.StartOIDCLogin(r.Context(), w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to initiate OIDC login: %v", err))
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	info, err := s.authMgr.ExchangeOIDCCallback(r.Context(), w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("OIDC callback failed: %v", err))
		return
	}
	isAdmin := s.authMgr.IsAdminEmail(info.Email)
	user, err := s.store.UpsertUser(r.Context(), info.Sub, info.Email, info.Name, isAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to persist user: %v", err))
		return
	}
	if err := s.authMgr.IssueSessionCookie(w, user.ID, user.Email, user.Role); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to issue session cookie")
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.DevAuthEnabled {
		writeError(w, http.StatusForbidden, "Dev login is disabled")
		return
	}
	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "A valid email address is required")
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	isAdmin := s.authMgr.IsAdminEmail(email)
	user, err := s.store.UpsertUser(r.Context(), "dev:"+email, email, name, isAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.authMgr.IssueSessionCookie(w, user.ID, user.Email, user.Role); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	s.authMgr.ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// User Peer & Network Handlers

func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	nets, err := s.store.ListNetworks(r.Context(), true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"networks": nets})
}

func (s *Server) handleListMyPeers(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	peers, err := s.store.ListPeersByUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.enrichPeersWithRouterOSStats(r.Context(), peers)
	writeJSON(w, http.StatusOK, map[string]any{
		"peers": peers,
		"quota": map[string]int{
			"used":  len(peers),
			"limit": user.EffectiveMax,
		},
	})
}

type peerProvisionResponse struct {
	Peer               *db.WGPeer `json:"peer"`
	ConfigText         string     `json:"config_text"`
	QRSupported        bool       `json:"qr_supported"`
	PrivateKeyIncluded bool       `json:"private_key_included"`
}

func (s *Server) handleCreatePeer(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var req struct {
		NetworkID int64  `json:"network_id"`
		Name      string `json:"name"`
		PublicKey string `json:"public_key"` // Optional: empty = Mode 1 (server generates), non-empty = Mode 2 (user provides)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	peerName := strings.TrimSpace(req.Name)
	if peerName == "" {
		writeError(w, http.StatusBadRequest, "Connection name is required (e.g. 'MacBook Pro' or 'iPhone')")
		return
	}
	if req.NetworkID <= 0 {
		writeError(w, http.StatusBadRequest, "Please select a WireGuard network")
		return
	}

	var (
		privKey   string
		pubKey    string
		psk       string
		keyMode   string
		qrAllowed bool
	)

	customPub := strings.TrimSpace(req.PublicKey)
	if customPub == "" {
		// Mode 1: Server generates Curve25519 keypair + PSK
		kp, err := wireguard.GenerateKeyPair()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate WireGuard keys: %v", err))
			return
		}
		privKey = kp.PrivateKey
		pubKey = kp.PublicKey
		psk = kp.PresharedKey
		keyMode = "server"
		qrAllowed = true
	} else {
		// Mode 2: User provided their own public key -> validate & generate PSK; no QR code
		if err := wireguard.ValidatePublicKey(customPub); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		generatedPSK, err := wireguard.GeneratePresharedKey()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		pubKey = customPub
		psk = generatedPSK
		keyMode = "custom"
		qrAllowed = false
	}

	peer, netCfg, err := s.store.CreatePeerWithIPAM(
		r.Context(),
		user.ID,
		req.NetworkID,
		peerName,
		pubKey,
		psk,
		keyMode,
		func(network *db.WGNetwork, allocatedIP string) (string, error) {
			rosName := sanitizeROSPeerName(user.Email, peerName, allocatedIP)
			comment := fmt.Sprintf("tikportal:user=%s:device=%s", user.Email, peerName)
			return s.ros.AddPeer(r.Context(), routeros.AddPeerRequest{
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
		switch {
		case errors.Is(err, db.ErrQuotaExceeded):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, db.ErrNetworkOff):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ipam.ErrSubnetExhausted):
			writeError(w, http.StatusConflict, err.Error())
		case strings.Contains(err.Error(), "UNIQUE constraint failed: wg_peers.public_key"):
			writeError(w, http.StatusConflict, "This WireGuard public key is already registered on another connection")
		default:
			writeError(w, http.StatusBadGateway, fmt.Sprintf("Failed to provision peer: %v", err))
		}
		return
	}

	cfgText := s.buildPeerConfigText(peer, netCfg, privKey)
	writeJSON(w, http.StatusCreated, peerProvisionResponse{
		Peer:               peer,
		ConfigText:         cfgText,
		QRSupported:        qrAllowed,
		PrivateKeyIncluded: qrAllowed,
	})
}

// handleRegeneratePeer rotates the keys for an existing peer in-place on RouterOS and SQLite,
// preserving the peer's allocated IP address so the user does not have to delete and recreate.
func (s *Server) handleRegeneratePeer(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	peerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid peer ID")
		return
	}

	existing, err := s.store.GetPeerByID(r.Context(), peerID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Peer not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing.UserID != user.ID && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "You do not have permission to modify this peer")
		return
	}

	var req struct {
		PublicKey string `json:"public_key"` // Optional: empty = server generates new keypair + QR, non-empty = Mode 2 custom public key
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	var (
		privKey   string
		pubKey    string
		psk       string
		keyMode   string
		qrAllowed bool
	)

	customPub := strings.TrimSpace(req.PublicKey)
	if customPub == "" {
		kp, err := wireguard.GenerateKeyPair()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		privKey = kp.PrivateKey
		pubKey = kp.PublicKey
		psk = kp.PresharedKey
		keyMode = "server"
		qrAllowed = true
	} else {
		if err := wireguard.ValidatePublicKey(customPub); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		generatedPSK, err := wireguard.GeneratePresharedKey()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		pubKey = customPub
		psk = generatedPSK
		keyMode = "custom"
		qrAllowed = false
	}

	netCfg, err := s.store.GetNetworkByID(r.Context(), existing.NetworkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rosPeerID := existing.ROSPeerID
	// Update peer keys in-place on RouterOS
	if err := s.ros.UpdatePeerKeys(r.Context(), rosPeerID, pubKey, psk); err != nil {
		// If the peer was manually removed on RouterOS, re-add it with the same IP
		rosName := sanitizeROSPeerName(existing.UserEmail, existing.Name, existing.AllocatedIP)
		comment := fmt.Sprintf("tikportal:user=%s:device=%s", existing.UserEmail, existing.Name)
		newROSPeerID, addErr := s.ros.AddPeer(r.Context(), routeros.AddPeerRequest{
			Interface:           netCfg.ROSInterface,
			Name:                rosName,
			PublicKey:           pubKey,
			PresharedKey:        psk,
			AllowedAddress:      existing.AllocatedIP + "/32",
			PersistentKeepalive: netCfg.PersistentKeepalive,
			Comment:             comment,
		})
		if addErr != nil {
			writeError(w, http.StatusBadGateway, fmt.Sprintf("Failed to update peer on RouterOS: %v", err))
			return
		}
		rosPeerID = newROSPeerID
	}

	updated, err := s.store.UpdatePeerKeys(r.Context(), existing.ID, pubKey, psk, keyMode, rosPeerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	cfgText := s.buildPeerConfigText(updated, netCfg, privKey)
	writeJSON(w, http.StatusOK, peerProvisionResponse{
		Peer:               updated,
		ConfigText:         cfgText,
		QRSupported:        qrAllowed,
		PrivateKeyIncluded: qrAllowed,
	})
}

func (s *Server) handleDownloadConfigTemplate(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	peerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid peer ID")
		return
	}
	peer, err := s.store.GetPeerByID(r.Context(), peerID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Peer not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if peer.UserID != user.ID && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "Forbidden")
		return
	}
	netCfg, err := s.store.GetNetworkByID(r.Context(), peer.NetworkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	cfgText := s.buildPeerConfigText(peer, netCfg, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"peer":        peer,
		"config_text": cfgText,
	})
}

func (s *Server) handleDeletePeer(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	peerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid peer ID")
		return
	}
	peer, err := s.store.GetPeerByID(r.Context(), peerID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Peer not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if peer.UserID != user.ID && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "You do not have permission to delete this peer")
		return
	}

	if err := s.ros.RemovePeer(r.Context(), peer.ROSPeerID); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Failed to remove peer from RouterOS: %v", err))
		return
	}
	if err := s.store.DeletePeer(r.Context(), peer.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":     true,
		"released_ip": peer.AllocatedIP,
	})
}

// Admin Handlers

func (s *Server) handleAdminROSInterfaces(w http.ResponseWriter, r *http.Request) {
	ifaces, err := s.ros.ListWireGuardInterfaces(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Failed to query RouterOS WireGuard interfaces: %v", err))
		return
	}
	filtered := make([]routeros.WGInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		if s.cfg.IsInterfaceAllowed(iface.Name, iface.Comment) {
			filtered = append(filtered, iface)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"interfaces":         filtered,
		"mock":               s.ros.IsMock(),
		"interface_tag":      s.cfg.RouterOSInterfaceTag,
		"allowed_interfaces": s.cfg.RouterOSAllowedInterfaces,
	})
}

func (s *Server) verifyAllowedROSInterface(ctx context.Context, rosInterfaceName string) error {
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

func (s *Server) handleAdminListNetworks(w http.ResponseWriter, r *http.Request) {
	nets, err := s.store.ListNetworks(r.Context(), false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"networks": nets})
}

func (s *Server) handleAdminCreateNetwork(w http.ResponseWriter, r *http.Request) {
	var n db.WGNetwork
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if err := validateNetworkInput(&n); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.verifyAllowedROSInterface(r.Context(), n.ROSInterface); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.store.CreateNetwork(r.Context(), &n)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"network": created})
}

func (s *Server) handleAdminUpdateNetwork(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid network ID")
		return
	}
	var n db.WGNetwork
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if err := validateNetworkInput(&n); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.verifyAllowedROSInterface(r.Context(), n.ROSInterface); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.store.UpdateNetwork(r.Context(), id, &n)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"network": updated})
}

func (s *Server) handleAdminDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid network ID")
		return
	}
	// Remove any existing peers on RouterOS for this network before deleting
	peers, _ := s.store.ListAllPeers(r.Context())
	for _, p := range peers {
		if p.NetworkID == id {
			_ = s.ros.RemovePeer(r.Context(), p.ROSPeerID)
		}
	}
	if err := s.store.DeleteNetwork(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defaultMax, _ := s.store.GetDefaultMaxPeers(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"users":                      users,
		"default_max_peers_per_user": defaultMax,
	})
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	var req struct {
		Role             string `json:"role"`
		MaxPeersOverride *int   `json:"max_peers_override"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	updated, err := s.store.UpdateUserPolicy(r.Context(), id, req.Role, req.MaxPeersOverride)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

func (s *Server) handleAdminListAllPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := s.store.ListAllPeers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.enrichPeersWithRouterOSStats(r.Context(), peers)
	writeJSON(w, http.StatusOK, map[string]any{"peers": peers})
}

func (s *Server) handleAdminGetSettings(w http.ResponseWriter, r *http.Request) {
	defaultMax, err := s.store.GetDefaultMaxPeers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"default_max_peers_per_user": defaultMax,
		"routeros_mock":              s.ros.IsMock(),
		"routeros_url":               s.cfg.RouterOSURL,
		"oidc_enabled":               s.authMgr.OIDCEnabled(),
		"oidc_issuer_url":            s.cfg.OIDCIssuerURL,
	})
}

func (s *Server) handleAdminUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DefaultMaxPeersPerUser int `json:"default_max_peers_per_user"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if req.DefaultMaxPeersPerUser < 0 || req.DefaultMaxPeersPerUser > 1000 {
		writeError(w, http.StatusBadRequest, "default_max_peers_per_user must be between 0 and 1000")
		return
	}
	if err := s.store.SetDefaultMaxPeers(r.Context(), req.DefaultMaxPeersPerUser); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"default_max_peers_per_user": req.DefaultMaxPeersPerUser,
	})
}

// Helpers

func (s *Server) buildPeerConfigText(peer *db.WGPeer, netCfg *db.WGNetwork, privateKey string) string {
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

func (s *Server) enrichPeersWithRouterOSStats(ctx context.Context, peers []db.WGPeer) {
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

func validateNetworkInput(n *db.WGNetwork) error {
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
		// Default client DNS to the router's WireGuard gateway IP on that subnet
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

func (s *Server) spaHandler() http.Handler {
	if s.frontend == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("TikPortal API server is running. Build frontend in web/dist to serve UI."))
		})
	}
	fileServer := http.FileServer(http.FS(s.frontend))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if f, err := s.frontend.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// Fallback to index.html for client-side routing
		r2 := new(http.Request)
		*r2 = *r
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
