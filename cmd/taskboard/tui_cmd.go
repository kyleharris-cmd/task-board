package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

func newTUICmd(repoRoot *string) *cobra.Command {
	var af actorFlags
	var ttl int
	var autoRenew bool

	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive task board TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			actor, err := af.actor()
			if err != nil {
				return err
			}

			return withService(*repoRoot, func(svc *app.Service) error {
				model := newTUIModel(svc, actor, ttl, autoRenew)
				p := tea.NewProgram(model, tea.WithAltScreen())
				if _, err := p.Run(); err != nil {
					return fmt.Errorf("run tui: %w", err)
				}
				return nil
			})
		},
	}

	af.actorType = string(domain.ActorTypeAgent)
	af.add(cmd)
	cmd.Flags().IntVar(&ttl, "ttl-minutes", 0, "default claim/renew TTL override")
	cmd.Flags().BoolVar(&autoRenew, "auto-renew", true, "use auto-renew on claim")

	return cmd
}
