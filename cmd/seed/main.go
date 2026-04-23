// seed bootstraps a fresh Docket database with an org and admin user.
// Run once after first migration:
//
//	go run ./cmd/seed
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/allmend/docket/internal/config"
	"github.com/allmend/docket/internal/service"
	"github.com/allmend/docket/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	st := store.New(db, db)
	authSvc := service.NewAuthService(st, cfg.JWTSecret)

	orgName := envOr("SEED_ORG_NAME", "Allmend")
	orgSlug := envOr("SEED_ORG_SLUG", "allmend")
	username := envOr("SEED_USERNAME", "admin")
	name := envOr("SEED_NAME", "Admin")
	email := envOr("SEED_EMAIL", "admin@example.com")
	password := envOr("SEED_PASSWORD", "changeme")

	user, err := authSvc.CreateOrgWithAdmin(ctx, orgName, orgSlug, username, name, email, password)
	if err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}

	fmt.Printf("\nDocket seeded successfully!\n\n")
	fmt.Printf("  Org:      %s (slug: %s)\n", orgName, orgSlug)
	fmt.Printf("  Username: %s\n", username)
	fmt.Printf("  Password: %s\n", password)
	fmt.Printf("  User ID:  %s\n\n", user.ID)
	fmt.Printf("Login at http://localhost:%s/login\n\n", cfg.HTTPPort)
	if password == "changeme" {
		fmt.Printf("  ⚠  Change the password — override with SEED_PASSWORD=yourpassword\n\n")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
