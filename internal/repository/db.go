package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/models"
)

// DBTX is the interface for executing queries.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

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

// RunInTx runs the given function in a transaction, setting the tenant config.
func (d *DB) RunInTx(ctx context.Context, schoolID string, fn func(ctx context.Context) error) error {
	if tx := TxFromContext(ctx); tx != nil {
		return fn(ctx)
	}

	tenantCtx, tx, err := d.WithTenantTx(ctx, schoolID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(tenantCtx) }()

	if err := fn(tenantCtx); err != nil {
		return err
	}
	return tx.Commit(tenantCtx)
}

// RunRead runs the given function in a read-only transaction, setting the tenant config.
func (d *DB) RunRead(ctx context.Context, schoolID string, fn func(ctx context.Context) error) error {
	if tx := TxFromContext(ctx); tx != nil {
		return fn(ctx)
	}

	tenantCtx, tx, err := d.WithTenantTx(ctx, schoolID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(tenantCtx) }()

	return fn(tenantCtx)
}

// Querier returns the transaction from context, or a self-managing pool querier if none exists.
func (d *DB) Querier(ctx context.Context) DBTX {
	if tx := TxFromContext(ctx); tx != nil {
		return tx
	}
	return &poolQuerier{pool: d.pool}
}

type poolQuerier struct {
	pool *pgxpool.Pool
}

func (q *poolQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	claims := auth.ClaimsFromContext(ctx)
	if claims != nil {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.current_school_id', $1, true)", claims.SchoolID.String()); err != nil {
			return pgconn.CommandTag{}, err
		}
	}
	tag, err := tx.Exec(ctx, sql, arguments...)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	err = tx.Commit(ctx)
	return tag, err
}

func (q *poolQuerier) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	claims := auth.ClaimsFromContext(ctx)
	if claims != nil {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.current_school_id', $1, true)", claims.SchoolID.String()); err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
	}
	rows, err := tx.Query(ctx, sql, arguments...)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return &rowsWrapper{Rows: rows, tx: tx}, nil
}

func (q *poolQuerier) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return &rowWrapper{err: err}
	}
	claims := auth.ClaimsFromContext(ctx)
	if claims != nil {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.current_school_id', $1, true)", claims.SchoolID.String()); err != nil {
			_ = tx.Rollback(ctx)
			return &rowWrapper{err: err}
		}
	}
	row := tx.QueryRow(ctx, sql, arguments...)
	return &rowWrapper{row: row, tx: tx}
}

type rowsWrapper struct {
	pgx.Rows
	tx pgx.Tx
}

func (w *rowsWrapper) Close() {
	w.Rows.Close()
	_ = w.tx.Rollback(context.Background())
}

type rowWrapper struct {
	row pgx.Row
	tx  pgx.Tx
	err error
}

func (w *rowWrapper) Scan(dest ...any) error {
	if w.err != nil {
		return w.err
	}
	err := w.row.Scan(dest...)
	if w.tx != nil {
		if err != nil {
			_ = w.tx.Rollback(context.Background())
		} else {
			if commitErr := w.tx.Commit(context.Background()); commitErr != nil {
				return commitErr
			}
		}
	}
	return err
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
