package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleharris/task-board/internal/domain"
	"github.com/kyleharris/task-board/internal/policy"
	"github.com/stretchr/testify/require"
)

func TestMigratePolicyFile_AddsLeaseRequiredByActor(t *testing.T) {
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

	changed, err := migratePolicyFile(path, false)
	require.NoError(t, err)
	require.True(t, changed)

	p, err := policy.Load(path)
	require.NoError(t, err)
	require.Contains(t, p.LeaseRequiredByActor, domain.ActorTypeAgent)
	require.Contains(t, p.LeaseRequiredByActor, domain.ActorTypeHuman)
	require.NotEmpty(t, p.LeaseRequiredByActor[domain.ActorTypeAgent])
}

func TestMigratePolicyFile_NoOpWhenAlreadyMigrated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := []byte(`version: 1
lease_required_states:
  - "In Progress"
lease_required_by_actor:
  agent: ["In Progress"]
  human: []
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

	changed, err := migratePolicyFile(path, false)
	require.NoError(t, err)
	require.False(t, changed)
}
