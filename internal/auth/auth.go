package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/canhnm/tikportal/internal/config"
)

const (
	sessionCookieName = "tikportal_session"
	oauthStateCookie  = "tikportal_oidc_state"
)

type SessionClaims struct {
	UserID  int64  `json:"uid"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Expires int64  `json:"exp"`
}

type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

type OIDCUserInfo struct {
	Sub               string `json:"sub"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

type Manager struct {
	cfg        *config.Config
	httpClient *http.Client
	mu         sync.RWMutex
	discovery  *oidcDiscovery
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (m *Manager) OIDCEnabled() bool {
	return m.cfg.OIDCIssuerURL != "" && m.cfg.OIDCClientID != ""
}

func (m *Manager) IsAdminEmail(email string) bool {
	normalized := strings.ToLower(strings.TrimSpace(email))
	for _, adminEmail := range m.cfg.AdminEmails {
		if strings.ToLower(strings.TrimSpace(adminEmail)) == normalized {
			return true
		}
	}
	return false
}

// IssueSessionCookie sets an HMAC-SHA256 signed session cookie on the response.
func (m *Manager) IssueSessionCookie(w http.ResponseWriter, userID int64, email, role string) error {
	claims := SessionClaims{
		UserID:  userID,
		Email:   email,
		Role:    role,
		Expires: time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return err
	}
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	sig := m.sign(b64)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    b64 + "." + sig,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(m.cfg.BaseURL, "https://"),
		MaxAge:   7 * 24 * 3600,
	})
	return nil
}

func (m *Manager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (m *Manager) ParseSessionRequest(r *http.Request) (*SessionClaims, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return nil, errors.New("malformed session token")
	}
	expectedSig := m.sign(parts[0])
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return nil, errors.New("invalid session signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var claims SessionClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, err
	}
	if time.Now().Unix() > claims.Expires {
		return nil, errors.New("session expired")
	}
	return &claims, nil
}

// StartOIDCLogin generates state + PKCE verifier, stores them in a short-lived cookie,
// and returns the authorization URL to redirect the browser.
func (m *Manager) StartOIDCLogin(ctx context.Context, w http.ResponseWriter) (string, error) {
	disc, err := m.getDiscovery(ctx)
	if err != nil {
		return "", err
	}

	state, err := randomURLSafe(24)
	if err != nil {
		return "", err
	}
	verifier, err := randomURLSafe(32)
	if err != nil {
		return "", err
	}

	stateData := state + "|" + verifier
	stateB64 := base64.RawURLEncoding.EncodeToString([]byte(stateData))
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    stateB64 + "." + m.sign(stateB64),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(m.cfg.BaseURL, "https://"),
		MaxAge:   600,
	})

	challengeHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])

	q := url.Values{}
	q.Set("client_id", m.cfg.OIDCClientID)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(m.cfg.OIDCScopes, " "))
	q.Set("redirect_uri", m.cfg.OIDCRedirectURL)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	return disc.AuthorizationEndpoint + "?" + q.Encode(), nil
}

// ExchangeOIDCCallback verifies the state cookie, exchanges the authorization code with PKCE,
// and returns the authenticated user's OIDC identity.
func (m *Manager) ExchangeOIDCCallback(ctx context.Context, w http.ResponseWriter, r *http.Request) (*OIDCUserInfo, error) {
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil {
		return nil, errors.New("missing OIDC state cookie")
	}
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/", MaxAge: -1})

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(m.sign(parts[0]))) {
		return nil, errors.New("invalid OIDC state cookie signature")
	}
	rawState, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	sv := strings.SplitN(string(rawState), "|", 2)
	if len(sv) != 2 {
		return nil, errors.New("malformed OIDC state payload")
	}
	expectedState, verifier := sv[0], sv[1]
	if r.URL.Query().Get("state") != expectedState {
		return nil, errors.New("OIDC state mismatch")
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		return nil, errors.New("missing authorization code")
	}

	disc, err := m.getDiscovery(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", m.cfg.OIDCRedirectURL)
	form.Set("client_id", m.cfg.OIDCClientID)
	form.Set("code_verifier", verifier)
	if m.cfg.OIDCClientSecret != "" {
		form.Set("client_secret", m.cfg.OIDCClientSecret)
	}

	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, disc.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.Header.Set("Accept", "application/json")

	tokenResp, err := m.httpClient.Do(tokenReq)
	if err != nil {
		return nil, fmt.Errorf("OIDC token exchange failed: %w", err)
	}
	defer tokenResp.Body.Close()

	tokenBytes, _ := io.ReadAll(tokenResp.Body)
	if tokenResp.StatusCode < 200 || tokenResp.StatusCode >= 300 {
		return nil, fmt.Errorf("OIDC token endpoint error (%d): %s", tokenResp.StatusCode, string(tokenBytes))
	}

	var tokenData struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.Unmarshal(tokenBytes, &tokenData); err != nil {
		return nil, err
	}

	info := &OIDCUserInfo{}
	// Parse ID token JWT payload first
	if tokenData.IDToken != "" {
		jwtParts := strings.Split(tokenData.IDToken, ".")
		if len(jwtParts) == 3 {
			if payloadBytes, err := base64.RawURLEncoding.DecodeString(jwtParts[1]); err == nil {
				_ = json.Unmarshal(payloadBytes, info)
			}
		}
	}

	// Enrich from userinfo endpoint if available
	if disc.UserinfoEndpoint != "" && tokenData.AccessToken != "" {
		uReq, err := http.NewRequestWithContext(ctx, http.MethodGet, disc.UserinfoEndpoint, nil)
		if err == nil {
			uReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
			if uResp, err := m.httpClient.Do(uReq); err == nil {
				defer uResp.Body.Close()
				if uResp.StatusCode == http.StatusOK {
					_ = json.NewDecoder(uResp.Body).Decode(info)
				}
			}
		}
	}

	if info.Sub == "" {
		return nil, errors.New("OIDC provider did not return a user 'sub' claim")
	}
	if info.Email == "" {
		if info.PreferredUsername != "" {
			info.Email = info.PreferredUsername
		} else {
			info.Email = info.Sub + "@oidc.local"
		}
	}
	if info.Name == "" {
		if info.PreferredUsername != "" {
			info.Name = info.PreferredUsername
		} else {
			info.Name = strings.Split(info.Email, "@")[0]
		}
	}
	return info, nil
}

func (m *Manager) getDiscovery(ctx context.Context) (*oidcDiscovery, error) {
	m.mu.RLock()
	if m.discovery != nil {
		defer m.mu.RUnlock()
		return m.discovery, nil
	}
	m.mu.RUnlock()

	wellKnown := strings.TrimRight(m.cfg.OIDCIssuerURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch OIDC discovery %s: %w", wellKnown, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery returned status %d", resp.StatusCode)
	}

	var disc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.discovery = &disc
	m.mu.Unlock()
	return &disc, nil
}

func (m *Manager) sign(data string) string {
	mac := hmac.New(sha256.New, []byte(m.cfg.SessionSecret))
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
