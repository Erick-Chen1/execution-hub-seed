package workflow

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	domainAction "github.com/execution-hub/execution-hub/internal/domain/action"
	"github.com/execution-hub/execution-hub/internal/domain/approval"
	"github.com/execution-hub/execution-hub/internal/domain/executor"
)

// Status represents workflow definition status.
type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
	StatusArchived Status = "ARCHIVED"
)

// Definition represents a versioned workflow definition.
type Definition struct {
	ID          int64           `json:"id"`
	WorkflowID  uuid.UUID       `json:"workflowId"`
	Name        string          `json:"name"`
	Version     int             `json:"version"`
	Description string          `json:"description"`
	Status      Status          `json:"status"`
	Definition  json.RawMessage `json:"definition"`
	CreatedAt   time.Time       `json:"createdAt"`
	CreatedBy   *string         `json:"createdBy,omitempty"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	UpdatedBy   *string         `json:"updatedBy,omitempty"`
}

// Spec is the parsed workflow specification used for orchestration.
type Spec struct {
	WorkflowID  uuid.UUID       `json:"workflow_id"`
	Version     int             `json:"version"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Steps       []Step          `json:"steps"`
	Edges       []Edge          `json:"edges,omitempty"`
	Conditions  []ConditionExpr `json:"conditions,omitempty"`
	Approvals   *Approvals      `json:"approvals,omitempty"`
}

// Step describes a step template in workflow definition.
type Step struct {
	StepKey        string            `json:"step_key"`
	Name           string            `json:"name"`
	ExecutorType   executor.Type     `json:"executor_type"`
	ExecutorRef    string            `json:"executor_ref"`
	ActionType     domainAction.Type `json:"action_type"`
	ActionConfig   json.RawMessage   `json:"action_config"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	MaxRetries     int               `json:"max_retries"`
	DependsOn      []string          `json:"depends_on,omitempty"`
	OnFail         json.RawMessage   `json:"on_fail,omitempty"`
}

// Edge represents a dependency between steps.
type Edge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
}

// ConditionExpr represents a named condition expression.
type ConditionExpr struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

// Approvals defines approval policies for workflow operations.
type Approvals struct {
	TaskStart     *approval.Requirement `json:"task_start,omitempty"`
	TaskCancel    *approval.Requirement `json:"task_cancel,omitempty"`
	StepAck       *approval.Requirement `json:"step_ack,omitempty"`
	StepResolve   *approval.Requirement `json:"step_resolve,omitempty"`
	ActionAck     *approval.Requirement `json:"action_ack,omitempty"`
	ActionResolve *approval.Requirement `json:"action_resolve,omitempty"`
}

// ParseSpec parses a workflow definition JSON into Spec.
func ParseSpec(data json.RawMessage) (*Spec, error) {
	if len(data) == 0 {
		return nil, errors.New("workflow definition is empty")
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// ValidateSpec validates a workflow specification.
func ValidateSpec(spec *Spec) error {
	if spec == nil {
		return errors.New("spec is nil")
	}
	if spec.Name == "" {
		return errors.New("name is required")
	}
	if len(spec.Steps) == 0 {
		return errors.New("steps are required")
	}
	seen := make(map[string]struct{})
	for _, step := range spec.Steps {
		if step.StepKey == "" {
			return errors.New("step_key is required")
		}
		if _, ok := seen[step.StepKey]; ok {
			return errors.New("duplicate step_key: " + step.StepKey)
		}
		seen[step.StepKey] = struct{}{}
		if step.ExecutorRef == "" {
			return errors.New("executor_ref is required for step: " + step.StepKey)
		}
		if len(step.ActionConfig) == 0 {
			return errors.New("action_config is required for step: " + step.StepKey)
		}
	}
	if spec.Approvals != nil {
		if err := validateRequirement("task_start", spec.Approvals.TaskStart); err != nil {
			return err
		}
		if err := validateRequirement("task_cancel", spec.Approvals.TaskCancel); err != nil {
			return err
		}
		if err := validateRequirement("step_ack", spec.Approvals.StepAck); err != nil {
			return err
		}
		if err := validateRequirement("step_resolve", spec.Approvals.StepResolve); err != nil {
			return err
		}
		if err := validateRequirement("action_ack", spec.Approvals.ActionAck); err != nil {
			return err
		}
		if err := validateRequirement("action_resolve", spec.Approvals.ActionResolve); err != nil {
			return err
		}
	}
	return nil
}

func validateRequirement(name string, req *approval.Requirement) error {
	if req == nil || !req.Enabled {
		return nil
	}
	if len(req.Roles) == 0 {
		return errors.New("approvals." + name + ".roles is required when enabled")
	}
	if req.MinApprovals < 1 {
		return errors.New("approvals." + name + ".min_approvals must be >= 1")
	}
	switch req.AppliesTo {
	case "", approval.AppliesToBoth, approval.AppliesToHuman, approval.AppliesToAgent:
		return nil
	default:
		return errors.New("approvals." + name + ".applies_to invalid")
	}
}

// ResolveDependencies builds dependency map for steps using edges and depends_on.
func ResolveDependencies(spec *Spec) map[string][]string {
	deps := make(map[string][]string)
	for _, step := range spec.Steps {
		deps[step.StepKey] = append(deps[step.StepKey], step.DependsOn...)
	}
	for _, edge := range spec.Edges {
		deps[edge.To] = append(deps[edge.To], edge.From)
	}
	return deps
}
