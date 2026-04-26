package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	HTTPPort    string
	MetricsPort string
	JWTSecret   string
	Mode        string
}

func Load() (*Config, error) {
	loadDotEnv(".env")

	c := &Config{
		DatabaseURL: env("DATABASE_URL", "postgres://docket:docket@localhost:5432/docket?sslmode=disable"),
		RedisURL:    env("REDIS_URL", "redis://localhost:6380"),
		HTTPPort:    env("HTTP_PORT", "8081"),
		MetricsPort: env("METRICS_PORT", "9412"),
		JWTSecret:   env("JWT_SECRET", ""),
		Mode:        env("MODE", "all"),
	}
	if c.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return c, nil
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
