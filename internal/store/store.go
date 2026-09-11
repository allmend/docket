package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// dbtx is what both *pgxpool.Pool and pgx.Tx provide, so every store method runs
// unchanged against a pool or inside a transaction.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Store holds primary (write) and replica (read) database handles.
type Store struct {
	primary dbtx
	replica dbtx
}

func New(primary, replica *pgxpool.Pool) *Store {
	return &Store{primary: primary, replica: replica}
}

// InTx runs fn against a Store bound to a single transaction, committing when fn
// returns nil and rolling back otherwise. Reads inside go to the transaction as
// well: a replica can't see uncommitted writes, and a multi-step change has to
// read its own. On a Store that is already transactional this nests as a
// savepoint.
func (s *Store) InTx(ctx context.Context, fn func(tx *Store) error) error {
	return pgx.BeginFunc(ctx, s.primary, func(tx pgx.Tx) error {
		return fn(&Store{primary: tx, replica: tx})
	})
}

// Connect opens and validates a pgxpool connection.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// noRowsIfUnaffected turns an Exec that matched nothing into pgx.ErrNoRows, the
// same signal a guarded INSERT … RETURNING gives when its ownership check fails.
func noRowsIfUnaffected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
