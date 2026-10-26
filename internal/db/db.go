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
