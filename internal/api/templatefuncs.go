package api

import (
	"fmt"
	"hash/fnv"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/allmend/docket/internal/version"
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
		"avatarColor": avatarColor,
		// The HTML variant on purpose — see timeAgoHTML. Every relative timestamp
		// gets the exact time as a hover tooltip without touching call sites.
		"timeAgo": timeAgoHTML,
		"dict":    dict,

		"priorityColor": priorityColor,
		"priorityLabel": priorityLabel,

		// Build version, so no template has to hardcode (and then forget) it.
		"version": func() string { return version.Version },
	}
}

// priorityColor maps a ticket priority to its color-bar utility class.
// These classes are safelisted in tailwind.config.js since they no longer
// appear literally in templates.
func priorityColor(priority string) string {
	switch priority {
	case "critical":
		return "bg-p0"
	case "high":
		return "bg-p1"
	case "medium":
		return "bg-p2"
	case "low":
		return "bg-p3"
	}
	return "bg-base-500"
}

// priorityLabel maps a ticket priority to its short display label.
func priorityLabel(priority string) string {
	switch priority {
	case "critical":
		return "P0"
	case "high":
		return "P1"
	case "medium":
		return "P2"
	case "low":
		return "P3"
	}
	return "—"
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

// avatarColor derives a stable per-user avatar color from a display name.
// The hue mixes hashes of the full name, the initials, and the number of
// name parts so similar names still land on different colors. Saturation
// and lightness stay in a band where the dark avatar text remains readable.
func avatarColor(name string) string {
	n := strings.Join(strings.Fields(strings.ToLower(name)), " ")
	if n == "" {
		return "#8957e5"
	}
	full := fnvHash(n)
	ini := fnvHash(strings.ToLower(initials(name)))
	parts := uint32(len(strings.Fields(name)))
	hue := float64((full + ini*131 + parts*977) % 360)
	sat := 0.52 + float64((full>>9)%18)/100  // 0.52–0.69
	lig := 0.60 + float64((full>>17)%12)/100 // 0.60–0.71
	r, g, b := hslToRGB(hue, sat, lig)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func fnvHash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// hslToRGB converts hue [0,360), saturation and lightness [0,1] to 8-bit RGB.
func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	c := (1 - math.Abs(2*l-1)) * s
	hp := h / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g, b = c, x, 0
	case hp < 2:
		r, g, b = x, c, 0
	case hp < 3:
		r, g, b = 0, c, x
	case hp < 4:
		r, g, b = 0, x, c
	case hp < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	m := l - c/2
	return uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255)
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

// timeAgoHTML is what templates actually get for {{timeAgo …}}: the relative
// phrase wrapped in a <time> element carrying the exact timestamp, so hovering
// any relative timestamp in the UI shows the ISO 8601 value as a native tooltip.
//
// Done at the func rather than the ~13 call sites so no existing site can be
// missed and no future {{timeAgo …}} can forget it. Safe as template.HTML: the
// only interpolated value is a fixed-format RFC 3339 string, escaped anyway.
//
// The timestamp is rendered in the server's zone; RFC 3339 carries the offset,
// so it is unambiguous even though it isn't the viewer's local time.
func timeAgoHTML(t time.Time) template.HTML {
	if t.IsZero() {
		return ""
	}
	stamp := template.HTMLEscapeString(t.Format(time.RFC3339))
	return template.HTML(fmt.Sprintf(
		`<time datetime="%s" title="%s">%s</time>`,
		stamp, stamp, template.HTMLEscapeString(timeAgo(t)),
	))
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
