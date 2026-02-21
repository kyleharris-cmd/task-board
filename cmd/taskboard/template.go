package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*
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

func writeTaskboardBootstrap(repoRoot, taskboardDir string) error {
	files := map[string]string{
		filepath.Join(taskboardDir, "WORKFLOW.md"):                                  "templates/workflow.md",
		filepath.Join(taskboardDir, "PROMPTS", "idea-to-design.txt"):                "templates/prompt.idea-to-design.txt",
		filepath.Join(taskboardDir, "PROMPTS", "design-to-parent-and-children.txt"): "templates/prompt.design-to-parent-and-children.txt",
		filepath.Join(taskboardDir, "PROMPTS", "implement-child-task.txt"):          "templates/prompt.implement-child-task.txt",
	}

	for outPath, templatePath := range files {
		if err := writeTemplateIfMissing(outPath, templatePath); err != nil {
			return err
		}
	}

	if err := ensureGitignoreContains(repoRoot, ".taskboard/"); err != nil {
		return err
	}
	return ensureAgentsTaskboardSnippet(repoRoot)
}

func writeTemplateIfMissing(outPath, templatePath string) error {
	if _, err := os.Stat(outPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check template output path: %w", err)
	}
	raw, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", templatePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create template output dir: %w", err)
	}
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		return fmt.Errorf("write template %s: %w", outPath, err)
	}
	return nil
}

func ensureGitignoreContains(repoRoot, line string) error {
	gitignorePath := filepath.Join(repoRoot, ".gitignore")
	existing := ""
	if raw, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(raw)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	if strings.Contains(existing, line) {
		return nil
	}

	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	existing += line + "\n"
	if err := os.WriteFile(gitignorePath, []byte(existing), 0o644); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}
	return nil
}

func ensureAgentsTaskboardSnippet(repoRoot string) error {
	agentsPath := filepath.Join(repoRoot, "AGENTS.md")
	raw, err := os.ReadFile(agentsPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read AGENTS.md: %w", err)
	}
	content := string(raw)
	if strings.Contains(content, "## Taskboard Protocol") {
		return nil
	}

	snippet, err := templateFS.ReadFile("templates/agents.taskboard.snippet.md")
	if err != nil {
		return fmt.Errorf("read AGENTS snippet template: %w", err)
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n" + string(snippet) + "\n"
	if err := os.WriteFile(agentsPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("patch AGENTS.md with taskboard protocol: %w", err)
	}
	return nil
}
