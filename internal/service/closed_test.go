package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeCloseChecker struct {
	closed bool
	err    error
	calls  int
}

func (f *fakeCloseChecker) IsTicketClosed(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	f.calls++
	return f.closed, f.err
}

func TestAssertTicketOpen(t *testing.T) {
	ctx := context.Background()
	org, ticket := uuid.New(), uuid.New()
	storeErr := errors.New("connection refused")

	tests := []struct {
		name    string
		checker *fakeCloseChecker
		want    error
	}{
		{"open ticket passes", &fakeCloseChecker{closed: false}, nil},
		{"closed ticket is rejected", &fakeCloseChecker{closed: true}, ErrTicketClosed},
		{"store error propagates", &fakeCloseChecker{err: storeErr}, storeErr},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := assertTicketOpen(ctx, tc.checker, org, ticket)
			if !errors.Is(err, tc.want) {
				t.Fatalf("assertTicketOpen = %v, want %v", err, tc.want)
			}
			if tc.checker.calls != 1 {
				t.Errorf("checker called %d times, want 1", tc.checker.calls)
			}
		})
	}
}

// TestErrTicketClosedIsDistinct guards the handler mapping: serviceError
// distinguishes ErrTicketClosed (409) from ErrForbidden (403), so the two
// sentinels must never compare equal.
func TestErrTicketClosedIsDistinct(t *testing.T) {
	if errors.Is(ErrTicketClosed, ErrForbidden) || errors.Is(ErrForbidden, ErrTicketClosed) {
		t.Fatal("ErrTicketClosed and ErrForbidden must be distinct sentinels")
	}
}
