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

type nextTaskView struct {
	Ref        string `json:"ref"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	State      string `json:"state"`
	Priority   int    `json:"priority"`
	LeaseOwner string `json:"lease_owner,omitempty"`
	UpdatedAt  string `json:"updated_at"`
}

func newNextCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Recommend the next actionable task",
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}

			return withServiceOptions(*repoRoot, app.OpenServiceOptions{ReadOnly: true}, func(svc *app.Service) error {
				statuses, err := svc.ListTaskStatus(context.Background(), nil)
				if err != nil {
					return err
				}
				next, ok := selectNextTask(statuses, actor)
				if !ok {
					if asJSON {
						cmd.Println(`{"task":null}`)
					} else {
						cmd.Println("No actionable tasks found.")
					}
					return nil
				}
				ref := next.Task.ShortRef
				if ref == "" {
					ref = next.Task.ID
				}
				view := nextTaskView{
					Ref:       ref,
					ID:        next.Task.ID,
					Title:     next.Task.Title,
					State:     string(next.Task.State),
					Priority:  next.Task.Priority,
					UpdatedAt: next.Task.UpdatedAt.Format(time.RFC3339),
				}
				if next.Lease != nil {
					view.LeaseOwner = fmt.Sprintf("%s:%s", next.Lease.ActorType, next.Lease.ActorID)
				}
				if asJSON {
					raw, err := json.MarshalIndent(map[string]any{"task": view}, "", "  ")
					if err != nil {
						return err
					}
					cmd.Println(string(raw))
					return nil
				}
				cmd.Printf("Next: %s [%s] %s\n", view.Ref, view.State, view.Title)
				cmd.Printf("  id=%s priority=%d updated=%s\n", view.ID, view.Priority, view.UpdatedAt)
				if view.LeaseOwner != "" {
					cmd.Printf("  lease=%s\n", view.LeaseOwner)
				}
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeAgent)
	cmd.Flags().BoolVar(&asJSON, "json", true, "emit next-task result as JSON")
	return cmd
}

func selectNextTask(statuses []app.TaskStatus, actor domain.Actor) (app.TaskStatus, bool) {
	filtered := make([]app.TaskStatus, 0, len(statuses))
	for _, s := range statuses {
		if s.Task.State == domain.StateComplete {
			continue
		}
		if s.LeaseActive && s.Lease != nil && (s.Lease.ActorID != actor.ID || s.Lease.ActorType != actor.Type) {
			continue
		}
		filtered = append(filtered, s)
	}
	if len(filtered) == 0 {
		return app.TaskStatus{}, false
	}

	rank := map[domain.State]int{
		domain.StateInProgress: 0,
		domain.StateDesign:     1,
		domain.StateScoping:    2,
		domain.StatePR:         3,
	}

	sort.Slice(filtered, func(i, j int) bool {
		ri := rank[filtered[i].Task.State]
		rj := rank[filtered[j].Task.State]
		if ri != rj {
			return ri < rj
		}
		if filtered[i].Task.Priority != filtered[j].Task.Priority {
			return filtered[i].Task.Priority < filtered[j].Task.Priority
		}
		return filtered[i].Task.UpdatedAt.Before(filtered[j].Task.UpdatedAt)
	})
	return filtered[0], true
}
