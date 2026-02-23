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
lease_required_by_actor:
  agent: ["In Progress"]
  human: []
transitions:
  - from: "Scoping"
    to: "Design"
    actor_types: ["human", "agent"]
required_artifacts_by_state:
  "Scoping": ["context"]
task_type_leases:
  default:
    default_ttl_minutes: 60
    allow_auto_renew: true
`)
	require.NoError(t, os.WriteFile(path, content, 0o644))

	p, err := Load(path)
	require.NoError(t, err)
	require.True(t, p.CanTransition(domain.ActorTypeHuman, domain.StateScoping, domain.StateDesign))
	require.True(t, p.CanTransition(domain.ActorTypeHuman, domain.StateScoping, domain.StateComplete))
	require.False(t, p.CanTransition(domain.ActorTypeHuman, domain.StateScoping, domain.StateScoping))
	require.False(t, p.CanTransition(domain.ActorTypeAgent, domain.StateScoping, domain.StateComplete))
	require.True(t, p.RequiresLeaseForState(domain.StateInProgress))
	require.True(t, p.RequiresLeaseForStateAndActor(domain.ActorTypeAgent, domain.StateInProgress))
	require.False(t, p.RequiresLeaseForStateAndActor(domain.ActorTypeHuman, domain.StateInProgress))
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

func TestRequiresLeaseForStateAndActor_FallbackLegacyStates(t *testing.T) {
	t.Parallel()

	p := Policy{
		Version:             1,
		LeaseRequiredStates: []domain.State{domain.StateInProgress},
		Transitions: []TransitionRule{
			{From: domain.StateScoping, To: domain.StateDesign, ActorTypes: []domain.ActorType{domain.ActorTypeHuman}},
		},
		TaskTypeLeases: map[string]LeaseRule{
			"default": {DefaultTTLMinutes: 60, AllowAutoRenew: true},
		},
	}

	require.True(t, p.RequiresLeaseForStateAndActor(domain.ActorTypeHuman, domain.StateInProgress))
	require.True(t, p.RequiresLeaseForStateAndActor(domain.ActorTypeAgent, domain.StateInProgress))
}
