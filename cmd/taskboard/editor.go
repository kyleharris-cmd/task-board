package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func editContentWithEditor(initial string) (string, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	if !isInteractiveInput() {
		return "", fmt.Errorf("non-interactive shell: provide content with flags or run in interactive terminal")
	}

	tmpDir, err := os.MkdirTemp("", "taskboard-editor-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir for editor: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "artifact.md")
	if err := os.WriteFile(tmpPath, []byte(initial), 0o600); err != nil {
		return "", fmt.Errorf("write temp editor file: %w", err)
	}

	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("open editor %q: %w", editor, err)
	}

	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read edited content: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}
