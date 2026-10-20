package store

import "testing"

func TestContainsPattern(t *testing.T) {
	cases := map[string]string{
		"login":   "%login%",
		"100%":    `%100\%%`,
		"snake_x": `%snake\_x%`,
		`a\b`:     `%a\\b%`,
		"":        "%%",
	}
	for in, want := range cases {
		if got := containsPattern(in); got != want {
			t.Errorf("containsPattern(%q) = %q, want %q", in, got, want)
		}
	}
}
