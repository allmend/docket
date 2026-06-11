package api

import "testing"

// TestParseTemplates guards against template breakage at startup: every page
// must parse, and pages must be able to reach the partials they invoke.
// Execution errors from a missing {{template "x"}} only surface at render
// time, so the lookups below cover the cross-file references.
func TestParseTemplates(t *testing.T) {
	tmpls, err := parseTemplates("../../templates")
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}

	pages := []string{
		"board.html", "planning.html", "backlog.html", "daily.html",
		"dashboard.html", "team.html", "teams.html", "ticket-page.html",
		"login.html", "inbox.html", "my-issues.html", "settings.html",
		"retro.html", "retros.html", "roadmap.html", "sprint-review.html",
	}
	for _, name := range pages {
		if tmpls[name] == nil {
			t.Errorf("page template %s missing from parsed set", name)
		}
	}

	// Cross-file template references that have broken before.
	lookups := map[string][]string{
		"planning.html":         {"planning-columns", "board-column", "backlog-planning-card", "add-column-card"},
		"board.html":            {"board-columns", "board-card"},
		"planning-columns.html": {"board-column", "backlog-planning-card", "add-column-card"},
		"board-columns.html":    {"board-card", "add-column-card"},
	}
	for page, names := range lookups {
		set := tmpls[page]
		if set == nil {
			t.Errorf("template %s missing from parsed set", page)
			continue
		}
		for _, name := range names {
			if set.Lookup(name) == nil {
				t.Errorf("%s cannot reach template %q", page, name)
			}
		}
	}
}
