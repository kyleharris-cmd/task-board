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

func TestService_TaskWorkflowToReady(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
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
	t.Cleanup(func() { _ = svc.Close() })

	ctx := context.Background()
	actor := domain.Actor{Type: domain.ActorTypeAgent, ID: "a-1", DisplayName: "Agent One"}

	taskID, err := svc.CreateTask(ctx, CreateTaskInput{Title: "Design task", TaskType: "design"})
	require.NoError(t, err)

	_, err = svc.ClaimTask(ctx, ClaimTaskInput{TaskID: taskID, Actor: actor, AutoRenew: true})
	require.NoError(t, err)

	_, _, err = svc.AddArtifact(ctx, taskID, domain.ArtifactContext, "context", actor)
	require.NoError(t, err)
	require.NoError(t, svc.TransitionTask(ctx, TransitionInput{TaskID: taskID, ToState: domain.StateContextAdded, Actor: actor}))

	_, _, err = svc.AddArtifact(ctx, taskID, domain.ArtifactDesign, "design", actor)
	require.NoError(t, err)
	require.NoError(t, svc.TransitionTask(ctx, TransitionInput{TaskID: taskID, ToState: domain.StateDesignDrafted, Actor: actor}))
	require.NoError(t, svc.TransitionTask(ctx, TransitionInput{TaskID: taskID, ToState: domain.StateRubricReview, Actor: actor}))

	_, _, err = svc.AddArtifact(ctx, taskID, domain.ArtifactRubricReview, "rubric review", actor)
	require.NoError(t, err)
	require.NoError(t, svc.EvaluateRubric(ctx, taskID, "v1", true, true, "ok", actor))

	require.NoError(t, svc.ReadyCheck(ctx, taskID, actor))
	require.NoError(t, svc.TransitionTask(ctx, TransitionInput{TaskID: taskID, ToState: domain.StateReadyForImplementation, Actor: actor}))
}

func TestService_ClaimDeniedWhenActiveLeaseOwnedByDifferentActor(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
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
	t.Cleanup(func() { _ = svc.Close() })

	ctx := context.Background()
	taskID, err := svc.CreateTask(ctx, CreateTaskInput{Title: "Lease task", TaskType: "default"})
	require.NoError(t, err)

	actor1 := domain.Actor{Type: domain.ActorTypeAgent, ID: "a-1", DisplayName: "Agent One"}
	actor2 := domain.Actor{Type: domain.ActorTypeHuman, ID: "u-1", DisplayName: "User One"}

	_, err = svc.ClaimTask(ctx, ClaimTaskInput{TaskID: taskID, Actor: actor1})
	require.NoError(t, err)

	_, err = svc.ClaimTask(ctx, ClaimTaskInput{TaskID: taskID, Actor: actor2})
	require.Error(t, err)
	require.Contains(t, err.Error(), "actively leased")
}

const testPolicyYAML = `version: 1
lease_required_states:
  - "Context Added"
  - "Design Drafted"
  - "Rubric Review"
  - "Ready for Implementation"
  - "In Progress"
  - "Testing"
  - "Documented"
transitions:
  - from: "Backlog"
    to: "Context Added"
    actor_types: ["human", "agent"]
  - from: "Context Added"
    to: "Design Drafted"
    actor_types: ["human", "agent"]
  - from: "Design Drafted"
    to: "Rubric Review"
    actor_types: ["human", "agent"]
  - from: "Rubric Review"
    to: "Ready for Implementation"
    actor_types: ["human", "agent"]
  - from: "Ready for Implementation"
    to: "In Progress"
    actor_types: ["human", "agent"]
  - from: "In Progress"
    to: "Testing"
    actor_types: ["human", "agent"]
  - from: "Testing"
    to: "Documented"
    actor_types: ["human", "agent"]
  - from: "Documented"
    to: "Done"
    actor_types: ["human", "agent"]
required_artifacts_by_state:
  "Context Added": ["context"]
  "Design Drafted": ["context", "design"]
  "Rubric Review": ["context", "design"]
  "Ready for Implementation": ["context", "design", "rubric_review"]
  "Testing": ["implementation_notes", "test_report"]
  "Documented": ["implementation_notes", "test_report", "docs_update"]
  "Done": ["implementation_notes", "test_report", "docs_update"]
task_type_leases:
  default:
    default_ttl_minutes: 60
    allow_auto_renew: true
  design:
    default_ttl_minutes: 90
    allow_auto_renew: true
  implementation:
    default_ttl_minutes: 120
    allow_auto_renew: true
`
