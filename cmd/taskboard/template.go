package main

import (
	"embed"
	"fmt"
	"os"
)

//go:embed templates/policy.default.yaml
var templateFS embed.FS

func writeDefaultPolicyIfMissing(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check existing policy file: %w", err)
	}

	raw, err := templateFS.ReadFile("templates/policy.default.yaml")
	if err != nil {
		return fmt.Errorf("read default policy template: %w", err)
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write default policy file: %w", err)
	}

	return nil
}
