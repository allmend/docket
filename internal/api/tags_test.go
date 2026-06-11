package api

import (
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

func TestFilterUnusedTags(t *testing.T) {
	bug := model.Tag{ID: uuid.New(), Name: "bug"}
	feat := model.Tag{ID: uuid.New(), Name: "feature"}
	chore := model.Tag{ID: uuid.New(), Name: "chore"}
	all := []model.Tag{bug, feat, chore}

	got := filterUnusedTags(all, []model.Tag{feat})
	if len(got) != 2 {
		t.Fatalf("got %d tags, want 2", len(got))
	}
	for _, tag := range got {
		if tag.ID == feat.ID {
			t.Errorf("applied tag %q still offered", tag.Name)
		}
	}

	if got := filterUnusedTags(all, nil); len(got) != 3 {
		t.Errorf("nothing applied should offer all %d, got %d", len(all), len(got))
	}
	if got := filterUnusedTags(all, all); len(got) != 0 {
		t.Errorf("everything applied should offer none, got %d", len(got))
	}
}

func TestPromEscape(t *testing.T) {
	tests := []struct{ in, want string }{
		{`plain`, `plain`},
		{`with "quotes"`, `with \"quotes\"`},
		{`back\slash`, `back\\slash`},
		{"line\nbreak", `line\nbreak`},
	}
	for _, tt := range tests {
		if got := promEscape(tt.in); got != tt.want {
			t.Errorf("promEscape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
