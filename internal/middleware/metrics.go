package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "docket_http_request_duration_seconds",
	Help:    "HTTP request duration by method, route pattern, and status.",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "path", "status"})

// Metrics records Prometheus HTTP metrics using the Chi route pattern as the
// path label (never the raw URL, which explodes cardinality with ticket IDs etc).
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(rw, r)

		// Use a constant sentinel for unmatched routes (404s). Falling back to
		// the raw URL would let an unauthenticated caller mint an unbounded
		// number of label series by spraying distinct paths — the exact
		// cardinality explosion the route pattern is meant to avoid.
		path := "other"
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			if p := rctx.RoutePattern(); p != "" {
				path = p
			}
		}

		httpDuration.WithLabelValues(
			r.Method,
			path,
			strconv.Itoa(rw.Status()),
		).Observe(time.Since(start).Seconds())
	})
}
