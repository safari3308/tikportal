package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/canhnm/tikportal/internal/ipam"
)

type WGPeer struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	UserEmail    string    `json:"user_email,omitempty"`
	UserName     string    `json:"user_name,omitempty"`
	NetworkID    int64     `json:"network_id"`
	NetworkName  string    `json:"network_name"`
	ROSInterface string    `json:"ros_interface"`
	ROSPeerID    string    `json:"ros_peer_id"`
	Name         string    `json:"name"`
	AllocatedIP  string    `json:"allocated_ip"`
	PublicKey    string    `json:"public_key"`
	PresharedKey string    `json:"-"`
	KeyMode      string    `json:"key_mode"` // "server" | "custom"
	CreatedAt    time.Time `json:"created_at"`

	// Live telemetry from RouterOS (populated at query time)
	LastHandshake string `json:"last_handshake,omitempty"`
	RxBytes       int64  `json:"rx_bytes"`
	TxBytes       int64  `json:"tx_bytes"`
	Online        bool   `json:"online"`
}

type ProvisionCallback func(network *WGNetwork, allocatedIP string) (rosPeerID string, err error)

// CreatePeerWithIPAM atomically verifies the user's global peer quota, allocates the lowest
// unused IP address in the network subnet (reusing any previously deleted peer IPs),
// calls the RouterOS provision callback, and inserts the peer row.
func (s *Store) CreatePeerWithIPAM(
	ctx context.Context,
	userID int64,
	networkID int64,
	peerName string,
	publicKey string,
	presharedKey string,
	keyMode string,
	provisionFn ProvisionCallback,
) (*WGPeer, *WGNetwork, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if user.PeerCount >= user.EffectiveMax {
		return nil, nil, ErrQuotaExceeded
	}

	netCfg, err := s.GetNetworkByID(ctx, networkID)
	if err != nil {
		return nil, nil, err
	}
	if !netCfg.Enabled {
		return nil, nil, ErrNetworkOff
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Re-verify quota inside transaction
	var currentCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM wg_peers WHERE user_id = ?`, userID).Scan(&currentCount); err != nil {
		return nil, nil, err
	}
	if currentCount >= user.EffectiveMax {
		return nil, nil, ErrQuotaExceeded
	}

	// Fetch all currently allocated IPs for this network
	rows, err := tx.QueryContext(ctx, `SELECT allocated_ip FROM wg_peers WHERE network_id = ? ORDER BY id ASC`, networkID)
	if err != nil {
		return nil, nil, err
	}
	var usedIPs []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		usedIPs = append(usedIPs, ip)
	}
	_ = rows.Close()

	allocatedIP, err := ipam.AllocateNextIP(netCfg.SubnetCIDR, netCfg.ServerIP, usedIPs)
	if err != nil {
		return nil, nil, err
	}

	// Provision peer on RouterOS before committing to SQLite
	rosPeerID, err := provisionFn(netCfg, allocatedIP)
	if err != nil {
		return nil, nil, err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO wg_peers (
			user_id, network_id, ros_peer_id, name, allocated_ip, public_key, preshared_key, key_mode
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, networkID, rosPeerID, peerName, allocatedIP, publicKey, presharedKey, keyMode)
	if err != nil {
		return nil, nil, err
	}
	peerID, err := res.LastInsertId()
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	peer, err := s.GetPeerByID(ctx, peerID)
	return peer, netCfg, err
}

func (s *Store) GetPeerByID(ctx context.Context, id int64) (*WGPeer, error) {
	var p WGPeer
	err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.user_id, u.email, u.display_name,
		       p.network_id, n.name, n.ros_interface,
		       p.ros_peer_id, p.name, p.allocated_ip, p.public_key, p.preshared_key, p.key_mode, p.created_at
		FROM wg_peers p
		JOIN users u ON u.id = p.user_id
		JOIN wg_networks n ON n.id = p.network_id
		WHERE p.id = ?
	`, id).Scan(
		&p.ID, &p.UserID, &p.UserEmail, &p.UserName,
		&p.NetworkID, &p.NetworkName, &p.ROSInterface,
		&p.ROSPeerID, &p.Name, &p.AllocatedIP, &p.PublicKey, &p.PresharedKey, &p.KeyMode, &p.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListPeersByUser(ctx context.Context, userID int64) ([]WGPeer, error) {
	return s.queryPeers(ctx, `WHERE p.user_id = ? ORDER BY p.id DESC`, userID)
}

func (s *Store) ListAllPeers(ctx context.Context) ([]WGPeer, error) {
	return s.queryPeers(ctx, `ORDER BY p.id DESC`)
}

func (s *Store) queryPeers(ctx context.Context, whereAndOrder string, args ...any) ([]WGPeer, error) {
	query := `
		SELECT p.id, p.user_id, u.email, u.display_name,
		       p.network_id, n.name, n.ros_interface,
		       p.ros_peer_id, p.name, p.allocated_ip, p.public_key, p.preshared_key, p.key_mode, p.created_at
		FROM wg_peers p
		JOIN users u ON u.id = p.user_id
		JOIN wg_networks n ON n.id = p.network_id
	` + " " + whereAndOrder

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WGPeer
	for rows.Next() {
		var p WGPeer
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.UserEmail, &p.UserName,
			&p.NetworkID, &p.NetworkName, &p.ROSInterface,
			&p.ROSPeerID, &p.Name, &p.AllocatedIP, &p.PublicKey, &p.PresharedKey, &p.KeyMode, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpdatePeerKeys(ctx context.Context, peerID int64, publicKey, presharedKey, keyMode, rosPeerID string) (*WGPeer, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE wg_peers
		SET public_key = ?, preshared_key = ?, key_mode = ?, ros_peer_id = ?
		WHERE id = ?
	`, publicKey, presharedKey, keyMode, rosPeerID, peerID)
	if err != nil {
		return nil, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, ErrNotFound
	}
	return s.GetPeerByID(ctx, peerID)
}

func (s *Store) DeletePeer(ctx context.Context, peerID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM wg_peers WHERE id = ?`, peerID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}
