package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "HTTP request duration by method, path, and status.",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "path", "status"})

// Metrics records Prometheus HTTP metrics.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		httpDuration.WithLabelValues(
			r.Method,
			r.URL.Path,
			strconv.Itoa(rw.status),
		).Observe(time.Since(start).Seconds())
	})
}
