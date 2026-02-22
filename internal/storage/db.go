package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type DB struct {
	SQL *sql.DB
}

type OpenOptions struct {
	ReadOnly      bool
	BusyTimeoutMS int
}

func Open(path string) (*DB, error) {
	return OpenWithOptions(path, OpenOptions{})
}

func OpenWithOptions(path string, opts OpenOptions) (*DB, error) {
	busyTimeout := opts.BusyTimeoutMS
	if busyTimeout <= 0 {
		busyTimeout = 5000
	}
	mode := "rwc"
	if opts.ReadOnly {
		mode = "ro"
	}
	dsn := fmt.Sprintf("file:%s?mode=%s&_pragma=foreign_keys(1)&_pragma=busy_timeout(%d)", path, mode, busyTimeout)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if !opts.ReadOnly {
		// WAL improves read/write concurrency between separate processes (e.g. agent + status monitor).
		if _, err := sqldb.Exec(`PRAGMA journal_mode=WAL`); err != nil {
			_ = sqldb.Close()
			return nil, fmt.Errorf("enable WAL mode: %w", err)
		}
		if _, err := sqldb.Exec(`PRAGMA synchronous=NORMAL`); err != nil {
			_ = sqldb.Close()
			return nil, fmt.Errorf("set synchronous mode: %w", err)
		}
	}

	return &DB{SQL: sqldb}, nil
}

func (db *DB) Close() error {
	if db == nil || db.SQL == nil {
		return nil
	}
	return db.SQL.Close()
}

func (db *DB) Migrate(ctx context.Context) error {
	if _, err := db.SQL.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var alreadyApplied int
		if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&alreadyApplied); err != nil {
			return fmt.Errorf("check migration record %s: %w", name, err)
		}
		if alreadyApplied > 0 {
			continue
		}

		raw, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		tx, err := db.SQL.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("start migration transaction %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(raw)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("run migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (name, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}
