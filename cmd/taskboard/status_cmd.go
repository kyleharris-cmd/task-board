package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleharris/task-board/internal/app"
	"github.com/spf13/cobra"
)

func newStatusCmd(repoRoot *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"stat"},
		Short:   "Open read-only status board (git-log style)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(*repoRoot, func(svc *app.Service) error {
				model := newStatusModel(svc)
				program := tea.NewProgram(model, tea.WithAltScreen())
				if _, err := program.Run(); err != nil {
					return fmt.Errorf("run status board: %w", err)
				}
				return nil
			})
		},
	}

	return cmd
}
