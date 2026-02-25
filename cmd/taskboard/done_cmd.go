package main

import (
	"context"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

func newDoneCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var reason string

	cmd := &cobra.Command{
		Use:   "done <task-id>",
		Short: "Move task to Complete",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				if err := svc.TransitionTask(context.Background(), app.TransitionInput{
					TaskID:  taskID,
					ToState: domain.StateComplete,
					Actor:   actor,
					Reason:  reason,
				}); err != nil {
					return err
				}
				cmd.Printf("moved %s to %s\n", taskID, domain.StateComplete)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().StringVar(&reason, "reason", "done workflow", "transition reason")
	return cmd
}
