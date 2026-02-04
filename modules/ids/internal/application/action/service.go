package action

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	domainAction "github.com/industrial-data-source/internal/domain/action"
	"github.com/industrial-data-source/internal/domain/rule"
)

// Service handles action operations
type Service struct {
	actionRepo domainAction.Repository
	ruleRepo   rule.Repository
	logger     zerolog.Logger
}

// NewService creates a new action service
func NewService(
	actionRepo domainAction.Repository,
	ruleRepo rule.Repository,
	logger zerolog.Logger,
) *Service {
	return &Service{
		actionRepo: actionRepo,
		ruleRepo:   ruleRepo,
		logger:     logger.With().Str("service", "action").Logger(),
	}
}

// CreateFromEvaluation creates an action from a rule evaluation
func (s *Service) CreateFromEvaluation(ctx context.Context, eval *rule.Evaluation, r *rule.Rule) (*domainAction.Action, error) {
	// Create the action
	action := domainAction.NewAction(
		eval.RuleID,
		eval.RuleVersion,
		eval.EvaluationID,
		domainAction.Type(r.ActionType),
		r.ActionConfig,
	)

	// Extract priority and dedupe settings from action config
	var actionCfg struct {
		Severity string `json:"severity"`
		Dedupe   *struct {
			Key             string `json:"key"`
			CooldownSeconds int    `json:"cooldownSeconds"`
		} `json:"dedupe,omitempty"`
	}
	if err := json.Unmarshal(r.ActionConfig, &actionCfg); err == nil {
		action.SetPriority(actionCfg.Severity)

		if actionCfg.Dedupe != nil && actionCfg.Dedupe.Key != "" {
			action.SetDedupeKey(actionCfg.Dedupe.Key)
		}
	}

	// Set trace ID from evaluation if available
	if eval.TraceID != nil {
		action.SetTraceID(*eval.TraceID)
	}

	// Save the action
	if err := s.actionRepo.Create(ctx, action); err != nil {
		return nil, fmt.Errorf("failed to create action: %w", err)
	}

	// Record the initial state transition
	transition := domainAction.NewStateTransition(
		action.ActionID,
		nil,
		domainAction.StatusCreated,
		strPtr("action created from evaluation"),
	)
	if err := s.actionRepo.RecordTransition(ctx, transition); err != nil {
		s.logger.Warn().Err(err).Msg("failed to record initial state transition")
	}

	s.logger.Info().
		Str("action_id", action.ActionID.String()).
		Str("rule_id", eval.RuleID.String()).
		Str("evaluation_id", eval.EvaluationID.String()).
		Str("action_type", string(action.ActionType)).
		Str("priority", string(action.Priority)).
		Msg("action created")

	return action, nil
}

// GetAction retrieves an action by ID
func (s *Service) GetAction(ctx context.Context, actionID uuid.UUID) (*domainAction.Action, error) {
	action, err := s.actionRepo.GetByID(ctx, actionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get action: %w", err)
	}
	if action == nil {
		return nil, fmt.Errorf("action not found: %s", actionID)
	}
	return action, nil
}

// ListActions lists actions with filters
func (s *Service) ListActions(ctx context.Context, filter domainAction.Filter, limit, offset int) ([]*domainAction.Action, error) {
	return s.actionRepo.List(ctx, filter, limit, offset)
}

// AcknowledgeAction acknowledges an action
func (s *Service) AcknowledgeAction(ctx context.Context, actionID uuid.UUID, by string) error {
	action, err := s.actionRepo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("failed to get action: %w", err)
	}
	if action == nil {
		return fmt.Errorf("action not found: %s", actionID)
	}

	fromStatus := action.Status
	if err := action.Acknowledge(by); err != nil {
		return fmt.Errorf("failed to acknowledge action: %w", err)
	}

	if err := s.actionRepo.Update(ctx, action); err != nil {
		return fmt.Errorf("failed to update action: %w", err)
	}

	// Record state transition
	transition := domainAction.NewStateTransition(
		actionID,
		&fromStatus,
		domainAction.StatusAcked,
		strPtr("acknowledged by "+by),
	)
	if err := s.actionRepo.RecordTransition(ctx, transition); err != nil {
		s.logger.Warn().Err(err).Msg("failed to record state transition")
	}

	s.logger.Info().
		Str("action_id", actionID.String()).
		Str("acked_by", by).
		Msg("action acknowledged")

	return nil
}

// ResolveAction resolves an action
func (s *Service) ResolveAction(ctx context.Context, actionID uuid.UUID, by string) error {
	action, err := s.actionRepo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("failed to get action: %w", err)
	}
	if action == nil {
		return fmt.Errorf("action not found: %s", actionID)
	}

	fromStatus := action.Status
	if err := action.Resolve(by); err != nil {
		return fmt.Errorf("failed to resolve action: %w", err)
	}

	if err := s.actionRepo.Update(ctx, action); err != nil {
		return fmt.Errorf("failed to update action: %w", err)
	}

	// Record state transition
	transition := domainAction.NewStateTransition(
		actionID,
		&fromStatus,
		domainAction.StatusResolved,
		strPtr("resolved by "+by),
	)
	if err := s.actionRepo.RecordTransition(ctx, transition); err != nil {
		s.logger.Warn().Err(err).Msg("failed to record state transition")
	}

	s.logger.Info().
		Str("action_id", actionID.String()).
		Str("resolved_by", by).
		Msg("action resolved")

	return nil
}

// GetActionEvidence retrieves the evidence for an action
func (s *Service) GetActionEvidence(ctx context.Context, actionID uuid.UUID) (*ActionEvidence, error) {
	action, err := s.actionRepo.GetByID(ctx, actionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get action: %w", err)
	}
	if action == nil {
		return nil, fmt.Errorf("action not found: %s", actionID)
	}

	eval, err := s.ruleRepo.GetEvaluationByID(ctx, action.EvaluationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get evaluation: %w", err)
	}

	r, err := s.ruleRepo.GetByRuleIDAndVersion(ctx, action.RuleID, action.RuleVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule: %w", err)
	}

	return &ActionEvidence{
		Action:     action,
		Evaluation: eval,
		Rule:       r,
	}, nil
}

// GetActionTransitions retrieves state transitions for an action
func (s *Service) GetActionTransitions(ctx context.Context, actionID uuid.UUID) ([]*domainAction.StateTransition, error) {
	return s.actionRepo.GetTransitions(ctx, actionID)
}

// DispatchAction dispatches an action (called by notification service)
func (s *Service) DispatchAction(ctx context.Context, actionID uuid.UUID) error {
	action, err := s.actionRepo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("failed to get action: %w", err)
	}
	if action == nil {
		return fmt.Errorf("action not found: %s", actionID)
	}

	fromStatus := action.Status
	if err := action.Dispatch(); err != nil {
		return fmt.Errorf("failed to dispatch action: %w", err)
	}

	if err := s.actionRepo.Update(ctx, action); err != nil {
		return fmt.Errorf("failed to update action: %w", err)
	}

	// Record state transition
	transition := domainAction.NewStateTransition(
		actionID,
		&fromStatus,
		domainAction.StatusDispatching,
		strPtr("dispatching action"),
	)
	s.actionRepo.RecordTransition(ctx, transition)

	return nil
}

// ConfirmDispatched confirms an action was dispatched
func (s *Service) ConfirmDispatched(ctx context.Context, actionID uuid.UUID) error {
	action, err := s.actionRepo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("failed to get action: %w", err)
	}
	if action == nil {
		return fmt.Errorf("action not found: %s", actionID)
	}

	fromStatus := action.Status
	if err := action.ConfirmDispatched(); err != nil {
		return fmt.Errorf("failed to confirm dispatched: %w", err)
	}

	if err := s.actionRepo.Update(ctx, action); err != nil {
		return fmt.Errorf("failed to update action: %w", err)
	}

	// Record state transition
	transition := domainAction.NewStateTransition(
		actionID,
		&fromStatus,
		domainAction.StatusDispatched,
		strPtr("action dispatched"),
	)
	s.actionRepo.RecordTransition(ctx, transition)

	return nil
}

// FailAction marks an action as failed
func (s *Service) FailAction(ctx context.Context, actionID uuid.UUID, errMsg string) error {
	action, err := s.actionRepo.GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("failed to get action: %w", err)
	}
	if action == nil {
		return fmt.Errorf("action not found: %s", actionID)
	}

	fromStatus := action.Status
	if err := action.Fail(errMsg); err != nil {
		return fmt.Errorf("failed to fail action: %w", err)
	}

	if err := s.actionRepo.Update(ctx, action); err != nil {
		return fmt.Errorf("failed to update action: %w", err)
	}

	// Record state transition
	transition := domainAction.NewStateTransition(
		actionID,
		&fromStatus,
		domainAction.StatusFailed,
		strPtr("action failed: "+errMsg),
	)
	s.actionRepo.RecordTransition(ctx, transition)

	s.logger.Warn().
		Str("action_id", actionID.String()).
		Str("error", errMsg).
		Int("retry_count", action.RetryCount).
		Msg("action failed")

	return nil
}

// ActionEvidence contains the full evidence chain for an action
type ActionEvidence struct {
	Action     *domainAction.Action
	Evaluation *rule.Evaluation
	Rule       *rule.Rule
}

func strPtr(s string) *string {
	return &s
}
