// Package store wraps the pgx connection pool, hand-written queries, and the
// golang-migrate migration runner. Everything that touches Postgres goes
// through here so the rest of the app stays driver-agnostic.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DBTX is implemented by both *pgxpool.Pool and pgx.Tx so query methods can run
// inside or outside a transaction without duplication.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Queries owns a single DBTX handle. Service code typically holds one Queries
// rooted at the pool, then constructs a transactional Queries by calling
// `q.WithTx(tx)` inside a transaction.
type Queries struct {
	db DBTX
}

// New wraps a DBTX (pool or transaction) in a Queries.
func New(db DBTX) *Queries {
	return &Queries{db: db}
}

// WithTx returns a copy of q bound to the given pgx.Tx.
func (q *Queries) WithTx(tx pgx.Tx) *Queries {
	return &Queries{db: tx}
}

// OpenPool dials Postgres and returns a pgx connection pool with sensible
// defaults for a long-lived API server.
func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}

// InTx runs fn inside a transaction. Rolls back on error or panic.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func newMigrate(dsn string) (*migrate.Migrate, error) {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}
	src, err := iofs.New(sub, ".")
	if err != nil {
		return nil, err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+trimScheme(dsn))
	if err != nil {
		return nil, err
	}
	return m, nil
}

// trimScheme strips a leading postgres:// or postgresql:// since the migrate
// driver wants only the DSN body. The migrate pgx5 driver re-adds its own.
func trimScheme(dsn string) string {
	for _, p := range []string{"postgres://", "postgresql://"} {
		if len(dsn) >= len(p) && dsn[:len(p)] == p {
			return dsn[len(p):]
		}
	}
	return dsn
}

// MigrateUp runs every pending migration.
func MigrateUp(dsn string) error {
	m, err := newMigrate(dsn)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrateDown reverses every migration.
func MigrateDown(dsn string) error {
	m, err := newMigrate(dsn)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrationVersion reports the current schema version, or 0 if uninitialised.
func MigrationVersion(dsn string) (uint, bool, error) {
	m, err := newMigrate(dsn)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return v, dirty, err
}

// ErrNotFound is returned by every Get* method when no row matches.
var ErrNotFound = errors.New("not found")

// normalize wraps pgx.ErrNoRows as ErrNotFound so callers can use errors.Is.
func normalize(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
