package collab

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	appAudit "github.com/execution-hub/execution-hub/internal/application/audit"
	"github.com/execution-hub/execution-hub/internal/domain/audit"
	"github.com/execution-hub/execution-hub/internal/domain/collab"
	"github.com/execution-hub/execution-hub/internal/domain/notification"
	"github.com/execution-hub/execution-hub/internal/domain/task"
	"github.com/execution-hub/execution-hub/internal/domain/workflow"
)

// Service manages leaderless collaboration sessions.
type Service struct {
	repo         collab.Repository
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	auditSvc     *appAudit.Service
	sseHub       notification.SSEHub
	logger       zerolog.Logger
}

// NewService creates a collaboration service.
func NewService(
	repo collab.Repository,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	auditSvc *appAudit.Service,
	sseHub notification.SSEHub,
	logger zerolog.Logger,
) *Service {
	return &Service{
		repo:         repo,
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		auditSvc:     auditSvc,
		sseHub:       sseHub,
		logger:       logger.With().Str("service", "collab").Logger(),
	}
}

// CreateSessionInput creates a new collaboration session.
type CreateSessionInput struct {
	WorkflowID uuid.UUID
	Title      string
	Context    json.RawMessage
	Actor      string
}

// CreateSession creates a running task + collaboration session and initializes steps.
func (s *Service) CreateSession(ctx context.Context, in CreateSessionInput) (*collab.Session, error) {
	if in.WorkflowID == uuid.Nil {
		return nil, fmt.Errorf("workflow_id is required")
	}
	if len(in.Context) > 0 {
		var raw json.RawMessage
		if err := json.Unmarshal(in.Context, &raw); err != nil {
			return nil, fmt.Errorf("context must be valid JSON")
		}
	}

	def, err := s.workflowRepo.GetByID(ctx, in.WorkflowID)
	if err != nil {
		return nil, err
	}
	if def == nil {
		return nil, fmt.Errorf("workflow not found: %s", in.WorkflowID)
	}

	spec, err := workflow.ParseSpec(def.Definition)
	if err != nil {
		return nil, err
	}
	if len(spec.Steps) == 0 {
		return nil, fmt.Errorf("workflow has no steps")
	}

	now := time.Now().UTC()
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = strings.TrimSpace(spec.Name)
	}
	if title == "" {
		title = "collaboration session"
	}

	t := &task.Task{
		TaskID:          uuid.New(),
		WorkflowID:      def.WorkflowID,
		WorkflowVersion: def.Version,
		Title:           title,
		Status:          task.StatusRunning,
		Context:         in.Context,
		TraceID:         uuid.NewString(),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.taskRepo.Create(ctx, t); err != nil {
		return nil, err
	}

	session := &collab.Session{
		SessionID:       uuid.New(),
		TaskID:          t.TaskID,
		WorkflowID:      def.WorkflowID,
		WorkflowVersion: def.Version,
		Name:            title,
		Status:          collab.SessionStatusActive,
		Context:         in.Context,
		TraceID:         t.TraceID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	deps := workflow.ResolveDependencies(spec)
	for _, ws := range spec.Steps {
		step := &collab.Step{
			StepID:               deterministicStepID(session.SessionID, ws.StepKey),
			SessionID:            session.SessionID,
			StepKey:              ws.StepKey,
			Name:                 ws.Name,
			Status:               collab.StepStatusOpen,
			RequiredCapabilities: extractRequiredCapabilities(ws),
			DependsOn:            deps[ws.StepKey],
			LeaseTTLSeconds:      defaultLeaseTTL(ws.TimeoutSeconds),
			ConsensusPolicy:      extractConsensusPolicy(ws),
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := s.repo.CreateStep(ctx, step); err != nil {
			return nil, err
		}
	}

	_ = s.logEvent(ctx, session.SessionID, nil, collab.EventTypeSessionCreated, actorOrSystem(in.Actor), map[string]interface{}{
		"taskId":     session.TaskID,
		"workflowId": session.WorkflowID,
		"title":      session.Name,
	})
	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   session.TaskID.String(),
		Action:     audit.ActionCreate,
		Actor:      actorOrSystem(in.Actor),
		TraceID:    session.TraceID,
		Reason:     "collaboration session created",
	})

	return session, nil
}

// JoinInput joins a participant into a session.
type JoinInput struct {
	SessionID    uuid.UUID
	Type         collab.ParticipantType
	Ref          string
	Capabilities []string
	TrustScore   int
	Actor        string
}

// JoinSession creates or reuses a participant in session.
func (s *Service) JoinSession(ctx context.Context, in JoinInput) (*collab.Participant, error) {
	if in.SessionID == uuid.Nil {
		return nil, fmt.Errorf("session_id is required")
	}
	ref := strings.TrimSpace(in.Ref)
	if ref == "" {
		return nil, fmt.Errorf("ref is required")
	}
	if in.Type != collab.ParticipantTypeHuman && in.Type != collab.ParticipantTypeAgent {
		return nil, fmt.Errorf("type must be HUMAN or AGENT")
	}

	session, err := s.repo.GetSessionByID(ctx, in.SessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", in.SessionID)
	}

	now := time.Now().UTC()
	existing, err := s.repo.GetParticipantByRef(ctx, in.SessionID, ref)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		_ = s.repo.TouchParticipant(ctx, existing.ParticipantID, now)
		existing.LastSeenAt = &now
		return existing, nil
	}

	p := &collab.Participant{
		ParticipantID: uuid.New(),
		SessionID:     in.SessionID,
		Type:          in.Type,
		Ref:           ref,
		Capabilities:  uniqNonEmpty(in.Capabilities),
		TrustScore:    in.TrustScore,
		JoinedAt:      now,
		LastSeenAt:    &now,
	}
	if err := s.repo.CreateParticipant(ctx, p); err != nil {
		return nil, err
	}

	_ = s.logEvent(ctx, in.SessionID, nil, collab.EventTypeParticipantJoin, actorOrSystem(in.Actor), map[string]interface{}{
		"participantId": p.ParticipantID,
		"type":          p.Type,
		"ref":           p.Ref,
	})
	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   session.TaskID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actorOrSystem(in.Actor),
		TraceID:    session.TraceID,
		Reason:     "participant joined session",
	})

	return p, nil
}

// OpenStepFilter filters open step listing.
type OpenStepFilter struct {
	SessionID     uuid.UUID
	ParticipantID *uuid.UUID
	Limit         int
	Offset        int
}

// ListOpenSteps lists all claimable steps after dependency and lease checks.
func (s *Service) ListOpenSteps(ctx context.Context, filter OpenStepFilter) ([]*collab.Step, error) {
	if filter.SessionID == uuid.Nil {
		return nil, fmt.Errorf("session_id is required")
	}

	session, err := s.repo.GetSessionByID(ctx, filter.SessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", filter.SessionID)
	}

	var participant *collab.Participant
	if filter.ParticipantID != nil {
		participant, err = s.repo.GetParticipantByID(ctx, *filter.ParticipantID)
		if err != nil {
			return nil, err
		}
		if participant == nil || participant.SessionID != filter.SessionID {
			return nil, fmt.Errorf("participant not found in session")
		}
	}

	steps, err := s.repo.ListStepsBySession(ctx, filter.SessionID)
	if err != nil {
		return nil, err
	}
	stepByKey := make(map[string]*collab.Step, len(steps))
	for _, st := range steps {
		stepByKey[st.StepKey] = st
	}

	now := time.Now().UTC()
	out := make([]*collab.Step, 0, len(steps))
	for _, st := range steps {
		if st.Status != collab.StepStatusOpen {
			continue
		}
		if !depsResolved(st, stepByKey) {
			continue
		}
		if participant != nil && !hasCapabilities(participant.Capabilities, st.RequiredCapabilities) {
			continue
		}
		active, err := s.repo.GetActiveClaimByStep(ctx, st.StepID, now)
		if err != nil {
			return nil, err
		}
		if active != nil {
			continue
		}
		out = append(out, st)
	}

	return pageSteps(out, filter.Limit, filter.Offset), nil
}

// ClaimInput claims an open step.
type ClaimInput struct {
	StepID        uuid.UUID
	ParticipantID uuid.UUID
	LeaseSeconds  int
	Actor         string
}

// ClaimStep creates an active lease for a step.
func (s *Service) ClaimStep(ctx context.Context, in ClaimInput) (*collab.Claim, error) {
	if in.StepID == uuid.Nil || in.ParticipantID == uuid.Nil {
		return nil, fmt.Errorf("step_id and participant_id are required")
	}

	step, participant, session, err := s.loadStepParticipantSession(ctx, in.StepID, in.ParticipantID)
	if err != nil {
		return nil, err
	}
	if step.Status != collab.StepStatusOpen {
		return nil, fmt.Errorf("step is not OPEN")
	}

	steps, err := s.repo.ListStepsBySession(ctx, step.SessionID)
	if err != nil {
		return nil, err
	}
	stepByKey := make(map[string]*collab.Step, len(steps))
	for _, st := range steps {
		stepByKey[st.StepKey] = st
	}
	if !depsResolved(step, stepByKey) {
		return nil, fmt.Errorf("step dependencies are not resolved")
	}
	if !hasCapabilities(participant.Capabilities, step.RequiredCapabilities) {
		return nil, fmt.Errorf("participant capabilities do not satisfy step requirements")
	}

	now := time.Now().UTC()
	existing, err := s.repo.GetActiveClaimByStep(ctx, step.StepID, now)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("step already claimed")
	}

	leaseSeconds := in.LeaseSeconds
	if leaseSeconds <= 0 {
		leaseSeconds = step.LeaseTTLSeconds
	}
	if leaseSeconds <= 0 {
		leaseSeconds = 900
	}

	claim := &collab.Claim{
		ClaimID:       uuid.New(),
		StepID:        step.StepID,
		ParticipantID: participant.ParticipantID,
		Status:        collab.ClaimStatusActive,
		LeaseUntil:    now.Add(time.Duration(leaseSeconds) * time.Second),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateClaim(ctx, claim); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("step already claimed")
		}
		return nil, err
	}

	step.Status = collab.StepStatusClaimed
	step.UpdatedAt = now
	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return nil, err
	}

	_ = s.logEvent(ctx, step.SessionID, &step.StepID, collab.EventTypeStepClaimed, actorOrSystem(in.Actor), map[string]interface{}{
		"claimId":       claim.ClaimID,
		"participantId": claim.ParticipantID,
		"leaseUntil":    claim.LeaseUntil,
	})
	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   session.TaskID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actorOrSystem(in.Actor),
		TraceID:    session.TraceID,
		Reason:     "step claimed",
	})

	return claim, nil
}

// ReleaseInput releases the active claim.
type ReleaseInput struct {
	StepID        uuid.UUID
	ParticipantID uuid.UUID
	Actor         string
}

// ReleaseStep releases active claim and re-opens the step.
func (s *Service) ReleaseStep(ctx context.Context, in ReleaseInput) error {
	if in.StepID == uuid.Nil || in.ParticipantID == uuid.Nil {
		return fmt.Errorf("step_id and participant_id are required")
	}

	step, _, session, err := s.loadStepParticipantSession(ctx, in.StepID, in.ParticipantID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	active, err := s.repo.GetActiveClaimByStepAndParticipant(ctx, in.StepID, in.ParticipantID, now)
	if err != nil {
		return err
	}
	if active == nil {
		return fmt.Errorf("active claim not found for participant")
	}

	if err := s.repo.UpdateClaimStatus(ctx, active.ClaimID, collab.ClaimStatusReleased, now); err != nil {
		return err
	}
	step.Status = collab.StepStatusOpen
	step.UpdatedAt = now
	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return err
	}

	_ = s.logEvent(ctx, step.SessionID, &step.StepID, collab.EventTypeStepReleased, actorOrSystem(in.Actor), map[string]interface{}{
		"claimId":       active.ClaimID,
		"participantId": in.ParticipantID,
	})
	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   session.TaskID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actorOrSystem(in.Actor),
		TraceID:    session.TraceID,
		Reason:     "step claim released",
	})
	return nil
}

// HandoffInput handoffs an active claim.
type HandoffInput struct {
	StepID            uuid.UUID
	FromParticipantID uuid.UUID
	ToParticipantID   uuid.UUID
	LeaseSeconds      int
	Actor             string
	Comment           *string
}

// HandoffStep transfers ownership from one participant to another.
func (s *Service) HandoffStep(ctx context.Context, in HandoffInput) (*collab.Claim, error) {
	if in.StepID == uuid.Nil || in.FromParticipantID == uuid.Nil || in.ToParticipantID == uuid.Nil {
		return nil, fmt.Errorf("step_id, from_participant_id and to_participant_id are required")
	}

	step, fromParticipant, session, err := s.loadStepParticipantSession(ctx, in.StepID, in.FromParticipantID)
	if err != nil {
		return nil, err
	}
	toParticipant, err := s.repo.GetParticipantByID(ctx, in.ToParticipantID)
	if err != nil {
		return nil, err
	}
	if toParticipant == nil || toParticipant.SessionID != step.SessionID {
		return nil, fmt.Errorf("target participant not found in session")
	}

	now := time.Now().UTC()
	active, err := s.repo.GetActiveClaimByStepAndParticipant(ctx, in.StepID, in.FromParticipantID, now)
	if err != nil {
		return nil, err
	}
	if active == nil {
		return nil, fmt.Errorf("source participant has no active claim")
	}

	if err := s.repo.UpdateClaimStatus(ctx, active.ClaimID, collab.ClaimStatusReleased, now); err != nil {
		return nil, err
	}

	leaseSeconds := in.LeaseSeconds
	if leaseSeconds <= 0 {
		leaseSeconds = step.LeaseTTLSeconds
	}
	if leaseSeconds <= 0 {
		leaseSeconds = 900
	}
	claim := &collab.Claim{
		ClaimID:       uuid.New(),
		StepID:        step.StepID,
		ParticipantID: toParticipant.ParticipantID,
		Status:        collab.ClaimStatusActive,
		LeaseUntil:    now.Add(time.Duration(leaseSeconds) * time.Second),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateClaim(ctx, claim); err != nil {
		return nil, err
	}

	step.Status = collab.StepStatusClaimed
	step.UpdatedAt = now
	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return nil, err
	}

	_ = s.logEvent(ctx, step.SessionID, &step.StepID, collab.EventTypeStepHandoff, actorOrSystem(in.Actor), map[string]interface{}{
		"fromParticipantId": fromParticipant.ParticipantID,
		"toParticipantId":   toParticipant.ParticipantID,
		"newClaimId":        claim.ClaimID,
		"comment":           in.Comment,
	})
	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   session.TaskID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actorOrSystem(in.Actor),
		TraceID:    session.TraceID,
		Reason:     "step handoff",
	})

	return claim, nil
}

// SubmitArtifactInput submits a step artifact.
type SubmitArtifactInput struct {
	StepID        uuid.UUID
	ParticipantID uuid.UUID
	Kind          string
	Content       json.RawMessage
	Actor         string
}

// SubmitArtifact stores artifact and marks step in review.
func (s *Service) SubmitArtifact(ctx context.Context, in SubmitArtifactInput) (*collab.Artifact, error) {
	if in.StepID == uuid.Nil || in.ParticipantID == uuid.Nil {
		return nil, fmt.Errorf("step_id and participant_id are required")
	}
	if len(in.Content) == 0 {
		return nil, fmt.Errorf("content is required")
	}
	var raw json.RawMessage
	if err := json.Unmarshal(in.Content, &raw); err != nil {
		return nil, fmt.Errorf("content must be valid JSON")
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "generic"
	}

	step, _, session, err := s.loadStepParticipantSession(ctx, in.StepID, in.ParticipantID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	active, err := s.repo.GetActiveClaimByStepAndParticipant(ctx, in.StepID, in.ParticipantID, now)
	if err != nil {
		return nil, err
	}
	if active == nil {
		return nil, fmt.Errorf("participant must hold active claim")
	}

	version, err := s.repo.GetLatestArtifactVersion(ctx, in.StepID)
	if err != nil {
		return nil, err
	}
	artifact := &collab.Artifact{
		ArtifactID: uuid.New(),
		StepID:     in.StepID,
		ProducerID: in.ParticipantID,
		Kind:       kind,
		Content:    in.Content,
		Version:    version + 1,
		CreatedAt:  now,
	}
	if err := s.repo.CreateArtifact(ctx, artifact); err != nil {
		return nil, err
	}

	step.Status = collab.StepStatusInReview
	step.UpdatedAt = now
	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return nil, err
	}

	_ = s.logEvent(ctx, step.SessionID, &step.StepID, collab.EventTypeArtifactSubmit, actorOrSystem(in.Actor), map[string]interface{}{
		"artifactId":    artifact.ArtifactID,
		"producerId":    artifact.ProducerID,
		"kind":          artifact.Kind,
		"version":       artifact.Version,
		"activeClaimId": active.ClaimID,
	})
	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   session.TaskID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actorOrSystem(in.Actor),
		TraceID:    session.TraceID,
		Reason:     "artifact submitted",
	})

	return artifact, nil
}

// OpenDecisionInput starts a decision process for a step.
type OpenDecisionInput struct {
	StepID   uuid.UUID
	Policy   json.RawMessage
	Deadline *time.Time
	Actor    string
}

// OpenDecision creates a decision in pending state.
func (s *Service) OpenDecision(ctx context.Context, in OpenDecisionInput) (*collab.Decision, error) {
	if in.StepID == uuid.Nil {
		return nil, fmt.Errorf("step_id is required")
	}
	step, err := s.repo.GetStepByID(ctx, in.StepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step not found: %s", in.StepID)
	}
	if step.Status != collab.StepStatusInReview && step.Status != collab.StepStatusClaimed {
		return nil, fmt.Errorf("step must be CLAIMED or IN_REVIEW")
	}

	existing, err := s.repo.GetLatestDecisionByStep(ctx, in.StepID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.Status == collab.DecisionStatusPending {
		return nil, fmt.Errorf("pending decision already exists")
	}

	policy := in.Policy
	if len(policy) == 0 {
		policy = defaultDecisionPolicy()
	} else {
		var raw json.RawMessage
		if err := json.Unmarshal(policy, &raw); err != nil {
			return nil, fmt.Errorf("policy must be valid JSON")
		}
	}

	now := time.Now().UTC()
	decision := &collab.Decision{
		DecisionID: uuid.New(),
		StepID:     step.StepID,
		Policy:     policy,
		Deadline:   in.Deadline,
		Status:     collab.DecisionStatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.CreateDecision(ctx, decision); err != nil {
		return nil, err
	}

	_ = s.logEvent(ctx, step.SessionID, &step.StepID, collab.EventTypeDecisionOpen, actorOrSystem(in.Actor), map[string]interface{}{
		"decisionId": decision.DecisionID,
		"deadline":   decision.Deadline,
		"policy":     json.RawMessage(policy),
	})

	return decision, nil
}

// VoteInput casts a vote on a decision.
type VoteInput struct {
	DecisionID    uuid.UUID
	ParticipantID uuid.UUID
	Choice        collab.VoteChoice
	Comment       *string
	Actor         string
}

// CastVote saves a vote and evaluates decision policy.
func (s *Service) CastVote(ctx context.Context, in VoteInput) (*collab.Decision, error) {
	if in.DecisionID == uuid.Nil || in.ParticipantID == uuid.Nil {
		return nil, fmt.Errorf("decision_id and participant_id are required")
	}
	if in.Choice != collab.VoteChoiceApprove && in.Choice != collab.VoteChoiceReject {
		return nil, fmt.Errorf("choice must be APPROVE or REJECT")
	}

	decision, err := s.repo.GetDecisionByID(ctx, in.DecisionID)
	if err != nil {
		return nil, err
	}
	if decision == nil {
		return nil, fmt.Errorf("decision not found: %s", in.DecisionID)
	}
	if decision.Status != collab.DecisionStatusPending {
		return nil, fmt.Errorf("decision is not pending")
	}
	if decision.Deadline != nil && time.Now().UTC().After(*decision.Deadline) {
		return nil, fmt.Errorf("decision already expired")
	}

	step, err := s.repo.GetStepByID(ctx, decision.StepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step not found: %s", decision.StepID)
	}

	participant, err := s.repo.GetParticipantByID(ctx, in.ParticipantID)
	if err != nil {
		return nil, err
	}
	if participant == nil || participant.SessionID != step.SessionID {
		return nil, fmt.Errorf("participant not found in decision session")
	}

	now := time.Now().UTC()
	vote := &collab.Vote{
		VoteID:        uuid.New(),
		DecisionID:    decision.DecisionID,
		ParticipantID: participant.ParticipantID,
		Choice:        in.Choice,
		Comment:       in.Comment,
		CreatedAt:     now,
	}
	if err := s.repo.CreateVote(ctx, vote); err != nil {
		return nil, err
	}

	votes, err := s.repo.ListVotesByDecision(ctx, decision.DecisionID)
	if err != nil {
		return nil, err
	}
	status, result := evaluateDecision(decision.Policy, votes)
	if status != collab.DecisionStatusPending {
		decision.Status = status
		decision.Result = &result
		decision.DecidedAt = &now
	}
	decision.UpdatedAt = now
	if err := s.repo.UpdateDecision(ctx, decision); err != nil {
		return nil, err
	}

	_ = s.logEvent(ctx, step.SessionID, &step.StepID, collab.EventTypeVoteCast, actorOrSystem(in.Actor), map[string]interface{}{
		"decisionId":    decision.DecisionID,
		"participantId": participant.ParticipantID,
		"choice":        in.Choice,
		"decisionState": decision.Status,
	})

	return decision, nil
}

// ResolveStepInput resolves one step.
type ResolveStepInput struct {
	StepID        uuid.UUID
	ParticipantID *uuid.UUID
	Actor         string
}

// ResolveStep resolves a step after policy checks.
func (s *Service) ResolveStep(ctx context.Context, in ResolveStepInput) (*collab.Step, error) {
	if in.StepID == uuid.Nil {
		return nil, fmt.Errorf("step_id is required")
	}
	step, err := s.repo.GetStepByID(ctx, in.StepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("step not found: %s", in.StepID)
	}
	if step.Status != collab.StepStatusInReview && step.Status != collab.StepStatusClaimed {
		return nil, fmt.Errorf("step must be CLAIMED or IN_REVIEW")
	}

	decision, err := s.repo.GetLatestDecisionByStep(ctx, step.StepID)
	if err != nil {
		return nil, err
	}
	if decision != nil {
		if decision.Status == collab.DecisionStatusPending {
			return nil, fmt.Errorf("pending decision must complete first")
		}
		if decision.Status == collab.DecisionStatusRejected {
			return nil, fmt.Errorf("decision rejected; cannot resolve step")
		}
	}

	now := time.Now().UTC()
	if in.ParticipantID != nil {
		activeByActor, err := s.repo.GetActiveClaimByStepAndParticipant(ctx, step.StepID, *in.ParticipantID, now)
		if err != nil {
			return nil, err
		}
		if activeByActor == nil {
			return nil, fmt.Errorf("participant does not hold active claim")
		}
	}
	active, err := s.repo.GetActiveClaimByStep(ctx, step.StepID, now)
	if err != nil {
		return nil, err
	}
	if active != nil {
		_ = s.repo.UpdateClaimStatus(ctx, active.ClaimID, collab.ClaimStatusReleased, now)
	}

	step.Status = collab.StepStatusResolved
	step.ResolvedAt = &now
	step.UpdatedAt = now
	if err := s.repo.UpdateStep(ctx, step); err != nil {
		return nil, err
	}

	session, err := s.repo.GetSessionByID(ctx, step.SessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", step.SessionID)
	}

	_ = s.logEvent(ctx, step.SessionID, &step.StepID, collab.EventTypeStepResolved, actorOrSystem(in.Actor), map[string]interface{}{
		"stepId": step.StepID,
	})
	s.auditSvc.Log(ctx, &audit.AuditEntry{
		EntityType: audit.EntityTypeTask,
		EntityID:   session.TaskID.String(),
		Action:     audit.ActionUpdate,
		Actor:      actorOrSystem(in.Actor),
		TraceID:    session.TraceID,
		Reason:     "step resolved",
	})

	remaining, err := s.repo.CountUnresolvedSteps(ctx, step.SessionID)
	if err == nil && remaining == 0 {
		_ = s.repo.UpdateSessionStatus(ctx, step.SessionID, collab.SessionStatusCompleted, now)
		_ = s.taskRepo.UpdateStatus(ctx, session.TaskID, task.StatusCompleted)
		_ = s.logEvent(ctx, step.SessionID, nil, collab.EventTypeSessionCompleted, actorOrSystem(in.Actor), map[string]interface{}{
			"sessionId": step.SessionID,
			"taskId":    session.TaskID,
		})
	}

	return step, nil
}

// GetSession returns one session.
func (s *Service) GetSession(ctx context.Context, sessionID uuid.UUID) (*collab.Session, error) {
	return s.repo.GetSessionByID(ctx, sessionID)
}

// GetStep returns one step.
func (s *Service) GetStep(ctx context.Context, stepID uuid.UUID) (*collab.Step, error) {
	return s.repo.GetStepByID(ctx, stepID)
}

// GetParticipant returns one participant.
func (s *Service) GetParticipant(ctx context.Context, participantID uuid.UUID) (*collab.Participant, error) {
	if participantID == uuid.Nil {
		return nil, fmt.Errorf("participant_id is required")
	}
	return s.repo.GetParticipantByID(ctx, participantID)
}

// GetParticipantByRef returns participant by stable ref in session.
func (s *Service) GetParticipantByRef(ctx context.Context, sessionID uuid.UUID, ref string) (*collab.Participant, error) {
	if sessionID == uuid.Nil {
		return nil, fmt.Errorf("session_id is required")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("ref is required")
	}
	return s.repo.GetParticipantByRef(ctx, sessionID, ref)
}

// ListEvents lists session timeline events.
func (s *Service) ListEvents(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]*collab.Event, error) {
	if sessionID == uuid.Nil {
		return nil, fmt.Errorf("session_id is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListEvents(ctx, sessionID, limit, offset)
}

// ListArtifacts returns artifacts for a step.
func (s *Service) ListArtifacts(ctx context.Context, stepID uuid.UUID) ([]*collab.Artifact, error) {
	if stepID == uuid.Nil {
		return nil, fmt.Errorf("step_id is required")
	}
	return s.repo.ListArtifactsByStep(ctx, stepID)
}

// ProcessExpiredClaims expires stale leases and reopens claimed steps.
func (s *Service) ProcessExpiredClaims(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	now := time.Now().UTC()
	claims, err := s.repo.ListExpiredActiveClaims(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, c := range claims {
		if err := s.repo.UpdateClaimStatus(ctx, c.ClaimID, collab.ClaimStatusExpired, now); err != nil {
			s.logger.Warn().Err(err).Str("claim_id", c.ClaimID.String()).Msg("failed to expire claim")
			continue
		}
		step, err := s.repo.GetStepByID(ctx, c.StepID)
		if err != nil || step == nil {
			continue
		}
		step.Status = collab.StepStatusOpen
		step.UpdatedAt = now
		_ = s.repo.UpdateStep(ctx, step)
		_ = s.logEvent(ctx, step.SessionID, &step.StepID, collab.EventTypeClaimExpired, "system", map[string]interface{}{
			"claimId":       c.ClaimID,
			"participantId": c.ParticipantID,
		})
		processed++
	}
	return processed, nil
}

func (s *Service) loadStepParticipantSession(ctx context.Context, stepID, participantID uuid.UUID) (*collab.Step, *collab.Participant, *collab.Session, error) {
	step, err := s.repo.GetStepByID(ctx, stepID)
	if err != nil {
		return nil, nil, nil, err
	}
	if step == nil {
		return nil, nil, nil, fmt.Errorf("step not found: %s", stepID)
	}
	participant, err := s.repo.GetParticipantByID(ctx, participantID)
	if err != nil {
		return nil, nil, nil, err
	}
	if participant == nil {
		return nil, nil, nil, fmt.Errorf("participant not found: %s", participantID)
	}
	if participant.SessionID != step.SessionID {
		return nil, nil, nil, fmt.Errorf("participant does not belong to step session")
	}
	session, err := s.repo.GetSessionByID(ctx, step.SessionID)
	if err != nil {
		return nil, nil, nil, err
	}
	if session == nil {
		return nil, nil, nil, fmt.Errorf("session not found: %s", step.SessionID)
	}
	return step, participant, session, nil
}

func (s *Service) logEvent(ctx context.Context, sessionID uuid.UUID, stepID *uuid.UUID, eventType collab.EventType, actor string, payload interface{}) error {
	var payloadRaw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		payloadRaw = b
	}

	event := &collab.Event{
		EventID:   uuid.New(),
		SessionID: sessionID,
		StepID:    stepID,
		Type:      eventType,
		Actor:     actorOrSystem(actor),
		Payload:   payloadRaw,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return err
	}

	stream := map[string]interface{}{
		"eventId":   event.EventID,
		"sessionId": event.SessionID,
		"stepId":    event.StepID,
		"type":      event.Type,
		"actor":     event.Actor,
		"payload":   json.RawMessage(event.Payload),
		"createdAt": event.CreatedAt,
	}
	data, err := json.Marshal(stream)
	if err == nil && s.sseHub != nil {
		s.sseHub.BroadcastToAll(notification.NewSSEMessage("collab", data))
	}
	return nil
}

func actorOrSystem(actor string) string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return "system"
	}
	return actor
}

func deterministicStepID(sessionID uuid.UUID, stepKey string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(sessionID.String()+":"+stepKey))
}

func defaultLeaseTTL(timeout int) int {
	if timeout > 0 {
		return timeout
	}
	return 900
}

func extractRequiredCapabilities(step workflow.Step) []string {
	out := make([]string, 0)
	if strings.HasPrefix(step.ExecutorRef, "capability:") {
		out = append(out, strings.TrimSpace(strings.TrimPrefix(step.ExecutorRef, "capability:")))
	}
	var cfg struct {
		RequiredCapabilities []string `json:"required_capabilities"`
		Capabilities         []string `json:"capabilities"`
	}
	if len(step.ActionConfig) > 0 && json.Unmarshal(step.ActionConfig, &cfg) == nil {
		out = append(out, cfg.RequiredCapabilities...)
		out = append(out, cfg.Capabilities...)
	}
	return uniqNonEmpty(out)
}

func extractConsensusPolicy(step workflow.Step) json.RawMessage {
	var cfg struct {
		ConsensusPolicy json.RawMessage `json:"consensus_policy"`
		MinApprovals    int             `json:"min_approvals"`
		Quorum          int             `json:"quorum"`
	}
	if len(step.ActionConfig) > 0 && json.Unmarshal(step.ActionConfig, &cfg) == nil {
		if len(cfg.ConsensusPolicy) > 0 {
			return cfg.ConsensusPolicy
		}
		if cfg.MinApprovals > 0 || cfg.Quorum > 0 {
			policy, _ := json.Marshal(map[string]int{
				"min_approvals": maxInt(1, cfg.MinApprovals),
				"quorum":        cfg.Quorum,
			})
			return policy
		}
	}
	return nil
}

func depsResolved(step *collab.Step, byKey map[string]*collab.Step) bool {
	if len(step.DependsOn) == 0 {
		return true
	}
	for _, key := range step.DependsOn {
		dep := byKey[key]
		if dep == nil || dep.Status != collab.StepStatusResolved {
			return false
		}
	}
	return true
}

func hasCapabilities(actual, required []string) bool {
	if len(required) == 0 {
		return true
	}
	if len(actual) == 0 {
		return false
	}
	for _, req := range required {
		if !slices.Contains(actual, req) {
			return false
		}
	}
	return true
}

func pageSteps(in []*collab.Step, limit, offset int) []*collab.Step {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 100
	}
	if offset >= len(in) {
		return []*collab.Step{}
	}
	end := offset + limit
	if end > len(in) {
		end = len(in)
	}
	return in[offset:end]
}

type decisionPolicy struct {
	MinApprovals    int `json:"min_approvals"`
	Quorum          int `json:"quorum"`
	RejectThreshold int `json:"reject_threshold"`
}

func defaultDecisionPolicy() json.RawMessage {
	b, _ := json.Marshal(decisionPolicy{
		MinApprovals: 1,
	})
	return b
}

func evaluateDecision(policyRaw json.RawMessage, votes []*collab.Vote) (collab.DecisionStatus, string) {
	p := decisionPolicy{MinApprovals: 1}
	if len(policyRaw) > 0 {
		_ = json.Unmarshal(policyRaw, &p)
	}
	if p.MinApprovals <= 0 {
		p.MinApprovals = 1
	}

	approves := 0
	rejects := 0
	for _, v := range votes {
		switch v.Choice {
		case collab.VoteChoiceApprove:
			approves++
		case collab.VoteChoiceReject:
			rejects++
		}
	}

	if p.RejectThreshold > 0 && rejects >= p.RejectThreshold {
		return collab.DecisionStatusRejected, "reject threshold reached"
	}
	if approves >= p.MinApprovals {
		if p.Quorum <= 0 || len(votes) >= p.Quorum {
			return collab.DecisionStatusPassed, "approval threshold reached"
		}
	}
	return collab.DecisionStatusPending, ""
}

func uniqNonEmpty(in []string) []string {
	set := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := set[v]; ok {
			continue
		}
		set[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func maxInt(a, b int) int {
	if a >= b {
		return a
	}
	return b
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
