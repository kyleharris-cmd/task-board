package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kyleharris/task-board/internal/domain"
)

func (db *DB) GetLease(ctx context.Context, taskID string) (domain.Lease, bool, error) {
	var (
		lease      domain.Lease
		expiresRaw string
	)
	row := db.SQL.QueryRowContext(ctx, `SELECT task_id, actor_type, actor_id, lease_expires_at, auto_renew FROM task_leases WHERE task_id = ?`, taskID)
	var autoRenew int
	if err := row.Scan(&lease.TaskID, &lease.ActorType, &lease.ActorID, &expiresRaw, &autoRenew); err != nil {
		if err == sql.ErrNoRows {
			return domain.Lease{}, false, nil
		}
		return domain.Lease{}, false, fmt.Errorf("get task lease: %w", err)
	}
	ts, err := time.Parse(time.RFC3339, expiresRaw)
	if err != nil {
		return domain.Lease{}, false, fmt.Errorf("parse lease expiry: %w", err)
	}
	lease.ExpiresAt = ts
	lease.AutoRenew = autoRenew == 1

	return lease, true, nil
}

func (db *DB) UpsertLease(ctx context.Context, taskID string, actor domain.Actor, expiresAt time.Time, autoRenew bool, now time.Time) error {
	_, err := db.SQL.ExecContext(
		ctx,
		`INSERT INTO task_leases (task_id, actor_type, actor_id, actor_display_name, lease_expires_at, auto_renew, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(task_id) DO UPDATE SET
		   actor_type = excluded.actor_type,
		   actor_id = excluded.actor_id,
		   actor_display_name = excluded.actor_display_name,
		   lease_expires_at = excluded.lease_expires_at,
		   auto_renew = excluded.auto_renew,
		   updated_at = excluded.updated_at`,
		taskID,
		actor.Type,
		actor.ID,
		actor.DisplayName,
		expiresAt.UTC().Format(time.RFC3339),
		boolToInt(autoRenew),
		now.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert task lease: %w", err)
	}

	return nil
}

func (db *DB) DeleteLease(ctx context.Context, taskID string) error {
	_, err := db.SQL.ExecContext(ctx, `DELETE FROM task_leases WHERE task_id = ?`, taskID)
	if err != nil {
		return fmt.Errorf("delete task lease: %w", err)
	}

	return nil
}
