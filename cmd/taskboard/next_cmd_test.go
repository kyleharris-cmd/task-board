package main

import (
	"testing"
	"time"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
)

func TestSelectNextTask_PrefersInProgressThenPriority(t *testing.T) {
	now := time.Now().UTC()
	actor := domain.Actor{Type: domain.ActorTypeAgent, ID: "a1", DisplayName: "Agent 1"}
	statuses := []app.TaskStatus{
		{Task: domain.Task{ID: "1", ShortRef: "T-1", State: domain.StateDesign, Priority: 1, UpdatedAt: now.Add(-2 * time.Hour)}},
		{Task: domain.Task{ID: "2", ShortRef: "T-2", State: domain.StateInProgress, Priority: 3, UpdatedAt: now.Add(-1 * time.Hour)}},
		{Task: domain.Task{ID: "3", ShortRef: "T-3", State: domain.StateInProgress, Priority: 1, UpdatedAt: now.Add(-30 * time.Minute)}},
	}

	next, ok := selectNextTask(statuses, actor)
	if !ok {
		t.Fatalf("expected task recommendation")
	}
	if next.Task.ID != "3" {
		t.Fatalf("got %s, want 3", next.Task.ID)
	}
}

func TestSelectNextTask_SkipsLeasedByOtherActor(t *testing.T) {
	now := time.Now().UTC()
	actor := domain.Actor{Type: domain.ActorTypeAgent, ID: "a1", DisplayName: "Agent 1"}
	statuses := []app.TaskStatus{
		{
			Task:       domain.Task{ID: "1", State: domain.StateInProgress, Priority: 1, UpdatedAt: now.Add(-1 * time.Hour)},
			Lease:      &domain.Lease{TaskID: "1", ActorType: domain.ActorTypeAgent, ActorID: "other"},
			LeaseActive: true,
		},
		{Task: domain.Task{ID: "2", State: domain.StateDesign, Priority: 2, UpdatedAt: now.Add(-2 * time.Hour)}},
	}

	next, ok := selectNextTask(statuses, actor)
	if !ok {
		t.Fatalf("expected task recommendation")
	}
	if next.Task.ID != "2" {
		t.Fatalf("got %s, want 2", next.Task.ID)
	}
}
