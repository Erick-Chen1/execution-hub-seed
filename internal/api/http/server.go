package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	appAction "github.com/execution-hub/execution-hub/internal/application/action"
	appApproval "github.com/execution-hub/execution-hub/internal/application/approval"
	appAudit "github.com/execution-hub/execution-hub/internal/application/audit"
	appAuth "github.com/execution-hub/execution-hub/internal/application/auth"
	appCollab "github.com/execution-hub/execution-hub/internal/application/collab"
	appExecutor "github.com/execution-hub/execution-hub/internal/application/executor"
	appNotification "github.com/execution-hub/execution-hub/internal/application/notification"
	appTask "github.com/execution-hub/execution-hub/internal/application/task"
	appTrust "github.com/execution-hub/execution-hub/internal/application/trust"
	appUser "github.com/execution-hub/execution-hub/internal/application/user"
	appWorkflow "github.com/execution-hub/execution-hub/internal/application/workflow"
	domainAction "github.com/execution-hub/execution-hub/internal/domain/action"
	"github.com/execution-hub/execution-hub/internal/domain/executor"
	"github.com/execution-hub/execution-hub/internal/domain/notification"
	"github.com/execution-hub/execution-hub/internal/domain/task"
	domainUser "github.com/execution-hub/execution-hub/internal/domain/user"
	"github.com/execution-hub/execution-hub/internal/domain/workflow"
	"github.com/execution-hub/execution-hub/internal/infrastructure/sse"
)

// Server holds dependencies for HTTP handlers.
type Server struct {
	workflowSvc         *appWorkflow.Service
	taskSvc             *appTask.Service
	executorSvc         *appExecutor.Service
	actionSvc           *appAction.Service
	notificationSvc     *appNotification.Service
	auditSvc            *appAudit.Service
	trustSvc            *appTrust.Service
	authSvc             *appAuth.Service
	userSvc             *appUser.Service
	approvalSvc         *appApproval.Service
	collabSvc           *appCollab.Service
	sseHub              *sse.Hub
	sessionCookieName   string
	sessionCookieSecure bool
}

func NewServer(
	workflowSvc *appWorkflow.Service,
	taskSvc *appTask.Service,
	executorSvc *appExecutor.Service,
	actionSvc *appAction.Service,
	notificationSvc *appNotification.Service,
	auditSvc *appAudit.Service,
	trustSvc *appTrust.Service,
	authSvc *appAuth.Service,
	userSvc *appUser.Service,
	approvalSvc *appApproval.Service,
	collabSvc *appCollab.Service,
	sseHub *sse.Hub,
	sessionCookieName string,
	sessionCookieSecure bool,
) *Server {
	return &Server{
		workflowSvc:         workflowSvc,
		taskSvc:             taskSvc,
		executorSvc:         executorSvc,
		actionSvc:           actionSvc,
		notificationSvc:     notificationSvc,
		auditSvc:            auditSvc,
		trustSvc:            trustSvc,
		authSvc:             authSvc,
		userSvc:             userSvc,
		approvalSvc:         approvalSvc,
		collabSvc:           collabSvc,
		sseHub:              sseHub,
		sessionCookieName:   sessionCookieName,
		sessionCookieSecure: sessionCookieSecure,
	}
}

// Router builds the HTTP router.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Route("/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", s.login)
			r.Post("/bootstrap", s.bootstrapAdmin)
			r.Group(func(r chi.Router) {
				r.Use(s.requireAuth)
				r.Post("/logout", s.logout)
				r.Get("/me", s.me)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Route("/workflows", func(r chi.Router) {
				r.Post("/", s.createWorkflow)
				r.Get("/", s.listWorkflows)
				r.Get("/{workflowId}", s.getWorkflow)
				r.Get("/{workflowId}/versions", s.listWorkflowVersions)
				r.Post("/{workflowId}/activate", s.activateWorkflow)
				r.Post("/{workflowId}/deactivate", s.deactivateWorkflow)
			})

			r.Route("/tasks", func(r chi.Router) {
				r.Post("/", s.createTask)
				r.Get("/", s.listTasks)
				r.Get("/{taskId}", s.getTask)
				r.Post("/{taskId}/start", s.startTask)
				r.Post("/{taskId}/cancel", s.cancelTask)

				r.Get("/{taskId}/steps", s.listSteps)
				r.Get("/{taskId}/steps/{stepId}", s.getStep)
				r.Post("/{taskId}/steps/{stepId}/dispatch", s.dispatchStep)
				r.Post("/{taskId}/steps/{stepId}/ack", s.ackStep)
				r.Post("/{taskId}/steps/{stepId}/resolve", s.resolveStep)
				r.Post("/{taskId}/steps/{stepId}/fail", s.failStep)

				r.Get("/{taskId}/evidence", s.getTaskEvidence)
				r.Get("/{taskId}/steps/{stepId}/evidence", s.getStepEvidence)
			})

			r.Route("/executors", func(r chi.Router) {
				r.Post("/", s.createExecutor)
				r.Get("/", s.listExecutors)
				r.Get("/{executorId}", s.getExecutor)
				r.Post("/{executorId}/activate", s.activateExecutor)
				r.Post("/{executorId}/deactivate", s.deactivateExecutor)
			})

			r.Route("/actions", func(r chi.Router) {
				r.Post("/{actionId}/ack", s.ackAction)
				r.Post("/{actionId}/resolve", s.resolveAction)
				r.Get("/{actionId}/transitions", s.getActionTransitions)
				r.Get("/{actionId}/evidence", s.getActionEvidence)
			})

			r.Route("/notifications", func(r chi.Router) {
				r.Get("/", s.listNotifications)
				r.Get("/{notificationId}", s.getNotification)
				r.Post("/{notificationId}/send", s.sendNotification)
				r.Get("/sse", s.sseEndpoint)
			})

			r.Route("/approvals", func(r chi.Router) {
				r.Get("/", s.listApprovals)
				r.Get("/{approvalId}", s.getApproval)
				r.Post("/{approvalId}/decide", s.decideApproval)
			})

			r.Route("/users", func(r chi.Router) {
				r.With(s.requireRole(string(domainUser.RoleAdmin))).Post("/", s.createUser)
				r.With(s.requireRole(string(domainUser.RoleAdmin))).Get("/", s.listUsers)
				r.Get("/{userId}", s.getUser)
				r.With(s.requireRole(string(domainUser.RoleAdmin))).Patch("/{userId}", s.updateUser)
				r.With(s.requireRole(string(domainUser.RoleAdmin))).Put("/{userId}/password", s.setUserPassword)
			})

			r.Route("/agents", func(r chi.Router) {
				r.Post("/", s.createAgent)
				r.Get("/", s.listAgents)
				r.Get("/{userId}", s.getUser)
				r.With(s.requireRole(string(domainUser.RoleAdmin))).Patch("/{userId}", s.updateUser)
				r.With(s.requireRole(string(domainUser.RoleAdmin))).Put("/{userId}/password", s.setUserPassword)
			})

			r.Route("/trust", func(r chi.Router) {
				r.Post("/events", s.ingestEvent)
				r.Get("/evidence/{bundleType}/{subjectId}", s.getTrustEvidence)
			})

			r.Route("/admin", func(r chi.Router) {
				r.With(s.requireRole(string(domainUser.RoleAdmin))).Get("/audit", s.queryAudit)
				r.With(s.requireRole(string(domainUser.RoleAdmin))).Get("/audit/{auditId}", s.getAudit)
			})
		})
	})

	r.Route("/v2", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Route("/collab", func(r chi.Router) {
				r.Post("/sessions", s.createCollabSession)
				r.Get("/sessions/{sessionId}", s.getCollabSession)
				r.Post("/sessions/{sessionId}/join", s.joinCollabSession)
				r.Get("/sessions/{sessionId}/steps/open", s.listCollabOpenSteps)
				r.Get("/sessions/{sessionId}/events", s.listCollabEvents)

				r.Post("/steps/{stepId}/claim", s.claimCollabStep)
				r.Post("/steps/{stepId}/release", s.releaseCollabStep)
				r.Post("/steps/{stepId}/handoff", s.handoffCollabStep)
				r.Post("/steps/{stepId}/artifacts", s.submitCollabArtifact)
				r.Get("/steps/{stepId}/artifacts", s.listCollabArtifacts)
				r.Post("/steps/{stepId}/decisions", s.openCollabDecision)
				r.Post("/steps/{stepId}/resolve", s.resolveCollabStep)
				r.Get("/steps/{stepId}", s.getCollabStep)

				r.Post("/decisions/{decisionId}/votes", s.voteCollabDecision)

				r.Get("/stream", s.collabSSEEndpoint)
			})
		})
	})

	return r
}

// Helpers
func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error":   code,
		"message": message,
	})
}

func parseUUIDParam(r *http.Request, key string) (uuid.UUID, error) {
	val := chi.URLParam(r, key)
	return uuid.Parse(val)
}

func decodeBody(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *Server) actorFromRequest(r *http.Request) string {
	if actor, err := s.approvalActorFromRequest(r); err == nil {
		return actor.ActorString()
	}
	if u := authUserFromContext(r.Context()); u != nil {
		return u.ActorString()
	}
	actor := r.Header.Get("X-Actor")
	if actor == "" {
		actor = "system"
	}
	return actor
}

func contextFromRequest(r *http.Request) context.Context {
	return r.Context()
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseLimitOffset(r *http.Request, defaultLimit, maxLimit int) (int, int) {
	limit := defaultLimit
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			limit = l
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if o, err := strconv.Atoi(v); err == nil {
			offset = o
		}
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// Data types for requests

type workflowCreateRequest struct {
	WorkflowID  *uuid.UUID               `json:"workflow_id,omitempty"`
	Version     int                      `json:"version,omitempty"`
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Steps       []workflow.Step          `json:"steps"`
	Edges       []workflow.Edge          `json:"edges,omitempty"`
	Conditions  []workflow.ConditionExpr `json:"conditions,omitempty"`
	Approvals   *workflow.Approvals      `json:"approvals,omitempty"`
}

type taskCreateRequest struct {
	Title      string          `json:"title"`
	WorkflowID string          `json:"workflow_id"`
	Context    json.RawMessage `json:"context,omitempty"`
}

type stepActionRequest struct {
	Actor    string          `json:"actor"`
	Comment  *string         `json:"comment,omitempty"`
	Evidence json.RawMessage `json:"evidence,omitempty"`
	Reason   string          `json:"reason,omitempty"`
}

type trustEventIngestRequest struct {
	EventID        *uuid.UUID      `json:"event_id,omitempty"`
	ClientRecordID string          `json:"client_record_id,omitempty"`
	SourceType     string          `json:"source_type"`
	SourceID       string          `json:"source_id"`
	TsDevice       *string         `json:"ts_device,omitempty"`
	TsGateway      *string         `json:"ts_gateway,omitempty"`
	Key            *string         `json:"key,omitempty"`
	EventType      string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload"`
	SchemaVersion  string          `json:"schema_version"`
}

type stepEvidenceResponse struct {
	Step           *task.Step                      `json:"step"`
	ActionEvidence *appAction.ActionEvidence       `json:"action_evidence,omitempty"`
	Transitions    []*domainAction.StateTransition `json:"transitions,omitempty"`
}

// Workflow handlers
func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) {
	var req workflowCreateRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}

	spec := workflow.Spec{
		Name:        req.Name,
		Description: req.Description,
		Steps:       req.Steps,
		Edges:       req.Edges,
		Conditions:  req.Conditions,
		Approvals:   req.Approvals,
	}
	if req.WorkflowID != nil {
		spec.WorkflowID = *req.WorkflowID
	}
	if req.Version > 0 {
		spec.Version = req.Version
	}

	actor := s.actorFromRequest(r)
	def, err := s.workflowSvc.CreateDefinition(contextFromRequest(r), spec, &actor)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"workflow_id": def.WorkflowID,
		"version":     def.Version,
	})
}

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	limit, offset := parseLimitOffset(r, 100, 200)
	defs, err := s.workflowSvc.ListDefinitions(contextFromRequest(r), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"workflows": defs})
}

func (s *Server) getWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "workflowId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid workflowId")
		return
	}
	def, err := s.workflowSvc.GetDefinition(contextFromRequest(r), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, def)
}

func (s *Server) listWorkflowVersions(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "workflowId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid workflowId")
		return
	}
	defs, err := s.workflowSvc.ListVersions(contextFromRequest(r), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"versions": defs})
}

func (s *Server) activateWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "workflowId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid workflowId")
		return
	}
	var req struct {
		Version int `json:"version"`
	}
	_ = decodeBody(r, &req)
	if req.Version == 0 {
		req.Version = 1
	}
	actor := s.actorFromRequest(r)
	if err := s.workflowSvc.Activate(contextFromRequest(r), id, req.Version, &actor); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"workflow_id": id, "version": req.Version, "status": "ACTIVE"})
}

func (s *Server) deactivateWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "workflowId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid workflowId")
		return
	}
	var req struct {
		Version int `json:"version"`
	}
	_ = decodeBody(r, &req)
	if req.Version == 0 {
		req.Version = 1
	}
	actor := s.actorFromRequest(r)
	if err := s.workflowSvc.Deactivate(contextFromRequest(r), id, req.Version, &actor); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"workflow_id": id, "version": req.Version, "status": "INACTIVE"})
}

// Task handlers
func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var req taskCreateRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	wid, err := uuid.Parse(req.WorkflowID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid workflow_id")
		return
	}
	actor := s.actorFromRequest(r)
	t, err := s.taskSvc.CreateTask(contextFromRequest(r), wid, req.Title, req.Context, &actor)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"task_id": t.TaskID, "status": t.Status})
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	var status *task.Status
	if st := r.URL.Query().Get("status"); st != "" {
		s := task.Status(st)
		status = &s
	}
	limit, offset := parseLimitOffset(r, 100, 200)
	tasks, err := s.taskSvc.ListTasks(contextFromRequest(r), status, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"tasks": tasks})
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "taskId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid taskId")
		return
	}
	t, err := s.taskSvc.GetTask(contextFromRequest(r), id)
	if err != nil || t == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "task not found")
		return
	}
	respondJSON(w, http.StatusOK, t)
}

func (s *Server) startTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "taskId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid taskId")
		return
	}
	actor, err := s.approvalActorFromRequest(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	pending, err := s.approvalSvc.RequireTaskStart(contextFromRequest(r), id, actor)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	if pending != nil {
		respondApprovalPending(w, pending)
		return
	}
	actorStr := actor.ActorString()
	if err := s.taskSvc.StartTask(contextFromRequest(r), id, actorStr); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"task_id": id, "status": "RUNNING"})
}

func (s *Server) cancelTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "taskId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid taskId")
		return
	}
	actor, err := s.approvalActorFromRequest(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	pending, err := s.approvalSvc.RequireTaskCancel(contextFromRequest(r), id, actor)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	if pending != nil {
		respondApprovalPending(w, pending)
		return
	}
	actorStr := actor.ActorString()
	if err := s.taskSvc.CancelTask(contextFromRequest(r), id, actorStr); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"task_id": id, "status": "CANCELLED"})
}

func (s *Server) listSteps(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "taskId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid taskId")
		return
	}
	steps, err := s.taskSvc.ListSteps(contextFromRequest(r), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"task_id": id, "steps": steps})
}

func (s *Server) getStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	step, err := s.taskSvc.GetStep(contextFromRequest(r), stepID)
	if err != nil || step == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "step not found")
		return
	}
	respondJSON(w, http.StatusOK, step)
}

func (s *Server) dispatchStep(w http.ResponseWriter, r *http.Request) {
	taskID, err := parseUUIDParam(r, "taskId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid taskId")
		return
	}
	if err := s.taskSvc.DispatchStep(contextFromRequest(r), taskID); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"task_id": taskID, "status": "DISPATCHED"})
}

func (s *Server) ackStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	var req stepActionRequest
	_ = decodeBody(r, &req)
	actor, err := s.approvalActorFromRequest(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	pending, err := s.approvalSvc.RequireStepAck(contextFromRequest(r), stepID, actor, req.Comment)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	if pending != nil {
		respondApprovalPending(w, pending)
		return
	}
	actorStr := actor.ActorString()
	if err := s.taskSvc.AckStep(contextFromRequest(r), stepID, actorStr, req.Comment); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"step_id": stepID, "status": "ACKED"})
}

func (s *Server) resolveStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	var req stepActionRequest
	_ = decodeBody(r, &req)
	actor, err := s.approvalActorFromRequest(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	pending, err := s.approvalSvc.RequireStepResolve(contextFromRequest(r), stepID, actor, req.Evidence)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	if pending != nil {
		respondApprovalPending(w, pending)
		return
	}
	actorStr := actor.ActorString()
	if err := s.taskSvc.ResolveStep(contextFromRequest(r), stepID, actorStr, req.Evidence); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"step_id": stepID, "status": "RESOLVED"})
}

func (s *Server) failStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	var req stepActionRequest
	_ = decodeBody(r, &req)
	actor := s.actorFromRequest(r)
	reason := req.Reason
	if reason == "" {
		reason = "step failed"
	}
	if err := s.taskSvc.FailStep(contextFromRequest(r), stepID, actor, reason); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"step_id": stepID, "status": "FAILED"})
}

func (s *Server) getTaskEvidence(w http.ResponseWriter, r *http.Request) {
	taskID, err := parseUUIDParam(r, "taskId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid taskId")
		return
	}
	t, err := s.taskSvc.GetTask(contextFromRequest(r), taskID)
	if err != nil || t == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "task not found")
		return
	}
	steps, err := s.taskSvc.ListSteps(contextFromRequest(r), taskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	evSteps := make([]*stepEvidenceResponse, 0, len(steps))
	for _, st := range steps {
		actionEv, _ := s.actionSvc.GetActionEvidence(contextFromRequest(r), st.ActionID)
		transitions, _ := s.actionSvc.GetActionTransitions(contextFromRequest(r), st.ActionID)
		evSteps = append(evSteps, &stepEvidenceResponse{
			Step:           st,
			ActionEvidence: actionEv,
			Transitions:    transitions,
		})
	}
	resp := map[string]interface{}{"task_id": taskID, "trace_id": t.TraceID, "steps": evSteps}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) getStepEvidence(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid stepId")
		return
	}
	step, err := s.taskSvc.GetStep(contextFromRequest(r), stepID)
	if err != nil || step == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "step not found")
		return
	}
	actionEv, _ := s.actionSvc.GetActionEvidence(contextFromRequest(r), step.ActionID)
	transitions, _ := s.actionSvc.GetActionTransitions(contextFromRequest(r), step.ActionID)
	respondJSON(w, http.StatusOK, &stepEvidenceResponse{
		Step:           step,
		ActionEvidence: actionEv,
		Transitions:    transitions,
	})
}

// Executor handlers
func (s *Server) createExecutor(w http.ResponseWriter, r *http.Request) {
	var exec struct {
		ExecutorID   string   `json:"executor_id"`
		ExecutorType string   `json:"executor_type"`
		DisplayName  string   `json:"display_name"`
		Capabilities []string `json:"capability_tags"`
	}
	if err := decodeBody(r, &exec); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	model := &executor.Executor{
		ExecutorID:   exec.ExecutorID,
		ExecutorType: executor.Type(exec.ExecutorType),
		DisplayName:  exec.DisplayName,
		Capabilities: exec.Capabilities,
		Status:       executor.StatusActive,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.executorSvc.Create(contextFromRequest(r), model); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"executor_id": model.ExecutorID})
}

func (s *Server) listExecutors(w http.ResponseWriter, r *http.Request) {
	limit, offset := parseLimitOffset(r, 100, 200)
	execs, err := s.executorSvc.List(contextFromRequest(r), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"executors": execs})
}

func (s *Server) getExecutor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "executorId")
	if id == "" {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid executorId")
		return
	}
	exec, err := s.executorSvc.Get(contextFromRequest(r), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, exec)
}

func (s *Server) activateExecutor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "executorId")
	if err := s.executorSvc.Activate(contextFromRequest(r), id); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"executor_id": id, "status": "ACTIVE"})
}

func (s *Server) deactivateExecutor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "executorId")
	if err := s.executorSvc.Deactivate(contextFromRequest(r), id); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"executor_id": id, "status": "INACTIVE"})
}

// Action handlers
func (s *Server) ackAction(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "actionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid actionId")
		return
	}
	actor, err := s.approvalActorFromRequest(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	pending, err := s.approvalSvc.RequireActionAck(contextFromRequest(r), id, actor)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	if pending != nil {
		respondApprovalPending(w, pending)
		return
	}
	actorStr := actor.ActorString()
	if err := s.actionSvc.AcknowledgeAction(contextFromRequest(r), id, actorStr); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"action_id": id, "status": "ACKED"})
}

func (s *Server) resolveAction(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "actionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid actionId")
		return
	}
	actor, err := s.approvalActorFromRequest(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	pending, err := s.approvalSvc.RequireActionResolve(contextFromRequest(r), id, actor)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	if pending != nil {
		respondApprovalPending(w, pending)
		return
	}
	actorStr := actor.ActorString()
	if err := s.actionSvc.ResolveAction(contextFromRequest(r), id, actorStr); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"action_id": id, "status": "RESOLVED"})
}

func (s *Server) getActionTransitions(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "actionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid actionId")
		return
	}
	trs, err := s.actionSvc.GetActionTransitions(contextFromRequest(r), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"action_id": id, "transitions": trs})
}

func (s *Server) getActionEvidence(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "actionId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid actionId")
		return
	}
	ev, err := s.actionSvc.GetActionEvidence(contextFromRequest(r), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, ev)
}

// Notification handlers
func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	limit, offset := parseLimitOffset(r, 100, 200)
	ns, err := s.notificationSvc.ListNotifications(contextFromRequest(r), notification.Filter{}, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"notifications": ns})
}

func (s *Server) getNotification(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "notificationId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid notificationId")
		return
	}
	n, err := s.notificationSvc.GetNotification(contextFromRequest(r), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, n)
}

func (s *Server) sendNotification(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "notificationId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid notificationId")
		return
	}
	if err := s.notificationSvc.SendNotification(contextFromRequest(r), id); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"notification_id": id, "status": "SENT"})
}

func (s *Server) sseEndpoint(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "client_id required")
		return
	}
	userID := r.URL.Query().Get("user_id")
	groups := splitCSV(r.URL.Query().Get("groups"))
	if auth := authUserFromContext(r.Context()); auth != nil {
		userID = auth.Username
		groups = []string{"role:" + strings.ToUpper(string(auth.Role))}
	}
	var userPtr *string
	if userID != "" {
		userPtr = &userID
	}
	client := notification.NewSSEClient(clientID, userPtr, groups)
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
	// Send an initial comment to flush headers and keep the connection alive.
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case msg := <-client.MessageChan:
			if msg == nil {
				return
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

// Trust handlers
func (s *Server) ingestEvent(w http.ResponseWriter, r *http.Request) {
	var req trustEventIngestRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	if req.SourceID == "" || req.SourceType == "" || req.EventType == "" || req.SchemaVersion == "" || len(req.Payload) == 0 {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "source_type, source_id, event_type, schema_version and payload are required")
		return
	}
	var tsDevice *time.Time
	if req.TsDevice != nil && *req.TsDevice != "" {
		if t, err := time.Parse(time.RFC3339, *req.TsDevice); err == nil {
			tsDevice = &t
		} else {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid ts_device")
			return
		}
	}
	var tsGateway *time.Time
	if req.TsGateway != nil && *req.TsGateway != "" {
		if t, err := time.Parse(time.RFC3339, *req.TsGateway); err == nil {
			tsGateway = &t
		} else {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid ts_gateway")
			return
		}
	}
	eventID := uuid.Nil
	if req.EventID != nil {
		eventID = *req.EventID
	}
	entry, err := s.trustSvc.AddToHashChain(contextFromRequest(r), appTrust.HashChainInput{
		EventID:        eventID,
		SourceID:       req.SourceID,
		ClientRecordID: req.ClientRecordID,
		SourceType:     req.SourceType,
		TsDevice:       tsDevice,
		TsGateway:      tsGateway,
		Key:            req.Key,
		EventType:      req.EventType,
		Payload:        req.Payload,
		SchemaVersion:  req.SchemaVersion,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"event_id":     entry.EventID,
		"source_id":    entry.SourceID,
		"sequence_num": entry.SequenceNum,
		"chain_hash":   entry.ChainHash,
		"trust_level":  entry.TrustLevel.String(),
	})
}

func (s *Server) getTrustEvidence(w http.ResponseWriter, r *http.Request) {
	bundleType := chi.URLParam(r, "bundleType")
	subjectStr := chi.URLParam(r, "subjectId")
	subjectID, err := uuid.Parse(subjectStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid subjectId")
		return
	}
	bundle, err := s.trustSvc.GenerateEvidenceBundle(contextFromRequest(r), bundleType, subjectID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, bundle)
}

// Audit handlers
func (s *Server) queryAudit(w http.ResponseWriter, r *http.Request) {
	params := appAudit.QueryParams{
		Limit: 50,
	}
	if v := r.URL.Query().Get("entityType"); v != "" {
		params.EntityType = &v
	}
	if v := r.URL.Query().Get("entityId"); v != "" {
		params.EntityID = &v
	}
	if v := r.URL.Query().Get("action"); v != "" {
		params.Action = &v
	}
	if v := r.URL.Query().Get("actor"); v != "" {
		params.Actor = &v
	}
	if v := r.URL.Query().Get("riskLevel"); v != "" {
		params.RiskLevel = &v
	}
	if v := r.URL.Query().Get("traceId"); v != "" {
		params.TraceID = &v
	}
	if v := r.URL.Query().Get("cursor"); v != "" {
		params.Cursor = &v
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			params.Limit = l
		}
	}
	res, err := s.auditSvc.Query(contextFromRequest(r), params, "")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, res)
}

func (s *Server) getAudit(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "auditId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid auditId")
		return
	}
	log, err := s.auditSvc.GetByID(contextFromRequest(r), id, "")
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, log)
}
