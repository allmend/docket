// Package health serves the liveness and readiness endpoints that
// orchestrators probe. They live on their own listener, never the app port,
// so they are not reachable through the Service or the public route.
package health

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// pingTimeout bounds the database check so a hung connection fails the
// probe instead of outlasting the probe's own timeout.
const pingTimeout = time.Second

// Checker holds the state /readyz reports on. Its zero value is not ready:
// migrations have not run, there is no database to ping, and nothing is
// draining.
type Checker struct {
	migrated atomic.Bool
	draining atomic.Bool

	mu   sync.RWMutex
	ping func(context.Context) error
}

// MarkMigrated records that startup migrations have finished.
func (c *Checker) MarkMigrated() { c.migrated.Store(true) }

// SetPing installs the database check, once a pool exists.
func (c *Checker) SetPing(fn func(context.Context) error) {
	c.mu.Lock()
	c.ping = fn
	c.mu.Unlock()
}

// Drain makes /readyz fail from now on, so the orchestrator stops sending
// traffic before the server shuts down.
func (c *Checker) Drain() { c.draining.Store(true) }

type result struct {
	name string
	err  error
}

func (c *Checker) checks(ctx context.Context) []result {
	var out []result

	var mig error
	if !c.migrated.Load() {
		mig = fmt.Errorf("not finished")
	}
	out = append(out, result{"migrations", mig})

	c.mu.RLock()
	ping := c.ping
	c.mu.RUnlock()
	var db error
	if ping == nil {
		db = fmt.Errorf("not connected")
	} else {
		pctx, cancel := context.WithTimeout(ctx, pingTimeout)
		db = ping(pctx)
		cancel()
	}
	out = append(out, result{"database", db})

	var drain error
	if c.draining.Load() {
		drain = fmt.Errorf("shutting down")
	}
	out = append(out, result{"shutdown", drain})
	return out
}

// Handler serves /livez and /readyz. Both answer 200 "ok" or 503; ?verbose
// lists every check as [+]/[-], the way the Kubernetes API server does.
func (c *Checker) Handler() http.Handler {
	mux := http.NewServeMux()
	// Liveness never looks at the database: a Postgres outage must not make
	// the orchestrator restart every replica at once.
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		write(w, r, "livez", nil)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		write(w, r, "readyz", c.checks(r.Context()))
	})
	return mux
}

func write(w http.ResponseWriter, r *http.Request, name string, results []result) {
	verbose := r.URL.Query().Has("verbose")
	failed := false
	var b strings.Builder
	for _, res := range results {
		switch {
		case res.err != nil:
			failed = true
			fmt.Fprintf(&b, "[-]%s failed: %v\n", res.name, res.err)
		case verbose:
			fmt.Fprintf(&b, "[+]%s ok\n", res.name)
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case failed:
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(&b, "%s check failed\n", name)
	case verbose:
		fmt.Fprintf(&b, "%s ok\n", name)
	default:
		b.WriteString("ok\n")
	}
	_, _ = w.Write([]byte(b.String()))
}
