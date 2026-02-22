package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var repoRoot string

	rootCmd := &cobra.Command{
		Use:     "tb",
		Aliases: []string{"taskboard"},
		Short:   "Local task board with policy-driven workflow gates",
	}

	rootCmd.PersistentFlags().StringVar(&repoRoot, "repo-root", ".", "repository root containing .taskboard")
	rootCmd.AddCommand(newInitCmd(&repoRoot))
	rootCmd.AddCommand(newPolicyCmd())
	rootCmd.AddCommand(newTaskCmd(&repoRoot))
	rootCmd.AddCommand(newArtifactCmd(&repoRoot))
	rootCmd.AddCommand(newRubricCmd(&repoRoot))
	rootCmd.AddCommand(newTUICmd(&repoRoot))
	rootCmd.AddCommand(newServeCmd(&repoRoot))
	rootCmd.AddCommand(newStartCmd(&repoRoot))
	rootCmd.AddCommand(newDesignCmd(&repoRoot))
	rootCmd.AddCommand(newReviewCmd(&repoRoot))
	rootCmd.AddCommand(newImplementCmd(&repoRoot))
	rootCmd.AddCommand(newFinishCmd(&repoRoot))
	rootCmd.AddCommand(newParentCmd(&repoRoot))
	rootCmd.AddCommand(newChildCmd(&repoRoot))
	rootCmd.AddCommand(newPickupCmd(&repoRoot))
	rootCmd.AddCommand(newStatusCmd(&repoRoot))
	rootCmd.AddCommand(newCodexCmd(&repoRoot))

	return rootCmd
}
