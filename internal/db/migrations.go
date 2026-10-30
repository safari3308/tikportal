package db

import (
	"database/sql"
	"fmt"
	"strconv"
)

func (s *Store) migrate(defaultMaxPeers int) error {
	// First, check if legacy wg_peers has user_id column
	var hasLegacyUserId bool
	rows, err := s.db.Query(`PRAGMA table_info(wg_peers)`)
	if err == nil {
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue sql.NullString
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err == nil {
				if name == "user_id" {
					hasLegacyUserId = true
				}
			}
		}
		rows.Close()
	}

	if hasLegacyUserId {
		// Migrate legacy wg_peers table by copying user metadata from users table
		migrationScript := `
		CREATE TABLE IF NOT EXISTS wg_peers_migrated (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			user_sub      TEXT NOT NULL,
			user_email    TEXT NOT NULL DEFAULT '',
			user_name     TEXT NOT NULL DEFAULT '',
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

		INSERT INTO wg_peers_migrated (
			id, user_sub, user_email, user_name, network_id, ros_peer_id, name, allocated_ip, public_key, preshared_key, key_mode, created_at
		)
		SELECT p.id,
		       COALESCE(u.oidc_sub, 'legacy:' || p.user_id),
		       COALESCE(u.email, ''),
		       COALESCE(u.display_name, ''),
		       p.network_id, p.ros_peer_id, p.name, p.allocated_ip, p.public_key, p.preshared_key, p.key_mode, p.created_at
		FROM wg_peers p
		LEFT JOIN users u ON u.id = p.user_id;

		DROP TABLE wg_peers;
		ALTER TABLE wg_peers_migrated RENAME TO wg_peers;
		DROP TABLE IF EXISTS users;
		`
		if _, err := s.db.Exec(migrationScript); err != nil {
			return fmt.Errorf("migrate legacy wg_peers: %w", err)
		}
	} else {
		// Drop users table if it still exists
		_, _ = s.db.Exec(`DROP TABLE IF EXISTS users;`)
	}

	schema := `
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
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
	user_sub      TEXT NOT NULL,
	user_email    TEXT NOT NULL DEFAULT '',
	user_name     TEXT NOT NULL DEFAULT '',
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

CREATE INDEX IF NOT EXISTS idx_wg_peers_user_sub ON wg_peers(user_sub);
CREATE INDEX IF NOT EXISTS idx_wg_peers_network_id ON wg_peers(network_id);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO settings (key, value) VALUES ('default_max_peers_per_user', ?)`,
		strconv.Itoa(defaultMaxPeers),
	)
	return err
}
