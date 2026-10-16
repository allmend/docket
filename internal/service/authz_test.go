package service

import (
	"context"
	"errors"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

func TestCanDo(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	caller := func(role string, scope model.TokenScope) context.Context {
		return WithScope(WithIdentity(context.Background(), org, user, role), scope)
	}

	cases := []struct {
		name  string
		ctx   context.Context
		tier  model.Tier
		allow bool
	}{
		{"unset tier is denied even for an admin session", caller("admin", model.ScopeAPIWrite), model.TierUnset, false},
		{"unknown tier is denied", caller("admin", model.ScopeAPIWrite), model.Tier(99), false},
		{"no identity is denied", WithScope(context.Background(), model.ScopeAPIWrite), model.TierRead, false},

		{"member session reads", caller("member", model.ScopeAPIWrite), model.TierRead, true},
		{"member session writes", caller("member", model.ScopeAPIWrite), model.TierWrite, true},
		{"member session is not admin", caller("member", model.ScopeAPIWrite), model.TierAdmin, false},
		{"admin session is admin", caller("admin", model.ScopeAPIWrite), model.TierAdmin, true},

		{"read token reads", caller("member", model.ScopeAPIRead), model.TierRead, true},
		{"read token cannot write", caller("member", model.ScopeAPIRead), model.TierWrite, false},
		{"read token scrapes metrics", caller("member", model.ScopeAPIRead), model.TierMetrics, true},
		{"metrics token scrapes metrics", caller("member", model.ScopeMetricsRead), model.TierMetrics, true},
		{"metrics token cannot read", caller("member", model.ScopeMetricsRead), model.TierRead, false},
		{"empty scope gets nothing", caller("admin", ""), model.TierMetrics, false},
		{"admin role without write scope is not admin", caller("admin", model.ScopeAPIRead), model.TierAdmin, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CanDo(tc.ctx, tc.tier)
			if tc.allow && err != nil {
				t.Fatalf("want allowed, got %v", err)
			}
			if !tc.allow && !errors.Is(err, ErrForbidden) {
				t.Fatalf("want ErrForbidden, got %v", err)
			}
		})
	}
}
