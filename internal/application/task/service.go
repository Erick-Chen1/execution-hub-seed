package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	appAction "github.com/execution-hub/execution-hub/internal/application/action"
	appAudit "github.com/execution-hub/execution-hub/internal/application/audit"
	"github.com/execution-hub/execution-hub/internal/application/orchestrator"
	domainAction "github.com/execution-hub/execution-hub/internal/domain/action"
	"github.com/execution-hub/execution-hub/internal/domain/audit"
	"github.com/execution-hub/execution-hub/internal/domain/task"
	"github.com/execution-hub/execution-hub/internal/domain/workflow"
)

// Service handles task operations.
type Service struct {
	taskRepo     task.Repository
	stepRepo     task.StepRepository
	workflowRepo workflow.Repository
	actionSvc    *appAction.Service
	auditSvc     *appAudit.Service
	orchestrator *orchestrator.Orchestrator
	logger       zerolog.Logger
}

// NewService creates a task service.
func NewService(
	taskRepo task.Repository,
	stepRepo task.StepRepository,
	workflowRepo workflow.Repository,
	actionSvc *appAction.Service,
	auditSvc *appAudit.Service,
	orchestrator *orchestrator.Orchestrator,
	logger zerolog.Logger,
) *Service {
	return &Service{
		taskRepo:     taskRepo,
		stepRepo:     stepRepo,
		workflowRepo: workflowRepo,
		actionSvc:    actionSvc,
		auditSvc:     auditSvc,
		orchestrator: orchestrator,
		logger:       logger.With().Str("service", "task").Logger(),
	}
}

// CreateTask creates a task instance.
func (s *Service) CreateTask(ctx context.Context, workflowID uuid.UUID, title string, contextData json.RawMessage, createdBy *string) (*task.Task, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if len(contextData) > 0 {
		var js json.RawMessage
		if err := json.Unmarshal(contextData, &js); err != nil {
			return nil, fmt.Errorf("context must be valid JSON")
		}
	}

	def, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if def == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	t := &task.Task{
		TaskID:          uuid.New(),
		WorkflowID:      workflowID,
		WorkflowVersion: def.Version,
		Title:           title,
		Status:          task.StatusDraft,
		Context:         contextData,
		TraceID:         uuid.New().String(),
		CreatedAt:       time.Now().UTC(),
		CreatedBy:       createdBy,
		UpdatedAt:       time.Now().UTC(),
		UpdatedBy:       createdBy,
	}

	if err := s.taskRepo.Create(ctx, t); err != nil {
		return nil, err
	}

	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   t.TaskID.String(),
		Action:     audit.ActionCreate,
		Actor:      actorOrSystem(createdBy),
		TraceID:    t.TraceID,
		Reason:     "task created",
	})

	return t, nil
}

// GetTask retrieves a task by ID.
func (s *Service) GetTask(ctx context.Context, taskID uuid.UUID) (*task.Task, error) {
	return s.taskRepo.GetByID(ctx, taskID)
}

// ListTasks lists tasks.
func (s *Service) ListTasks(ctx context.Context, status *task.Status, limit, offset int) ([]*task.Task, error) {
	return s.taskRepo.List(ctx, status, limit, offset)
}

// StartTask starts a task and invokes the orchestrator.
func (s *Service) StartTask(ctx context.Context, taskID uuid.UUID, actor string) error {
	t, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if err := t.Start(); err != nil {
		return err
	}
	t.UpdatedAt = time.Now().UTC()
	if err := s.taskRepo.Update(ctx, t); err != nil {
		return err
	}

	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   t.TaskID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actor,
		TraceID:    t.TraceID,
		Reason:     "task started",
	})

	return s.orchestrator.StartTask(ctx, taskID)
}

// CancelTask cancels a task.
func (s *Service) CancelTask(ctx context.Context, taskID uuid.UUID, actor string) error {
	t, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if err := t.Cancel(); err != nil {
		return err
	}
	t.UpdatedAt = time.Now().UTC()
	if err := s.taskRepo.Update(ctx, t); err != nil {
		return err
	}

	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   t.TaskID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actor,
		TraceID:    t.TraceID,
		Reason:     "task cancelled",
	})

	return nil
}

// CompleteTask marks a task as completed.
func (s *Service) CompleteTask(ctx context.Context, taskID uuid.UUID) error {
	t, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if err := t.Complete(); err != nil {
		return err
	}
	t.UpdatedAt = time.Now().UTC()
	return s.taskRepo.Update(ctx, t)
}

// ListSteps lists steps for a task.
func (s *Service) ListSteps(ctx context.Context, taskID uuid.UUID) ([]*task.Step, error) {
	return s.stepRepo.ListByTask(ctx, taskID)
}

// GetStep retrieves a step by ID.
func (s *Service) GetStep(ctx context.Context, stepID uuid.UUID) (*task.Step, error) {
	return s.stepRepo.GetByID(ctx, stepID)
}

// DispatchStep triggers orchestration advancement (manual nudge).
func (s *Service) DispatchStep(ctx context.Context, taskID uuid.UUID) error {
	return s.orchestrator.AdvanceTask(ctx, taskID)
}

// AckStep acknowledges a step action.
func (s *Service) AckStep(ctx context.Context, stepID uuid.UUID, actor string, comment *string) error {
	step, err := s.stepRepo.GetByID(ctx, stepID)
	if err != nil {
		return err
	}
	if step == nil {
		return fmt.Errorf("step not found: %s", stepID)
	}

	if err := s.actionSvc.AcknowledgeAction(ctx, step.ActionID, actor); err != nil {
		return err
	}

	now := time.Now().UTC()
	step.Status = task.StepStatusAcked
	step.AckedAt = &now
	if comment != nil {
		step.Evidence = json.RawMessage(fmt.Sprintf("{\"actor\":\"%s\",\"comment\":\"%s\"}", actor, *comment))
	}
	if err := s.stepRepo.Update(ctx, step); err != nil {
		return err
	}

	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeStep,
		EntityID:   step.StepID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actor,
		TraceID:    taskTrace(step.TaskID, s.taskRepo),
		Reason:     "step acknowledged",
	})

	return s.orchestrator.AdvanceTask(ctx, step.TaskID)
}

// ResolveStep resolves a step action.
func (s *Service) ResolveStep(ctx context.Context, stepID uuid.UUID, actor string, evidence json.RawMessage) error {
	step, err := s.stepRepo.GetByID(ctx, stepID)
	if err != nil {
		return err
	}
	if step == nil {
		return fmt.Errorf("step not found: %s", stepID)
	}

	if err := s.actionSvc.ResolveAction(ctx, step.ActionID, actor); err != nil {
		return err
	}

	now := time.Now().UTC()
	step.Status = task.StepStatusResolved
	step.ResolvedAt = &now
	if len(evidence) > 0 {
		step.Evidence = evidence
	}
	if err := s.stepRepo.Update(ctx, step); err != nil {
		return err
	}

	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeStep,
		EntityID:   step.StepID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actor,
		TraceID:    taskTrace(step.TaskID, s.taskRepo),
		Reason:     "step resolved",
	})

	return s.orchestrator.AdvanceTask(ctx, step.TaskID)
}

// FailStep marks a step as failed.
func (s *Service) FailStep(ctx context.Context, stepID uuid.UUID, actor string, reason string) error {
	step, err := s.stepRepo.GetByID(ctx, stepID)
	if err != nil {
		return err
	}
	if step == nil {
		return fmt.Errorf("step not found: %s", stepID)
	}

	if err := s.actionSvc.FailAction(ctx, step.ActionID, reason); err != nil {
		return err
	}

	now := time.Now().UTC()
	step.Status = task.StepStatusFailed
	step.FailedAt = &now
	step.RetryCount++
	step.Evidence = json.RawMessage(fmt.Sprintf("{\"actor\":\"%s\",\"reason\":\"%s\"}", actor, reason))
	if err := s.stepRepo.Update(ctx, step); err != nil {
		return err
	}

	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeStep,
		EntityID:   step.StepID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actor,
		TraceID:    taskTrace(step.TaskID, s.taskRepo),
		Reason:     "step failed",
	})

	if len(step.OnFail) > 0 {
		s.auditSvc.Log(ctx, &audit.AuditEntry{
			EntityType: audit.EntityTypeStep,
			EntityID:   step.StepID.String(),
			Action:     audit.ActionUpdate,
			Actor:      actor,
			TraceID:    taskTrace(step.TaskID, s.taskRepo),
			Reason:     "step escalation triggered",
		})
		s.orchestrator.HandleOnFail(ctx, step)
	}

	return s.orchestrator.AdvanceTask(ctx, step.TaskID)
}

func actorOrSystem(actor *string) string {
	if actor == nil || *actor == "" {
		return "system"
	}
	return *actor
}

func taskTrace(taskID uuid.UUID, repo task.Repository) string {
	ctx := context.Background()
	t, err := repo.GetByID(ctx, taskID)
	if err != nil || t == nil {
		return ""
	}
	return t.TraceID
}

// ActionEvidenceForStep returns action evidence for a step using action repository.
func (s *Service) ActionEvidenceForStep(ctx context.Context, stepID uuid.UUID) (*domainAction.Action, error) {
	step, err := s.stepRepo.GetByID(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step not found: %s", stepID)
	}
	return s.actionSvc.GetAction(ctx, step.ActionID)
}
