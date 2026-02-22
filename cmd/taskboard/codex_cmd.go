package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

type codexTaskView struct {
	Ref        string `json:"ref"`
	ID         string `json:"id"`
	ParentRef  string `json:"parent_ref,omitempty"`
	ParentID   string `json:"parent_id,omitempty"`
	Title      string `json:"title"`
	State      string `json:"state"`
	UpdatedAt  string `json:"updated_at"`
	LeaseOwner string `json:"lease_owner,omitempty"`
	LeaseState string `json:"lease_state,omitempty"`
}

type codexSnapshot struct {
	RepoRoot        string          `json:"repo_root"`
	GeneratedAt     string          `json:"generated_at"`
	ActiveCheckouts []codexTaskView `json:"active_checkouts"`
	ReadyForImpl    []codexTaskView `json:"ready_for_implementation"`
}

func newCodexCmd(repoRoot *string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Print a Codex-friendly taskboard snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withServiceOptions(*repoRoot, app.OpenServiceOptions{ReadOnly: true}, func(svc *app.Service) error {
				ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
				defer cancel()

				statuses, err := svc.ListTaskStatus(ctx, nil)
				if err != nil {
					return err
				}

				snap := buildCodexSnapshot(*repoRoot, statuses)
				if asJSON {
					raw, err := json.MarshalIndent(snap, "", "  ")
					if err != nil {
						return fmt.Errorf("marshal codex snapshot: %w", err)
					}
					cmd.Println(string(raw))
					return nil
				}

				renderCodexSnapshot(cmd, snap)
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "emit snapshot as JSON")
	return cmd
}

func buildCodexSnapshot(repoRoot string, statuses []app.TaskStatus) codexSnapshot {
	refByID := make(map[string]string, len(statuses))
	for _, s := range statuses {
		ref := s.Task.ShortRef
		if ref == "" {
			ref = s.Task.ID
		}
		refByID[s.Task.ID] = ref
	}

	active := make([]app.TaskStatus, 0)
	ready := make([]app.TaskStatus, 0)
	for _, s := range statuses {
		if s.LeaseActive {
			active = append(active, s)
		}
		if s.Task.State == domain.StateReadyForImplementation {
			ready = append(ready, s)
		}
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].Task.UpdatedAt.After(active[j].Task.UpdatedAt)
	})
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].Task.UpdatedAt.After(ready[j].Task.UpdatedAt)
	})

	return codexSnapshot{
		RepoRoot:        repoRoot,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		ActiveCheckouts: mapCodexTasks(active, refByID),
		ReadyForImpl:    mapCodexTasks(ready, refByID),
	}
}

func mapCodexTasks(statuses []app.TaskStatus, refByID map[string]string) []codexTaskView {
	out := make([]codexTaskView, 0, len(statuses))
	for _, s := range statuses {
		ref := s.Task.ShortRef
		if ref == "" {
			ref = s.Task.ID
		}
		v := codexTaskView{
			Ref:       ref,
			ID:        s.Task.ID,
			Title:     s.Task.Title,
			State:     string(s.Task.State),
			UpdatedAt: s.Task.UpdatedAt.Format(time.RFC3339),
		}
		if s.Task.ParentID != nil {
			v.ParentID = *s.Task.ParentID
			v.ParentRef = refByID[*s.Task.ParentID]
		}
		if s.Lease != nil {
			v.LeaseOwner = fmt.Sprintf("%s:%s", s.Lease.ActorType, s.Lease.ActorID)
			if s.LeaseActive {
				v.LeaseState = "active"
			} else {
				v.LeaseState = "expired"
			}
		}
		out = append(out, v)
	}
	return out
}

func renderCodexSnapshot(cmd *cobra.Command, snap codexSnapshot) {
	cmd.Printf("Codex Taskboard Snapshot\n")
	cmd.Printf("Repo: %s\n", snap.RepoRoot)
	cmd.Printf("Generated: %s\n\n", snap.GeneratedAt)

	cmd.Printf("Active Checkouts (%d)\n", len(snap.ActiveCheckouts))
	if len(snap.ActiveCheckouts) == 0 {
		cmd.Println("- none")
	} else {
		for _, t := range snap.ActiveCheckouts {
			line := fmt.Sprintf("- %s [%s] %s", t.Ref, t.State, t.Title)
			if t.LeaseOwner != "" {
				line += fmt.Sprintf(" (%s)", t.LeaseOwner)
			}
			cmd.Println(line)
		}
	}
	cmd.Println()

	cmd.Printf("Ready for Implementation (%d)\n", len(snap.ReadyForImpl))
	if len(snap.ReadyForImpl) == 0 {
		cmd.Println("- none")
	} else {
		for _, t := range snap.ReadyForImpl {
			parent := ""
			if t.ParentRef != "" {
				parent = fmt.Sprintf(" parent=%s", t.ParentRef)
			}
			cmd.Printf("- %s [%s] %s%s\n", t.Ref, t.State, t.Title, parent)
		}
	}
	cmd.Println()

	cmd.Println("Next Commands")
	cmd.Println("- tb stat")
	cmd.Println("- tb pickup <task-ref>")
	cmd.Println("- tb start <task-ref>")
}
