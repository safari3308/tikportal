package auth

import (
	"context"
)

type contextKey string

const userContextKey = contextKey("tikportal_auth_user")

type AuthenticatedUser struct {
	Sub         string `json:"sub"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"` // "admin" | "user"
}

func WithUser(ctx context.Context, user *AuthenticatedUser) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *AuthenticatedUser {
	if val := ctx.Value(userContextKey); val != nil {
		if u, ok := val.(*AuthenticatedUser); ok {
			return u
		}
	}
	return nil
}
