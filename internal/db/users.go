package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

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
