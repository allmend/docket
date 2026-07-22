package api

import (
	"bytes"
	"strings"
	"testing"

	"github.com/allmend/docket/internal/version"
)

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
		// The activity stream is one shared partial used by the drawer and the
		// permalink page; it in turn renders the comment partial.
		"ticket-page.html":   {"ticket-activity", "comment", "rich-editor"},
		"ticket-detail.html": {"ticket-activity", "comment", "rich-editor"},
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

// TestLoginPageRenders executes the login page end to end. It is the one page
// served on the base-auth layout, which is easy to forget when a dependency is
// added to base.html — the show-password toggle shipped broken for exactly that
// reason: login.html uses Alpine directives but base-auth.html never loaded
// Alpine, so the button did nothing and x-cloak hid the "hide" icon forever.
func TestLoginPageRenders(t *testing.T) {
	tmpls, err := parseTemplates("../../templates")
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	set := tmpls["login.html"]
	if set == nil {
		t.Fatal("login.html missing from parsed set")
	}

	var buf bytes.Buffer
	// Same shape the handler passes. Not nil: templates run under
	// missingkey=error, which rejects nil data outright.
	data := map[string]any{"Error": "", "OrgName": "Acme"}
	if err := set.ExecuteTemplate(&buf, "base", data); err != nil {
		t.Fatalf("execute login.html: %v", err)
	}
	html := buf.String()

	// login.html drives the show-password toggle with x-data/@click/x-show.
	if strings.Contains(html, "x-data") && !strings.Contains(html, "alpine.min.js") {
		t.Error("login page uses Alpine directives but the layout does not load alpine.min.js")
	}
	// The version is rendered from internal/version, never hardcoded.
	if want := "v" + version.Version; !strings.Contains(html, want) {
		t.Errorf("login page does not show %q", want)
	}
	if strings.Contains(html, "v0.1.0") && version.Version != "0.1.0" {
		t.Error("login page still carries the hardcoded v0.1.0 version")
	}
}
