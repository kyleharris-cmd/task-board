package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

func newTaskCmd(repoRoot *string) *cobra.Command {
	cmd := &cobra.Command{Use: "task", Short: "Task operations"}
	cmd.AddCommand(newTaskCreateCmd(repoRoot))
	cmd.AddCommand(newTaskListCmd(repoRoot))
	cmd.AddCommand(newTaskClaimCmd(repoRoot))
	cmd.AddCommand(newTaskRenewCmd(repoRoot))
	cmd.AddCommand(newTaskReleaseCmd(repoRoot))
	cmd.AddCommand(newTaskTransitionCmd(repoRoot))
	cmd.AddCommand(newTaskArchiveCmd(repoRoot))
	cmd.AddCommand(newTaskDeleteCmd(repoRoot))
	cmd.AddCommand(newReadyCheckCmd(repoRoot))
	return cmd
}

func newTaskCreateCmd(repoRoot *string) *cobra.Command {
	var title, description, taskType, parentID string
	var priority int
	var requiredForParent bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(title) == "" {
				return fmt.Errorf("title is required")
			}
			var p *string
			if strings.TrimSpace(parentID) != "" {
				v := strings.TrimSpace(parentID)
				p = &v
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				id, err := svc.CreateTask(context.Background(), app.CreateTaskInput{
					Title:             title,
					Description:       description,
					TaskType:          taskType,
					Priority:          priority,
					ParentID:          p,
					RequiredForParent: requiredForParent,
				})
				if err != nil {
					return err
				}
				task, err := svc.GetTask(context.Background(), id)
				if err != nil {
					return err
				}
				ref := task.ShortRef
				if ref == "" {
					ref = task.ID
				}
				cmd.Printf("created task %s (id=%s)\n", ref, task.ID)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "task title")
	cmd.Flags().StringVar(&description, "description", "", "task description")
	cmd.Flags().StringVar(&taskType, "type", "default", "task type")
	cmd.Flags().IntVar(&priority, "priority", 3, "priority (1 high - 5 low)")
	cmd.Flags().StringVar(&parentID, "parent-id", "", "optional parent task ID")
	cmd.Flags().BoolVar(&requiredForParent, "required-for-parent", true, "when child task, whether it gates parent readiness")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func newTaskListCmd(repoRoot *string) *cobra.Command {
	var stateRaw string
	var includeArchived bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			var state *domain.State
			if strings.TrimSpace(stateRaw) != "" {
				s, err := domain.ParseState(stateRaw)
				if err != nil {
					return err
				}
				state = &s
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				tasks, err := svc.ListTasksWithArchived(context.Background(), state, includeArchived)
				if err != nil {
					return err
				}
				for _, t := range tasks {
					parent := "-"
					if t.ParentID != nil {
						parent = *t.ParentID
					}
					ref := t.ShortRef
					if ref == "" {
						ref = t.ID
					}
					archived := "active"
					if t.ArchivedAt != nil {
						archived = "archived"
					}
					cmd.Printf("%s | id=%s | %s | %s | type=%s | parent=%s | rubric=%t | %s\n", ref, t.ID, t.State, t.Title, t.TaskType, parent, t.RubricPassed, archived)
				}
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&stateRaw, "state", "", "optional state filter")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "include archived tasks")
	return cmd
}

func newTaskClaimCmd(repoRoot *string) *cobra.Command {
	var id string
	var ttl int
	var autoRenew bool
	var af actorFlags

	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Claim or re-claim a task lease",
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := af.actor()
			if err != nil {
				return err
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				expiresAt, err := svc.ClaimTask(context.Background(), app.ClaimTaskInput{TaskID: id, Actor: actor, TTLMinutes: ttl, AutoRenew: autoRenew})
				if err != nil {
					return err
				}
				cmd.Printf("claimed %s until %s\n", id, expiresAt.Format("2006-01-02T15:04:05Z07:00"))
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "task ID")
	cmd.Flags().IntVar(&ttl, "ttl-minutes", 0, "override lease TTL minutes")
	cmd.Flags().BoolVar(&autoRenew, "auto-renew", false, "set lease auto-renew")
	af.add(cmd)
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newTaskRenewCmd(repoRoot *string) *cobra.Command {
	var id string
	var ttl int
	var af actorFlags

	cmd := &cobra.Command{
		Use:   "renew",
		Short: "Renew task lease",
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := af.actor()
			if err != nil {
				return err
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				expiresAt, err := svc.RenewTaskLease(context.Background(), id, actor, ttl)
				if err != nil {
					return err
				}
				cmd.Printf("renewed %s until %s\n", id, expiresAt.Format("2006-01-02T15:04:05Z07:00"))
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "task ID")
	cmd.Flags().IntVar(&ttl, "ttl-minutes", 0, "override lease TTL minutes")
	af.add(cmd)
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newTaskReleaseCmd(repoRoot *string) *cobra.Command {
	var id string
	var af actorFlags

	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release task lease",
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := af.actor()
			if err != nil {
				return err
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				if err := svc.ReleaseTaskLease(context.Background(), id, actor); err != nil {
					return err
				}
				cmd.Printf("released lease for %s\n", id)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "task ID")
	af.add(cmd)
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newTaskTransitionCmd(repoRoot *string) *cobra.Command {
	var id, toStateRaw, reason string
	var af actorFlags

	cmd := &cobra.Command{
		Use:   "transition",
		Short: "Transition task state using policy gates",
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := af.actor()
			if err != nil {
				return err
			}
			toState, err := domain.ParseState(toStateRaw)
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				if err := svc.TransitionTask(context.Background(), app.TransitionInput{TaskID: id, ToState: toState, Actor: actor, Reason: reason}); err != nil {
					return err
				}
				cmd.Printf("transitioned %s to %s\n", id, toState)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "task ID")
	cmd.Flags().StringVar(&toStateRaw, "to", "", "destination state")
	cmd.Flags().StringVar(&reason, "reason", "", "transition reason")
	af.add(cmd)
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func newReadyCheckCmd(repoRoot *string) *cobra.Command {
	var id string
	var af actorFlags

	cmd := &cobra.Command{
		Use:   "ready-check",
		Short: "Validate task gates for In Progress",
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := af.actor()
			if err != nil {
				return err
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				err := svc.ReadyCheck(context.Background(), id, actor)
				if err != nil {
					return err
				}
				cmd.Printf("task %s is ready for In Progress\n", id)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "task ID")
	af.add(cmd)
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newTaskArchiveCmd(repoRoot *string) *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive a task (hidden from default list/status views)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(*repoRoot, func(svc *app.Service) error {
				if err := svc.ArchiveTask(context.Background(), id); err != nil {
					return err
				}
				cmd.Printf("archived %s\n", id)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "task ID or short ref")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newTaskDeleteCmd(repoRoot *string) *cobra.Command {
	var id string
	var force bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Permanently delete a task and all task audit records",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("delete is destructive; re-run with --force")
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				if err := svc.DeleteTask(context.Background(), id, force); err != nil {
					return err
				}
				cmd.Printf("deleted %s\n", id)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "task ID or short ref")
	cmd.Flags().BoolVar(&force, "force", false, "required for destructive delete")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}
