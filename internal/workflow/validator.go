package workflow

import (
	"fmt"
	"slices"

	"github.com/kyleharris/task-board/internal/domain"
	"github.com/kyleharris/task-board/internal/policy"
)

type TransitionInput struct {
	Task                domain.Task
	Actor               domain.Actor
	ToState             domain.State
	HasValidLease       bool
	PresentArtifacts    []domain.ArtifactType
	ParentChildrenReady bool
}

func ValidateTransition(p policy.Policy, in TransitionInput) error {
	if !p.CanTransition(in.Actor.Type, in.Task.State, in.ToState) {
		return fmt.Errorf("transition %q -> %q not allowed for actor type %q", in.Task.State, in.ToState, in.Actor.Type)
	}

	if p.RequiresLeaseForStateAndActor(in.Actor.Type, in.ToState) && !in.HasValidLease {
		return fmt.Errorf("state %q requires a valid task lease", in.ToState)
	}

	requiredArtifacts := p.RequiredArtifacts(in.ToState)
	for _, req := range requiredArtifacts {
		if !slices.Contains(in.PresentArtifacts, req) {
			return fmt.Errorf("state %q requires artifact %q", in.ToState, req)
		}
	}

	if in.ToState == domain.StateReadyForImplementation {
		if !in.Task.RubricPassed {
			return fmt.Errorf("task must pass rubric review before entering %q", domain.StateReadyForImplementation)
		}
		if in.Task.IsParent && !in.ParentChildrenReady {
			return fmt.Errorf("all required child tasks must be rubric-ready before parent can enter %q", domain.StateReadyForImplementation)
		}
	}

	return nil
}
