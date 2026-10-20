package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL string
	HTTPPort    string
	MetricsPort string
	// MetricsAddr is the interface the Prometheus endpoint listens on. It defaults
	// to loopback, since /metrics has no authentication. Orchestrators that scrape
	// from another pod need 0.0.0.0 and network policy to fence the port off.
	MetricsAddr string
	JWTSecret   string
	Mode        string
	// CookieSecure marks the session cookies Secure, which browsers only store
	// over HTTPS. True unless explicitly disabled: turning it off on a network
	// that is not trusted exposes sessions to anyone watching the wire.
	CookieSecure bool
	// HealthAddr/HealthPort is the listener for /livez and /readyz. It binds
	// wide by default: kubelet probes reach the pod IP, and the endpoints
	// expose nothing but pass/fail.
	HealthAddr string
	HealthPort string
	// ShutdownDelay is how long the server keeps serving after SIGTERM with
	// /readyz failing, so the orchestrator stops routing to it first.
	ShutdownDelay time.Duration
}

func Load() (*Config, error) {
	loadDotEnv(".env")

	c := &Config{
		DatabaseURL:  env("DATABASE_URL", "postgres://docket:docket@localhost:5432/docket?sslmode=disable"),
		HTTPPort:     env("HTTP_PORT", "8081"),
		MetricsPort:  env("METRICS_PORT", "9412"),
		MetricsAddr:  env("METRICS_ADDR", "127.0.0.1"),
		JWTSecret:    env("JWT_SECRET", ""),
		Mode:         env("MODE", "all"),
		CookieSecure: envBool("COOKIE_SECURE", true),
	}
	c.HealthAddr = env("HEALTH_ADDR", "0.0.0.0")
	c.HealthPort = env("HEALTH_PORT", "9413")
	c.ShutdownDelay = envDuration("SHUTDOWN_DELAY", 5*time.Second)
	if err := ValidateJWTSecret(c.JWTSecret); err != nil {
		return nil, err
	}
	return c, nil
}

// MinJWTSecretLen is the shortest JWT_SECRET accepted: 32 bytes, the HS256 key size.
const MinJWTSecretLen = 32

// placeholderSecrets are values that ship in Docket's own examples. Anyone can
// read them, so a deployment using one signs sessions with a public key.
var placeholderSecrets = map[string]bool{
	"replace-with-a-long-random-string": true,
	"dev-secret-change-in-prod":         true,
	"changeme":                          true,
}

// ValidateJWTSecret refuses an empty, placeholder or short JWT_SECRET. Anyone
// holding the secret can forge a session for any user, so starting with a weak
// one is worse than not starting.
func ValidateJWTSecret(s string) error {
	const hint = "generate one with: openssl rand -hex 32"
	switch {
	case s == "":
		return fmt.Errorf("JWT_SECRET is required; %s", hint)
	case placeholderSecrets[s]:
		return fmt.Errorf("JWT_SECRET is still the example placeholder %q; %s", s, hint)
	case len(s) < MinJWTSecretLen:
		return fmt.Errorf("JWT_SECRET is %d bytes, at least %d are required; %s", len(s), MinJWTSecretLen, hint)
	}
	return nil
}

// ValidateSeedPassword refuses to create the first admin with the example
// password. It only applies on a fresh database, where the seed actually runs.
// An empty value means SEED_PASSWORD was not set.
func ValidateSeedPassword(pw string) error {
	switch {
	case pw == "":
		return fmt.Errorf("SEED_PASSWORD is required on first start: it becomes the initial admin's password")
	case placeholderSecrets[pw]:
		return fmt.Errorf("SEED_PASSWORD is still the example value %q; set a real password for the initial admin", pw)
	}
	return nil
}

// envBool reads a boolean env var. Anything strconv.ParseBool accepts works
// ("false", "0", "FALSE"); an unset or unparseable value keeps the fallback.
func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

// envDuration reads a Go duration ("5s", "0"). An unset or unparseable value
// keeps the fallback.
func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return fallback
	}
	return d
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv reads a .env file and sets any key not already in the environment.
// Lines starting with # are comments. Inline comments are not supported.
// Silently does nothing if the file doesn't exist.
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
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Strip optional surrounding quotes
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		// Don't override variables already set in the environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
