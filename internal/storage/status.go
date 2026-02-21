package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kyleharris/task-board/internal/domain"
)

type TaskStatusRow struct {
	Task        domain.Task
	Lease       *domain.Lease
	LeaseActive bool
}

func (db *DB) ListTaskStatusRows(ctx context.Context, state *domain.State, now time.Time) ([]TaskStatusRow, error) {
	query := `
SELECT
  t.id,
  COALESCE(t.short_ref, ''),
  t.title,
  COALESCE(t.description, ''),
  t.state,
  t.parent_id,
  t.required_for_parent,
  t.priority,
  t.task_type,
  t.rubric_passed,
  t.updated_at,
  l.actor_type,
  l.actor_id,
  l.lease_expires_at,
  l.auto_renew
FROM tasks t
LEFT JOIN task_leases l ON l.task_id = t.id
`
	args := []any{}
	if state != nil {
		query += " WHERE t.state = ?"
		args = append(args, string(*state))
	}
	query += " ORDER BY t.updated_at DESC, t.id ASC"

	rows, err := db.SQL.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list task status rows: %w", err)
	}
	defer rows.Close()

	out := []TaskStatusRow{}
	for rows.Next() {
		var (
			task              domain.Task
			parentID          sql.NullString
			requiredForParent int
			rubricPassed      int
			updatedAtRaw      string
			leaseActorType    sql.NullString
			leaseActorID      sql.NullString
			leaseExpiresRaw   sql.NullString
			leaseAutoRenew    sql.NullInt64
		)

		if err := rows.Scan(
			&task.ID,
			&task.ShortRef,
			&task.Title,
			&task.Description,
			&task.State,
			&parentID,
			&requiredForParent,
			&task.Priority,
			&task.TaskType,
			&rubricPassed,
			&updatedAtRaw,
			&leaseActorType,
			&leaseActorID,
			&leaseExpiresRaw,
			&leaseAutoRenew,
		); err != nil {
			return nil, fmt.Errorf("scan task status row: %w", err)
		}

		if parentID.Valid {
			p := strings.TrimSpace(parentID.String)
			task.ParentID = &p
		}
		task.RequiredForParent = requiredForParent == 1
		task.RubricPassed = rubricPassed == 1
		if ts, err := time.Parse(time.RFC3339, updatedAtRaw); err == nil {
			task.UpdatedAt = ts
		}

		row := TaskStatusRow{Task: task}
		if leaseActorType.Valid && leaseActorID.Valid && leaseExpiresRaw.Valid {
			expiresAt, err := time.Parse(time.RFC3339, leaseExpiresRaw.String)
			if err != nil {
				return nil, fmt.Errorf("parse lease_expires_at for task %s: %w", task.ID, err)
			}
			lease := domain.Lease{
				TaskID:    task.ID,
				ActorType: domain.ActorType(leaseActorType.String),
				ActorID:   leaseActorID.String,
				ExpiresAt: expiresAt,
				AutoRenew: leaseAutoRenew.Valid && leaseAutoRenew.Int64 == 1,
			}
			row.Lease = &lease
			row.LeaseActive = expiresAt.After(now)
		}

		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task status rows: %w", err)
	}

	return out, nil
}
