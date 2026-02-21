package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

func (db *DB) AllocateTaskShortRef(ctx context.Context, boardID string) (string, error) {
	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin short ref tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	value, err := nextTaskRefValue(ctx, tx, boardID)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit short ref tx: %w", err)
	}
	return fmt.Sprintf("T-%d", value), nil
}

func (db *DB) EnsureTaskShortRefs(ctx context.Context, boardID string) error {
	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ensure short refs tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	maxRef := 0
	rows, err := tx.QueryContext(ctx, `SELECT short_ref FROM tasks WHERE board_id = ? AND short_ref IS NOT NULL`, boardID)
	if err != nil {
		return fmt.Errorf("query existing short refs: %w", err)
	}
	for rows.Next() {
		var shortRef string
		if err := rows.Scan(&shortRef); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan existing short ref: %w", err)
		}
		if n := parseTaskRef(shortRef); n > maxRef {
			maxRef = n
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate existing short refs: %w", err)
	}
	_ = rows.Close()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_ref_counters WHERE board_id = ?`, boardID).Scan(&exists); err != nil {
		return fmt.Errorf("query task ref counter: %w", err)
	}
	if exists == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_ref_counters (board_id, next_value) VALUES (?, ?)`, boardID, maxRef+1); err != nil {
			return fmt.Errorf("insert task ref counter: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE task_ref_counters SET next_value = CASE WHEN next_value <= ? THEN ? ELSE next_value END WHERE board_id = ?`, maxRef, maxRef+1, boardID); err != nil {
			return fmt.Errorf("sync task ref counter: %w", err)
		}
	}

	missing, err := tx.QueryContext(ctx, `SELECT id FROM tasks WHERE board_id = ? AND (short_ref IS NULL OR short_ref = '') ORDER BY created_at ASC, id ASC`, boardID)
	if err != nil {
		return fmt.Errorf("query tasks missing short refs: %w", err)
	}
	missingIDs := []string{}
	for missing.Next() {
		var id string
		if err := missing.Scan(&id); err != nil {
			_ = missing.Close()
			return fmt.Errorf("scan missing short ref task: %w", err)
		}
		missingIDs = append(missingIDs, id)
	}
	if err := missing.Err(); err != nil {
		_ = missing.Close()
		return fmt.Errorf("iterate missing short ref tasks: %w", err)
	}
	_ = missing.Close()

	for _, id := range missingIDs {
		n, err := nextTaskRefValue(ctx, tx, boardID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE tasks SET short_ref = ? WHERE id = ?`, fmt.Sprintf("T-%d", n), id); err != nil {
			return fmt.Errorf("update task short ref: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ensure short refs tx: %w", err)
	}
	return nil
}

func nextTaskRefValue(ctx context.Context, tx *sql.Tx, boardID string) (int, error) {
	var nextValue int
	err := tx.QueryRowContext(ctx, `SELECT next_value FROM task_ref_counters WHERE board_id = ?`, boardID).Scan(&nextValue)
	if err == sql.ErrNoRows {
		nextValue = 1
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_ref_counters (board_id, next_value) VALUES (?, ?)`, boardID, 2); err != nil {
			return 0, fmt.Errorf("init task ref counter: %w", err)
		}
		return nextValue, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read task ref counter: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE task_ref_counters SET next_value = ? WHERE board_id = ?`, nextValue+1, boardID); err != nil {
		return 0, fmt.Errorf("increment task ref counter: %w", err)
	}
	return nextValue, nil
}

func parseTaskRef(shortRef string) int {
	parts := strings.Split(strings.TrimSpace(shortRef), "-")
	if len(parts) != 2 {
		return 0
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return n
}
