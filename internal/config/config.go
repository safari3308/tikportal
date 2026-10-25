package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config holds the runtime configuration for TikPortal.
type Config struct {
	ListenAddr string
	DBPath     string
	BaseURL    string

	// Session / Auth settings
	SessionSecret        string
	DefaultMaxPeersTotal int
	AdminEmails          []string
	DevAuthEnabled       bool

	// OIDC configuration (Zitadel, Keycloak, etc.)
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCScopes       []string

	// RouterOS v7 REST API configuration
	RouterOSURL                   string
	RouterOSUser                  string
	RouterOSPassword              string
	RouterOSInsecureSkipTLSVerify bool
	RouterOSMock                  bool
	RouterOSInterfaceTag          string
	RouterOSAllowedInterfaces     []string
}

// Load reads configuration from .env (if present) and environment variables with sensible defaults.
func Load() *Config {
	loadDotEnv(".env")

	defaultMaxPeers, err := strconv.Atoi(getEnv("DEFAULT_MAX_PEERS_PER_USER", "5"))
	if err != nil || defaultMaxPeers < 1 {
		defaultMaxPeers = 5
	}

	listenAddr := normalizeListenAddr(getEnv("LISTEN_ADDR", ":8080"))
	baseURL := strings.TrimRight(getEnv("BASE_URL", "http://localhost:8080"), "/")
	oidcIssuer := strings.TrimRight(getEnv("OIDC_ISSUER_URL", ""), "/")

	// Enable dev login fallback only when OIDC is not configured or DEV_AUTH_ENABLED=true
	devAuthDefault := "false"
	if oidcIssuer == "" {
		devAuthDefault = "true"
	}

	rosURL := strings.TrimRight(getEnv("ROUTEROS_URL", ""), "/")
	rosMockDefault := "false"
	if rosURL == "" {
		rosMockDefault = "true"
	}

	return &Config{
		ListenAddr:                    listenAddr,
		DBPath:                        getEnv("DB_PATH", "./data/tikportal.db"),
		BaseURL:                       baseURL,
		SessionSecret:                 getEnv("SESSION_SECRET", "tikportal-change-me-in-production-secret-key"),
		DefaultMaxPeersTotal:          defaultMaxPeers,
		AdminEmails:                   splitCSV(getEnv("ADMIN_EMAILS", "admin@localhost")),
		DevAuthEnabled:                parseBool(getEnv("DEV_AUTH_ENABLED", devAuthDefault)),
		OIDCIssuerURL:                 oidcIssuer,
		OIDCClientID:                  getEnv("OIDC_CLIENT_ID", ""),
		OIDCClientSecret:              getEnv("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:               getEnv("OIDC_REDIRECT_URL", baseURL+"/api/auth/callback"),
		OIDCScopes:                    splitCSV(getEnv("OIDC_SCOPES", "openid,profile,email")),
		RouterOSURL:                   rosURL,
		RouterOSUser:                  getEnv("ROUTEROS_USER", "admin"),
		RouterOSPassword:              getEnv("ROUTEROS_PASSWORD", ""),
		RouterOSInsecureSkipTLSVerify: parseBool(getEnv("ROUTEROS_INSECURE_SKIP_TLS_VERIFY", "true")),
		RouterOSMock:                  parseBool(getEnv("ROUTEROS_MOCK", rosMockDefault)),
		RouterOSInterfaceTag:          getEnv("ROUTEROS_INTERFACE_TAG", "tikportal"),
		RouterOSAllowedInterfaces:     splitCSV(getEnv("ROUTEROS_ALLOWED_INTERFACES", "")),
	}
}

// IsInterfaceAllowed checks whether a RouterOS WireGuard interface is permitted
// by ROUTEROS_ALLOWED_INTERFACES (if non-empty) and ROUTEROS_INTERFACE_TAG (unless "*" or "all").
func (c *Config) IsInterfaceAllowed(name, comment string) bool {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return false
	}

	if len(c.RouterOSAllowedInterfaces) > 0 {
		matchedName := false
		for _, allowed := range c.RouterOSAllowedInterfaces {
			if strings.EqualFold(strings.TrimSpace(allowed), trimmedName) {
				matchedName = true
				break
			}
		}
		if !matchedName {
			return false
		}
	}

	tag := strings.TrimSpace(c.RouterOSInterfaceTag)
	if tag != "" && tag != "*" && !strings.EqualFold(tag, "all") {
		if !strings.Contains(strings.ToLower(comment), strings.ToLower(tag)) {
			return false
		}
	}

	return true
}

// loadDotEnv reads key=value pairs from path and sets them in the process environment
// if they are not already set in the environment.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Strip optional surrounding quotes
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, val)
			}
		}
	}
}

func normalizeListenAddr(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ":8080"
	}
	if !strings.Contains(trimmed, ":") {
		return ":" + trimmed
	}
	return trimmed
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
