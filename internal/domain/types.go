package domain

import (
	"fmt"
	"strings"
	"time"
)

type State string

const (
	StateBacklog                State = "Backlog"
	StateContextAdded           State = "Context Added"
	StateDesignDrafted          State = "Design Drafted"
	StateRubricReview           State = "Rubric Review"
	StateReadyForImplementation State = "Ready for Implementation"
	StateInProgress             State = "In Progress"
	StateTesting                State = "Testing"
	StateDocumented             State = "Documented"
	StateDone                   State = "Done"
)

var AllStates = []State{
	StateBacklog,
	StateContextAdded,
	StateDesignDrafted,
	StateRubricReview,
	StateReadyForImplementation,
	StateInProgress,
	StateTesting,
	StateDocumented,
	StateDone,
}

func ParseState(raw string) (State, error) {
	norm := strings.TrimSpace(raw)
	for _, s := range AllStates {
		if strings.EqualFold(norm, string(s)) {
			return s, nil
		}
	}
	return "", fmt.Errorf("invalid state %q", raw)
}

type ActorType string

const (
	ActorTypeHuman ActorType = "human"
	ActorTypeAgent ActorType = "agent"
)

func ParseActorType(raw string) (ActorType, error) {
	norm := strings.ToLower(strings.TrimSpace(raw))
	switch ActorType(norm) {
	case ActorTypeHuman, ActorTypeAgent:
		return ActorType(norm), nil
	default:
		return "", fmt.Errorf("invalid actor type %q", raw)
	}
}

type Actor struct {
	Type        ActorType
	ID          string
	DisplayName string
}

type Task struct {
	ID                string
	ShortRef          string
	Title             string
	Description       string
	State             State
	ParentID          *string
	TaskType          string
	Priority          int
	RubricPassed      bool
	IsParent          bool
	ChildrenReady     bool
	LeaseRequired     bool
	UpdatedAt         time.Time
	RequiredForParent bool
}

type Lease struct {
	TaskID    string
	ActorType ActorType
	ActorID   string
	ExpiresAt time.Time
	AutoRenew bool
}

type ArtifactType string

const (
	ArtifactContext             ArtifactType = "context"
	ArtifactDesign              ArtifactType = "design"
	ArtifactParentDesign        ArtifactType = "parent_design"
	ArtifactChildDesign         ArtifactType = "child_design"
	ArtifactRubricReview        ArtifactType = "rubric_review"
	ArtifactImplementationNotes ArtifactType = "implementation_notes"
	ArtifactTestReport          ArtifactType = "test_report"
	ArtifactDocsUpdate          ArtifactType = "docs_update"
)

var AllArtifactTypes = []ArtifactType{
	ArtifactContext,
	ArtifactDesign,
	ArtifactParentDesign,
	ArtifactChildDesign,
	ArtifactRubricReview,
	ArtifactImplementationNotes,
	ArtifactTestReport,
	ArtifactDocsUpdate,
}

func ParseArtifactType(raw string) (ArtifactType, error) {
	norm := strings.ToLower(strings.TrimSpace(raw))
	for _, t := range AllArtifactTypes {
		if norm == string(t) {
			return t, nil
		}
	}
	return "", fmt.Errorf("invalid artifact type %q", raw)
}
