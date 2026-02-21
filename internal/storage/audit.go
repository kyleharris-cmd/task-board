package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kyleharris/task-board/internal/domain"
)

type TransitionEvent struct {
	TaskID     string
	FromState  domain.State
	ToState    domain.State
	Actor      domain.Actor
	Reason     string
	OccurredAt time.Time
}

type ArtifactEvent struct {
	TaskID          string
	ArtifactType    domain.ArtifactType
	MarkdownPath    string
	ContentSnapshot string
	Version         int
	Actor           domain.Actor
	OccurredAt      time.Time
}

type RubricEvent struct {
	TaskID                 string
	RubricVersion          string
	RequiredFieldsComplete bool
	Pass                   bool
	Actor                  domain.Actor
	Notes                  string
	OccurredAt             time.Time
}

func (db *DB) RecordTransition(ctx context.Context, e TransitionEvent) error {
	_, err := db.SQL.ExecContext(
		ctx,
		`INSERT INTO task_transitions (task_id, from_state, to_state, actor_type, actor_id, actor_display_name, reason, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.TaskID,
		e.FromState,
		e.ToState,
		e.Actor.Type,
		e.Actor.ID,
		e.Actor.DisplayName,
		e.Reason,
		e.OccurredAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert task transition: %w", err)
	}

	return nil
}

func (db *DB) RecordArtifact(ctx context.Context, e ArtifactEvent) error {
	_, err := db.SQL.ExecContext(
		ctx,
		`INSERT INTO task_artifacts (task_id, artifact_type, markdown_path, content_snapshot, version, actor_type, actor_id, actor_display_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.TaskID,
		e.ArtifactType,
		e.MarkdownPath,
		e.ContentSnapshot,
		e.Version,
		e.Actor.Type,
		e.Actor.ID,
		e.Actor.DisplayName,
		e.OccurredAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert task artifact: %w", err)
	}

	return nil
}

func (db *DB) RecordRubricResult(ctx context.Context, e RubricEvent) error {
	_, err := db.SQL.ExecContext(
		ctx,
		`INSERT INTO rubric_results (task_id, rubric_version, required_fields_complete, pass_fail, actor_type, actor_id, actor_display_name, notes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.TaskID,
		e.RubricVersion,
		boolToInt(e.RequiredFieldsComplete),
		boolToInt(e.Pass),
		e.Actor.Type,
		e.Actor.ID,
		e.Actor.DisplayName,
		e.Notes,
		e.OccurredAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert rubric result: %w", err)
	}

	if _, err := db.SQL.ExecContext(
		ctx,
		`UPDATE tasks SET rubric_passed = ?, updated_at = ? WHERE id = ?`,
		boolToInt(e.Pass),
		e.OccurredAt.UTC().Format(time.RFC3339),
		e.TaskID,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("update task rubric status: %w", err)
	}

	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
