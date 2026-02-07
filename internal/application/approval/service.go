package approval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	appAction "github.com/execution-hub/execution-hub/internal/application/action"
	appAudit "github.com/execution-hub/execution-hub/internal/application/audit"
	appTask "github.com/execution-hub/execution-hub/internal/application/task"
	domainApproval "github.com/execution-hub/execution-hub/internal/domain/approval"
	"github.com/execution-hub/execution-hub/internal/domain/audit"
	"github.com/execution-hub/execution-hub/internal/domain/notification"
	"github.com/execution-hub/execution-hub/internal/domain/task"
	"github.com/execution-hub/execution-hub/internal/domain/user"
	"github.com/execution-hub/execution-hub/internal/domain/workflow"
)

// Actor describes an authenticated actor.
type Actor struct {
	UserID   uuid.UUID
	Username string
	Role     user.Role
	Type     user.Type
}

func (a Actor) ActorString() string {
	prefix := "user"
	if a.Type == user.TypeAgent {
		prefix = "agent"
	}
	return prefix + ":" + a.Username
}

// Service handles approvals.
type Service struct {
	approvalRepo domainApproval.Repository
	userRepo     user.Repository
	taskRepo     task.Repository
	stepRepo     task.StepRepository
	workflowRepo workflow.Repository
	taskSvc      *appTask.Service
	actionSvc    *appAction.Service
	auditSvc     *appAudit.Service
	sseHub       notification.SSEHub
	logger       zerolog.Logger
}

// NewService creates an approval service.
func NewService(
	approvalRepo domainApproval.Repository,
	userRepo user.Repository,
	taskRepo task.Repository,
	stepRepo task.StepRepository,
	workflowRepo workflow.Repository,
	taskSvc *appTask.Service,
	actionSvc *appAction.Service,
	auditSvc *appAudit.Service,
	sseHub notification.SSEHub,
	logger zerolog.Logger,
) *Service {
	return &Service{
		approvalRepo: approvalRepo,
		userRepo:     userRepo,
		taskRepo:     taskRepo,
		stepRepo:     stepRepo,
		workflowRepo: workflowRepo,
		taskSvc:      taskSvc,
		actionSvc:    actionSvc,
		auditSvc:     auditSvc,
		sseHub:       sseHub,
		logger:       logger.With().Str("service", "approval").Logger(),
	}
}

// RequireTaskStart checks approval requirement and creates approval if needed.
func (s *Service) RequireTaskStart(ctx context.Context, taskID uuid.UUID, actor Actor) (*domainApproval.Approval, error) {
	t, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	req, err := s.requirementForWorkflow(ctx, t.WorkflowID, t.WorkflowVersion, func(a *workflow.Approvals) *domainApproval.Requirement {
		if a == nil {
			return nil
		}
		return a.TaskStart
	})
	if err != nil || !requiresApproval(req, actor.Type) {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"taskId": taskID.String(),
		"actor":  actor.ActorString(),
	})
	return s.createApproval(ctx, "TASK", taskID.String(), domainApproval.OpTaskStart, req, actor, payload)
}

// RequireTaskCancel checks approval requirement and creates approval if needed.
func (s *Service) RequireTaskCancel(ctx context.Context, taskID uuid.UUID, actor Actor) (*domainApproval.Approval, error) {
	t, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	req, err := s.requirementForWorkflow(ctx, t.WorkflowID, t.WorkflowVersion, func(a *workflow.Approvals) *domainApproval.Requirement {
		if a == nil {
			return nil
		}
		return a.TaskCancel
	})
	if err != nil || !requiresApproval(req, actor.Type) {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"taskId": taskID.String(),
		"actor":  actor.ActorString(),
	})
	return s.createApproval(ctx, "TASK", taskID.String(), domainApproval.OpTaskCancel, req, actor, payload)
}

// RequireStepAck checks approval requirement and creates approval if needed.
func (s *Service) RequireStepAck(ctx context.Context, stepID uuid.UUID, actor Actor, comment *string) (*domainApproval.Approval, error) {
	step, err := s.stepRepo.GetByID(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step not found: %s", stepID)
	}
	req, err := s.requirementForTask(ctx, step.TaskID, func(a *workflow.Approvals) *domainApproval.Requirement {
		if a == nil {
			return nil
		}
		return a.StepAck
	})
	if err != nil || !requiresApproval(req, actor.Type) {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"stepId":  stepID.String(),
		"actor":   actor.ActorString(),
		"comment": comment,
	})
	return s.createApproval(ctx, "STEP", stepID.String(), domainApproval.OpStepAck, req, actor, payload)
}

// RequireStepResolve checks approval requirement and creates approval if needed.
func (s *Service) RequireStepResolve(ctx context.Context, stepID uuid.UUID, actor Actor, evidence json.RawMessage) (*domainApproval.Approval, error) {
	step, err := s.stepRepo.GetByID(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step not found: %s", stepID)
	}
	req, err := s.requirementForTask(ctx, step.TaskID, func(a *workflow.Approvals) *domainApproval.Requirement {
		if a == nil {
			return nil
		}
		return a.StepResolve
	})
	if err != nil || !requiresApproval(req, actor.Type) {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"stepId":   stepID.String(),
		"actor":    actor.ActorString(),
		"evidence": json.RawMessage(evidence),
	})
	return s.createApproval(ctx, "STEP", stepID.String(), domainApproval.OpStepResolve, req, actor, payload)
}

// RequireActionAck checks approval requirement and creates approval if needed.
func (s *Service) RequireActionAck(ctx context.Context, actionID uuid.UUID, actor Actor) (*domainApproval.Approval, error) {
	step, err := s.stepRepo.GetByActionID(ctx, actionID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step not found for action: %s", actionID)
	}
	req, err := s.requirementForTask(ctx, step.TaskID, func(a *workflow.Approvals) *domainApproval.Requirement {
		if a == nil {
			return nil
		}
		return a.ActionAck
	})
	if err != nil || !requiresApproval(req, actor.Type) {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"actionId": actionID.String(),
		"actor":    actor.ActorString(),
	})
	return s.createApproval(ctx, "ACTION", actionID.String(), domainApproval.OpActionAck, req, actor, payload)
}

// RequireActionResolve checks approval requirement and creates approval if needed.
func (s *Service) RequireActionResolve(ctx context.Context, actionID uuid.UUID, actor Actor) (*domainApproval.Approval, error) {
	step, err := s.stepRepo.GetByActionID(ctx, actionID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step not found for action: %s", actionID)
	}
	req, err := s.requirementForTask(ctx, step.TaskID, func(a *workflow.Approvals) *domainApproval.Requirement {
		if a == nil {
			return nil
		}
		return a.ActionResolve
	})
	if err != nil || !requiresApproval(req, actor.Type) {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"actionId": actionID.String(),
		"actor":    actor.ActorString(),
	})
	return s.createApproval(ctx, "ACTION", actionID.String(), domainApproval.OpActionResolve, req, actor, payload)
}

// Decide records a decision and executes operation on approval.
func (s *Service) Decide(ctx context.Context, approvalID uuid.UUID, decision domainApproval.Decision, actor Actor, comment *string) (*domainApproval.Approval, error) {
	a, err := s.approvalRepo.GetByID(ctx, approvalID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("approval not found: %s", approvalID)
	}
	if a.Status != domainApproval.StatusPending {
		return a, nil
	}
	if !roleAllowed(actor.Role, a.RequiredRoles) {
		return nil, fmt.Errorf("role not allowed to approve")
	}
	decidedBy := actor.ActorString()
	exists, err := s.approvalRepo.HasDecision(ctx, approvalID, decidedBy)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("decision already recorded")
	}
	rec := &domainApproval.DecisionRecord{
		DecisionID:    uuid.New(),
		ApprovalID:    approvalID,
		Decision:      decision,
		DecidedBy:     decidedBy,
		DecidedByType: string(actor.Type),
		DecidedByRole: string(actor.Role),
		Comment:       comment,
		DecidedAt:     time.Now().UTC(),
	}
	if err := s.approvalRepo.CreateDecision(ctx, rec); err != nil {
		return nil, err
	}

	approvals, rejections, err := s.approvalRepo.CountDecisions(ctx, approvalID)
	if err != nil {
		return nil, err
	}
	a.ApprovalsCount = approvals
	a.RejectionsCount = rejections

	now := time.Now().UTC()
	if decision == domainApproval.DecisionReject {
		a.Status = domainApproval.StatusRejected
		a.DecidedAt = &now
		if err := s.approvalRepo.Update(ctx, a); err != nil {
			return nil, err
		}
		s.auditSvc.Log(ctx, &audit.AuditEntry{
			EntityType: audit.EntityTypeApproval,
			EntityID:   a.ApprovalID.String(),
			Action:     audit.ActionReject,
			Actor:      decidedBy,
			ActorRoles: []string{string(actor.Role)},
			Reason:     "approval rejected",
		})
		s.notifyApproval(a)
		return a, nil
	}

	if a.ApprovalsCount >= a.MinApprovals {
		a.Status = domainApproval.StatusApproved
		a.DecidedAt = &now
		if err := s.approvalRepo.Update(ctx, a); err != nil {
			return nil, err
		}
		if execErr := s.executeApproval(ctx, a); execErr != nil {
			errMsg := execErr.Error()
			a.Status = domainApproval.StatusFailed
			a.ExecutionError = &errMsg
			failAt := time.Now().UTC()
			a.ExecutedAt = &failAt
			_ = s.approvalRepo.Update(ctx, a)
		} else {
			execAt := time.Now().UTC()
			a.Status = domainApproval.StatusExecuted
			a.ExecutedAt = &execAt
			_ = s.approvalRepo.Update(ctx, a)
		}
		s.auditSvc.Log(ctx, &audit.AuditEntry{
			EntityType: audit.EntityTypeApproval,
			EntityID:   a.ApprovalID.String(),
			Action:     audit.ActionApprove,
			Actor:      decidedBy,
			ActorRoles: []string{string(actor.Role)},
			Reason:     "approval approved",
		})
		s.notifyApproval(a)
		return a, nil
	}

	if err := s.approvalRepo.Update(ctx, a); err != nil {
		return nil, err
	}
	s.notifyApproval(a)
	return a, nil
}

// List returns approvals.
func (s *Service) List(ctx context.Context, filter domainApproval.Filter, limit, offset int) ([]*domainApproval.Approval, error) {
	return s.approvalRepo.List(ctx, filter, limit, offset)
}

// Get retrieves approval by ID.
func (s *Service) Get(ctx context.Context, approvalID uuid.UUID) (*domainApproval.Approval, error) {
	return s.approvalRepo.GetByID(ctx, approvalID)
}

func (s *Service) requirementForTask(ctx context.Context, taskID uuid.UUID, picker func(*workflow.Approvals) *domainApproval.Requirement) (*domainApproval.Requirement, error) {
	t, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return s.requirementForWorkflow(ctx, t.WorkflowID, t.WorkflowVersion, picker)
}

func (s *Service) requirementForWorkflow(ctx context.Context, workflowID uuid.UUID, version int, picker func(*workflow.Approvals) *domainApproval.Requirement) (*domainApproval.Requirement, error) {
	def, err := s.workflowRepo.GetByIDAndVersion(ctx, workflowID, version)
	if err != nil {
		return nil, err
	}
	if def == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	spec, err := workflow.ParseSpec(def.Definition)
	if err != nil {
		return nil, err
	}
	if picker == nil {
		return nil, nil
	}
	req := picker(spec.Approvals)
	return normalizeRequirement(req), nil
}

func normalizeRequirement(req *domainApproval.Requirement) *domainApproval.Requirement {
	if req == nil {
		return nil
	}
	if req.AppliesTo == "" {
		req.AppliesTo = domainApproval.AppliesToBoth
	}
	if req.MinApprovals <= 0 {
		req.MinApprovals = 1
	}
	if len(req.Roles) == 0 {
		req.Roles = []string{string(user.RoleAdmin)}
	}
	for i, role := range req.Roles {
		req.Roles[i] = strings.ToUpper(role)
	}
	return req
}

func requiresApproval(req *domainApproval.Requirement, actorType user.Type) bool {
	if req == nil || !req.Enabled {
		return false
	}
	switch req.AppliesTo {
	case domainApproval.AppliesToHuman:
		return actorType == user.TypeHuman
	case domainApproval.AppliesToAgent:
		return actorType == user.TypeAgent
	default:
		return true
	}
}

func (s *Service) createApproval(ctx context.Context, entityType, entityID string, op domainApproval.Operation, req *domainApproval.Requirement, actor Actor, payload json.RawMessage) (*domainApproval.Approval, error) {
	a := &domainApproval.Approval{
		ApprovalID:      uuid.New(),
		EntityType:      entityType,
		EntityID:        entityID,
		Operation:       op,
		Status:          domainApproval.StatusPending,
		RequestPayload:  payload,
		RequestedBy:     actor.ActorString(),
		RequestedByType: string(actor.Type),
		RequiredRoles:   req.Roles,
		AppliesTo:       req.AppliesTo,
		MinApprovals:    req.MinApprovals,
		RequestedAt:     time.Now().UTC(),
	}
	if err := s.approvalRepo.Create(ctx, a); err != nil {
		return nil, err
	}

	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeApproval,
		EntityID:   a.ApprovalID.String(),
		Action:     audit.ActionCreate,
		Actor:      actor.ActorString(),
		ActorRoles: []string{string(actor.Role)},
		Reason:     "approval requested",
	})

	s.notifyApproval(a)
	return a, nil
}

func (s *Service) executeApproval(ctx context.Context, a *domainApproval.Approval) error {
	switch a.Operation {
	case domainApproval.OpTaskStart:
		var payload struct {
			TaskID string `json:"taskId"`
			Actor  string `json:"actor"`
		}
		if err := json.Unmarshal(a.RequestPayload, &payload); err != nil {
			return err
		}
		taskID, err := uuid.Parse(payload.TaskID)
		if err != nil {
			return err
		}
		return s.taskSvc.StartTask(ctx, taskID, payload.Actor)
	case domainApproval.OpTaskCancel:
		var payload struct {
			TaskID string `json:"taskId"`
			Actor  string `json:"actor"`
		}
		if err := json.Unmarshal(a.RequestPayload, &payload); err != nil {
			return err
		}
		taskID, err := uuid.Parse(payload.TaskID)
		if err != nil {
			return err
		}
		return s.taskSvc.CancelTask(ctx, taskID, payload.Actor)
	case domainApproval.OpStepAck:
		var payload struct {
			StepID  string  `json:"stepId"`
			Actor   string  `json:"actor"`
			Comment *string `json:"comment"`
		}
		if err := json.Unmarshal(a.RequestPayload, &payload); err != nil {
			return err
		}
		stepID, err := uuid.Parse(payload.StepID)
		if err != nil {
			return err
		}
		return s.taskSvc.AckStep(ctx, stepID, payload.Actor, payload.Comment)
	case domainApproval.OpStepResolve:
		var payload struct {
			StepID   string          `json:"stepId"`
			Actor    string          `json:"actor"`
			Evidence json.RawMessage `json:"evidence"`
		}
		if err := json.Unmarshal(a.RequestPayload, &payload); err != nil {
			return err
		}
		stepID, err := uuid.Parse(payload.StepID)
		if err != nil {
			return err
		}
		return s.taskSvc.ResolveStep(ctx, stepID, payload.Actor, payload.Evidence)
	case domainApproval.OpActionAck:
		var payload struct {
			ActionID string `json:"actionId"`
			Actor    string `json:"actor"`
		}
		if err := json.Unmarshal(a.RequestPayload, &payload); err != nil {
			return err
		}
		actionID, err := uuid.Parse(payload.ActionID)
		if err != nil {
			return err
		}
		return s.actionSvc.AcknowledgeAction(ctx, actionID, payload.Actor)
	case domainApproval.OpActionResolve:
		var payload struct {
			ActionID string `json:"actionId"`
			Actor    string `json:"actor"`
		}
		if err := json.Unmarshal(a.RequestPayload, &payload); err != nil {
			return err
		}
		actionID, err := uuid.Parse(payload.ActionID)
		if err != nil {
			return err
		}
		return s.actionSvc.ResolveAction(ctx, actionID, payload.Actor)
	default:
		return fmt.Errorf("unsupported approval operation: %s", a.Operation)
	}
}

func roleAllowed(role user.Role, required []string) bool {
	for _, r := range required {
		if strings.EqualFold(r, string(role)) {
			return true
		}
	}
	return false
}

func (s *Service) notifyApproval(a *domainApproval.Approval) {
	payload, err := json.Marshal(map[string]interface{}{
		"approvalId":      a.ApprovalID.String(),
		"status":          a.Status,
		"operation":       a.Operation,
		"entityType":      a.EntityType,
		"entityId":        a.EntityID,
		"requestedBy":     a.RequestedBy,
		"requestedAt":     a.RequestedAt.Format(time.RFC3339),
		"requiredRoles":   a.RequiredRoles,
		"minApprovals":    a.MinApprovals,
		"approvalsCount":  a.ApprovalsCount,
		"rejectionsCount": a.RejectionsCount,
	})
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to marshal approval notification")
		return
	}
	msg := notification.NewSSEMessage("approval", payload)

	// Notify all users with required roles.
	for _, role := range a.RequiredRoles {
		r := user.Role(strings.ToUpper(role))
		filter := user.Filter{Role: &r, Status: ptrStatus(user.StatusActive)}
		users, err := s.userRepo.List(context.Background(), filter, 200, 0)
		if err != nil {
			s.logger.Warn().Err(err).Msg("failed to list approvers")
			continue
		}
		for _, u := range users {
			s.sseHub.BroadcastToUser(u.Username, msg)
		}
	}
	// Also notify requester.
	s.sseHub.BroadcastToUser(actorFromRequestedBy(a.RequestedBy), msg)
}

func ptrStatus(status user.Status) *user.Status {
	return &status
}

func actorFromRequestedBy(actor string) string {
	parts := strings.SplitN(actor, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return actor
}
