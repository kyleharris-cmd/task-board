package main

import (
	"context"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/spf13/cobra"
)

func newRubricCmd(repoRoot *string) *cobra.Command {
	var taskID, rubricVersion, notes string
	var requiredFieldsComplete bool
	var pass bool
	var af actorFlags

	cmd := &cobra.Command{
		Use:   "rubric",
		Short: "Rubric evaluation commands",
	}

	evaluateCmd := &cobra.Command{
		Use:   "evaluate",
		Short: "Store rubric evaluation result",
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := af.actor()
			if err != nil {
				return err
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				if err := svc.EvaluateRubric(context.Background(), taskID, rubricVersion, requiredFieldsComplete, pass, notes, actor); err != nil {
					return err
				}
				cmd.Printf("rubric evaluation saved for %s\n", taskID)
				return nil
			})
		},
	}

	evaluateCmd.Flags().StringVar(&taskID, "id", "", "task ID")
	evaluateCmd.Flags().StringVar(&rubricVersion, "rubric-version", "v1", "rubric version label")
	evaluateCmd.Flags().BoolVar(&requiredFieldsComplete, "required-fields-complete", true, "whether required rubric fields are complete")
	evaluateCmd.Flags().BoolVar(&pass, "pass", false, "rubric pass/fail")
	evaluateCmd.Flags().StringVar(&notes, "notes", "", "review notes")
	af.add(evaluateCmd)
	_ = evaluateCmd.MarkFlagRequired("id")

	cmd.AddCommand(evaluateCmd)
	return cmd
}
