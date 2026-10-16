package model

// Tier is what a route requires of its caller. Every route declares one, and
// service.CanDo is the only place a tier is checked against the caller.
//
// The zero value is TierUnset, which CanDo always denies: a route that never
// declared its tier is closed, not open.
type Tier int

const (
	TierUnset   Tier = iota
	TierMetrics      // scrape business metrics
	TierRead         // read anything in the caller's org
	TierWrite        // change things in the caller's org
	TierAdmin        // org administration: members and roles
)

func (t Tier) String() string {
	switch t {
	case TierMetrics:
		return "metrics"
	case TierRead:
		return "read"
	case TierWrite:
		return "write"
	case TierAdmin:
		return "admin"
	default:
		return "unset"
	}
}
