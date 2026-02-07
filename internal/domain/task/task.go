package task

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	domainAction "github.com/execution-hub/execution-hub/internal/domain/action"
	"github.com/execution-hub/execution-hub/internal/domain/executor"
)

// Status represents task status.
type Status string

const (
	StatusDraft     Status = "DRAFT"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
	StatusCancelled Status = "CANCELLED"
	StatusBlocked   Status = "BLOCKED"
)

var ErrInvalidTransition = errors.New("invalid task status transition")

// Task represents a task instance.
type Task struct {
	ID              int64           `json:"id"`
	TaskID          uuid.UUID       `json:"taskId"`
	WorkflowID      uuid.UUID       `json:"workflowId"`
	WorkflowVersion int             `json:"workflowVersion"`
	Title           string          `json:"title"`
	Status          Status          `json:"status"`
	Context         json.RawMessage `json:"context"`
	TraceID         string          `json:"traceId"`
	CreatedAt       time.Time       `json:"createdAt"`
	CreatedBy       *string         `json:"createdBy,omitempty"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	UpdatedBy       *string         `json:"updatedBy,omitempty"`
}

// CanTransitionTo validates task status transition.
func (t *Task) CanTransitionTo(target Status) bool {
	transitions := map[Status][]Status{
		StatusDraft:     {StatusRunning, StatusCancelled},
		StatusRunning:   {StatusCompleted, StatusFailed, StatusCancelled, StatusBlocked},
		StatusBlocked:   {StatusRunning, StatusCancelled, StatusFailed},
		StatusCompleted: {},
		StatusFailed:    {},
		StatusCancelled: {},
	}
	allowed := transitions[t.Status]
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

// Start sets task to running.
func (t *Task) Start() error {
	if !t.CanTransitionTo(StatusRunning) {
		return ErrInvalidTransition
	}
	t.Status = StatusRunning
	return nil
}

// Complete sets task to completed.
func (t *Task) Complete() error {
	if !t.CanTransitionTo(StatusCompleted) {
		return ErrInvalidTransition
	}
	t.Status = StatusCompleted
	return nil
}

// Fail sets task to failed.
func (t *Task) Fail() error {
	if !t.CanTransitionTo(StatusFailed) {
		return ErrInvalidTransition
	}
	t.Status = StatusFailed
	return nil
}

// Cancel sets task to cancelled.
func (t *Task) Cancel() error {
	if !t.CanTransitionTo(StatusCancelled) {
		return ErrInvalidTransition
	}
	t.Status = StatusCancelled
	return nil
}

// Block sets task to blocked.
func (t *Task) Block() error {
	if !t.CanTransitionTo(StatusBlocked) {
		return ErrInvalidTransition
	}
	t.Status = StatusBlocked
	return nil
}

// StepStatus aliases action.Status for compatibility.
type StepStatus = domainAction.Status

const (
	StepStatusCreated     StepStatus = domainAction.StatusCreated
	StepStatusDispatching StepStatus = domainAction.StatusDispatching
	StepStatusDispatched  StepStatus = domainAction.StatusDispatched
	StepStatusAcked       StepStatus = domainAction.StatusAcked
	StepStatusResolved    StepStatus = domainAction.StatusResolved
	StepStatusFailed      StepStatus = domainAction.StatusFailed
)

// Step represents a task step instance.
type Step struct {
	ID             int64             `json:"id"`
	StepID         uuid.UUID         `json:"stepId"`
	TaskID         uuid.UUID         `json:"taskId"`
	TraceID        string            `json:"traceId"`
	StepKey        string            `json:"stepKey"`
	Name           string            `json:"name"`
	Status         StepStatus        `json:"status"`
	ExecutorType   executor.Type     `json:"executorType"`
	ExecutorRef    string            `json:"executorRef"`
	ActionType     domainAction.Type `json:"actionType"`
	ActionConfig   json.RawMessage   `json:"actionConfig"`
	TimeoutSeconds int               `json:"timeoutSeconds"`
	RetryCount     int               `json:"retryCount"`
	MaxRetries     int               `json:"maxRetries"`
	DependsOn      []uuid.UUID       `json:"dependsOn,omitempty"`
	ActionID       uuid.UUID         `json:"actionId"`
	OnFail         json.RawMessage   `json:"onFail,omitempty"`
	Evidence       json.RawMessage   `json:"evidence,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	DispatchedAt   *time.Time        `json:"dispatchedAt,omitempty"`
	AckedAt        *time.Time        `json:"ackedAt,omitempty"`
	ResolvedAt     *time.Time        `json:"resolvedAt,omitempty"`
	FailedAt       *time.Time        `json:"failedAt,omitempty"`
}
