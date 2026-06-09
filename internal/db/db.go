package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"time"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Connect(dsn string) (*sql.DB, error) {
	database, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(10)
	database.SetConnMaxLifetime(time.Hour)

	var pingErr error

	for i := 0; i < 20; i++ {
		if pingErr = database.Ping(); pingErr == nil {
			return database, nil
		}

		time.Sleep(time.Second)
	}
	_ = database.Close()

	return nil, fmt.Errorf("database ping failed after retries: %w", pingErr)
}

func Migrate(database *sql.DB) error {
	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}

	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))

	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}

	sort.Strings(names)

	for _, name := range names {
		var exists bool

		if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, name).Scan(&exists); err != nil {
			return err
		}

		if exists {
			continue
		}

		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}

		tx, err := database.Begin()
		if err != nil {
			return err
		}

		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", name, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations(name) VALUES($1)`, name); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}
