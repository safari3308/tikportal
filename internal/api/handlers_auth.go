package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/canhnm/tikportal/internal/auth"
)

func (s *Server) handlePublicConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"oidc_issuer":        s.cfg.OIDCIssuerURL,
		"oidc_client_id":     s.cfg.OIDCClientID,
		"oidc_client_secret": s.cfg.OIDCClientSecret,
		"oidc_scopes":        strings.Join(s.cfg.OIDCScopes, " "),
		"oidc_provider_name": s.cfg.OIDCProviderName,
		"oidc_enabled":       s.authValidator.OIDCEnabled(),
		"dev_auth_enabled":   s.cfg.DevAuthEnabled,
		"routeros_mock":      s.ros.IsMock(),
		"routeros_url":       s.cfg.RouterOSURL,
	})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	var user *auth.AuthenticatedUser
	var tokenString string

	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			tokenString = parts[1]
		}
	} else if tokenParam := r.URL.Query().Get("token"); tokenParam != "" {
		tokenString = tokenParam
	}

	if tokenString != "" {
		if u, err := s.authValidator.AuthenticateToken(r.Context(), tokenString); err == nil {
			user = u
		}
	}

	var quotaInfo map[string]any
	if user != nil {
		defaultMax, _ := s.store.GetDefaultMaxPeers(r.Context())
		count, _ := s.store.CountPeersByUser(r.Context(), user.Sub)
		quotaInfo = map[string]any{
			"used":  count,
			"limit": defaultMax,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated":    user != nil,
		"user":             user,
		"quota":            quotaInfo,
		"oidc_enabled":     s.authValidator.OIDCEnabled(),
		"dev_auth_enabled": s.cfg.DevAuthEnabled,
		"routeros_mock":    s.ros.IsMock(),
		"routeros_url":     s.cfg.RouterOSURL,
	})
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

	token := s.authValidator.CreateDevToken(email, name)
	user, err := s.authValidator.AuthenticateToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
