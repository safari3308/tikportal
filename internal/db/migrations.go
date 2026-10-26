package db

import (
	"fmt"
	"strconv"
)

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
