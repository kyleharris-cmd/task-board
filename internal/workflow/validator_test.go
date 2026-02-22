package workflow

import (
	"testing"

	"github.com/kyleharris/task-board/internal/domain"
	"github.com/kyleharris/task-board/internal/policy"
	"github.com/stretchr/testify/require"
)

func TestValidateTransition_ReadyForImplementationRequiresRubric(t *testing.T) {
	t.Parallel()

	p := testPolicy()
	in := TransitionInput{
		Task: domain.Task{
			State:        domain.StateRubricReview,
			RubricPassed: false,
		},
		Actor:            domain.Actor{Type: domain.ActorTypeAgent},
		ToState:          domain.StateReadyForImplementation,
		HasValidLease:    true,
		PresentArtifacts: []domain.ArtifactType{domain.ArtifactContext, domain.ArtifactDesign, domain.ArtifactRubricReview},
	}

	err := ValidateTransition(p, in)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pass rubric review")
}

func TestValidateTransition_ParentRequiresChildrenReady(t *testing.T) {
	t.Parallel()

	p := testPolicy()
	in := TransitionInput{
		Task: domain.Task{
			State:        domain.StateRubricReview,
			RubricPassed: true,
			IsParent:     true,
		},
		Actor:               domain.Actor{Type: domain.ActorTypeHuman},
		ToState:             domain.StateReadyForImplementation,
		HasValidLease:       true,
		PresentArtifacts:    []domain.ArtifactType{domain.ArtifactContext, domain.ArtifactDesign, domain.ArtifactRubricReview},
		ParentChildrenReady: false,
	}

	err := ValidateTransition(p, in)
	require.Error(t, err)
	require.Contains(t, err.Error(), "required child")
}

func TestValidateTransition_SucceedsWhenGatesPass(t *testing.T) {
	t.Parallel()

	p := testPolicy()
	in := TransitionInput{
		Task: domain.Task{
			State:        domain.StateRubricReview,
			RubricPassed: true,
			IsParent:     true,
		},
		Actor:               domain.Actor{Type: domain.ActorTypeAgent},
		ToState:             domain.StateReadyForImplementation,
		HasValidLease:       true,
		PresentArtifacts:    []domain.ArtifactType{domain.ArtifactContext, domain.ArtifactDesign, domain.ArtifactRubricReview},
		ParentChildrenReady: true,
	}

	err := ValidateTransition(p, in)
	require.NoError(t, err)
}

func testPolicy() policy.Policy {
	return policy.Policy{
		Version: 1,
		Transitions: []policy.TransitionRule{
			{From: domain.StateRubricReview, To: domain.StateReadyForImplementation, ActorTypes: []domain.ActorType{domain.ActorTypeHuman, domain.ActorTypeAgent}},
		},
		LeaseRequiredByActor: map[domain.ActorType][]domain.State{
			domain.ActorTypeAgent: {domain.StateReadyForImplementation},
			domain.ActorTypeHuman: {},
		},
		RequiredArtifactsByState: map[domain.State][]domain.ArtifactType{
			domain.StateReadyForImplementation: {domain.ArtifactContext, domain.ArtifactDesign, domain.ArtifactRubricReview},
		},
		TaskTypeLeases: map[string]policy.LeaseRule{"default": {DefaultTTLMinutes: 60, AllowAutoRenew: true}},
	}
}

func TestValidateTransition_HumanNoLeaseWhenActorPolicyAllows(t *testing.T) {
	t.Parallel()

	p := testPolicy()
	in := TransitionInput{
		Task: domain.Task{
			State:        domain.StateRubricReview,
			RubricPassed: true,
			IsParent:     false,
		},
		Actor:               domain.Actor{Type: domain.ActorTypeHuman},
		ToState:             domain.StateReadyForImplementation,
		HasValidLease:       false,
		PresentArtifacts:    []domain.ArtifactType{domain.ArtifactContext, domain.ArtifactDesign, domain.ArtifactRubricReview},
		ParentChildrenReady: true,
	}

	err := ValidateTransition(p, in)
	require.NoError(t, err)
}
