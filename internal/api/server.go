package api

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/canhnm/tikportal/internal/auth"
	"github.com/canhnm/tikportal/internal/config"
	"github.com/canhnm/tikportal/internal/db"
	"github.com/canhnm/tikportal/internal/routeros"
	"github.com/canhnm/tikportal/internal/service"
)

type Server struct {
	cfg           *config.Config
	store         *db.Store
	ros           routeros.Client
	vpn           *service.VPNService
	authValidator *auth.Validator
	frontend      fs.FS
}

func NewServer(
	cfg *config.Config,
	store *db.Store,
	ros routeros.Client,
	authValidator *auth.Validator,
	frontend fs.FS,
) *Server {
	vpnSvc := service.NewVPNService(cfg, store, ros)
	s := &Server{
		cfg:           cfg,
		store:         store,
		ros:           ros,
		vpn:           vpnSvc,
		authValidator: authValidator,
		frontend:      frontend,
	}
	if ros.IsMock() {
		vpnSvc.SeedMockNetworks(context.Background())
	}
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public config & status endpoints
	mux.HandleFunc("GET /api/config", s.handlePublicConfig)
	mux.HandleFunc("GET /api/auth/me", s.handleAuthMe)
	mux.HandleFunc("POST /api/auth/dev-login", s.handleDevLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// User endpoints (Bearer token required)
	mux.Handle("GET /api/networks", s.requireAuth(http.HandlerFunc(s.handleListNetworks)))
	mux.Handle("GET /api/peers", s.requireAuth(http.HandlerFunc(s.handleListMyPeers)))
	mux.Handle("POST /api/peers", s.requireAuth(http.HandlerFunc(s.handleCreatePeer)))
	mux.Handle("POST /api/peers/{id}/regenerate", s.requireAuth(http.HandlerFunc(s.handleRegeneratePeer)))
	mux.Handle("GET /api/peers/{id}/config", s.requireAuth(http.HandlerFunc(s.handleDownloadConfigTemplate)))
	mux.Handle("DELETE /api/peers/{id}", s.requireAuth(http.HandlerFunc(s.handleDeletePeer)))

	// Admin endpoints (Bearer token + Admin role required)
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
	return s.authValidator.Middleware(next)
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return s.authValidator.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)
		if user == nil || user.Role != "admin" {
			writeError(w, http.StatusForbidden, "Admin privileges required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func currentUser(r *http.Request) *auth.AuthenticatedUser {
	return auth.UserFromContext(r.Context())
}
