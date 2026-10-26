package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/canhnm/tikportal/internal/db"
)

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
