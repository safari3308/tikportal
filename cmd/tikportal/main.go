package main

import (
	"log"
	"net/http"
	"time"

	"github.com/canhnm/tikportal"
	"github.com/canhnm/tikportal/internal/api"
	"github.com/canhnm/tikportal/internal/auth"
	"github.com/canhnm/tikportal/internal/config"
	"github.com/canhnm/tikportal/internal/db"
	"github.com/canhnm/tikportal/internal/routeros"
)

func main() {
	cfg := config.Load()

	store, err := db.Open(cfg.DBPath, cfg.DefaultMaxPeersTotal)
	if err != nil {
		log.Fatalf("failed to open SQLite database (%s): %v", cfg.DBPath, err)
	}
	defer store.Close()

	var rosClient routeros.Client
	if cfg.RouterOSMock {
		log.Printf("RouterOS client starting in MOCK mode (set ROUTEROS_URL and ROUTEROS_MOCK=false for RB5009 REST API)")
		rosClient = routeros.NewMockClient()
	} else {
		log.Printf("RouterOS client targeting RB5009 REST API at %s", cfg.RouterOSURL)
		rosClient = routeros.NewRESTClient(
			cfg.RouterOSURL,
			cfg.RouterOSUser,
			cfg.RouterOSPassword,
			cfg.RouterOSInsecureSkipTLSVerify,
		)
	}

	authMgr := auth.NewManager(cfg)
	feFS, err := tikportal.FrontendFS()
	if err != nil {
		log.Printf("warn: embedded frontend filesystem not available: %v", err)
	}

	srv := api.NewServer(cfg, store, rosClient, authMgr, feFS)

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("TikPortal listening on %s (BaseURL: %s)", cfg.ListenAddr, cfg.BaseURL)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server exited: %v", err)
	}
}
