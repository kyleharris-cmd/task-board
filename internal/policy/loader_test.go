package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleharris/task-board/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidPolicy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := []byte(`version: 1
lease_required_states:
  - "In Progress"
transitions:
  - from: "Backlog"
    to: "Context Added"
    actor_types: ["human", "agent"]
required_artifacts_by_state:
  "Context Added": ["context"]
task_type_leases:
  default:
    default_ttl_minutes: 60
    allow_auto_renew: true
`)
	require.NoError(t, os.WriteFile(path, content, 0o644))

	p, err := Load(path)
	require.NoError(t, err)
	require.True(t, p.CanTransition(domain.ActorTypeHuman, domain.StateBacklog, domain.StateContextAdded))
	require.True(t, p.RequiresLeaseForState(domain.StateInProgress))
}

func TestLoad_InvalidPolicy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := []byte(`version: 0
transitions: []
`)
	require.NoError(t, os.WriteFile(path, content, 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validate policy")
}
