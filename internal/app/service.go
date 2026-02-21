package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kyleharris/task-board/internal/domain"
	"github.com/kyleharris/task-board/internal/policy"
	"github.com/kyleharris/task-board/internal/storage"
	"github.com/kyleharris/task-board/internal/workflow"
)

const defaultBoardID = "default"

type Service struct {
	repoRoot string
	taskDir  string
	policy   policy.Policy
	db       *storage.DB
}

type ArtifactSnapshot struct {
	MarkdownPath    string
	ContentSnapshot string
	Version         int
	CreatedAt       time.Time
}

func OpenService(repoRoot string) (*Service, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	taskboardDir := filepath.Join(absRoot, ".taskboard")
	dbPath := filepath.Join(taskboardDir, "board.db")
	policyPath := filepath.Join(taskboardDir, "policy.yaml")
	taskDir := filepath.Join(taskboardDir, "tasks")

	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("board DB not found at %s (run taskboard init first)", dbPath)
	}
	p, err := policy.Load(policyPath)
	if err != nil {
		return nil, err
	}

	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, err
	}

	return &Service{
		repoRoot: absRoot,
		taskDir:  taskDir,
		policy:   p,
		db:       db,
	}, nil
}

func (s *Service) Close() error {
	return s.db.Close()
}

type CreateTaskInput struct {
	Title             string
	Description       string
	TaskType          string
	Priority          int
	ParentID          *string
	RequiredForParent bool
}

func (s *Service) CreateTask(ctx context.Context, in CreateTaskInput) (string, error) {
	now := time.Now().UTC()
	id := newTaskID(now)
	if strings.TrimSpace(in.TaskType) == "" {
		in.TaskType = "default"
	}
	if in.Priority == 0 {
		in.Priority = 3
	}
	if err := s.db.CreateTask(ctx, storage.CreateTaskInput{
		ID:                id,
		BoardID:           defaultBoardID,
		Title:             strings.TrimSpace(in.Title),
		Description:       in.Description,
		ParentID:          in.ParentID,
		RequiredForParent: in.RequiredForParent,
		Priority:          in.Priority,
		TaskType:          in.TaskType,
		State:             domain.StateBacklog,
		Now:               now,
	}); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(s.taskDir, id), 0o755); err != nil {
		return "", fmt.Errorf("create task directory: %w", err)
	}
	return id, nil
}

func (s *Service) ListTasks(ctx context.Context, state *domain.State) ([]domain.Task, error) {
	return s.db.ListTasks(ctx, state)
}

func (s *Service) GetTask(ctx context.Context, taskID string) (domain.Task, error) {
	return s.db.GetTask(ctx, taskID)
}

func (s *Service) GetLatestArtifact(ctx context.Context, taskID string, artifactType domain.ArtifactType) (ArtifactSnapshot, bool, error) {
	snap, ok, err := s.db.LatestArtifactSnapshot(ctx, taskID, artifactType)
	if err != nil {
		return ArtifactSnapshot{}, false, err
	}
	return ArtifactSnapshot{
		MarkdownPath:    snap.MarkdownPath,
		ContentSnapshot: snap.ContentSnapshot,
		Version:         snap.Version,
		CreatedAt:       snap.CreatedAt,
	}, ok, nil
}

type ClaimTaskInput struct {
	TaskID     string
	Actor      domain.Actor
	AutoRenew  bool
	TTLMinutes int
}

func (s *Service) ClaimTask(ctx context.Context, in ClaimTaskInput) (time.Time, error) {
	now := time.Now().UTC()
	task, err := s.db.GetTask(ctx, in.TaskID)
	if err != nil {
		return time.Time{}, err
	}

	lease, exists, err := s.db.GetLease(ctx, in.TaskID)
	if err != nil {
		return time.Time{}, err
	}
	if exists && lease.ExpiresAt.After(now) && (lease.ActorID != in.Actor.ID || lease.ActorType != in.Actor.Type) {
		return time.Time{}, fmt.Errorf("task is actively leased by %s:%s until %s", lease.ActorType, lease.ActorID, lease.ExpiresAt.Format(time.RFC3339))
	}

	ttl := in.TTLMinutes
	if ttl <= 0 {
		if rule, ok := s.policy.LeaseRuleForTaskType(task.TaskType); ok {
			ttl = rule.DefaultTTLMinutes
			if in.AutoRenew && !rule.AllowAutoRenew {
				return time.Time{}, fmt.Errorf("task type %q does not allow auto-renew", task.TaskType)
			}
		} else {
			ttl = 60
		}
	}

	expiresAt := now.Add(time.Duration(ttl) * time.Minute)
	if err := s.db.UpsertLease(ctx, in.TaskID, in.Actor, expiresAt, in.AutoRenew, now); err != nil {
		return time.Time{}, err
	}

	return expiresAt, nil
}

func (s *Service) RenewTaskLease(ctx context.Context, taskID string, actor domain.Actor, ttlMinutes int) (time.Time, error) {
	now := time.Now().UTC()
	lease, exists, err := s.db.GetLease(ctx, taskID)
	if err != nil {
		return time.Time{}, err
	}
	if !exists {
		return time.Time{}, fmt.Errorf("task has no active lease")
	}
	if lease.ActorID != actor.ID || lease.ActorType != actor.Type {
		return time.Time{}, fmt.Errorf("task lease is owned by %s:%s", lease.ActorType, lease.ActorID)
	}
	if lease.ExpiresAt.Before(now) {
		return time.Time{}, fmt.Errorf("task lease already expired at %s", lease.ExpiresAt.Format(time.RFC3339))
	}

	task, err := s.db.GetTask(ctx, taskID)
	if err != nil {
		return time.Time{}, err
	}
	if ttlMinutes <= 0 {
		if rule, ok := s.policy.LeaseRuleForTaskType(task.TaskType); ok {
			ttlMinutes = rule.DefaultTTLMinutes
		} else {
			ttlMinutes = 60
		}
	}

	expiresAt := now.Add(time.Duration(ttlMinutes) * time.Minute)
	if err := s.db.UpsertLease(ctx, taskID, actor, expiresAt, lease.AutoRenew, now); err != nil {
		return time.Time{}, err
	}

	return expiresAt, nil
}

func (s *Service) ReleaseTaskLease(ctx context.Context, taskID string, actor domain.Actor) error {
	lease, exists, err := s.db.GetLease(ctx, taskID)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if lease.ActorID != actor.ID || lease.ActorType != actor.Type {
		return fmt.Errorf("task lease is owned by %s:%s", lease.ActorType, lease.ActorID)
	}
	return s.db.DeleteLease(ctx, taskID)
}

type TransitionInput struct {
	TaskID        string
	ToState       domain.State
	Actor         domain.Actor
	Reason        string
	ChildrenReady bool
}

func (s *Service) TransitionTask(ctx context.Context, in TransitionInput) error {
	now := time.Now().UTC()
	task, err := s.db.GetTask(ctx, in.TaskID)
	if err != nil {
		return err
	}

	lease, exists, err := s.db.GetLease(ctx, in.TaskID)
	if err != nil {
		return err
	}
	hasValidLease := exists && lease.ActorID == in.Actor.ID && lease.ActorType == in.Actor.Type && lease.ExpiresAt.After(now)

	artifacts, err := s.db.PresentArtifactTypes(ctx, in.TaskID)
	if err != nil {
		return err
	}

	childrenReady := in.ChildrenReady
	if task.IsParent {
		ready, err := s.db.AreRequiredChildrenRubricReady(ctx, in.TaskID)
		if err != nil {
			return err
		}
		childrenReady = ready
	}

	if err := workflow.ValidateTransition(s.policy, workflow.TransitionInput{
		Task:                task,
		Actor:               in.Actor,
		ToState:             in.ToState,
		HasValidLease:       hasValidLease,
		PresentArtifacts:    artifacts,
		ParentChildrenReady: childrenReady,
	}); err != nil {
		return err
	}

	if err := s.db.UpdateTaskState(ctx, in.TaskID, in.ToState, now); err != nil {
		return err
	}
	return s.db.RecordTransition(ctx, storage.TransitionEvent{
		TaskID:     in.TaskID,
		FromState:  task.State,
		ToState:    in.ToState,
		Actor:      in.Actor,
		Reason:     in.Reason,
		OccurredAt: now,
	})
}

func (s *Service) AddArtifact(ctx context.Context, taskID string, artifactType domain.ArtifactType, content string, actor domain.Actor) (string, int, error) {
	now := time.Now().UTC()
	if _, err := s.db.GetTask(ctx, taskID); err != nil {
		return "", 0, err
	}
	version, err := s.db.LatestArtifactVersion(ctx, taskID, artifactType)
	if err != nil {
		return "", 0, err
	}
	version++

	taskFolder := filepath.Join(s.taskDir, taskID)
	if err := os.MkdirAll(taskFolder, 0o755); err != nil {
		return "", 0, fmt.Errorf("create task artifact folder: %w", err)
	}
	filename := fmt.Sprintf("%s.v%d.md", artifactType, version)
	repoRelative := filepath.ToSlash(filepath.Join(".taskboard", "tasks", taskID, filename))
	absPath := filepath.Join(s.repoRoot, repoRelative)
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return "", 0, fmt.Errorf("write artifact file: %w", err)
	}

	if err := s.db.RecordArtifact(ctx, storage.ArtifactEvent{
		TaskID:          taskID,
		ArtifactType:    artifactType,
		MarkdownPath:    repoRelative,
		ContentSnapshot: content,
		Version:         version,
		Actor:           actor,
		OccurredAt:      now,
	}); err != nil {
		return "", 0, err
	}

	return repoRelative, version, nil
}

func (s *Service) EvaluateRubric(ctx context.Context, taskID, rubricVersion string, requiredFieldsComplete, pass bool, notes string, actor domain.Actor) error {
	now := time.Now().UTC()
	if _, err := s.db.GetTask(ctx, taskID); err != nil {
		return err
	}
	return s.db.RecordRubricResult(ctx, storage.RubricEvent{
		TaskID:                 taskID,
		RubricVersion:          rubricVersion,
		RequiredFieldsComplete: requiredFieldsComplete,
		Pass:                   pass,
		Actor:                  actor,
		Notes:                  notes,
		OccurredAt:             now,
	})
}

func (s *Service) ReadyCheck(ctx context.Context, taskID string, actor domain.Actor) error {
	task, err := s.db.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	lease, exists, err := s.db.GetLease(ctx, taskID)
	if err != nil {
		return err
	}
	artifacts, err := s.db.PresentArtifactTypes(ctx, taskID)
	if err != nil {
		return err
	}
	childrenReady, err := s.db.AreRequiredChildrenRubricReady(ctx, taskID)
	if err != nil {
		return err
	}
	return workflow.ValidateTransition(s.policy, workflow.TransitionInput{
		Task:                task,
		Actor:               actor,
		ToState:             domain.StateReadyForImplementation,
		HasValidLease:       exists && lease.ActorID == actor.ID && lease.ActorType == actor.Type && lease.ExpiresAt.After(time.Now().UTC()),
		PresentArtifacts:    artifacts,
		ParentChildrenReady: childrenReady,
	})
}

func newTaskID(now time.Time) string {
	return fmt.Sprintf("T-%d", now.UnixNano())
}
