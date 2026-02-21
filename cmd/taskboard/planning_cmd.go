package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

func newParentCmd(repoRoot *string) *cobra.Command {
	cmd := &cobra.Command{Use: "parent", Short: "Parent task workflow commands"}
	cmd.AddCommand(newParentCreateCmd(repoRoot))
	cmd.AddCommand(newParentDesignEditCmd(repoRoot))
	return cmd
}

func newParentCreateCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var title, description, designContent string
	var priority int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a parent task and canonical parent design artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(title) == "" {
				return fmt.Errorf("title is required")
			}
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				parentID, err := svc.CreateTask(context.Background(), app.CreateTaskInput{Title: title, Description: description, TaskType: "design", Priority: priority})
				if err != nil {
					return err
				}
				content, err := resolveArtifactContent(designContent, defaultParentDesignTemplate(parentID, title))
				if err != nil {
					return err
				}
				if _, _, err := svc.AddArtifact(context.Background(), parentID, domain.ArtifactParentDesign, content, actor); err != nil {
					return err
				}
				cmd.Printf("created parent task %s\n", parentID)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().StringVar(&title, "title", "", "parent task title")
	cmd.Flags().StringVar(&description, "description", "", "parent task description")
	cmd.Flags().IntVar(&priority, "priority", 2, "task priority")
	cmd.Flags().StringVar(&designContent, "design", "", "parent design markdown content (skip editor)")
	_ = cmd.MarkFlagRequired("title")

	return cmd
}

func newParentDesignEditCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var content string

	cmd := &cobra.Command{
		Use:   "design-edit <parent-task-id>",
		Short: "Add a new parent design artifact version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parentID := args[0]
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				task, err := svc.GetTask(context.Background(), parentID)
				if err != nil {
					return err
				}
				artifact, hasExisting, err := svc.GetLatestArtifact(context.Background(), parentID, domain.ArtifactParentDesign)
				if err != nil {
					return err
				}
				initial := defaultParentDesignTemplate(parentID, task.Title)
				if hasExisting {
					initial = artifact.ContentSnapshot
				}
				designContent := strings.TrimSpace(content)
				if designContent == "" {
					designContent, err = editContentWithEditor(initial)
					if err != nil {
						return err
					}
				}
				if designContent == "" {
					return fmt.Errorf("parent design content cannot be empty")
				}
				_, _, err = svc.AddArtifact(context.Background(), parentID, domain.ArtifactParentDesign, designContent, actor)
				if err != nil {
					return err
				}
				cmd.Printf("updated parent design for %s\n", parentID)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().StringVar(&content, "content", "", "parent design markdown content (skip editor)")

	return cmd
}

func newChildCmd(repoRoot *string) *cobra.Command {
	cmd := &cobra.Command{Use: "child", Short: "Child task planning commands"}
	cmd.AddCommand(newChildCreateCmd(repoRoot))
	return cmd
}

func newChildCreateCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var parentID, title, description, brief string
	var files []string
	var priority int
	var requiredForParent bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create child task linked to parent with child design and file context",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(parentID) == "" || strings.TrimSpace(title) == "" {
				return fmt.Errorf("--parent-id and --title are required")
			}
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}
			return withService(*repoRoot, func(svc *app.Service) error {
				childID, err := svc.CreateTask(context.Background(), app.CreateTaskInput{
					Title:             title,
					Description:       description,
					TaskType:          "implementation",
					Priority:          priority,
					ParentID:          &parentID,
					RequiredForParent: requiredForParent,
				})
				if err != nil {
					return err
				}
				childDesign := strings.TrimSpace(brief)
				if childDesign == "" {
					childDesign, err = editContentWithEditor(defaultChildDesignTemplate(childID, parentID, title))
					if err != nil {
						return err
					}
				}
				if childDesign == "" {
					return fmt.Errorf("child design content cannot be empty")
				}
				if _, _, err := svc.AddArtifact(context.Background(), childID, domain.ArtifactChildDesign, childDesign, actor); err != nil {
					return err
				}

				filesContext := defaultFileContextTemplate(parentID, files)
				if _, _, err := svc.AddArtifact(context.Background(), childID, domain.ArtifactContext, filesContext, actor); err != nil {
					return err
				}

				cmd.Printf("created child task %s under parent %s\n", childID, parentID)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().StringVar(&parentID, "parent-id", "", "parent task ID")
	cmd.Flags().StringVar(&title, "title", "", "child task title")
	cmd.Flags().StringVar(&description, "description", "", "child task description")
	cmd.Flags().StringVar(&brief, "brief", "", "child design markdown content (skip editor)")
	cmd.Flags().StringSliceVar(&files, "files", nil, "comma-separated file paths relevant to this child task")
	cmd.Flags().IntVar(&priority, "priority", 3, "task priority")
	cmd.Flags().BoolVar(&requiredForParent, "required-for-parent", true, "whether this child gates parent readiness")

	return cmd
}

func newPickupCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var claim bool
	var ttlMinutes int

	cmd := &cobra.Command{
		Use:   "pickup <child-task-id>",
		Short: "Claim task and print full implementation brief for a fresh agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				if claim {
					if _, err := svc.ClaimTask(context.Background(), app.ClaimTaskInput{TaskID: taskID, Actor: actor, TTLMinutes: ttlMinutes, AutoRenew: true}); err != nil {
						return err
					}
				}
				task, err := svc.GetTask(context.Background(), taskID)
				if err != nil {
					return err
				}
				parentDesign := "(none found)"
				if task.ParentID != nil {
					snap, ok, err := svc.GetLatestArtifact(context.Background(), *task.ParentID, domain.ArtifactParentDesign)
					if err != nil {
						return err
					}
					if ok {
						parentDesign = snap.ContentSnapshot
					}
				}
				childDesign := "(none found)"
				if snap, ok, err := svc.GetLatestArtifact(context.Background(), taskID, domain.ArtifactChildDesign); err == nil && ok {
					childDesign = snap.ContentSnapshot
				} else if err != nil {
					return err
				}
				fileContext := "(none found)"
				if snap, ok, err := svc.GetLatestArtifact(context.Background(), taskID, domain.ArtifactContext); err == nil && ok {
					fileContext = snap.ContentSnapshot
				} else if err != nil {
					return err
				}

				brief := buildPickupBrief(*repoRoot, taskID, task.ParentID, parentDesign, childDesign, fileContext)
				cmd.Println(brief)
				return nil
			})
		},
	}

	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().BoolVar(&claim, "claim", true, "claim task before generating pickup brief")
	cmd.Flags().IntVar(&ttlMinutes, "ttl-minutes", 0, "lease TTL override in minutes")

	return cmd
}

func defaultParentDesignTemplate(taskID, title string) string {
	return fmt.Sprintf("# Parent Design: %s\n\nTask: %s\n\n## Goal\n- \n\n## Scope\n- \n\n## Major Components\n- \n\n## Risks\n- \n", title, taskID)
}

func defaultChildDesignTemplate(childID, parentID, title string) string {
	return fmt.Sprintf("# Child Design: %s\n\nChild Task: %s\nParent Task: %s\n\n## Objective\n- \n\n## Implementation Plan\n- \n\n## Validation\n- \n", title, childID, parentID)
}

func defaultFileContextTemplate(parentID string, files []string) string {
	lines := []string{
		"# Context",
		"",
		fmt.Sprintf("Parent Task: %s", parentID),
		"",
		"Files to read first:",
	}
	if len(files) == 0 {
		lines = append(lines, "- (add files)")
	} else {
		for _, file := range files {
			f := strings.TrimSpace(file)
			if f != "" {
				lines = append(lines, "- "+f)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func buildPickupBrief(repoRoot, taskID string, parentID *string, parentDesign, childDesign, fileContext string) string {
	sections := []string{
		fmt.Sprintf("# Pickup Brief for %s", taskID),
		"",
		"1. Orient yourself with package conventions:",
	}
	for _, name := range []string{"README.md", "AGENTS.md", "CLAUDE.md"} {
		path := filepath.Join(repoRoot, name)
		if _, err := os.Stat(path); err == nil {
			sections = append(sections, "- Read "+name)
		}
	}
	if parentID != nil {
		sections = append(sections, "", fmt.Sprintf("2. Parent Task: %s", *parentID), "", "## Parent Design", parentDesign)
	} else {
		sections = append(sections, "", "2. Parent Task: (none)")
	}
	sections = append(sections,
		"",
		"3. Child Task Design",
		childDesign,
		"",
		"4. File Context",
		fileContext,
		"",
		"5. Execute",
		"- Ask clarifying questions if needed",
		"- Implement incrementally",
		"- Test and document changes",
		"- Update taskboard artifacts and states",
	)
	return strings.Join(sections, "\n")
}
