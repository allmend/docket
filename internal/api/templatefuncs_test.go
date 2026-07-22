package api

import (
	"html/template"
	"regexp"
	"strings"
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
	first := hashColor("alice")
	if got := hashColor("alice"); got != first {
		t.Errorf("hashColor not deterministic: %q then %q", first, got)
	}
	if hashColor("") == "" {
		t.Error("hashColor of empty string must still return a color")
	}
}

func TestPriorityColorAndLabel(t *testing.T) {
	tests := []struct {
		priority string
		color    string
		label    string
	}{
		{"critical", "bg-p0", "P0"},
		{"high", "bg-p1", "P1"},
		{"medium", "bg-p2", "P2"},
		{"low", "bg-p3", "P3"},
		{"", "bg-base-500", "—"},
		{"bogus", "bg-base-500", "—"},
	}
	for _, tt := range tests {
		if got := priorityColor(tt.priority); got != tt.color {
			t.Errorf("priorityColor(%q) = %q, want %q", tt.priority, got, tt.color)
		}
		if got := priorityLabel(tt.priority); got != tt.label {
			t.Errorf("priorityLabel(%q) = %q, want %q", tt.priority, got, tt.label)
		}
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

func TestAvatarColor(t *testing.T) {
	hexRe := regexp.MustCompile(`^#[0-9a-f]{6}$`)

	// Whitespace/case normalisation (also covers determinism).
	if avatarColor("  john   smith ") != avatarColor("John Smith") {
		t.Error("avatarColor should normalise whitespace and case")
	}

	names := []string{
		"John Smith", "Jane Smith", "John Smyth",
		"Ludwig van Beethoven", "David H. Letterman", "Anders",
	}
	seen := map[string]string{}
	for _, n := range names {
		c := avatarColor(n)
		if !hexRe.MatchString(c) {
			t.Errorf("avatarColor(%q) = %q, not a #rrggbb hex color", n, c)
		}
		if prev, dup := seen[c]; dup {
			t.Errorf("avatarColor collision: %q and %q both map to %s", prev, n, c)
		}
		seen[c] = n
	}

	// Empty name falls back to a fixed color rather than erroring.
	if !hexRe.MatchString(avatarColor("")) {
		t.Error("avatarColor(\"\") should return a valid hex fallback")
	}
}

// TestTimeAgoHTML checks the rendered form: the relative phrase stays the visible
// text, and the exact ISO 8601 timestamp rides along as both the tooltip (title)
// and the machine-readable datetime attribute.
func TestTimeAgoHTML(t *testing.T) {
	ts := time.Date(2026, 7, 22, 9, 12, 33, 0, time.UTC)
	got := string(timeAgoHTML(ts))

	if want := `datetime="2026-07-22T09:12:33Z"`; !strings.Contains(got, want) {
		t.Errorf("missing %s in %s", want, got)
	}
	if want := `title="2026-07-22T09:12:33Z"`; !strings.Contains(got, want) {
		t.Errorf("missing %s in %s", want, got)
	}
	if !strings.HasPrefix(got, "<time ") || !strings.HasSuffix(got, "</time>") {
		t.Errorf("not a <time> element: %s", got)
	}
	// The visible text must still be the relative phrase.
	if !strings.Contains(got, timeAgo(ts)) {
		t.Errorf("visible text is not the relative phrase: %s", got)
	}
}

func TestTimeAgoHTMLZeroTime(t *testing.T) {
	if got := timeAgoHTML(time.Time{}); got != "" {
		t.Errorf("zero time rendered %q, want empty", got)
	}
}

// TestTimeAgoFuncIsHTMLVariant pins the wiring: templates must get the tooltip
// version. Reverting the FuncMap entry to the plain timeAgo would silently drop
// the tooltip from every relative timestamp in the app.
func TestTimeAgoFuncIsHTMLVariant(t *testing.T) {
	fn, ok := templateFuncs()["timeAgo"].(func(time.Time) template.HTML)
	if !ok {
		t.Fatal(`templateFuncs()["timeAgo"] is not the template.HTML variant`)
	}
	if !strings.Contains(string(fn(time.Now().Add(-3*time.Hour))), "<time ") {
		t.Error("timeAgo template func no longer emits a <time> element")
	}
}
