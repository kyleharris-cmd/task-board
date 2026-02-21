package storage

import (
	"context"
	"fmt"
	"time"
)

func (db *DB) UpsertBoard(ctx context.Context, id, repoRoot string, now time.Time) error {
	_, err := db.SQL.ExecContext(
		ctx,
		`INSERT INTO boards (id, repo_root, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET repo_root = excluded.repo_root, updated_at = excluded.updated_at`,
		id,
		repoRoot,
		now.UTC().Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert board: %w", err)
	}

	return nil
}
