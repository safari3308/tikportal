package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/canhnm/tikportal/internal/ipam"
	_ "modernc.org/sqlite"
)

var (
	ErrNotFound      = errors.New("record not found")
	ErrQuotaExceeded = errors.New("maximum VPN peer limit reached for this user")
	ErrNetworkOff    = errors.New("selected WireGuard network is currently disabled")
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID               int64     `json:"id"`
	OIDCSub          string    `json:"oidc_sub"`
	Email            string    `json:"email"`
	DisplayName      string    `json:"display_name"`
	Role             string    `json:"role"`
	MaxPeersOverride *int      `json:"max_peers_override"`
	EffectiveMax     int       `json:"effective_max_peers"`
	PeerCount        int       `json:"peer_count"`
	CreatedAt        time.Time `json:"created_at"`
}

type WGNetwork struct {
	ID                  int64     `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	ROSInterface        string    `json:"ros_interface"`
	ServerPublicKey     string    `json:"server_public_key"`
	SubnetCIDR          string    `json:"subnet_cidr"`
	ServerIP            string    `json:"server_ip"`
	EndpointHost        string    `json:"endpoint_host"`
	EndpointPort        int       `json:"endpoint_port"`
	DNSServers          string    `json:"dns_servers"`
	AllowedIPs          string    `json:"allowed_ips"`
	PersistentKeepalive int       `json:"persistent_keepalive"`
	Enabled             bool      `json:"enabled"`
	AllocatedCount      int       `json:"allocated_count"`
	SubnetCapacity      int       `json:"subnet_capacity"`
	CreatedAt           time.Time `json:"created_at"`
}

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

func Open(dbPath string, defaultMaxPeers int) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles concurrent writes best with a single connection or serialized writer
	sqlDB.SetMaxOpenConns(1)

	s := &Store{db: sqlDB}
	if err := s.migrate(defaultMaxPeers); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(defaultMaxPeers int) error {
	schema := `
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	oidc_sub           TEXT NOT NULL UNIQUE,
	email              TEXT NOT NULL,
	display_name       TEXT NOT NULL,
	role               TEXT NOT NULL DEFAULT 'user',
	max_peers_override INTEGER,
	created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS wg_networks (
	id                   INTEGER PRIMARY KEY AUTOINCREMENT,
	name                 TEXT NOT NULL,
	description          TEXT NOT NULL DEFAULT '',
	ros_interface        TEXT NOT NULL UNIQUE,
	server_public_key    TEXT NOT NULL,
	subnet_cidr          TEXT NOT NULL,
	server_ip            TEXT NOT NULL,
	endpoint_host        TEXT NOT NULL,
	endpoint_port        INTEGER NOT NULL,
	dns_servers          TEXT NOT NULL DEFAULT '1.1.1.1, 8.8.8.8',
	allowed_ips          TEXT NOT NULL DEFAULT '0.0.0.0/0',
	persistent_keepalive INTEGER NOT NULL DEFAULT 25,
	enabled              INTEGER NOT NULL DEFAULT 1,
	created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS wg_peers (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	network_id    INTEGER NOT NULL REFERENCES wg_networks(id) ON DELETE CASCADE,
	ros_peer_id   TEXT NOT NULL,
	name          TEXT NOT NULL,
	allocated_ip  TEXT NOT NULL,
	public_key    TEXT NOT NULL UNIQUE,
	preshared_key TEXT NOT NULL DEFAULT '',
	key_mode      TEXT NOT NULL DEFAULT 'server',
	created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(network_id, allocated_ip)
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}

	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO settings (key, value) VALUES ('default_max_peers_per_user', ?)`,
		strconv.Itoa(defaultMaxPeers),
	)
	return err
}

// GetDefaultMaxPeers returns the global default max peers per user.
func (s *Store) GetDefaultMaxPeers(ctx context.Context) (int, error) {
	var val string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'default_max_peers_per_user'`).Scan(&val)
	if err != nil {
		return 5, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return 5, nil
	}
	return n, nil
}

// SetDefaultMaxPeers updates the global default max peers per user.
func (s *Store) SetDefaultMaxPeers(ctx context.Context, maxPeers int) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO settings (key, value) VALUES ('default_max_peers_per_user', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		strconv.Itoa(maxPeers),
	)
	return err
}

// UpsertUser creates or updates a user on OIDC or Dev login.
func (s *Store) UpsertUser(ctx context.Context, oidcSub, email, displayName string, isAdmin bool) (*User, error) {
	role := "user"
	if isAdmin {
		role = "admin"
	}

	// If this is the very first user in the database, promote to admin automatically
	var totalUsers int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	if totalUsers == 0 {
		role = "admin"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (oidc_sub, email, display_name, role)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(oidc_sub) DO UPDATE SET
			email = excluded.email,
			display_name = excluded.display_name,
			role = CASE WHEN users.role = 'admin' OR excluded.role = 'admin' THEN 'admin' ELSE users.role END
	`, oidcSub, email, displayName, role)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE oidc_sub = ?`, oidcSub).Scan(&id); err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	defaultMax, _ := s.GetDefaultMaxPeers(ctx)
	var u User
	var override sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.oidc_sub, u.email, u.display_name, u.role, u.max_peers_override, u.created_at,
		       (SELECT COUNT(*) FROM wg_peers p WHERE p.user_id = u.id) AS peer_count
		FROM users u
		WHERE u.id = ?
	`, id).Scan(&u.ID, &u.OIDCSub, &u.Email, &u.DisplayName, &u.Role, &override, &u.CreatedAt, &u.PeerCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if override.Valid {
		v := int(override.Int64)
		u.MaxPeersOverride = &v
		u.EffectiveMax = v
	} else {
		u.EffectiveMax = defaultMax
	}
	return &u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	defaultMax, _ := s.GetDefaultMaxPeers(ctx)
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.oidc_sub, u.email, u.display_name, u.role, u.max_peers_override, u.created_at,
		       (SELECT COUNT(*) FROM wg_peers p WHERE p.user_id = u.id) AS peer_count
		FROM users u
		ORDER BY u.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		var override sql.NullInt64
		if err := rows.Scan(&u.ID, &u.OIDCSub, &u.Email, &u.DisplayName, &u.Role, &override, &u.CreatedAt, &u.PeerCount); err != nil {
			return nil, err
		}
		if override.Valid {
			v := int(override.Int64)
			u.MaxPeersOverride = &v
			u.EffectiveMax = v
		} else {
			u.EffectiveMax = defaultMax
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUserPolicy(ctx context.Context, userID int64, role string, maxPeersOverride *int) (*User, error) {
	if role != "admin" && role != "user" {
		return nil, errors.New("role must be 'admin' or 'user'")
	}
	var overrideVal any
	if maxPeersOverride != nil {
		overrideVal = *maxPeersOverride
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET role = ?, max_peers_override = ? WHERE id = ?
	`, role, overrideVal, userID)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, ErrNotFound
	}
	return s.GetUserByID(ctx, userID)
}

// Network CRUD

func (s *Store) ListNetworks(ctx context.Context, onlyEnabled bool) ([]WGNetwork, error) {
	query := `
		SELECT n.id, n.name, n.description, n.ros_interface, n.server_public_key,
		       n.subnet_cidr, n.server_ip, n.endpoint_host, n.endpoint_port,
		       n.dns_servers, n.allowed_ips, n.persistent_keepalive, n.enabled, n.created_at,
		       (SELECT COUNT(*) FROM wg_peers p WHERE p.network_id = n.id) AS allocated_count
		FROM wg_networks n
	`
	if onlyEnabled {
		query += ` WHERE n.enabled = 1 `
	}
	query += ` ORDER BY n.id ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WGNetwork
	for rows.Next() {
		var n WGNetwork
		var enabledInt int
		if err := rows.Scan(
			&n.ID, &n.Name, &n.Description, &n.ROSInterface, &n.ServerPublicKey,
			&n.SubnetCIDR, &n.ServerIP, &n.EndpointHost, &n.EndpointPort,
			&n.DNSServers, &n.AllowedIPs, &n.PersistentKeepalive, &enabledInt, &n.CreatedAt,
			&n.AllocatedCount,
		); err != nil {
			return nil, err
		}
		n.Enabled = enabledInt != 0
		if info, err := ipam.ValidateSubnet(n.SubnetCIDR, n.ServerIP); err == nil {
			n.SubnetCapacity = info.Capacity
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) GetNetworkByID(ctx context.Context, id int64) (*WGNetwork, error) {
	var n WGNetwork
	var enabledInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT n.id, n.name, n.description, n.ros_interface, n.server_public_key,
		       n.subnet_cidr, n.server_ip, n.endpoint_host, n.endpoint_port,
		       n.dns_servers, n.allowed_ips, n.persistent_keepalive, n.enabled, n.created_at,
		       (SELECT COUNT(*) FROM wg_peers p WHERE p.network_id = n.id) AS allocated_count
		FROM wg_networks n
		WHERE n.id = ?
	`, id).Scan(
		&n.ID, &n.Name, &n.Description, &n.ROSInterface, &n.ServerPublicKey,
		&n.SubnetCIDR, &n.ServerIP, &n.EndpointHost, &n.EndpointPort,
		&n.DNSServers, &n.AllowedIPs, &n.PersistentKeepalive, &enabledInt, &n.CreatedAt,
		&n.AllocatedCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	n.Enabled = enabledInt != 0
	if info, err := ipam.ValidateSubnet(n.SubnetCIDR, n.ServerIP); err == nil {
		n.SubnetCapacity = info.Capacity
	}
	return &n, nil
}

func (s *Store) CreateNetwork(ctx context.Context, n *WGNetwork) (*WGNetwork, error) {
	info, err := ipam.ValidateSubnet(n.SubnetCIDR, n.ServerIP)
	if err != nil {
		return nil, err
	}
	n.SubnetCIDR = info.Prefix.String()
	n.ServerIP = info.GatewayIP.String()
	if n.PersistentKeepalive <= 0 {
		n.PersistentKeepalive = 25
	}
	enabledInt := 1
	if !n.Enabled {
		enabledInt = 0
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO wg_networks (
			name, description, ros_interface, server_public_key,
			subnet_cidr, server_ip, endpoint_host, endpoint_port,
			dns_servers, allowed_ips, persistent_keepalive, enabled
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		n.Name, n.Description, n.ROSInterface, n.ServerPublicKey,
		n.SubnetCIDR, n.ServerIP, n.EndpointHost, n.EndpointPort,
		n.DNSServers, n.AllowedIPs, n.PersistentKeepalive, enabledInt,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetNetworkByID(ctx, id)
}

func (s *Store) UpdateNetwork(ctx context.Context, id int64, n *WGNetwork) (*WGNetwork, error) {
	info, err := ipam.ValidateSubnet(n.SubnetCIDR, n.ServerIP)
	if err != nil {
		return nil, err
	}
	n.SubnetCIDR = info.Prefix.String()
	n.ServerIP = info.GatewayIP.String()
	enabledInt := 0
	if n.Enabled {
		enabledInt = 1
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE wg_networks SET
			name = ?, description = ?, ros_interface = ?, server_public_key = ?,
			subnet_cidr = ?, server_ip = ?, endpoint_host = ?, endpoint_port = ?,
			dns_servers = ?, allowed_ips = ?, persistent_keepalive = ?, enabled = ?
		WHERE id = ?
	`,
		n.Name, n.Description, n.ROSInterface, n.ServerPublicKey,
		n.SubnetCIDR, n.ServerIP, n.EndpointHost, n.EndpointPort,
		n.DNSServers, n.AllowedIPs, n.PersistentKeepalive, enabledInt,
		id,
	)
	if err != nil {
		return nil, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, ErrNotFound
	}
	return s.GetNetworkByID(ctx, id)
}

func (s *Store) DeleteNetwork(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM wg_networks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Peer Provisioning & Atomic IP Reuse

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
