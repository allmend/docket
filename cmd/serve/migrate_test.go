package main

import (
	"os"
	"testing"
)

func TestLatestMigration(t *testing.T) {
	names := []string{
		"000001_init.up.sql", "000001_init.down.sql",
		"000025_ticket_team_cascade.up.sql", "000025_ticket_team_cascade.down.sql",
		"000003_x.up.sql", "README.md", "notanumber_x.up.sql",
	}
	if got := latestMigration(names); got != 25 {
		t.Fatalf("latestMigration = %d, want 25", got)
	}
	if got := latestMigration(nil); got != 0 {
		t.Fatalf("latestMigration(nil) = %d, want 0", got)
	}
}

// TestLatestMigrationMatchesRepo keeps the parser honest against the real
// migrations directory: it must find at least the files that exist today.
func TestLatestMigrationMatchesRepo(t *testing.T) {
	files, err := os.ReadDir("../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Name())
	}
	if got := latestMigration(names); got < 25 {
		t.Fatalf("latestMigration on the repo = %d, want >= 25", got)
	}
}

func TestSchemaAhead(t *testing.T) {
	cases := []struct {
		name   string
		db     uint
		dirty  bool
		latest uint
		ahead  bool
	}{
		{"newer database: start anyway", 26, false, 25, true},
		{"same version", 25, false, 25, false},
		{"older database: migrate up", 24, false, 25, false},
		{"fresh database", 0, false, 25, false},
		{"dirty newer database still fails in Up", 26, true, 25, false},
		{"no migrations found never skips", 26, false, 0, false},
	}
	for _, c := range cases {
		if got := schemaAhead(c.db, c.dirty, c.latest); got != c.ahead {
			t.Errorf("%s: schemaAhead(%d, %v, %d) = %v, want %v", c.name, c.db, c.dirty, c.latest, got, c.ahead)
		}
	}
}
