package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func editContentWithEditor(initial string) (string, error) {
	cmd, tmpPath, cleanup, err := prepareEditorProcess(initial)
	if err != nil {
		return "", err
	}
	defer cleanup()

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("open editor: %w", err)
	}
	return readEditedContent(tmpPath)
}

func prepareEditorProcess(initial string) (*exec.Cmd, string, func(), error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	if !isInteractiveInput() {
		return nil, "", nil, fmt.Errorf("non-interactive shell: provide content with flags or run in interactive terminal")
	}

	tmpDir, err := os.MkdirTemp("", "taskboard-editor-*")
	if err != nil {
		return nil, "", nil, fmt.Errorf("create temp dir for editor: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	tmpPath := filepath.Join(tmpDir, "artifact.md")
	if err := os.WriteFile(tmpPath, []byte(initial), 0o600); err != nil {
		cleanup()
		return nil, "", nil, fmt.Errorf("write temp editor file: %w", err)
	}

	parts := strings.Fields(editor)
	if len(parts) == 0 {
		cleanup()
		return nil, "", nil, fmt.Errorf("invalid EDITOR value")
	}
	args := append(parts[1:], tmpPath)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, tmpPath, cleanup, nil
}

func readEditedContent(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read edited content: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}
