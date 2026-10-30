package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/canhnm/tikportal/internal/db"
	"github.com/canhnm/tikportal/internal/ipam"
	"github.com/canhnm/tikportal/internal/service"
)

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
	peers, err := s.store.ListPeersByUser(r.Context(), user.Sub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.vpn.EnrichPeersWithRouterOSStats(r.Context(), peers)
	defaultMax, _ := s.store.GetDefaultMaxPeers(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"peers": peers,
		"quota": map[string]int{
			"used":  len(peers),
			"limit": defaultMax,
		},
	})
}

func (s *Server) handleCreatePeer(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var req struct {
		NetworkID int64  `json:"network_id"`
		Name      string `json:"name"`
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	res, err := s.vpn.ProvisionPeer(r.Context(), user, req.NetworkID, req.Name, req.PublicKey)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBadInput):
			writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "invalid input: "))
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

	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) handleRegeneratePeer(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	peerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid peer ID")
		return
	}

	var req struct {
		PublicKey string `json:"public_key"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	res, err := s.vpn.RegeneratePeerKeys(r.Context(), user, peerID, req.PublicKey)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			writeError(w, http.StatusNotFound, "Peer not found")
		case errors.Is(err, service.ErrForbidden):
			writeError(w, http.StatusForbidden, strings.TrimPrefix(err.Error(), "forbidden: "))
		case errors.Is(err, service.ErrBadInput):
			writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "invalid input: "))
		default:
			writeError(w, http.StatusBadGateway, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDownloadConfigTemplate(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	peerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid peer ID")
		return
	}

	peer, cfgText, err := s.vpn.GetPeerConfigTemplate(r.Context(), user, peerID)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			writeError(w, http.StatusNotFound, "Peer not found")
		case errors.Is(err, service.ErrForbidden):
			writeError(w, http.StatusForbidden, "Forbidden")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

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

	releasedIP, err := s.vpn.DeletePeer(r.Context(), user, peerID)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			writeError(w, http.StatusNotFound, "Peer not found")
		case errors.Is(err, service.ErrForbidden):
			writeError(w, http.StatusForbidden, "You do not have permission to delete this peer")
		default:
			writeError(w, http.StatusBadGateway, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":     true,
		"released_ip": releasedIP,
	})
}
