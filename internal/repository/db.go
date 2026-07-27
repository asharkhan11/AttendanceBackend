package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/school/attendance-backend/internal/models"
)

// DB wraps a pgxpool and provides tenant-aware helpers.
type DB struct {
	pool *pgxpool.Pool
}

func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{pool: pool}
}

// Pool returns the underlying pool (for health checks etc.)
func (d *DB) Pool() *pgxpool.Pool { return d.pool }

// WithTenantTx begins a transaction, sets app.current_school_id for RLS,
// stores the tx in ctx, and returns the enriched ctx and a commit function.
// The caller must defer the rollback and call commit on success.
func (d *DB) WithTenantTx(ctx context.Context, schoolID string) (context.Context, pgx.Tx, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return ctx, nil, fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.Exec(ctx,
		"SELECT set_config('app.current_school_id', $1, true)", schoolID); err != nil {
		_ = tx.Rollback(ctx)
		return ctx, nil, fmt.Errorf("set tenant config: %w", err)
	}
	ctx = context.WithValue(ctx, models.CtxTx, tx)
	return ctx, tx, nil
}

// TxFromContext retrieves the transaction stored by WithTenantTx.
func TxFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(models.CtxTx).(pgx.Tx)
	return tx
}

// Querier returns the transaction from context (for use in repository methods).
func (d *DB) Querier(ctx context.Context) pgx.Tx {
	return TxFromContext(ctx)
}

// Ping verifies DB connectivity (no tenant required).
func (d *DB) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

// PoolConfig holds tuning parameters for the connection pool.
type PoolConfig struct {
	MaxConns        int32
	MinConns        int32
	MaxConnIdleTime time.Duration
	MaxConnLifetime time.Duration
}

// Connect creates a pgxpool from a connection URL with the given pool config.
func Connect(ctx context.Context, url string, pc PoolConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	cfg.MaxConns = pc.MaxConns
	cfg.MinConns = pc.MinConns
	cfg.MaxConnIdleTime = pc.MaxConnIdleTime
	cfg.MaxConnLifetime = pc.MaxConnLifetime
	cfg.HealthCheckPeriod = 30 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}
