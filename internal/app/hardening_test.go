package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleharris/task-board/internal/domain"
	"github.com/kyleharris/task-board/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestLeaseExpiryAllowsReclaim(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	svc := setupService(t, repoRoot)
	t.Cleanup(func() { _ = svc.Close() })

	ctx := context.Background()
	taskID, err := svc.CreateTask(ctx, CreateTaskInput{Title: "Expiring lease task", TaskType: "default"})
	require.NoError(t, err)

	actor1 := domain.Actor{Type: domain.ActorTypeAgent, ID: "a1", DisplayName: "Agent 1"}
	actor2 := domain.Actor{Type: domain.ActorTypeHuman, ID: "u1", DisplayName: "User 1"}

	_, err = svc.ClaimTask(ctx, ClaimTaskInput{TaskID: taskID, Actor: actor1, TTLMinutes: 60})
	require.NoError(t, err)

	_, err = svc.db.SQL.ExecContext(ctx, `UPDATE task_leases SET lease_expires_at = ? WHERE task_id = ?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339), taskID)
	require.NoError(t, err)

	_, err = svc.ClaimTask(ctx, ClaimTaskInput{TaskID: taskID, Actor: actor2, TTLMinutes: 60})
	require.NoError(t, err)
}

func TestParentReadyCheckBlockedByRequiredChild(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	svc := setupService(t, repoRoot)
	t.Cleanup(func() { _ = svc.Close() })

	ctx := context.Background()
	actor := domain.Actor{Type: domain.ActorTypeAgent, ID: "a1", DisplayName: "Agent 1"}

	parentID, err := svc.CreateTask(ctx, CreateTaskInput{Title: "Parent", TaskType: "design"})
	require.NoError(t, err)
	childID, err := svc.CreateTask(ctx, CreateTaskInput{Title: "Child", TaskType: "design", ParentID: &parentID, RequiredForParent: true})
	require.NoError(t, err)

	_, err = svc.ClaimTask(ctx, ClaimTaskInput{TaskID: parentID, Actor: actor, TTLMinutes: 60})
	require.NoError(t, err)
	_, err = svc.ClaimTask(ctx, ClaimTaskInput{TaskID: childID, Actor: actor, TTLMinutes: 60})
	require.NoError(t, err)

	_, _, err = svc.AddArtifact(ctx, parentID, domain.ArtifactContext, "ctx", actor)
	require.NoError(t, err)
	_, _, err = svc.AddArtifact(ctx, parentID, domain.ArtifactDesign, "design", actor)
	require.NoError(t, err)
	_, _, err = svc.AddArtifact(ctx, parentID, domain.ArtifactRubricReview, "rr", actor)
	require.NoError(t, err)

	_, err = svc.db.SQL.ExecContext(ctx, `UPDATE tasks SET state = ?, rubric_passed = 1 WHERE id = ?`, string(domain.StateRubricReview), parentID)
	require.NoError(t, err)
	_, err = svc.db.SQL.ExecContext(ctx, `UPDATE tasks SET rubric_passed = 0 WHERE id = ?`, childID)
	require.NoError(t, err)

	err = svc.ReadyCheck(ctx, parentID, actor)
	require.Error(t, err)
	require.Contains(t, err.Error(), "required child")
}

func setupService(t *testing.T, repoRoot string) *Service {
	t.Helper()
	taskboardDir := filepath.Join(repoRoot, ".taskboard")
	require.NoError(t, os.MkdirAll(filepath.Join(taskboardDir, "tasks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(taskboardDir, "policy.yaml"), []byte(testPolicyYAML), 0o644))

	db, err := storage.Open(filepath.Join(taskboardDir, "board.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background()))
	require.NoError(t, db.UpsertBoard(context.Background(), "default", repoRoot, time.Now().UTC()))
	require.NoError(t, db.Close())

	svc, err := OpenService(repoRoot)
	require.NoError(t, err)
	return svc
}
