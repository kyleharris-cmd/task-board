package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kyleharris/task-board/internal/storage"
	"github.com/spf13/cobra"
)

func newInitCmd(repoRoot *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize .taskboard storage and default policy in a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			absRoot, err := filepath.Abs(*repoRoot)
			if err != nil {
				return fmt.Errorf("resolve repo root: %w", err)
			}

			taskboardDir := filepath.Join(absRoot, ".taskboard")
			tasksDir := filepath.Join(taskboardDir, "tasks")
			dbPath := filepath.Join(taskboardDir, "board.db")
			policyPath := filepath.Join(taskboardDir, "policy.yaml")

			if err := os.MkdirAll(tasksDir, 0o755); err != nil {
				return fmt.Errorf("create taskboard dir: %w", err)
			}

			db, err := storage.Open(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			if err := db.Migrate(ctx); err != nil {
				return err
			}
			if err := db.UpsertBoard(ctx, "default", absRoot, time.Now()); err != nil {
				return err
			}

			if err := writeDefaultPolicyIfMissing(policyPath); err != nil {
				return err
			}

			cmd.Printf("initialized task board in %s\n", taskboardDir)
			return nil
		},
	}

	return cmd
}
