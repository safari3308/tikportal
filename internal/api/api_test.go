package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/canhnm/tikportal/internal/auth"
	"github.com/canhnm/tikportal/internal/config"
	"github.com/canhnm/tikportal/internal/db"
	"github.com/canhnm/tikportal/internal/routeros"
)

func setupTestServer(t *testing.T) (*Server, *auth.Validator, *db.Store) {
	t.Helper()

	cfg := &config.Config{
		DefaultMaxPeersTotal: 5,
		AdminEmails:          []string{"admin@localhost"},
		DevAuthEnabled:       true,
		RouterOSMock:         true,
	}

	store, err := db.Open(":memory:", 5)
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}

	rosClient := routeros.NewMockClient()
	authValidator := auth.NewValidator("", "", "", true, cfg.AdminEmails)

	srv := NewServer(cfg, store, rosClient, authValidator, nil)
	return srv, authValidator, store
}

func TestPublicConfigEndpoint(t *testing.T) {
	srv, _, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest("GET", "/api/config", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var res map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if res["dev_auth_enabled"] != true || res["routeros_mock"] != true {
		t.Fatalf("unexpected config response: %+v", res)
	}
}

func TestAuthAndRBAC(t *testing.T) {
	srv, validator, store := setupTestServer(t)
	defer store.Close()

	adminToken := validator.CreateDevToken("admin@localhost", "Admin")
	userToken := validator.CreateDevToken("alice@example.com", "Alice")

	// 1. Unauthenticated request to /api/peers -> 401
	{
		req := httptest.NewRequest("GET", "/api/peers", nil)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	}

	// 2. Authenticated request to /api/peers with user token -> 200
	{
		req := httptest.NewRequest("GET", "/api/peers", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	}

	// 3. User request to admin endpoint -> 403 Forbidden
	{
		req := httptest.NewRequest("GET", "/api/admin/networks", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	}

	// 4. Admin request to admin endpoint -> 200 OK
	{
		req := httptest.NewRequest("GET", "/api/admin/networks", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	}
}

func TestPeerLifecycleWithIPAM(t *testing.T) {
	srv, validator, store := setupTestServer(t)
	defer store.Close()

	userToken := validator.CreateDevToken("bob@example.com", "Bob")

	// 1. List networks to get a network ID
	nets, err := store.ListNetworks(context.Background(), true)
	if err != nil || len(nets) == 0 {
		t.Fatalf("expected mock networks seeded, got %v", nets)
	}
	netID := nets[0].ID

	// 2. Create peer
	payload := map[string]any{
		"network_id": netID,
		"name":       "Bob Laptop",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/peers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var provRes map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&provRes); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	peerData, ok := provRes["peer"].(map[string]any)
	if !ok {
		t.Fatalf("missing peer in response: %+v", provRes)
	}
	if peerData["allocated_ip"] != "10.10.0.2" {
		t.Fatalf("expected allocated_ip 10.10.0.2, got %v", peerData["allocated_ip"])
	}

	peerID := int64(peerData["id"].(float64))

	// 3. List my peers
	{
		req := httptest.NewRequest("GET", "/api/peers", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		var listRes struct {
			Peers []db.WGPeer `json:"peers"`
			Quota struct {
				Used  int `json:"used"`
				Limit int `json:"limit"`
			} `json:"quota"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&listRes); err != nil {
			t.Fatalf("decode list: %v", err)
		}
		if len(listRes.Peers) != 1 || listRes.Quota.Used != 1 {
			t.Fatalf("unexpected peers list: %+v", listRes)
		}
	}

	// 4. Delete peer
	{
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/peers/%d", peerID), nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
		}
	}

	// 5. Verify peer list is now empty
	{
		req := httptest.NewRequest("GET", "/api/peers", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		var listRes struct {
			Peers []db.WGPeer `json:"peers"`
		}
		_ = json.NewDecoder(rr.Body).Decode(&listRes)
		if len(listRes.Peers) != 0 {
			t.Fatalf("expected 0 peers, got %d", len(listRes.Peers))
		}
	}
}
