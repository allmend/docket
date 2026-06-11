package api

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"
)

// templateFuncs returns the helper functions available to all templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"sub":         func(a, b int) int { return a - b },
		"add":         func(a, b int) int { return a + b },
		"mul":         func(a, b int) int { return a * b },
		"addf":        addf,
		"pctf":        pctf,
		"pct":         pct,
		"slice":       func(items ...any) []any { return items },
		"lower":       strings.ToLower,
		"upper":       strings.ToUpper,
		"contains":    strings.Contains,
		"initials":    initials,
		"deref":       derefFloat,
		"derefStr":    derefStr,
		"daysBetween": daysBetween,
		"now":         func() time.Time { return time.Now() },
		"isToday":     isToday,
		"formatDate":  func(t time.Time) string { return t.Format("Monday, 2 Jan 2006") },
		"hashColor":   hashColor,
		"timeAgo":     timeAgo,
		"dict":        dict,
	}
}

// toFloat widens ints and float64s; everything else counts as zero.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func addf(a, b any) float64 { return toFloat(a) + toFloat(b) }

// pctf returns part/total as a percentage, 0 when total is zero.
func pctf(part, total any) float64 {
	t := toFloat(total)
	if t == 0 {
		return 0
	}
	return toFloat(part) / t * 100
}

func pct(part, total int) int {
	if total == 0 {
		return 0
	}
	return int(float64(part) / float64(total) * 100)
}

// initials derives an avatar label: first letters of the first and last word,
// or the first two letters of a single word. Unicode-safe.
func initials(s string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return "?"
	}
	if len(words) == 1 {
		r := []rune(words[0])
		if len(r) >= 2 {
			return strings.ToUpper(string(r[:2]))
		}
		return strings.ToUpper(string(r[:1]))
	}
	r0 := []rune(words[0])
	r1 := []rune(words[len(words)-1])
	return strings.ToUpper(string(r0[:1]) + string(r1[:1]))
}

func derefFloat(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func daysBetween(a, b *time.Time) int {
	if a == nil || b == nil {
		return 0
	}
	return int(b.Sub(*a).Hours() / 24)
}

func isToday(t time.Time) bool {
	n := time.Now()
	y, m, d := t.Date()
	yn, mn, dn := n.Date()
	return y == yn && m == mn && d == dn
}

// hashColor picks a stable avatar color for a name from a fixed palette.
func hashColor(s string) string {
	palette := []string{"#8957e5", "#db61a2", "#39c5cf", "#a3e635", "#768390", "#ff8c42", "#58a6ff", "#10b981"}
	h := 0
	for _, c := range s {
		h = (h*31 + int(c)) & 0xffff
	}
	return palette[h%len(palette)]
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < 2*time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// dict builds a map from alternating key/value pairs, for passing multiple
// values to a nested template.
func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict: odd number of arguments")
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key must be string")
		}
		m[key] = pairs[i+1]
	}
	return m, nil
}
