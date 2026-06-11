package api

import (
	"testing"
	"time"
)

func TestInitials(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Anders Adamsson", "AA"},
		{"alice", "AL"},
		{"Ada Lovelace King", "AK"}, // first + last word
		{"x", "X"},
		{"", "?"},
		{"   ", "?"},
		{"Åsa Öberg", "ÅÖ"}, // unicode-safe
	}
	for _, tt := range tests {
		if got := initials(tt.in); got != tt.want {
			t.Errorf("initials(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAddfAndPctf(t *testing.T) {
	if got := addf(2, 3.5); got != 5.5 {
		t.Errorf("addf(int, float64) = %v, want 5.5", got)
	}
	if got := addf("junk", 1); got != 1 {
		t.Errorf("addf with non-numeric = %v, want 1", got)
	}
	if got := pctf(1, 4); got != 25 {
		t.Errorf("pctf(1,4) = %v, want 25", got)
	}
	if got := pctf(5, 0); got != 0 {
		t.Errorf("pctf with zero total = %v, want 0", got)
	}
}

func TestPct(t *testing.T) {
	if got := pct(3, 4); got != 75 {
		t.Errorf("pct(3,4) = %d, want 75", got)
	}
	if got := pct(1, 0); got != 0 {
		t.Errorf("pct with zero total = %d, want 0", got)
	}
}

func TestDerefFloat(t *testing.T) {
	if got := derefFloat(nil); got != "" {
		t.Errorf("derefFloat(nil) = %q, want empty", got)
	}
	v := 3.0
	if got := derefFloat(&v); got != "3" {
		t.Errorf("derefFloat(3.0) = %q, want %q (no trailing zeros)", got, "3")
	}
	half := 2.5
	if got := derefFloat(&half); got != "2.5" {
		t.Errorf("derefFloat(2.5) = %q, want %q", got, "2.5")
	}
}

func TestDaysBetween(t *testing.T) {
	a := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	b := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if got := daysBetween(&a, &b); got != 14 {
		t.Errorf("daysBetween = %d, want 14", got)
	}
	if got := daysBetween(nil, &b); got != 0 {
		t.Errorf("daysBetween with nil = %d, want 0", got)
	}
}

func TestTimeAgo(t *testing.T) {
	now := time.Now()
	tests := []struct {
		age  time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{30 * time.Minute, "30m ago"},
		{3 * time.Hour, "3h ago"},
		{30 * time.Hour, "yesterday"},
		{72 * time.Hour, "3d ago"},
	}
	for _, tt := range tests {
		if got := timeAgo(now.Add(-tt.age)); got != tt.want {
			t.Errorf("timeAgo(-%v) = %q, want %q", tt.age, got, tt.want)
		}
	}
}

func TestHashColorIsStable(t *testing.T) {
	if hashColor("alice") != hashColor("alice") {
		t.Error("hashColor must be deterministic")
	}
	if hashColor("") == "" {
		t.Error("hashColor of empty string must still return a color")
	}
}

func TestDict(t *testing.T) {
	m, err := dict("a", 1, "b", "two")
	if err != nil {
		t.Fatalf("dict: %v", err)
	}
	if m["a"] != 1 || m["b"] != "two" {
		t.Errorf("dict = %v", m)
	}
	if _, err := dict("odd"); err == nil {
		t.Error("dict with odd arguments should error")
	}
	if _, err := dict(1, "x"); err == nil {
		t.Error("dict with non-string key should error")
	}
}
