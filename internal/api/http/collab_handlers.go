package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	appCollab "github.com/execution-hub/execution-hub/internal/application/collab"
	"github.com/execution-hub/execution-hub/internal/domain/collab"
	"github.com/execution-hub/execution-hub/internal/domain/notification"
	domainUser "github.com/execution-hub/execution-hub/internal/domain/user"
)

type createCollabSessionRequest struct {
	WorkflowID string          `json:"workflow_id"`
	Title      string          `json:"title"`
	Context    json.RawMessage `json:"context,omitempty"`
}

func (s *Server) createCollabSession(w http.ResponseWriter, r *http.Request) {
	var req createCollabSessionRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	workflowID, err := uuid.Parse(req.WorkflowID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid workflow_id")
		return
	}

	session, err := s.collabSvc.CreateSession(contextFromRequest(r), appCollab.CreateSessionInput{
		WorkflowID: workflowID,
		Title:      req.Title,
		Context:    req.Context,
		Actor:      s.actorFromRequest(r),
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, session)
}

func (s *Server) getCollabSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseUUIDParam(r, "sessionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid sessionId")
		return
	}
	session, err := s.collabSvc.GetSession(contextFromRequest(r), sessionID)
	if err != nil || session == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "session not found")
		return
	}
	if err := s.ensureSessionAccess(r, sessionID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, session)
}

type joinCollabSessionRequest struct {
	Type         string   `json:"type"`
	Ref          string   `json:"ref,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	TrustScore   int      `json:"trust_score,omitempty"`
}

func (s *Server) joinCollabSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseUUIDParam(r, "sessionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid sessionId")
		return
	}
	var req joinCollabSessionRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}

	user := authUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth")
		return
	}

	ptype, pref, err := participantFromRequest(user, req.Type, req.Ref)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}

	participant, err := s.collabSvc.JoinSession(contextFromRequest(r), appCollab.JoinInput{
		SessionID:    sessionID,
		Type:         ptype,
		Ref:          pref,
		Capabilities: req.Capabilities,
		TrustScore:   req.TrustScore,
		Actor:        s.actorFromRequest(r),
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, participant)
}

func participantFromRequest(user *AuthUser, reqType, reqRef string) (collab.ParticipantType, string, error) {
	isAdmin := user.Role == domainUser.RoleAdmin
	if !isAdmin {
		if user.Type == domainUser.TypeAgent {
			return collab.ParticipantTypeAgent, "agent:" + user.Username, nil
		}
		return collab.ParticipantTypeHuman, "user:" + user.Username, nil
	}

	t := strings.ToUpper(strings.TrimSpace(reqType))
	ref := strings.TrimSpace(reqRef)
	if t == "" {
		t = string(collab.ParticipantTypeHuman)
	}
	if ref == "" {
		ref = "user:" + user.Username
	}
	switch t {
	case string(collab.ParticipantTypeHuman):
		return collab.ParticipantTypeHuman, ref, nil
	case string(collab.ParticipantTypeAgent):
		return collab.ParticipantTypeAgent, ref, nil
	default:
		return "", "", errBadParticipantType
	}
}

var errBadParticipantType = &apiError{message: "type must be HUMAN or AGENT"}

type apiError struct {
	message string
}

func (e *apiError) Error() string { return e.message }

func (s *Server) listCollabParticipants(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseUUIDParam(r, "sessionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid sessionId")
		return
	}
	if err := s.ensureSessionAccess(r, sessionID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	limit, offset := parseLimitOffset(r, 100, 500)
	participants, err := s.collabSvc.ListParticipants(contextFromRequest(r), sessionID, limit, offset)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"session_id":   sessionID,
		"participants": participants,
	})
}

func (s *Server) listCollabOpenSteps(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseUUIDParam(r, "sessionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid sessionId")
		return
	}
	if err := s.ensureSessionAccess(r, sessionID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	var participantID *uuid.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("participant_id")); raw != "" {
		pid, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid participant_id")
			return
		}
		if _, err := s.ensureParticipantOwnership(r, pid); err != nil {
			respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		participantID = &pid
	}
	limit, offset := parseLimitOffset(r, 100, 500)

	steps, err := s.collabSvc.ListOpenSteps(contextFromRequest(r), appCollab.OpenStepFilter{
		SessionID:     sessionID,
		ParticipantID: participantID,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": sessionID,
		"steps":      steps,
	})
}

type claimCollabStepRequest struct {
	ParticipantID string `json:"participant_id"`
	LeaseSeconds  int    `json:"lease_seconds,omitempty"`
}

func (s *Server) claimCollabStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	var req claimCollabStepRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	participantID, err := uuid.Parse(req.ParticipantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid participant_id")
		return
	}
	if _, err := s.ensureParticipantOwnership(r, participantID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}

	claim, err := s.collabSvc.ClaimStep(contextFromRequest(r), appCollab.ClaimInput{
		StepID:        stepID,
		ParticipantID: participantID,
		LeaseSeconds:  req.LeaseSeconds,
		Actor:         s.actorFromRequest(r),
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, claim)
}

type releaseCollabStepRequest struct {
	ParticipantID string `json:"participant_id"`
}

func (s *Server) releaseCollabStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	var req releaseCollabStepRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	participantID, err := uuid.Parse(req.ParticipantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid participant_id")
		return
	}
	if _, err := s.ensureParticipantOwnership(r, participantID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}

	if err := s.collabSvc.ReleaseStep(contextFromRequest(r), appCollab.ReleaseInput{
		StepID:        stepID,
		ParticipantID: participantID,
		Actor:         s.actorFromRequest(r),
	}); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"step_id": stepID, "status": "OPEN"})
}

type handoffCollabStepRequest struct {
	FromParticipantID string  `json:"from_participant_id"`
	ToParticipantID   string  `json:"to_participant_id"`
	LeaseSeconds      int     `json:"lease_seconds,omitempty"`
	Comment           *string `json:"comment,omitempty"`
}

func (s *Server) handoffCollabStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	var req handoffCollabStepRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}

	fromID, err := uuid.Parse(req.FromParticipantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid from_participant_id")
		return
	}
	if _, err := s.ensureParticipantOwnership(r, fromID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	toID, err := uuid.Parse(req.ToParticipantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid to_participant_id")
		return
	}

	claim, err := s.collabSvc.HandoffStep(contextFromRequest(r), appCollab.HandoffInput{
		StepID:            stepID,
		FromParticipantID: fromID,
		ToParticipantID:   toID,
		LeaseSeconds:      req.LeaseSeconds,
		Comment:           req.Comment,
		Actor:             s.actorFromRequest(r),
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, claim)
}

type submitCollabArtifactRequest struct {
	ParticipantID string          `json:"participant_id"`
	Kind          string          `json:"kind,omitempty"`
	Content       json.RawMessage `json:"content"`
}

func (s *Server) submitCollabArtifact(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	var req submitCollabArtifactRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	participantID, err := uuid.Parse(req.ParticipantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid participant_id")
		return
	}
	if _, err := s.ensureParticipantOwnership(r, participantID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	artifact, err := s.collabSvc.SubmitArtifact(contextFromRequest(r), appCollab.SubmitArtifactInput{
		StepID:        stepID,
		ParticipantID: participantID,
		Kind:          req.Kind,
		Content:       req.Content,
		Actor:         s.actorFromRequest(r),
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, artifact)
}

func (s *Server) listCollabArtifacts(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	step, err := s.collabSvc.GetStep(contextFromRequest(r), stepID)
	if err != nil || step == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "step not found")
		return
	}
	if err := s.ensureSessionAccess(r, step.SessionID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	artifacts, err := s.collabSvc.ListArtifacts(contextFromRequest(r), stepID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"step_id":   stepID,
		"artifacts": artifacts,
	})
}

type openCollabDecisionRequest struct {
	Policy   json.RawMessage `json:"policy,omitempty"`
	Deadline *string         `json:"deadline,omitempty"`
}

func (s *Server) openCollabDecision(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	var req openCollabDecisionRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}

	var deadline *time.Time
	if req.Deadline != nil && strings.TrimSpace(*req.Deadline) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.Deadline))
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid deadline")
			return
		}
		deadline = &parsed
	}
	step, err := s.collabSvc.GetStep(contextFromRequest(r), stepID)
	if err != nil || step == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "step not found")
		return
	}
	if err := s.ensureSessionAccess(r, step.SessionID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}

	decision, err := s.collabSvc.OpenDecision(contextFromRequest(r), appCollab.OpenDecisionInput{
		StepID:   stepID,
		Policy:   req.Policy,
		Deadline: deadline,
		Actor:    s.actorFromRequest(r),
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, decision)
}

type voteCollabDecisionRequest struct {
	ParticipantID string  `json:"participant_id"`
	Choice        string  `json:"choice"`
	Comment       *string `json:"comment,omitempty"`
}

func (s *Server) voteCollabDecision(w http.ResponseWriter, r *http.Request) {
	decisionID, err := parseUUIDParam(r, "decisionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid decisionId")
		return
	}
	var req voteCollabDecisionRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	participantID, err := uuid.Parse(req.ParticipantID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid participant_id")
		return
	}
	if _, err := s.ensureParticipantOwnership(r, participantID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}

	decision, err := s.collabSvc.CastVote(contextFromRequest(r), appCollab.VoteInput{
		DecisionID:    decisionID,
		ParticipantID: participantID,
		Choice:        collab.VoteChoice(strings.ToUpper(strings.TrimSpace(req.Choice))),
		Comment:       req.Comment,
		Actor:         s.actorFromRequest(r),
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, decision)
}

type resolveCollabStepRequest struct {
	ParticipantID *string `json:"participant_id,omitempty"`
}

func (s *Server) resolveCollabStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	var req resolveCollabStepRequest
	if err := decodeBody(r, &req); err != nil && err != io.EOF {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	var participantID *uuid.UUID
	if req.ParticipantID != nil && strings.TrimSpace(*req.ParticipantID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.ParticipantID))
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid participant_id")
			return
		}
		if _, err := s.ensureParticipantOwnership(r, parsed); err != nil {
			respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		participantID = &parsed
	}

	step, err := s.collabSvc.ResolveStep(contextFromRequest(r), appCollab.ResolveStepInput{
		StepID:        stepID,
		ParticipantID: participantID,
		Actor:         s.actorFromRequest(r),
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, step)
}

func (s *Server) getCollabStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	step, err := s.collabSvc.GetStep(contextFromRequest(r), stepID)
	if err != nil || step == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "step not found")
		return
	}
	if err := s.ensureSessionAccess(r, step.SessionID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, step)
}

func (s *Server) listCollabEvents(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseUUIDParam(r, "sessionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid sessionId")
		return
	}
	if err := s.ensureSessionAccess(r, sessionID); err != nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	limit, offset := parseLimitOffset(r, 100, 500)
	events, err := s.collabSvc.ListEvents(contextFromRequest(r), sessionID, limit, offset)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": sessionID,
		"events":     events,
	})
}

// collabSSEEndpoint streams collab events from shared SSE hub.
func (s *Server) collabSSEEndpoint(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	if clientID == "" {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "client_id required")
		return
	}
	sessionFilter := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionFilter != "" {
		sid, err := uuid.Parse(sessionFilter)
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid session_id")
			return
		}
		if err := s.ensureSessionAccess(r, sid); err != nil {
			respondError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
	}

	auth := authUserFromContext(r.Context())
	var userID *string
	groups := []string{}
	if auth != nil {
		u := auth.Username
		userID = &u
		groups = append(groups, "role:"+strings.ToUpper(string(auth.Role)))
	}

	client := notification.NewSSEClient(clientID, userID, groups)
	s.sseHub.Register(client)
	defer s.sseHub.Unregister(clientID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "streaming not supported")
		return
	}
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case msg := <-client.MessageChan:
			if msg == nil {
				return
			}
			if msg.Event != "collab" {
				continue
			}
			if sessionFilter != "" && !matchSessionFilter(msg.Data, sessionFilter) {
				continue
			}
			payload, _ := json.Marshal(msg)
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(payload)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func matchSessionFilter(data json.RawMessage, sessionID string) bool {
	var msg struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(msg.SessionID), strings.TrimSpace(sessionID))
}

func (s *Server) ensureSessionAccess(r *http.Request, sessionID uuid.UUID) error {
	user := authUserFromContext(r.Context())
	if user == nil {
		return &apiError{message: "missing auth"}
	}
	if user.Role == domainUser.RoleAdmin {
		return nil
	}
	ref := "user:" + user.Username
	if user.Type == domainUser.TypeAgent {
		ref = "agent:" + user.Username
	}
	p, err := s.collabSvc.GetParticipantByRef(contextFromRequest(r), sessionID, ref)
	if err != nil {
		return err
	}
	if p == nil {
		return &apiError{message: "session access denied"}
	}
	return nil
}

func (s *Server) ensureParticipantOwnership(r *http.Request, participantID uuid.UUID) (*collab.Participant, error) {
	user := authUserFromContext(r.Context())
	if user == nil {
		return nil, &apiError{message: "missing auth"}
	}
	p, err := s.collabSvc.GetParticipant(contextFromRequest(r), participantID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, &apiError{message: "participant not found"}
	}
	if user.Role == domainUser.RoleAdmin {
		return p, nil
	}
	expected := "user:" + user.Username
	if user.Type == domainUser.TypeAgent {
		expected = "agent:" + user.Username
	}
	if !strings.EqualFold(strings.TrimSpace(p.Ref), expected) {
		return nil, &apiError{message: "participant access denied"}
	}
	return p, nil
}
