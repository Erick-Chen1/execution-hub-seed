package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a pgx connection pool.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, config)
}
