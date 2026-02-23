package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

func newStartCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var ttlMinutes int
	var autoRenew bool
	var content string

	cmd := &cobra.Command{
		Use:   "start <task-id>",
		Short: "Claim task, add context, and move task to Scoping",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				if _, err := svc.ClaimTask(context.Background(), app.ClaimTaskInput{TaskID: taskID, Actor: actor, TTLMinutes: ttlMinutes, AutoRenew: autoRenew}); err != nil {
					return err
				}
				ctxContent := strings.TrimSpace(content)
				if ctxContent == "" {
					ctxContent, err = editContentWithEditor(defaultContextTemplate(taskID))
					if err != nil {
						return err
					}
				}
				if ctxContent == "" {
					return errors.New("context content cannot be empty")
				}
				if _, _, err := svc.AddArtifact(context.Background(), taskID, domain.ArtifactContext, ctxContent, actor); err != nil {
					return err
				}
				if err := svc.TransitionTask(context.Background(), app.TransitionInput{TaskID: taskID, ToState: domain.StateScoping, Actor: actor, Reason: "start workflow"}); err != nil {
					return err
				}
				cmd.Printf("started task %s\n", taskID)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().IntVar(&ttlMinutes, "ttl-minutes", 0, "lease TTL override in minutes")
	cmd.Flags().BoolVar(&autoRenew, "auto-renew", true, "set lease auto-renew")
	cmd.Flags().StringVar(&content, "content", "", "context artifact markdown content (skip editor)")

	return cmd
}

func newDesignCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var content string

	cmd := &cobra.Command{
		Use:   "design <task-id>",
		Short: "Add design artifact and move task to Design",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				designContent := strings.TrimSpace(content)
				if designContent == "" {
					designContent, err = editContentWithEditor(defaultDesignTemplate(taskID))
					if err != nil {
						return err
					}
				}
				if designContent == "" {
					return errors.New("design content cannot be empty")
				}
				if _, _, err := svc.AddArtifact(context.Background(), taskID, domain.ArtifactDesign, designContent, actor); err != nil {
					return err
				}
				if err := svc.TransitionTask(context.Background(), app.TransitionInput{TaskID: taskID, ToState: domain.StateDesign, Actor: actor, Reason: "design workflow"}); err != nil {
					return err
				}
				cmd.Printf("designed task %s\n", taskID)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().StringVar(&content, "content", "", "design artifact markdown content (skip editor)")

	return cmd
}

func newReviewCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var content string
	var pass bool
	var fail bool
	var rubricVersion string
	var notes string

	cmd := &cobra.Command{
		Use:   "review <task-id>",
		Short: "Add rubric artifact, evaluate, and validate readiness for In Progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}
			passValue, err := resolvePassFail(pass, fail)
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				rubricContent := strings.TrimSpace(content)
				if rubricContent == "" {
					rubricContent, err = editContentWithEditor(defaultRubricTemplate(taskID, passValue))
					if err != nil {
						return err
					}
				}
				if rubricContent == "" {
					return errors.New("rubric review content cannot be empty")
				}
				if _, _, err := svc.AddArtifact(context.Background(), taskID, domain.ArtifactRubricReview, rubricContent, actor); err != nil {
					return err
				}
				if err := svc.EvaluateRubric(context.Background(), taskID, rubricVersion, true, passValue, notes, actor); err != nil {
					return err
				}
				if err := svc.ReadyCheck(context.Background(), taskID, actor); err != nil {
					return err
				}
				cmd.Printf("reviewed task %s (in-progress readiness check passed)\n", taskID)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().StringVar(&content, "content", "", "rubric review markdown content (skip editor)")
	cmd.Flags().BoolVar(&pass, "pass", false, "mark rubric as pass")
	cmd.Flags().BoolVar(&fail, "fail", false, "mark rubric as fail")
	cmd.Flags().StringVar(&rubricVersion, "rubric-version", "v1", "rubric version label")
	cmd.Flags().StringVar(&notes, "notes", "", "rubric notes")

	return cmd
}

func newImplementCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var ttlMinutes int
	var autoRenew bool

	cmd := &cobra.Command{
		Use:   "implement <task-id>",
		Short: "Claim/renew lease and move task to In Progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				if _, err := svc.ClaimTask(context.Background(), app.ClaimTaskInput{TaskID: taskID, Actor: actor, TTLMinutes: ttlMinutes, AutoRenew: autoRenew}); err != nil {
					return err
				}
				if err := svc.TransitionTask(context.Background(), app.TransitionInput{TaskID: taskID, ToState: domain.StateInProgress, Actor: actor, Reason: "implementation workflow"}); err != nil {
					return err
				}
				cmd.Printf("task %s moved to In Progress\n", taskID)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().IntVar(&ttlMinutes, "ttl-minutes", 0, "lease TTL override in minutes")
	cmd.Flags().BoolVar(&autoRenew, "auto-renew", true, "set lease auto-renew")

	return cmd
}

func newFinishCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var notesContent string
	var testContent string
	var docsContent string

	cmd := &cobra.Command{
		Use:   "finish <task-id>",
		Short: "Add implementation/test/docs artifacts and move task through PR to Complete",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				notes, err := resolveArtifactContent(notesContent, defaultImplementationNotesTemplate(taskID))
				if err != nil {
					return err
				}
				tests, err := resolveArtifactContent(testContent, defaultTestReportTemplate(taskID))
				if err != nil {
					return err
				}
				docs, err := resolveArtifactContent(docsContent, defaultDocsTemplate(taskID))
				if err != nil {
					return err
				}

				if _, _, err := svc.AddArtifact(context.Background(), taskID, domain.ArtifactImplementationNotes, notes, actor); err != nil {
					return err
				}
				if _, _, err := svc.AddArtifact(context.Background(), taskID, domain.ArtifactTestReport, tests, actor); err != nil {
					return err
				}
				if _, _, err := svc.AddArtifact(context.Background(), taskID, domain.ArtifactDocsUpdate, docs, actor); err != nil {
					return err
				}

				if err := svc.TransitionTask(context.Background(), app.TransitionInput{TaskID: taskID, ToState: domain.StatePR, Actor: actor, Reason: "finish workflow"}); err != nil {
					return err
				}
				if err := svc.TransitionTask(context.Background(), app.TransitionInput{TaskID: taskID, ToState: domain.StateComplete, Actor: actor, Reason: "finish workflow"}); err != nil {
					return err
				}

				cmd.Printf("finished task %s\n", taskID)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().StringVar(&notesContent, "notes", "", "implementation notes markdown content (skip editor)")
	cmd.Flags().StringVar(&testContent, "tests", "", "test report markdown content (skip editor)")
	cmd.Flags().StringVar(&docsContent, "docs", "", "docs update markdown content (skip editor)")

	return cmd
}

func resolvePassFail(pass, fail bool) (bool, error) {
	if pass && fail {
		return false, errors.New("use only one of --pass or --fail")
	}
	if pass {
		return true, nil
	}
	if fail {
		return false, nil
	}
	if !isInteractiveInput() {
		return false, errors.New("must set --pass or --fail in non-interactive mode")
	}
	fmt.Print("Rubric result (p=pass, f=fail): ")
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "p" || input == "pass", nil
}

func resolveArtifactContent(raw, template string) (string, error) {
	content := strings.TrimSpace(raw)
	if content != "" {
		return content, nil
	}
	content, err := editContentWithEditor(template)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(content) == "" {
		return "", errors.New("artifact content cannot be empty")
	}
	return content, nil
}

func defaultContextTemplate(taskID string) string {
	return fmt.Sprintf("# Context for %s\n\n## Problem\n- \n\n## Constraints\n- \n\n## Relevant Files\n- \n", taskID)
}

func defaultDesignTemplate(taskID string) string {
	return fmt.Sprintf("# Design for %s\n\n## Approach\n- \n\n## Risks\n- \n\n## Open Questions\n- \n", taskID)
}

func defaultRubricTemplate(taskID string, pass bool) string {
	result := "FAIL"
	if pass {
		result = "PASS"
	}
	return fmt.Sprintf("# Rubric Review for %s\n\nResult: %s\n\n## Notes\n- \n", taskID, result)
}

func defaultImplementationNotesTemplate(taskID string) string {
	return fmt.Sprintf("# Implementation Notes for %s\n\n## Changes\n- \n\n## Decisions\n- \n", taskID)
}

func defaultTestReportTemplate(taskID string) string {
	return fmt.Sprintf("# Test Report for %s\n\n## Tests Run\n- \n\n## Result\n- \n", taskID)
}

func defaultDocsTemplate(taskID string) string {
	return fmt.Sprintf("# Docs Update for %s\n\n## Updated Docs\n- \n\n## Summary\n- \n", taskID)
}
