package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleharris/task-board/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRecordAuditEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "board.db")
	db, err := Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.Migrate(ctx))
	now := time.Now().UTC()

	_, err = db.SQL.ExecContext(ctx, `
		INSERT INTO boards (id, repo_root, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, "default", "/tmp/repo", now.Format(time.RFC3339), now.Format(time.RFC3339))
	require.NoError(t, err)

	_, err = db.SQL.ExecContext(ctx, `
		INSERT INTO tasks (id, board_id, title, state, task_type, rubric_passed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "T-1", "default", "Task", string(domain.StateBacklog), "default", 0, now.Format(time.RFC3339), now.Format(time.RFC3339))
	require.NoError(t, err)

	actor := domain.Actor{Type: domain.ActorTypeAgent, ID: "agent-1", DisplayName: "Agent 1"}
	require.NoError(t, db.RecordTransition(ctx, TransitionEvent{
		TaskID:     "T-1",
		FromState:  domain.StateBacklog,
		ToState:    domain.StateContextAdded,
		Actor:      actor,
		Reason:     "context gathered",
		OccurredAt: now,
	}))
	require.NoError(t, db.RecordArtifact(ctx, ArtifactEvent{
		TaskID:          "T-1",
		ArtifactType:    domain.ArtifactContext,
		MarkdownPath:    ".taskboard/tasks/T-1/context.md",
		ContentSnapshot: "context v1",
		Version:         1,
		Actor:           actor,
		OccurredAt:      now,
	}))
	require.NoError(t, db.RecordRubricResult(ctx, RubricEvent{
		TaskID:                 "T-1",
		RubricVersion:          "v1",
		RequiredFieldsComplete: true,
		Pass:                   true,
		Actor:                  actor,
		Notes:                  "looks good",
		OccurredAt:             now,
	}))

	var transitions int
	require.NoError(t, db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_transitions WHERE task_id = ?`, "T-1").Scan(&transitions))
	require.Equal(t, 1, transitions)

	var artifacts int
	require.NoError(t, db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_artifacts WHERE task_id = ?`, "T-1").Scan(&artifacts))
	require.Equal(t, 1, artifacts)

	var rubricPassed int
	require.NoError(t, db.SQL.QueryRowContext(ctx, `SELECT rubric_passed FROM tasks WHERE id = ?`, "T-1").Scan(&rubricPassed))
	require.Equal(t, 1, rubricPassed)
}
