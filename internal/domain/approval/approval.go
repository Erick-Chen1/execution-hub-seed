package approval

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status represents approval status.
type Status string

const (
	StatusPending  Status = "PENDING"
	StatusApproved Status = "APPROVED"
	StatusRejected Status = "REJECTED"
	StatusExecuted Status = "EXECUTED"
	StatusFailed   Status = "FAILED"
)

// Decision represents approval decision type.
type Decision string

const (
	DecisionApprove Decision = "APPROVE"
	DecisionReject  Decision = "REJECT"
)

// AppliesTo indicates which actor types require approval.
type AppliesTo string

const (
	AppliesToHuman AppliesTo = "HUMAN"
	AppliesToAgent AppliesTo = "AGENT"
	AppliesToBoth  AppliesTo = "BOTH"
)

// Operation represents the operation that requires approval.
type Operation string

const (
	OpTaskStart     Operation = "TASK_START"
	OpTaskCancel    Operation = "TASK_CANCEL"
	OpStepAck       Operation = "STEP_ACK"
	OpStepResolve   Operation = "STEP_RESOLVE"
	OpActionAck     Operation = "ACTION_ACK"
	OpActionResolve Operation = "ACTION_RESOLVE"
)

// Requirement defines approval requirement.
type Requirement struct {
	Enabled      bool      `json:"enabled"`
	Roles        []string  `json:"roles,omitempty"`
	AppliesTo    AppliesTo `json:"applies_to,omitempty"`
	MinApprovals int       `json:"min_approvals,omitempty"`
}

// Approval represents an approval request.
type Approval struct {
	ID              int64           `json:"id"`
	ApprovalID      uuid.UUID       `json:"approvalId"`
	EntityType      string          `json:"entityType"`
	EntityID        string          `json:"entityId"`
	Operation       Operation       `json:"operation"`
	Status          Status          `json:"status"`
	RequestPayload  json.RawMessage `json:"requestPayload"`
	RequestedBy     string          `json:"requestedBy"`
	RequestedByType string          `json:"requestedByType"`
	RequiredRoles   []string        `json:"requiredRoles"`
	AppliesTo       AppliesTo       `json:"appliesTo"`
	MinApprovals    int             `json:"minApprovals"`
	ApprovalsCount  int             `json:"approvalsCount"`
	RejectionsCount int             `json:"rejectionsCount"`
	RequestedAt     time.Time       `json:"requestedAt"`
	DecidedAt       *time.Time      `json:"decidedAt,omitempty"`
	ExecutedAt      *time.Time      `json:"executedAt,omitempty"`
	ExecutionError  *string         `json:"executionError,omitempty"`
}

// DecisionRecord represents a decision on an approval request.
type DecisionRecord struct {
	ID            int64     `json:"id"`
	DecisionID    uuid.UUID `json:"decisionId"`
	ApprovalID    uuid.UUID `json:"approvalId"`
	Decision      Decision  `json:"decision"`
	DecidedBy     string    `json:"decidedBy"`
	DecidedByType string    `json:"decidedByType"`
	DecidedByRole string    `json:"decidedByRole"`
	Comment       *string   `json:"comment,omitempty"`
	DecidedAt     time.Time `json:"decidedAt"`
}
