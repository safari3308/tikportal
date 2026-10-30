package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestOIDCValidator(t *testing.T) {
	// 1. Generate RSA key for test
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}
	publicKey := &privateKey.PublicKey

	nStr := base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())
	eBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(eBytes, uint32(publicKey.E))
	for len(eBytes) > 0 && eBytes[0] == 0 {
		eBytes = eBytes[1:]
	}
	eStr := base64.RawURLEncoding.EncodeToString(eBytes)

	// 2. Setup mock OIDC server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"jwks_uri":          fmt.Sprintf("http://%s/keys", r.Host),
				"userinfo_endpoint": fmt.Sprintf("http://%s/userinfo", r.Host),
			})
		case "/userinfo":
			authH := r.Header.Get("Authorization")
			if authH == "Bearer opaque-token-123" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{
					"sub":   "userinfo-sub-999",
					"email": "userinfo@example.com",
					"name":  "Userinfo User",
				})
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		case "/keys":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"keys": []map[string]string{
					{
						"kid": "test-kid",
						"kty": "RSA",
						"alg": "RS256",
						"use": "sig",
						"n":   nStr,
						"e":   eStr,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	validator := NewValidator(ts.URL, "", "test-client", true, []string{"admin@example.com"})

	// Test 1: Dev Token
	t.Run("DevToken", func(t *testing.T) {
		token := validator.CreateDevToken("admin@example.com", "Admin User")
		user, err := validator.AuthenticateToken(t.Context(), token)
		if err != nil {
			t.Fatalf("AuthenticateToken failed: %v", err)
		}
		if user.Email != "admin@example.com" || user.Role != "admin" {
			t.Fatalf("unexpected dev user: %+v", user)
		}
	})

	// Test 2: Standard JWT with RS256
	t.Run("RS256_JWT", func(t *testing.T) {
		claims := &StandardOIDCClaims{
			Email: "alice@example.com",
			Name:  "Alice Wonder",
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    ts.URL,
				Subject:   "alice-sub-123",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = "test-kid"
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		user, err := validator.AuthenticateToken(t.Context(), tokenString)
		if err != nil {
			t.Fatalf("AuthenticateToken failed: %v", err)
		}
		if user.Sub != "alice-sub-123" || user.Email != "alice@example.com" || user.Role != "user" {
			t.Fatalf("unexpected user: %+v", user)
		}
	})

	// Test 3: Userinfo Fallback for Opaque Token
	t.Run("UserinfoFallback", func(t *testing.T) {
		user, err := validator.AuthenticateToken(t.Context(), "opaque-token-123")
		if err != nil {
			t.Fatalf("AuthenticateToken via userinfo failed: %v", err)
		}
		if user.Sub != "userinfo-sub-999" || user.Email != "userinfo@example.com" {
			t.Fatalf("unexpected user: %+v", user)
		}
	})

	// Test 4: Middleware with Bearer header
	t.Run("Middleware", func(t *testing.T) {
		devToken := validator.CreateDevToken("alice@example.com", "Alice")
		req := httptest.NewRequest("GET", "/api/peers", nil)
		req.Header.Set("Authorization", "Bearer "+devToken)
		rr := httptest.NewRecorder()

		var calledUser *AuthenticatedUser
		handler := validator.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calledUser = UserFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if calledUser == nil || calledUser.Email != "alice@example.com" {
			t.Fatalf("expected alice in context, got %+v", calledUser)
		}
	})
}
