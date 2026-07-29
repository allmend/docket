package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	if c.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return c, nil
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
