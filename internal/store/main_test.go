package store

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/allmend/docket/internal/model"
)

// testStore is the shared Store backed by a throwaway Postgres container.
// nil when the container could not be started (no Docker, or -short); tests
// call requireStore(t) which skips cleanly in that case.
var (
	testStore     *Store
	testPool      *pgxpool.Pool
	testStoreSkip string
)

func TestMain(m *testing.M) {
	flag.Parse() // required before testing.Short() in a custom TestMain
	if testing.Short() {
		testStoreSkip = "skipped in -short mode (needs Docker/Postgres)"
		os.Exit(m.Run())
	}

	code, err := runWithContainer(m)
	if err != nil {
		// Container couldn't start (e.g. Docker unavailable in CI without a
		// daemon). Don't fail the build — run the package so requireStore skips.
		testStoreSkip = "Postgres testcontainer unavailable: " + err.Error()
		os.Exit(m.Run())
	}
	os.Exit(code)
}

func runWithContainer(m *testing.M) (int, error) {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("docket_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return 0, fmt.Errorf("start container: %w", err)
	}
	defer func() { _ = ctr.Terminate(ctx) }()

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return 0, fmt.Errorf("connection string: %w", err)
	}

	if err := applyMigrations(dsn); err != nil {
		return 0, fmt.Errorf("migrate: %w", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return 0, fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()

	testPool = pool
	// Single-node test DB: primary and replica are the same pool.
	testStore = New(pool, pool)

	return m.Run(), nil
}

func applyMigrations(dsn string) error {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("cannot resolve caller path")
	}
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
	mig, err := migrate.New("file://"+migrationsDir, dsn)
	if err != nil {
		return err
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// requireStore skips the test when no database is available.
func requireStore(t *testing.T) *Store {
	t.Helper()
	if testStore == nil {
		t.Skip(testStoreSkip)
	}
	return testStore
}

// resetDB truncates every application table so each test starts clean.
func resetDB(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	rows, err := testPool.Query(ctx,
		`SELECT tablename FROM pg_tables
		 WHERE schemaname = 'public' AND tablename <> 'schema_migrations'`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		tables = append(tables, `"`+name+`"`)
	}
	rows.Close()
	if len(tables) == 0 {
		return
	}
	stmt := "TRUNCATE " + join(tables, ", ") + " RESTART IDENTITY CASCADE"
	if _, err := testPool.Exec(ctx, stmt); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// --- Seed helpers: minimal rows for exercising store methods. ---

func seedOrg(t *testing.T, slug string) *model.Org {
	t.Helper()
	org, err := testStore.CreateOrg(context.Background(), slug, slug)
	if err != nil {
		t.Fatalf("seed org %q: %v", slug, err)
	}
	return org
}

func seedUser(t *testing.T, orgID uuid.UUID, username string) *model.User {
	t.Helper()
	u, err := testStore.CreateUser(context.Background(), orgID, username, username, username+"@example.com", "member")
	if err != nil {
		t.Fatalf("seed user %q: %v", username, err)
	}
	return u
}

// seedTicket creates an org-scoped board, its first column, and one ticket on it.
func seedTicket(t *testing.T, orgID, createdBy uuid.UUID, title string) *model.Ticket {
	t.Helper()
	ctx := context.Background()
	board, err := testStore.CreateBoard(ctx, orgID, createdBy, nil, "Board", "", model.BoardModeScrum)
	if err != nil {
		t.Fatalf("seed board: %v", err)
	}
	col, err := testStore.CreateColumn(ctx, orgID, board.ID, "To Do", 1000)
	if err != nil {
		t.Fatalf("seed column: %v", err)
	}
	ticket, err := testStore.CreateTicket(ctx, orgID, board.ID, col.ID, createdBy, nil, 1, title, "", model.PriorityMedium, 1000)
	if err != nil {
		t.Fatalf("seed ticket: %v", err)
	}
	return ticket
}
