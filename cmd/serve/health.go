package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/allmend/docket/internal/config"
	"github.com/allmend/docket/internal/health"
)

// startHealthServer serves /livez and /readyz on their own port until ctx is
// done. It starts before migrations so a startup probe sees "not ready"
// rather than a refused connection.
func startHealthServer(ctx context.Context, cfg *config.Config, checker *health.Checker) {
	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.HealthAddr, cfg.HealthPort),
		Handler:           checker.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("health listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("health server", "err", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
}

// runHealthcheck implements `docket healthcheck [--live]`: GET the local
// health endpoint and exit 0 when it answers 200. The image is distroless, so
// this is what a Docker HEALTHCHECK runs instead of curl.
func runHealthcheck(args []string) int {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	live := fs.Bool("live", false, "check /livez instead of /readyz")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	host := os.Getenv("HEALTH_ADDR")
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	port := os.Getenv("HEALTH_PORT")
	if port == "" {
		port = "9413"
	}
	path := "/readyz"
	if *live {
		path = "/livez"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck:", path, resp.Status)
		return 1
	}
	return 0
}
