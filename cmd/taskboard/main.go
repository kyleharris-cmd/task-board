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
	rootCmd := &cobra.Command{
		Use:   "taskboard",
		Short: "Local task board with policy-driven workflow gates",
	}

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newPolicyCmd())

	return rootCmd
}
