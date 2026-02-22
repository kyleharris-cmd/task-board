package policy

import (
	"fmt"
	"sort"

	"github.com/kyleharris/task-board/internal/domain"
)

type Policy struct {
	Version                  int                                    `yaml:"version"`
	LeaseRequiredStates      []domain.State                         `yaml:"lease_required_states"`
	LeaseRequiredByActor     map[domain.ActorType][]domain.State    `yaml:"lease_required_by_actor"`
	Transitions              []TransitionRule                       `yaml:"transitions"`
	RequiredArtifactsByState map[domain.State][]domain.ArtifactType `yaml:"required_artifacts_by_state"`
	TaskTypeLeases           map[string]LeaseRule                   `yaml:"task_type_leases"`
}

type TransitionRule struct {
	From       domain.State       `yaml:"from"`
	To         domain.State       `yaml:"to"`
	ActorTypes []domain.ActorType `yaml:"actor_types"`
}

type LeaseRule struct {
	DefaultTTLMinutes int  `yaml:"default_ttl_minutes"`
	AllowAutoRenew    bool `yaml:"allow_auto_renew"`
}

func (p Policy) Validate() error {
	if p.Version <= 0 {
		return fmt.Errorf("policy version must be > 0")
	}
	if len(p.Transitions) == 0 {
		return fmt.Errorf("policy transitions must not be empty")
	}
	for _, rule := range p.Transitions {
		if rule.From == "" || rule.To == "" {
			return fmt.Errorf("transition from/to must be non-empty")
		}
		if len(rule.ActorTypes) == 0 {
			return fmt.Errorf("transition %q -> %q must define actor_types", rule.From, rule.To)
		}
	}
	for taskType, lease := range p.TaskTypeLeases {
		if lease.DefaultTTLMinutes <= 0 {
			return fmt.Errorf("task_type_leases.%s.default_ttl_minutes must be > 0", taskType)
		}
	}
	for actorType := range p.LeaseRequiredByActor {
		if _, err := domain.ParseActorType(string(actorType)); err != nil {
			return fmt.Errorf("lease_required_by_actor has invalid actor type key %q", actorType)
		}
	}

	return nil
}

func (p Policy) CanTransition(actorType domain.ActorType, from, to domain.State) bool {
	if from == to {
		return false
	}
	if actorType == domain.ActorTypeHuman {
		return true
	}

	for _, rule := range p.Transitions {
		if rule.From != from || rule.To != to {
			continue
		}
		for _, at := range rule.ActorTypes {
			if at == actorType {
				return true
			}
		}
	}

	return false
}

func (p Policy) RequiresLeaseForState(state domain.State) bool {
	for _, s := range p.LeaseRequiredStates {
		if s == state {
			return true
		}
	}

	return false
}

func (p Policy) RequiresLeaseForStateAndActor(actorType domain.ActorType, state domain.State) bool {
	if len(p.LeaseRequiredByActor) > 0 {
		states, ok := p.LeaseRequiredByActor[actorType]
		if !ok {
			return false
		}
		for _, s := range states {
			if s == state {
				return true
			}
		}
		return false
	}
	return p.RequiresLeaseForState(state)
}

func (p Policy) RequiredArtifacts(state domain.State) []domain.ArtifactType {
	arts, ok := p.RequiredArtifactsByState[state]
	if !ok {
		return nil
	}
	clone := make([]domain.ArtifactType, 0, len(arts))
	clone = append(clone, arts...)
	sort.Slice(clone, func(i, j int) bool { return clone[i] < clone[j] })
	return clone
}

func (p Policy) LeaseRuleForTaskType(taskType string) (LeaseRule, bool) {
	v, ok := p.TaskTypeLeases[taskType]
	return v, ok
}
