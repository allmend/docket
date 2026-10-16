package service

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// CanDo reports whether the caller on ctx may use something that requires
// tier. It is the single place a tier is resolved; routes declare their tier
// and never check roles or scopes themselves. Anything it does not explicitly
// allow is denied with ErrForbidden, including TierUnset.
//
// Today a tier depends on the org role and the token scope only. Team roles
// will add a team to the decision; this is where that goes.
func CanDo(ctx context.Context, tier model.Tier) error {
	if OrgIDFromContext(ctx) == uuid.Nil || UserIDFromContext(ctx) == uuid.Nil {
		return ErrForbidden
	}
	scope := ScopeFromContext(ctx)

	var ok bool
	switch tier {
	case model.TierMetrics:
		ok = scope.Allows(model.ScopeMetricsRead)
	case model.TierRead:
		ok = scope.Allows(model.ScopeAPIRead)
	case model.TierWrite:
		ok = scope.Allows(model.ScopeAPIWrite)
	case model.TierAdmin:
		// API tokens act as "member" whatever their creator's role, so only a
		// signed-in admin reaches this tier until tokens get an admin scope.
		ok = scope.Allows(model.ScopeAPIWrite) && RoleFromContext(ctx) == "admin"
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}
