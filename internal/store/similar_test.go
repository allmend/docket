package store

import (
	"context"
	"testing"
	"time"

	"github.com/allmend/docket/internal/model"
)

func TestSimilarOpenTickets(t *testing.T) {
	st := requireStore(t)
	ctx := context.Background()
	org := seedOrg(t, "similar")
	user := seedUser(t, org.ID, "similar-user")
	login := seedTicket(t, org.ID, user.ID, "Login page rejects valid passwords")
	other := seedTicket(t, org.ID, user.ID, "Roadmap shows completed sprints first")
	closed := seedTicket(t, org.ID, user.ID, "Login button misaligned")
	now := time.Now()
	if _, err := testPool.Exec(ctx, `UPDATE tickets SET closed_at = $3 WHERE org_id = $1 AND id = $2`, org.ID, closed.ID, now); err != nil {
		t.Fatal(err)
	}
	foreignOrg := seedOrg(t, "similar-foreign")
	foreignUser := seedUser(t, foreignOrg.ID, "similar-foreign-user")
	seedTicket(t, foreignOrg.ID, foreignUser.ID, "Login fails with valid password")

	// Extra words and another inflection ("password" vs "passwords") still
	// match; the closed ticket and the other org's ticket never do.
	got, err := st.SimilarOpenTickets(ctx, org.ID, "users can't login with their password", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != login.ID {
		t.Fatalf("got %d tickets (%v), want only %s", len(got), titles(got), login.Title)
	}

	// Stop words alone and punctuation don't match everything, or error.
	for _, q := range []string{"", "the and of", "--- !!! 'quoted' a-b:c"} {
		got, err := st.SimilarOpenTickets(ctx, org.ID, q, 3)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		for _, tk := range got {
			if tk.ID == other.ID {
				t.Fatalf("%q matched an unrelated ticket", q)
			}
		}
	}
}

func titles(ts []model.Ticket) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Title
	}
	return out
}
