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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/allmend/docket/internal/api"
	"github.com/allmend/docket/internal/config"
	"github.com/allmend/docket/internal/health"
	appMiddleware "github.com/allmend/docket/internal/middleware"
	"github.com/allmend/docket/internal/model"
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
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck(os.Args[2:]))
	}

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

	// The health listener outlives ctx: during the shutdown drain it must keep
	// answering, with /readyz failing, rather than refuse connections.
	checker := &health.Checker{}
	healthCtx, stopHealth := context.WithCancel(context.Background())
	defer stopHealth()
	startHealthServer(healthCtx, cfg, checker)

	// Run migrations.
	if err := runMigrations(cfg.DatabaseURL); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("migrations", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations ok")
	checker.MarkMigrated()

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
	checker.SetPing(primary.Ping)

	st := store.New(primary, primary) // replica slot is unused today: reads also go to the primary

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
	if err := autoSeed(ctx, authSvc, cfg); err != nil {
		slog.Error("refusing to start", "err", err)
		os.Exit(1)
	}

	switch *mode {
	case "api", "all":
		if err := startAPI(ctx, cfg, checker, authSvc, teamSvc, boardSvc, ticketSvc, commentSvc, linkSvc, notifSvc, retroSvc, metricsSvc, tokenSvc); err != nil {
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
	checker *health.Checker,
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
	h, err := api.NewHandler(authSvc, teamSvc, boardSvc, ticketSvc, commentSvc, linkSvc, notifSvc, retroSvc, metricsSvc, tokenSvc, "templates", cfg.CookieSecure)
	if err != nil {
		return fmt.Errorf("init handler: %w", err)
	}

	// Metrics server — separate port, not exposed through the app reverse proxy.
	metricsSrv := &http.Server{
		Addr:         cfg.MetricsAddr + ":" + cfg.MetricsPort,
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
	r.Use(appMiddleware.LimitBody(maxRequestBody))
	r.Use(compressResponses())

	// Static assets — directory listings disabled, files only.
	r.Handle("/static/*", http.StripPrefix("/static/", staticCaching(noDirListing(http.FileServer(http.Dir(api.StaticDir))))))

	// Public routes (no auth)
	r.Get("/login", h.LoginPage)
	// Rate-limit login attempts per account, plus a loose ceiling on the endpoint
	// as a whole. Deliberately not keyed on IP: behind an ingress or gateway every
	// request carries the proxy's address, so one attacker would lock out the whole
	// instance, and the RealIP variants trust X-Forwarded-For from anyone that can
	// reach the pod, which turns the limit off for whoever forges it. Keying on the
	// username costs an attacker the ability to lock one known account for five
	// minutes and costs everyone else nothing.
	r.With(loginRateLimiters()...).Post("/login", h.Login)
	r.Post("/logout", h.Logout)
	r.Post("/auth/refresh", h.RefreshToken)

	// Authenticated routes — HTMX UI (HTML responses)
	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.Authenticate(authSvc, tokenSvc, cfg.CookieSecure))
		// The UI surface mutates freely, so read-only and metrics-only API
		// tokens must not reach it. JWT sessions always carry api:write scope.
		r.Use(appMiddleware.RequireScope(model.ScopeAPIWrite))
		h.Routes(r)
	})

	// Public API v1 — JSON responses, human-readable identifiers
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(appMiddleware.Authenticate(authSvc, tokenSvc, cfg.CookieSecure))
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
		// Fail readiness first and keep serving for ShutdownDelay, so the
		// orchestrator takes this replica out of rotation before it stops
		// accepting connections.
		checker.Drain()
		slog.Info("draining", "delay", cfg.ShutdownDelay.String())
		time.Sleep(cfg.ShutdownDelay)
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

// maxRequestBody bounds every request body. Tickets and comments are markdown
// text; 2 MiB leaves room for very long descriptions without letting a single
// request tie up memory.
const maxRequestBody = 2 << 20

// loginRateLimiters returns the middleware guarding POST /login: a per-account
// limit, plus a ceiling on the endpoint as a whole.
func loginRateLimiters() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		httprate.Limit(10, 5*time.Minute, httprate.WithKeyFuncs(keyByLoginUsername)),
		httprate.LimitAll(100, 5*time.Minute),
	}
}

// keyByLoginUsername buckets login attempts by the account being tried. Calling
// ParseForm here is safe: it caches into r.PostForm, so the handler's own parse
// reuses the result rather than reading a consumed body.
func keyByLoginUsername(r *http.Request) (string, error) {
	if err := r.ParseForm(); err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(r.PostFormValue("username"))), nil
}

// noDirListing rejects directory paths so the file server never renders an index.
// compressResponses gzips/deflates text responses — static JS/CSS, full pages and
// HTMX partials alike. Without it every cold page load shipped ~280 KB of assets
// that compress to ~80 KB. Level 5 is chi's recommended speed/size balance; the
// default type list covers html, css, js, json and svg.
func compressResponses() func(http.Handler) http.Handler {
	return middleware.Compress(5)
}

func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// staticCaching lets browsers keep a content-hashed asset (?v=, see the asset
// template func) forever: a new build changes its URL. Anything else, such as a
// stale bookmark to /static/app.css, revalidates on every use.
func staticCaching(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

func runMigrations(dsn string) error {
	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}

	// A database migrated by a newer release has a version this build has no
	// file for, and m.Up() fails on it ("no migration found for version N"),
	// which turned every image rollback into a crash loop. Migrations stay
	// compatible with the previous release's code, so starting is safe: warn
	// and skip. A dirty database still goes to m.Up(), which refuses it.
	dbVersion, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("read schema version: %w", err)
	}
	files, err := os.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name())
	}
	latest := latestMigration(names)
	if schemaAhead(dbVersion, dirty, latest) {
		slog.Warn("database schema is newer than this build; starting anyway",
			"db_version", dbVersion, "build_version", latest)
		return migrate.ErrNoChange
	}
	return m.Up()
}

// latestMigration returns the highest version among golang-migrate file names
// ("000025_name.up.sql"). Names that don't start with a number are ignored.
func latestMigration(names []string) uint {
	var latest uint
	for _, n := range names {
		if !strings.HasSuffix(n, ".up.sql") {
			continue
		}
		num, _, ok := strings.Cut(n, "_")
		if !ok {
			continue
		}
		v, err := strconv.ParseUint(num, 10, 64)
		if err != nil {
			continue
		}
		if uint(v) > latest {
			latest = uint(v)
		}
	}
	return latest
}

// schemaAhead reports whether the database was migrated past what this build
// ships. A dirty database is never "ahead": it needs an operator, not a skip.
func schemaAhead(dbVersion uint, dirty bool, latest uint) bool {
	return !dirty && latest > 0 && dbVersion > latest
}

// autoSeed creates the default org and admin user on a fresh database — and
// only on a fresh database. It used to attempt the seed on every startup and
// rely on the insert colliding, which it does only while SEED_USERNAME still
// names an existing user: change it, or drop the seed env vars after first boot,
// and the next restart quietly created an "admin" admin with password
// "changeme".
//
// On a fresh database it refuses an unset or example SEED_PASSWORD: the seed
// would otherwise create an admin whose password is in Docket's own docs.
func autoSeed(ctx context.Context, authSvc *service.AuthService, cfg *config.Config) error {
	exists, err := authSvc.OrgExists(ctx)
	if err != nil {
		slog.Error("auto-seed: checking for an existing org", "err", err)
		return nil
	}
	if exists {
		return nil
	}
	if err := config.ValidateSeedPassword(os.Getenv("SEED_PASSWORD")); err != nil {
		return err
	}

	orgName := getEnvOr("SEED_ORG_NAME", "Allmend")
	orgSlug := getEnvOr("SEED_ORG_SLUG", "allmend")
	username := getEnvOr("SEED_USERNAME", "admin")
	name := getEnvOr("SEED_NAME", "Admin")
	email := getEnvOr("SEED_EMAIL", "admin@example.com")
	password := os.Getenv("SEED_PASSWORD")

	if _, err := authSvc.CreateOrgWithAdmin(ctx, orgName, orgSlug, username, name, email, password); err != nil {
		slog.Error("auto-seed failed", "err", err)
		return nil
	}
	slog.Info("auto-seed: created org and admin user",
		"org_slug", orgSlug, "username", username)
	return nil
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
