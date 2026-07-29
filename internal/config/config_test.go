package config

import "testing"

// TestEnvBool covers the parsing behind COOKIE_SECURE. The default must stay
// true: a deployment that forgets to set it should be secure, not convenient.
func TestEnvBool(t *testing.T) {
	cases := []struct {
		set  string
		want bool
	}{
		{"", true}, // unset keeps the fallback
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"true", true},
		{"1", true},
		{"nonsense", true}, // unparseable keeps the fallback rather than opening up
	}
	for _, c := range cases {
		t.Setenv("TEST_COOKIE_SECURE", c.set)
		if got := envBool("TEST_COOKIE_SECURE", true); got != c.want {
			t.Errorf("envBool(%q) = %v, want %v", c.set, got, c.want)
		}
	}
}
