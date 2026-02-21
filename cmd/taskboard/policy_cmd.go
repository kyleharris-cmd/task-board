package main

import (
	"fmt"

	"github.com/kyleharris/task-board/internal/policy"
	"github.com/spf13/cobra"
)

func newPolicyCmd() *cobra.Command {
	policyCmd := &cobra.Command{
		Use:   "policy",
		Short: "Policy management commands",
	}

	policyCmd.AddCommand(newPolicyValidateCmd())
	return policyCmd
}

func newPolicyValidateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a board policy file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := policy.Load(file)
			if err != nil {
				return err
			}
			cmd.Printf("policy %s is valid\n", file)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", ".taskboard/policy.yaml", "policy file to validate")
	if err := cmd.MarkFlagRequired("file"); err != nil {
		panic(fmt.Sprintf("mark required flag: %v", err))
	}

	return cmd
}
