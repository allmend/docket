package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/allmend/docket/internal/model"
)

// The description editor sends model.BodyVersion of the text it started from,
// and the store compares it with bodyVersionSQL. The two must agree on every
// input, multi-byte text included, or every save would read as a conflict.
func TestUpdateTicketBodyVersionGuard(t *testing.T) {
	st := requireStore(t)
	ctx := context.Background()
	org := seedOrg(t, "body-version")
	user := seedUser(t, org.ID, "bodyv")
	tk := seedTicket(t, org.ID, user.ID, "guarded")

	original := "Första versionen — åäö ✓"
	if _, err := st.UpdateTicketBody(ctx, org.ID, tk.ID, original, ""); err != nil {
		t.Fatalf("unconditional save: %v", err)
	}

	// Saving on top of the version you loaded works.
	theirs := "Their edit"
	if _, err := st.UpdateTicketBody(ctx, org.ID, tk.ID, theirs, model.BodyVersion(original)); err != nil {
		t.Fatalf("save from the current version: %v", err)
	}

	// A second editor that also started from the original is refused, and
	// the stored body keeps the first save.
	_, err := st.UpdateTicketBody(ctx, org.ID, tk.ID, "Mine", model.BodyVersion(original))
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale save: got %v, want pgx.ErrNoRows", err)
	}
	got, err := st.GetTicket(ctx, org.ID, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != theirs {
		t.Fatalf("body = %q after a refused save, want %q", got.Body, theirs)
	}

	// Both editors typing the same text is not a conflict.
	if _, err := st.UpdateTicketBody(ctx, org.ID, tk.ID, theirs, model.BodyVersion(original)); err != nil {
		t.Fatalf("identical stale save: %v", err)
	}

	// Another org's ticket is never written, whatever the version.
	other := seedOrg(t, "body-version-other")
	if _, err := st.UpdateTicketBody(ctx, other.ID, tk.ID, "x", ""); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-org save: got %v, want pgx.ErrNoRows", err)
	}
}
