package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store holds primary (write) and replica (read) database pools.
type Store struct {
	primary *pgxpool.Pool
	replica *pgxpool.Pool
}

func New(primary, replica *pgxpool.Pool) *Store {
	return &Store{primary: primary, replica: replica}
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

// Primary returns the write pool. Use for INSERT / UPDATE / DELETE.
func (s *Store) Primary() *pgxpool.Pool { return s.primary }

// Replica returns the read pool. Use for SELECT.
func (s *Store) Replica() *pgxpool.Pool { return s.replica }
