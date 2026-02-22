package main

import (
	"fmt"
	"os"

	"github.com/kyleharris/task-board/internal/domain"
	"github.com/kyleharris/task-board/internal/policy"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newPolicyCmd() *cobra.Command {
	policyCmd := &cobra.Command{
		Use:   "policy",
		Short: "Policy management commands",
	}

	policyCmd.AddCommand(newPolicyValidateCmd())
	policyCmd.AddCommand(newPolicyMigrateCmd())
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

func newPolicyMigrateCmd() *cobra.Command {
	var file string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate policy file to latest schema (adds actor-specific lease config)",
		RunE: func(cmd *cobra.Command, args []string) error {
			changed, err := migratePolicyFile(file, dryRun)
			if err != nil {
				return err
			}
			if dryRun {
				if changed {
					cmd.Printf("policy %s requires migration\n", file)
				} else {
					cmd.Printf("policy %s is already up to date\n", file)
				}
				return nil
			}
			if changed {
				cmd.Printf("migrated policy %s\n", file)
			} else {
				cmd.Printf("policy %s is already up to date\n", file)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", ".taskboard/policy.yaml", "policy file to migrate")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report whether migration is needed without writing changes")
	return cmd
}

func migratePolicyFile(path string, dryRun bool) (bool, error) {
	p, err := policy.Load(path)
	if err != nil {
		return false, err
	}
	if len(p.LeaseRequiredByActor) > 0 {
		return false, nil
	}

	agentStates := make([]domain.State, 0, len(p.LeaseRequiredStates))
	agentStates = append(agentStates, p.LeaseRequiredStates...)
	p.LeaseRequiredByActor = map[domain.ActorType][]domain.State{
		domain.ActorTypeAgent: agentStates,
		domain.ActorTypeHuman: {},
	}

	if err := p.Validate(); err != nil {
		return false, fmt.Errorf("validate migrated policy: %w", err)
	}
	if dryRun {
		return true, nil
	}

	raw, err := yaml.Marshal(p)
	if err != nil {
		return false, fmt.Errorf("marshal migrated policy: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return false, fmt.Errorf("write migrated policy: %w", err)
	}
	return true, nil
}
