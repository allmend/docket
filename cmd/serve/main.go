package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/allmend/docket/internal/api"
	"github.com/allmend/docket/internal/config"
	appMiddleware "github.com/allmend/docket/internal/middleware"
	"github.com/allmend/docket/internal/service"
	"github.com/allmend/docket/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	mode := flag.String("mode", "all", "Run mode: all | api | worker")
	migrateOnly := flag.Bool("migrate-only", false, "Run migrations and exit")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.Mode != "" {
		*mode = cfg.Mode
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run migrations.
	if err := runMigrations(cfg.DatabaseURL); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("migrations", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations ok")

	if *migrateOnly {
		return
	}

	// Connect to database (primary = replica in dev / single-node).
	primary, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect primary", "err", err)
		os.Exit(1)
	}
	defer primary.Close()

	st := store.New(primary, primary) // replica = primary until read replica is provisioned

	// Wire services.
	authSvc := service.NewAuthService(st, cfg.JWTSecret)
	boardSvc := service.NewBoardService(st)
	teamSvc := service.NewTeamService(st, boardSvc)
	notifSvc := service.NewNotificationService(st)
	ticketSvc := service.NewTicketService(st, notifSvc)
	commentSvc := service.NewCommentService(st)
	linkSvc := service.NewLinkService(st)
	retroSvc := service.NewRetroService(st, ticketSvc, boardSvc)
	metricsSvc := service.NewMetricsService(st)
	tokenSvc := service.NewTokenService(st)

	// Auto-seed: create default org + admin user on first run.
	// Silently skips if org already exists (idempotent).
	autoSeed(ctx, authSvc, cfg)

	switch *mode {
	case "api", "all":
		if err := startAPI(ctx, cfg, authSvc, teamSvc, boardSvc, ticketSvc, commentSvc, linkSvc, notifSvc, retroSvc, metricsSvc, tokenSvc); err != nil {
			slog.Error("api server", "err", err)
			os.Exit(1)
		}
	case "worker":
		slog.Info("worker mode: no background jobs for docket yet")
		<-ctx.Done()
	default:
		slog.Error("unknown mode", "mode", *mode)
		os.Exit(1)
	}
}

func startAPI(
	ctx context.Context,
	cfg *config.Config,
	authSvc *service.AuthService,
	teamSvc *service.TeamService,
	boardSvc *service.BoardService,
	ticketSvc *service.TicketService,
	commentSvc *service.CommentService,
	linkSvc *service.LinkService,
	notifSvc *service.NotificationService,
	retroSvc *service.RetroService,
	metricsSvc *service.MetricsService,
	tokenSvc *service.TokenService,
) error {
	h, err := api.NewHandler(authSvc, teamSvc, boardSvc, ticketSvc, commentSvc, linkSvc, notifSvc, retroSvc, metricsSvc, tokenSvc, "templates")
	if err != nil {
		return fmt.Errorf("init handler: %w", err)
	}

	// Metrics server — separate port, not exposed through the app reverse proxy.
	metricsSrv := &http.Server{
		Addr:         "127.0.0.1:" + cfg.MetricsPort,
		Handler:      promhttp.Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("metrics listening", "addr", metricsSrv.Addr)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("metrics server", "err", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutCtx)
	}()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(appMiddleware.Logger)
	r.Use(appMiddleware.Metrics)
	r.Use(appMiddleware.SecurityHeaders)

	// Static assets
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static/dist"))))

	// Public routes (no auth)
	r.Get("/login", h.LoginPage)
	// Rate-limit login attempts: 10 per 5 minutes per IP.
	r.With(httprate.LimitByIP(10, 5*time.Minute)).Post("/login", h.Login)
	r.Post("/logout", h.Logout)
	r.Post("/auth/refresh", h.RefreshToken)

	// Authenticated routes — HTMX UI (HTML responses)
	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.Authenticate(authSvc, tokenSvc))
		h.Routes(r)
	})

	// Public API v1 — JSON responses, human-readable identifiers
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(appMiddleware.Authenticate(authSvc, tokenSvc))
		h.V1Routes(r)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Error("shutdown", "err", err)
		}
	}()

	slog.Info("listening", "addr", srv.Addr, "mode", "api")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runMigrations(dsn string) error {
	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}
	return m.Up()
}

// autoSeed creates the default org and admin user on a fresh database.
// Safe to call on every startup — silently skips if the org already exists.
func autoSeed(ctx context.Context, authSvc *service.AuthService, cfg *config.Config) {
	orgName := getEnvOr("SEED_ORG_NAME", "Allmend")
	orgSlug := getEnvOr("SEED_ORG_SLUG", "allmend")
	username := getEnvOr("SEED_USERNAME", "admin")
	name := getEnvOr("SEED_NAME", "Admin")
	email := getEnvOr("SEED_EMAIL", "admin@example.com")
	password := getEnvOr("SEED_PASSWORD", "changeme")
	if os.Getenv("SEED_PASSWORD") == "" {
		slog.Warn("SEED_PASSWORD not set — using default 'changeme'. Change the admin password before exposing this instance.")
	}

	_, err := authSvc.CreateOrgWithAdmin(ctx, orgName, orgSlug, username, name, email, password)
	if err != nil {
		// Org already exists or other transient error — not fatal.
		slog.Debug("auto-seed skipped", "err", err)
		return
	}
	slog.Info("auto-seed: created org and admin user",
		"org_slug", orgSlug, "username", username)
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
