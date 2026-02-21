package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

func newArtifactCmd(repoRoot *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Artifact operations",
	}
	cmd.AddCommand(newArtifactAddCmd(repoRoot))
	return cmd
}

func newArtifactAddCmd(repoRoot *string) *cobra.Command {
	var taskID, artifactTypeRaw, content string
	var af actorFlags

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a task artifact and write markdown snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("content is required")
			}
			actor, err := af.actor()
			if err != nil {
				return err
			}
			typeParsed, err := domain.ParseArtifactType(artifactTypeRaw)
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				path, version, err := svc.AddArtifact(context.Background(), taskID, typeParsed, content, actor)
				if err != nil {
					return err
				}
				cmd.Printf("artifact written: %s (v%d)\n", path, version)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&taskID, "id", "", "task ID")
	cmd.Flags().StringVar(&artifactTypeRaw, "type", "", "artifact type")
	cmd.Flags().StringVar(&content, "content", "", "artifact markdown content")
	af.add(cmd)
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("content")

	return cmd
}
