// Package metrics provides Docket-specific Prometheus metrics for the
// internal operator endpoint (port 9412).
//
// Business metrics (ticket counts, backlog size, sprint stats) are intentionally
// NOT registered here — they contain tenant data and are served per-org via the
// authenticated GET /api/v1/metrics endpoint instead.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// TicketTransitions counts ticket column moves across all orgs.
// Operator-level flow metric — safe for the internal port since it carries
// org UUID (not slug) and no tenant-identifying names in label values.
var TicketTransitions = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "docket_ticket_transitions_total",
	Help: "Number of ticket column moves by org, team, and column pair.",
}, []string{"org", "team", "from_column", "to_column"})

// SprintUnplannedPoints counts story points added to an active sprint after it started.
var SprintUnplannedPoints = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "docket_sprint_unplanned_points_total",
	Help: "Story points added to an active sprint mid-flight (unplanned work).",
}, []string{"org", "sprint"})
