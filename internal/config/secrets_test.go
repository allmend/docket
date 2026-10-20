package config

import (
	"strings"
	"testing"
)

func TestValidateJWTSecret(t *testing.T) {
	cases := []struct {
		secret string
		ok     bool
	}{
		{"", false},
		{"replace-with-a-long-random-string", false}, // 33 bytes, but public
		{"dev-secret-change-in-prod", false},
		{"short-but-random-1234", false},
		{strings.Repeat("a", MinJWTSecretLen-1), false},
		{strings.Repeat("a", MinJWTSecretLen), true},
		{"9f2c4e8a1b7d3f6e0a5c9b2d8e4f1a7c3b6d0e9f2a5c8b1d4e7f0a3c6b9d2e5f", true},
	}
	for _, c := range cases {
		err := ValidateJWTSecret(c.secret)
		if (err == nil) != c.ok {
			t.Errorf("ValidateJWTSecret(%q): err=%v, want ok=%v", c.secret, err, c.ok)
		}
		if err != nil && !strings.Contains(err.Error(), "openssl rand") {
			t.Errorf("error for %q should say how to generate a secret: %v", c.secret, err)
		}
	}
}

func TestValidateSeedPassword(t *testing.T) {
	for pw, ok := range map[string]bool{
		"":                    false,
		"changeme":            false,
		"a-real-admin-pass-1": true,
	} {
		if err := ValidateSeedPassword(pw); (err == nil) != ok {
			t.Errorf("ValidateSeedPassword(%q): err=%v, want ok=%v", pw, err, ok)
		}
	}
}
