package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

func newStatusCmd(repoRoot *string) *cobra.Command {
	var af optionalActorFlags
	var editable bool

	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"stat"},
		Short:   "Open read-only status board (git-log style)",
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := af.resolve(*repoRoot)
			if err != nil {
				return err
			}
			return withServiceOptions(*repoRoot, app.OpenServiceOptions{ReadOnly: !editable}, func(svc *app.Service) error {
				model := newStatusModel(svc, actor, editable)
				program := tea.NewProgram(model, tea.WithAltScreen())
				if _, err := program.Run(); err != nil {
					return fmt.Errorf("run status board: %w", err)
				}
				return nil
			})
		},
	}
	af.add(cmd, domain.ActorTypeHuman)
	cmd.Flags().BoolVar(&editable, "editable", false, "allow write commands (:e/:cp/:cc) in status view")

	return cmd
}
