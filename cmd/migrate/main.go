// cmd/migrate/main.go — run database migrations.
// Usage:
//
//	go run ./cmd/migrate/ up           # apply all pending migrations
//	go run ./cmd/migrate/ down 1       # roll back N migrations
//	go run ./cmd/migrate/ version      # show current migration version
//
// DATABASE_URL env var must be set (uses the owner/superuser role, not attendance_app).
// For the owner URL, set MIGRATE_DATABASE_URL; if unset falls back to DATABASE_URL.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down|version> [N]")
		os.Exit(1)
	}

	dbURL := os.Getenv("MIGRATE_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL or MIGRATE_DATABASE_URL is required")
		os.Exit(1)
	}

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "internal/db/migrations"
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(1)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "postgres driver:", err)
		os.Exit(1)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsDir, "postgres", driver)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate init:", err)
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			fmt.Fprintln(os.Stderr, "migrate up:", err)
			os.Exit(1)
		}
		v, _, _ := m.Version()
		fmt.Printf("migrations applied — now at version %d\n", v)

	case "down":
		n := 1
		if len(os.Args) >= 3 {
			n, _ = strconv.Atoi(os.Args[2])
		}
		if err := m.Steps(-n); err != nil && err != migrate.ErrNoChange {
			fmt.Fprintln(os.Stderr, "migrate down:", err)
			os.Exit(1)
		}
		v, _, _ := m.Version()
		fmt.Printf("rolled back %d migration(s) — now at version %d\n", n, v)

	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			fmt.Fprintln(os.Stderr, "version:", err)
			os.Exit(1)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)

	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		os.Exit(1)
	}
}
