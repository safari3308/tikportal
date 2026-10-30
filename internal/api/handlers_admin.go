package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/canhnm/tikportal/internal/db"
)

func (s *Server) handleAdminROSInterfaces(w http.ResponseWriter, r *http.Request) {
	filtered, err := s.vpn.ListAllowedROSInterfaces(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Failed to query RouterOS WireGuard interfaces: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"interfaces":         filtered,
		"mock":               s.ros.IsMock(),
		"interface_tag":      s.cfg.RouterOSInterfaceTag,
		"allowed_interfaces": s.cfg.RouterOSAllowedInterfaces,
	})
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
	created, err := s.vpn.CreateNetwork(r.Context(), &n)
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
	updated, err := s.vpn.UpdateNetwork(r.Context(), id, &n)
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
	if err := s.vpn.DeleteNetwork(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListActiveUsersFromPeers(r.Context(), s.cfg.AdminEmails)
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
	// With stateless auth, roles are managed via OIDC claims/ADMIN_EMAILS and limits via global settings.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminListAllPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := s.store.ListAllPeers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.vpn.EnrichPeersWithRouterOSStats(r.Context(), peers)
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
		"oidc_enabled":               s.authValidator.OIDCEnabled(),
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
