package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	appAction "github.com/execution-hub/execution-hub/internal/application/action"
	appNotification "github.com/execution-hub/execution-hub/internal/application/notification"
	domainAction "github.com/execution-hub/execution-hub/internal/domain/action"
	"github.com/execution-hub/execution-hub/internal/domain/notification"
	"github.com/execution-hub/execution-hub/internal/domain/task"
	"github.com/execution-hub/execution-hub/internal/domain/workflow"
)

// AgentRunner executes agent steps.
type AgentRunner interface {
	Run(ctx context.Context, t *task.Task, step *task.Step) (json.RawMessage, error)
}

// Orchestrator coordinates task execution.
type Orchestrator struct {
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	stepRepo     task.StepRepository
	actionSvc    *appAction.Service
	notifySvc    *appNotification.Service
	agentRunner  AgentRunner
	logger       zerolog.Logger
}

// NewOrchestrator creates a new orchestrator.
func NewOrchestrator(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	stepRepo task.StepRepository,
	actionSvc *appAction.Service,
	notifySvc *appNotification.Service,
	agentRunner AgentRunner,
	logger zerolog.Logger,
) *Orchestrator {
	return &Orchestrator{
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		stepRepo:     stepRepo,
		actionSvc:    actionSvc,
		notifySvc:    notifySvc,
		agentRunner:  agentRunner,
		logger:       logger.With().Str("service", "orchestrator").Logger(),
	}
}

// StartTask initializes steps (idempotent) and advances execution.
func (o *Orchestrator) StartTask(ctx context.Context, taskID uuid.UUID) error {
	t, err := o.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	spec, err := o.loadSpec(ctx, t)
	if err != nil {
		return err
	}

	steps, err := o.stepRepo.ListByTask(ctx, taskID)
	if err != nil {
		return err
	}
	if len(steps) == 0 {
		if err := o.createSteps(ctx, t, spec); err != nil {
			return err
		}
	}

	return o.AdvanceTask(ctx, taskID)
}

// AdvanceTask dispatches all ready steps.
func (o *Orchestrator) AdvanceTask(ctx context.Context, taskID uuid.UUID) error {
	t, err := o.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if t.Status != task.StatusRunning {
		return nil
	}

	spec, err := o.loadSpec(ctx, t)
	if err != nil {
		return err
	}

	steps, err := o.stepRepo.ListByTask(ctx, taskID)
	if err != nil {
		return err
	}

	stepByKey := make(map[string]*task.Step)
	for _, s := range steps {
		s := s
		stepByKey[s.StepKey] = s
	}

	deps := o.resolveDependencies(spec, t.Context)

	for _, s := range steps {
		if !o.isStepDispatchable(s) {
			continue
		}

		ready := true
		for _, depKey := range deps[s.StepKey] {
			depStep := stepByKey[depKey]
			if depStep == nil || depStep.Status != task.StepStatusResolved {
				ready = false
				break
			}
		}
		if !ready {
			continue
		}

		if err := o.dispatchStep(ctx, t, s); err != nil {
			o.logger.Warn().Err(err).
				Str("task_id", t.TaskID.String()).
				Str("step_id", s.StepID.String()).
				Msg("failed to dispatch step")
		}
	}

	// Check if all steps resolved
	allResolved := true
	for _, s := range steps {
		if s.Status != task.StepStatusResolved {
			allResolved = false
			break
		}
	}
	if allResolved {
		if err := t.Complete(); err == nil {
			t.UpdatedAt = time.Now().UTC()
			_ = o.taskRepo.Update(ctx, t)
		}
	}

	return nil
}

// ProcessTimeouts scans for timed-out steps and fails them.
func (o *Orchestrator) ProcessTimeouts(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	now := time.Now().UTC()
	steps, err := o.stepRepo.ListTimedOut(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, s := range steps {
		if err := o.failStep(ctx, s, "system", "step timeout"); err != nil {
			o.logger.Warn().Err(err).
				Str("step_id", s.StepID.String()).
				Msg("failed to mark step timeout")
			continue
		}
		processed++
	}
	return processed, nil
}

func (o *Orchestrator) loadSpec(ctx context.Context, t *task.Task) (*workflow.Spec, error) {
	def, err := o.workflowRepo.GetByIDAndVersion(ctx, t.WorkflowID, t.WorkflowVersion)
	if err != nil {
		return nil, err
	}
	if def == nil {
		return nil, fmt.Errorf("workflow not found: %s", t.WorkflowID)
	}
	spec, err := workflow.ParseSpec(def.Definition)
	if err != nil {
		return nil, err
	}
	return spec, nil
}

func (o *Orchestrator) createSteps(ctx context.Context, t *task.Task, spec *workflow.Spec) error {
	stepIDs := make(map[string]uuid.UUID)
	for _, s := range spec.Steps {
		stepIDs[s.StepKey] = deterministicStepID(t.TaskID, s.StepKey)
	}

	deps := o.resolveDependencies(spec, t.Context)

	for _, s := range spec.Steps {
		id := stepIDs[s.StepKey]
		depIDs := []uuid.UUID{}
		for _, depKey := range deps[s.StepKey] {
			if depID, ok := stepIDs[depKey]; ok {
				depIDs = append(depIDs, depID)
			}
		}

		step := &task.Step{
			StepID:         id,
			TaskID:         t.TaskID,
			TraceID:        t.TraceID,
			StepKey:        s.StepKey,
			Name:           s.Name,
			Status:         task.StepStatusCreated,
			ExecutorType:   s.ExecutorType,
			ExecutorRef:    s.ExecutorRef,
			ActionType:     s.ActionType,
			ActionConfig:   s.ActionConfig,
			TimeoutSeconds: s.TimeoutSeconds,
			MaxRetries:     s.MaxRetries,
			DependsOn:      depIDs,
			ActionID:       id,
			OnFail:         s.OnFail,
			CreatedAt:      time.Now().UTC(),
		}

		if err := o.stepRepo.Create(ctx, step); err != nil {
			return err
		}
	}

	return nil
}

func deterministicStepID(taskID uuid.UUID, stepKey string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(taskID.String()+":"+stepKey))
}

func (o *Orchestrator) resolveDependencies(spec *workflow.Spec, ctxData json.RawMessage) map[string][]string {
	deps := make(map[string][]string)
	for _, step := range spec.Steps {
		if len(step.DependsOn) > 0 {
			deps[step.StepKey] = append(deps[step.StepKey], step.DependsOn...)
		}
	}

	condMap := make(map[string]string)
	for _, c := range spec.Conditions {
		if c.Name != "" && c.Expression != "" {
			condMap[c.Name] = c.Expression
		}
	}

	for _, edge := range spec.Edges {
		cond := strings.TrimSpace(edge.Condition)
		if cond != "" {
			if expr, ok := condMap[cond]; ok {
				cond = expr
			}
			ok, err := EvaluateCondition(cond, ctxData)
			if err != nil {
				o.logger.Warn().Err(err).
					Str("condition", cond).
					Msg("condition evaluation failed; skipping edge")
				continue
			}
			if !ok {
				continue
			}
		}
		deps[edge.To] = append(deps[edge.To], edge.From)
	}

	return deps
}

func (o *Orchestrator) isStepDispatchable(step *task.Step) bool {
	if step.Status == task.StepStatusCreated {
		return true
	}
	if step.Status == task.StepStatusFailed && step.RetryCount < step.MaxRetries {
		return true
	}
	return false
}

func (o *Orchestrator) dispatchStep(ctx context.Context, t *task.Task, step *task.Step) error {
	// Reset for retry if needed
	if step.Status == task.StepStatusFailed && step.RetryCount < step.MaxRetries {
		if err := o.actionSvc.ResetForRetry(ctx, step.ActionID); err != nil {
			return err
		}
		step.Status = task.StepStatusCreated
		step.FailedAt = nil
		if err := o.stepRepo.Update(ctx, step); err != nil {
			return err
		}
	}

	// Ensure action exists
	action, err := o.actionSvc.GetAction(ctx, step.ActionID)
	if err != nil {
		if !isNotFoundError(err) {
			return err
		}
		action, err = o.actionSvc.CreateForStep(
			ctx,
			step.ActionID,
			step.ActionType,
			step.ActionConfig,
			&t.TraceID,
			step.MaxRetries,
			domainAction.PriorityMedium,
		)
		if err != nil {
			return err
		}
	}

	if err := o.actionSvc.DispatchAction(ctx, step.ActionID); err != nil {
		return err
	}
	if err := o.actionSvc.ConfirmDispatched(ctx, step.ActionID); err != nil {
		return err
	}

	now := time.Now().UTC()
	step.Status = task.StepStatusDispatched
	step.DispatchedAt = &now
	if err := o.stepRepo.Update(ctx, step); err != nil {
		return err
	}

	switch step.ActionType {
	case domainAction.TypeAgentRun:
		output, err := o.agentRunner.Run(ctx, t, step)
		if err != nil {
			return o.failStep(ctx, step, "agent", err.Error())
		}
		return o.resolveStep(ctx, step, "agent", output)
	case domainAction.TypeNotify, domainAction.TypeWebhook, domainAction.TypeEscalate:
		n, err := o.notifySvc.CreateFromAction(ctx, action)
		if err != nil {
			return err
		}
		if n.Status != notification.StatusPending {
			o.logger.Info().
				Str("notification_id", n.NotificationID.String()).
				Str("status", string(n.Status)).
				Msg("skip send for non-pending notification")
			return nil
		}
		_ = o.notifySvc.SendNotification(ctx, n.NotificationID)
		return nil
	default:
		return fmt.Errorf("unsupported action type: %s", step.ActionType)
	}
}

func (o *Orchestrator) resolveStep(ctx context.Context, step *task.Step, actor string, evidence json.RawMessage) error {
	if err := o.actionSvc.ResolveAction(ctx, step.ActionID, actor); err != nil {
		return err
	}
	now := time.Now().UTC()
	step.Status = task.StepStatusResolved
	step.ResolvedAt = &now
	step.Evidence = evidence
	if err := o.stepRepo.Update(ctx, step); err != nil {
		return err
	}
	return o.AdvanceTask(ctx, step.TaskID)
}

func (o *Orchestrator) failStep(ctx context.Context, step *task.Step, actor, reason string) error {
	if err := o.actionSvc.FailAction(ctx, step.ActionID, reason); err != nil {
		return err
	}
	now := time.Now().UTC()
	step.Status = task.StepStatusFailed
	step.FailedAt = &now
	step.RetryCount++
	step.Evidence = json.RawMessage(fmt.Sprintf("{\"actor\":\"%s\",\"reason\":\"%s\"}", actor, reason))
	if err := o.stepRepo.Update(ctx, step); err != nil {
		return err
	}
	o.handleOnFail(ctx, step)
	return o.AdvanceTask(ctx, step.TaskID)
}

type onFailConfig struct {
	Type    string `json:"type"`
	Target  string `json:"target,omitempty"` // user:alice | group:ops
	UserID  string `json:"user_id,omitempty"`
	Group   string `json:"group,omitempty"`
	Title   string `json:"title,omitempty"`
	Body    string `json:"body,omitempty"`
	Channel string `json:"channel,omitempty"`
}

func (o *Orchestrator) handleOnFail(ctx context.Context, step *task.Step) {
	if len(step.OnFail) == 0 {
		return
	}
	var cfg onFailConfig
	if err := json.Unmarshal(step.OnFail, &cfg); err != nil {
		o.logger.Warn().Err(err).Str("step_id", step.StepID.String()).Msg("invalid on_fail config")
		return
	}
	if strings.ToUpper(cfg.Type) != "ESCALATE" {
		return
	}

	var targetUser *string
	var targetGroup *string
	if cfg.UserID != "" {
		u := cfg.UserID
		targetUser = &u
	}
	if cfg.Group != "" {
		g := cfg.Group
		targetGroup = &g
	}
	if cfg.Target != "" {
		if strings.HasPrefix(cfg.Target, "user:") {
			u := strings.TrimPrefix(cfg.Target, "user:")
			targetUser = &u
		} else if strings.HasPrefix(cfg.Target, "group:") {
			g := strings.TrimPrefix(cfg.Target, "group:")
			targetGroup = &g
		}
	}

	channel := notification.Channel(cfg.Channel)
	if channel == "" {
		channel = notification.ChannelSSE
	}
	var tracePtr *string
	if step.TraceID != "" {
		tracePtr = &step.TraceID
	}

	n, err := o.notifySvc.CreateEscalation(ctx, step.ActionID, channel, cfg.Title, cfg.Body, targetUser, targetGroup, tracePtr)
	if err != nil {
		o.logger.Warn().Err(err).Str("step_id", step.StepID.String()).Msg("failed to create escalation notification")
		return
	}
	_ = o.notifySvc.SendNotification(ctx, n.NotificationID)
}

// HandleOnFail is exposed for external callers (e.g., manual fail via API).
func (o *Orchestrator) HandleOnFail(ctx context.Context, step *task.Step) {
	o.handleOnFail(ctx, step)
}

func isNotFoundError(err error) bool {
	return err != nil && (contains(err.Error(), "not found") || contains(err.Error(), "notfound"))
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (stringIndex(s, substr) >= 0)
}

func stringIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
