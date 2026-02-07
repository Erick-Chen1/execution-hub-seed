package collab

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SessionStatus describes collaboration session state.
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "ACTIVE"
	SessionStatusCompleted SessionStatus = "COMPLETED"
	SessionStatusFailed    SessionStatus = "FAILED"
	SessionStatusCancelled SessionStatus = "CANCELLED"
)

// ParticipantType describes who joins a session.
type ParticipantType string

const (
	ParticipantTypeHuman ParticipantType = "HUMAN"
	ParticipantTypeAgent ParticipantType = "AGENT"
)

// StepStatus describes collaboration step state.
type StepStatus string

const (
	StepStatusOpen     StepStatus = "OPEN"
	StepStatusClaimed  StepStatus = "CLAIMED"
	StepStatusInReview StepStatus = "IN_REVIEW"
	StepStatusResolved StepStatus = "RESOLVED"
	StepStatusFailed   StepStatus = "FAILED"
)

// ClaimStatus describes claim lease state.
type ClaimStatus string

const (
	ClaimStatusActive   ClaimStatus = "ACTIVE"
	ClaimStatusExpired  ClaimStatus = "EXPIRED"
	ClaimStatusReleased ClaimStatus = "RELEASED"
)

// DecisionStatus describes decision state.
type DecisionStatus string

const (
	DecisionStatusPending  DecisionStatus = "PENDING"
	DecisionStatusPassed   DecisionStatus = "PASSED"
	DecisionStatusRejected DecisionStatus = "REJECTED"
)

// VoteChoice describes the vote result from one participant.
type VoteChoice string

const (
	VoteChoiceApprove VoteChoice = "APPROVE"
	VoteChoiceReject  VoteChoice = "REJECT"
)

// EventType describes collaboration event type.
type EventType string

const (
	EventTypeSessionCreated   EventType = "SESSION_CREATED"
	EventTypeSessionCompleted EventType = "SESSION_COMPLETED"
	EventTypeParticipantJoin  EventType = "PARTICIPANT_JOINED"
	EventTypeStepClaimed      EventType = "STEP_CLAIMED"
	EventTypeStepReleased     EventType = "STEP_RELEASED"
	EventTypeStepHandoff      EventType = "STEP_HANDOFF"
	EventTypeArtifactSubmit   EventType = "ARTIFACT_SUBMITTED"
	EventTypeDecisionOpen     EventType = "DECISION_OPENED"
	EventTypeVoteCast         EventType = "VOTE_CAST"
	EventTypeStepResolved     EventType = "STEP_RESOLVED"
	EventTypeStepFailed       EventType = "STEP_FAILED"
	EventTypeClaimExpired     EventType = "CLAIM_EXPIRED"
)

// Session is a collaboration runtime instance bound to a task/workflow.
type Session struct {
	ID              int64           `json:"id"`
	SessionID       uuid.UUID       `json:"sessionId"`
	TaskID          uuid.UUID       `json:"taskId"`
	WorkflowID      uuid.UUID       `json:"workflowId"`
	WorkflowVersion int             `json:"workflowVersion"`
	Name            string          `json:"name"`
	Status          SessionStatus   `json:"status"`
	Context         json.RawMessage `json:"context,omitempty"`
	TraceID         string          `json:"traceId"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// Participant joins a collaboration session.
type Participant struct {
	ID            int64           `json:"id"`
	ParticipantID uuid.UUID       `json:"participantId"`
	SessionID     uuid.UUID       `json:"sessionId"`
	Type          ParticipantType `json:"type"`
	Ref           string          `json:"ref"`
	Capabilities  []string        `json:"capabilities,omitempty"`
	TrustScore    int             `json:"trustScore"`
	JoinedAt      time.Time       `json:"joinedAt"`
	LastSeenAt    *time.Time      `json:"lastSeenAt,omitempty"`
}

// Step represents an executable collaboration step.
type Step struct {
	ID                   int64           `json:"id"`
	StepID               uuid.UUID       `json:"stepId"`
	SessionID            uuid.UUID       `json:"sessionId"`
	StepKey              string          `json:"stepKey"`
	Name                 string          `json:"name"`
	Status               StepStatus      `json:"status"`
	RequiredCapabilities []string        `json:"requiredCapabilities,omitempty"`
	DependsOn            []string        `json:"dependsOn,omitempty"`
	LeaseTTLSeconds      int             `json:"leaseTtlSeconds"`
	ConsensusPolicy      json.RawMessage `json:"consensusPolicy,omitempty"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
	ResolvedAt           *time.Time      `json:"resolvedAt,omitempty"`
}

// Claim represents an active lease for a step.
type Claim struct {
	ID            int64       `json:"id"`
	ClaimID       uuid.UUID   `json:"claimId"`
	StepID        uuid.UUID   `json:"stepId"`
	ParticipantID uuid.UUID   `json:"participantId"`
	Status        ClaimStatus `json:"status"`
	LeaseUntil    time.Time   `json:"leaseUntil"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
}

// Artifact is a versioned output for a step.
type Artifact struct {
	ID         int64           `json:"id"`
	ArtifactID uuid.UUID       `json:"artifactId"`
	StepID     uuid.UUID       `json:"stepId"`
	ProducerID uuid.UUID       `json:"producerId"`
	Kind       string          `json:"kind"`
	Content    json.RawMessage `json:"content"`
	Version    int             `json:"version"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// Decision captures review policy and result.
type Decision struct {
	ID         int64           `json:"id"`
	DecisionID uuid.UUID       `json:"decisionId"`
	StepID     uuid.UUID       `json:"stepId"`
	Policy     json.RawMessage `json:"policy"`
	Deadline   *time.Time      `json:"deadline,omitempty"`
	Status     DecisionStatus  `json:"status"`
	Result     *string         `json:"result,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	DecidedAt  *time.Time      `json:"decidedAt,omitempty"`
}

// Vote is one participant decision input.
type Vote struct {
	ID            int64      `json:"id"`
	VoteID        uuid.UUID  `json:"voteId"`
	DecisionID    uuid.UUID  `json:"decisionId"`
	ParticipantID uuid.UUID  `json:"participantId"`
	Choice        VoteChoice `json:"choice"`
	Comment       *string    `json:"comment,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// Event is append-only collaboration timeline.
type Event struct {
	ID        int64           `json:"id"`
	EventID   uuid.UUID       `json:"eventId"`
	SessionID uuid.UUID       `json:"sessionId"`
	StepID    *uuid.UUID      `json:"stepId,omitempty"`
	Type      EventType       `json:"type"`
	Actor     string          `json:"actor"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}
