package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kyleharris/task-board/internal/domain"
	"github.com/spf13/cobra"
)

type optionalActorFlags struct {
	actorType string
	actorID   string
	actorName string
}

func (f *optionalActorFlags) add(cmd *cobra.Command, defaultType domain.ActorType) {
	cmd.Flags().StringVar(&f.actorType, "actor-type", string(defaultType), "actor type (human|agent)")
	cmd.Flags().StringVar(&f.actorID, "actor-id", "", "actor ID (optional for human when git identity is configured)")
	cmd.Flags().StringVar(&f.actorName, "actor-name", "", "actor display name (optional for human when git identity is configured)")
}

func (f optionalActorFlags) resolve(repoRoot string) (domain.Actor, error) {
	actorType, err := domain.ParseActorType(f.actorType)
	if err != nil {
		return domain.Actor{}, err
	}

	if actorType == domain.ActorTypeAgent {
		if strings.TrimSpace(f.actorID) == "" || strings.TrimSpace(f.actorName) == "" {
			return domain.Actor{}, errors.New("agent calls require explicit --actor-id and --actor-name")
		}
		return domain.Actor{Type: actorType, ID: strings.TrimSpace(f.actorID), DisplayName: strings.TrimSpace(f.actorName)}, nil
	}

	humanID := strings.TrimSpace(f.actorID)
	humanName := strings.TrimSpace(f.actorName)
	if humanID == "" {
		humanID, _ = gitConfigGet(repoRoot, "user.email")
	}
	if humanName == "" {
		humanName, _ = gitConfigGet(repoRoot, "user.name")
	}

	if humanID == "" || humanName == "" {
		if !isInteractiveInput() {
			return domain.Actor{}, errors.New("human git identity missing; configure git user.name/user.email or run in interactive terminal")
		}
		name, email, scope, err := promptForGitIdentity(humanName, humanID)
		if err != nil {
			return domain.Actor{}, err
		}
		if err := gitConfigSet(repoRoot, scope == "global", "user.name", name); err != nil {
			return domain.Actor{}, err
		}
		if err := gitConfigSet(repoRoot, scope == "global", "user.email", email); err != nil {
			return domain.Actor{}, err
		}
		humanName = name
		humanID = email
	}

	return domain.Actor{Type: domain.ActorTypeHuman, ID: humanID, DisplayName: humanName}, nil
}

func isInteractiveInput() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func promptForGitIdentity(defaultName, defaultEmail string) (name, email, scope string, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Human identity is missing in git config.")
	fmt.Println("Set git user.name/user.email to continue.")

	name = strings.TrimSpace(defaultName)
	email = strings.TrimSpace(defaultEmail)

	if name == "" {
		fmt.Print("Enter git user.name: ")
		raw, readErr := reader.ReadString('\n')
		if readErr != nil {
			return "", "", "", fmt.Errorf("read git user.name: %w", readErr)
		}
		name = strings.TrimSpace(raw)
	}
	if email == "" {
		fmt.Print("Enter git user.email: ")
		raw, readErr := reader.ReadString('\n')
		if readErr != nil {
			return "", "", "", fmt.Errorf("read git user.email: %w", readErr)
		}
		email = strings.TrimSpace(raw)
	}

	if name == "" || email == "" {
		return "", "", "", errors.New("git user.name and user.email are required")
	}

	fmt.Print("Apply scope? [1=repo-local, 2=global]: ")
	rawScope, readErr := reader.ReadString('\n')
	if readErr != nil {
		return "", "", "", fmt.Errorf("read git config scope: %w", readErr)
	}
	rawScope = strings.TrimSpace(rawScope)
	scope = "local"
	if rawScope == "2" {
		scope = "global"
	}

	return name, email, scope, nil
}

func gitConfigGet(repoRoot, key string) (string, error) {
	cmd := exec.Command("git", "-C", filepath.Clean(repoRoot), "config", "--get", key)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitConfigSet(repoRoot string, global bool, key, value string) error {
	args := []string{}
	if global {
		args = append(args, "config", "--global", key, value)
	} else {
		args = append(args, "-C", filepath.Clean(repoRoot), "config", key, value)
	}
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set git config %s: %w (%s)", key, err, strings.TrimSpace(string(out)))
	}
	return nil
}
