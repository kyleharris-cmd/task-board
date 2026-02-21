package main

import (
	"fmt"
	"strings"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

type actorFlags struct {
	actorType string
	actorID   string
	actorName string
}

func (f *actorFlags) add(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.actorType, "actor-type", "agent", "actor type (human|agent)")
	cmd.Flags().StringVar(&f.actorID, "actor-id", "", "actor ID")
	cmd.Flags().StringVar(&f.actorName, "actor-name", "", "actor display name")
	_ = cmd.MarkFlagRequired("actor-id")
	_ = cmd.MarkFlagRequired("actor-name")
}

func (f actorFlags) actor() (domain.Actor, error) {
	at, err := domain.ParseActorType(f.actorType)
	if err != nil {
		return domain.Actor{}, err
	}
	if strings.TrimSpace(f.actorID) == "" || strings.TrimSpace(f.actorName) == "" {
		return domain.Actor{}, fmt.Errorf("actor-id and actor-name are required")
	}
	return domain.Actor{Type: at, ID: strings.TrimSpace(f.actorID), DisplayName: strings.TrimSpace(f.actorName)}, nil
}

func withService(repoRoot string, fn func(*app.Service) error) error {
	svc, err := app.OpenService(repoRoot)
	if err != nil {
		return err
	}
	defer svc.Close()
	return fn(svc)
}
