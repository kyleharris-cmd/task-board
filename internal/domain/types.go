package domain

import "time"

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

type ActorType string

const (
	ActorTypeHuman ActorType = "human"
	ActorTypeAgent ActorType = "agent"
)

type Actor struct {
	Type        ActorType
	ID          string
	DisplayName string
}

type Task struct {
	ID                string
	Title             string
	State             State
	ParentID          *string
	TaskType          string
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
	ArtifactRubricReview        ArtifactType = "rubric_review"
	ArtifactImplementationNotes ArtifactType = "implementation_notes"
	ArtifactTestReport          ArtifactType = "test_report"
	ArtifactDocsUpdate          ArtifactType = "docs_update"
)
