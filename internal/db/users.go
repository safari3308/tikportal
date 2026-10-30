package db

import (
	"context"
	"strings"
	"time"
)

type UserSummary struct {
	OIDCSub      string    `json:"oidc_sub"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	Role         string    `json:"role"`
	EffectiveMax int       `json:"effective_max_peers"`
	PeerCount    int       `json:"peer_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListActiveUsersFromPeers aggregates users that currently have active WireGuard peers in SQLite.
func (s *Store) ListActiveUsersFromPeers(ctx context.Context, adminEmails []string) ([]UserSummary, error) {
	defaultMax, _ := s.GetDefaultMaxPeers(ctx)
	query := `
		SELECT user_sub, user_email, user_name, COUNT(*) as peer_count, MIN(created_at) as first_created
		FROM wg_peers
		GROUP BY user_sub
		ORDER BY first_created ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	isAdmin := func(email string) bool {
		norm := strings.ToLower(strings.TrimSpace(email))
		for _, a := range adminEmails {
			if strings.ToLower(strings.TrimSpace(a)) == norm {
				return true
			}
		}
		return false
	}

	var out []UserSummary
	for rows.Next() {
		var u UserSummary
		var rawCreated string
		if err := rows.Scan(&u.OIDCSub, &u.Email, &u.DisplayName, &u.PeerCount, &rawCreated); err != nil {
			return nil, err
		}
		if t, err := time.Parse("2006-01-02 15:04:05", rawCreated); err == nil {
			u.CreatedAt = t
		} else if t, err := time.Parse(time.RFC3339, rawCreated); err == nil {
			u.CreatedAt = t
		}
		if isAdmin(u.Email) {
			u.Role = "admin"
		} else {
			u.Role = "user"
		}
		u.EffectiveMax = defaultMax
		out = append(out, u)
	}
	return out, rows.Err()
}
