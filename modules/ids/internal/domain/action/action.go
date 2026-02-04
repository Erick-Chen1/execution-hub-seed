package action

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status represents the status of an action
type Status string

const (
	StatusCreated     Status = "CREATED"
	StatusDispatching Status = "DISPATCHING"
	StatusDispatched  Status = "DISPATCHED"
	StatusAcked       Status = "ACKED"
	StatusResolved    Status = "RESOLVED"
	StatusFailed      Status = "FAILED"
)

// Priority represents the priority of an action
type Priority string

const (
	PriorityLow      Priority = "LOW"
	PriorityMedium   Priority = "MEDIUM"
	PriorityHigh     Priority = "HIGH"
	PriorityCritical Priority = "CRITICAL"
)

// Type represents the type of action
type Type string

const (
	TypeNotify   Type = "NOTIFY"
	TypeWebhook  Type = "WEBHOOK"
	TypeEscalate Type = "ESCALATE"
)

var (
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrCannotRetry       = errors.New("cannot retry: max retries exceeded or not in failed state")
)

// Action represents an action triggered by a rule evaluation
type Action struct {
	ID            int64           `json:"id"`
	ActionID      uuid.UUID       `json:"actionId"`
	RuleID        uuid.UUID       `json:"ruleId"`
	RuleVersion   int             `json:"ruleVersion"`
	EvaluationID  uuid.UUID       `json:"evaluationId"`
	ActionType    Type            `json:"actionType"`
	ActionConfig  json.RawMessage `json:"actionConfig"`
	Status        Status          `json:"status"`
	DedupeKey     *string         `json:"dedupeKey,omitempty"`
	CooldownUntil *time.Time      `json:"cooldownUntil,omitempty"`
	Priority      Priority        `json:"priority"`
	TTL           *time.Duration  `json:"ttl,omitempty"`
	RetryCount    int             `json:"retryCount"`
	MaxRetries    int             `json:"maxRetries"`
	LastError     *string         `json:"lastError,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	DispatchedAt  *time.Time      `json:"dispatchedAt,omitempty"`
	AckedAt       *time.Time      `json:"ackedAt,omitempty"`
	AckedBy       *string         `json:"ackedBy,omitempty"`
	ResolvedAt    *time.Time      `json:"resolvedAt,omitempty"`
	ResolvedBy    *string         `json:"resolvedBy,omitempty"`
	FailedAt      *time.Time      `json:"failedAt,omitempty"`
	TraceID       *string         `json:"traceId,omitempty"`
}

// NewAction creates a new Action
func NewAction(
	ruleID uuid.UUID,
	ruleVersion int,
	evaluationID uuid.UUID,
	actionType Type,
	actionConfig json.RawMessage,
) *Action {
	return &Action{
		ActionID:     uuid.New(),
		RuleID:       ruleID,
		RuleVersion:  ruleVersion,
		EvaluationID: evaluationID,
		ActionType:   actionType,
		ActionConfig: actionConfig,
		Status:       StatusCreated,
		Priority:     PriorityMedium,
		MaxRetries:   3,
		CreatedAt:    time.Now().UTC(),
	}
}

// SetPriority sets the priority based on severity string
func (a *Action) SetPriority(severity string) {
	switch severity {
	case "CRITICAL":
		a.Priority = PriorityCritical
	case "HIGH":
		a.Priority = PriorityHigh
	case "MEDIUM":
		a.Priority = PriorityMedium
	default:
		a.Priority = PriorityLow
	}
}

// SetDedupeKey sets the deduplication key
func (a *Action) SetDedupeKey(key string) {
	a.DedupeKey = &key
}

// SetCooldown sets the cooldown period
func (a *Action) SetCooldown(until time.Time) {
	a.CooldownUntil = &until
}

// SetTraceID sets the trace ID
func (a *Action) SetTraceID(traceID string) {
	a.TraceID = &traceID
}

// CanTransitionTo checks if a transition to the target status is valid
func (a *Action) CanTransitionTo(target Status) bool {
	transitions := map[Status][]Status{
		StatusCreated:     {StatusDispatching, StatusFailed},
		StatusDispatching: {StatusDispatched, StatusFailed},
		StatusDispatched:  {StatusAcked, StatusResolved, StatusFailed},
		StatusAcked:       {StatusResolved},
		StatusResolved:    {},
		StatusFailed:      {StatusCreated}, // Retry
	}

	allowed, ok := transitions[a.Status]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

// Dispatch starts the dispatch process
func (a *Action) Dispatch() error {
	if !a.CanTransitionTo(StatusDispatching) {
		return ErrInvalidTransition
	}
	a.Status = StatusDispatching
	return nil
}

// ConfirmDispatched confirms the action was dispatched
func (a *Action) ConfirmDispatched() error {
	if !a.CanTransitionTo(StatusDispatched) {
		return ErrInvalidTransition
	}
	a.Status = StatusDispatched
	now := time.Now().UTC()
	a.DispatchedAt = &now
	return nil
}

// Acknowledge acknowledges the action
func (a *Action) Acknowledge(by string) error {
	if !a.CanTransitionTo(StatusAcked) {
		return ErrInvalidTransition
	}
	a.Status = StatusAcked
	now := time.Now().UTC()
	a.AckedAt = &now
	a.AckedBy = &by
	return nil
}

// Resolve resolves the action
func (a *Action) Resolve(by string) error {
	if !a.CanTransitionTo(StatusResolved) {
		return ErrInvalidTransition
	}
	a.Status = StatusResolved
	now := time.Now().UTC()
	a.ResolvedAt = &now
	a.ResolvedBy = &by
	return nil
}

// Fail marks the action as failed
func (a *Action) Fail(errMsg string) error {
	if !a.CanTransitionTo(StatusFailed) {
		return ErrInvalidTransition
	}
	a.Status = StatusFailed
	now := time.Now().UTC()
	a.FailedAt = &now
	a.LastError = &errMsg
	a.RetryCount++
	return nil
}

// CanRetry checks if the action can be retried
func (a *Action) CanRetry() bool {
	return a.Status == StatusFailed && a.RetryCount < a.MaxRetries
}

// ResetForRetry resets the action for retry
func (a *Action) ResetForRetry() error {
	if !a.CanRetry() {
		return ErrCannotRetry
	}
	a.Status = StatusCreated
	a.FailedAt = nil
	return nil
}

// IsTerminal returns true if the action is in a terminal state
func (a *Action) IsTerminal() bool {
	return a.Status == StatusResolved || (a.Status == StatusFailed && !a.CanRetry())
}

// StateTransition represents a state transition record
type StateTransition struct {
	ID             int64                  `json:"id"`
	ActionID       uuid.UUID              `json:"actionId"`
	FromStatus     *Status                `json:"fromStatus,omitempty"`
	ToStatus       Status                 `json:"toStatus"`
	TransitionedAt time.Time              `json:"transitionedAt"`
	Reason         *string                `json:"reason,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// NewStateTransition creates a new state transition record
func NewStateTransition(actionID uuid.UUID, from *Status, to Status, reason *string) *StateTransition {
	return &StateTransition{
		ActionID:       actionID,
		FromStatus:     from,
		ToStatus:       to,
		TransitionedAt: time.Now().UTC(),
		Reason:         reason,
	}
}

// Filter represents filters for querying actions
type Filter struct {
	RuleID       *uuid.UUID
	EvaluationID *uuid.UUID
	Status       *Status
	Priority     *Priority
	Since        *time.Time
	Until        *time.Time
}
