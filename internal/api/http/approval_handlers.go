package httpapi

import (
	"net/http"
	"strings"

	appApproval "github.com/execution-hub/execution-hub/internal/application/approval"
	domainApproval "github.com/execution-hub/execution-hub/internal/domain/approval"
)

type approvalDecisionRequest struct {
	Decision string  `json:"decision"`
	Comment  *string `json:"comment,omitempty"`
}

func (s *Server) listApprovals(w http.ResponseWriter, r *http.Request) {
	limit, offset := parseLimitOffset(r, 100, 200)
	filter := domainApproval.Filter{}
	if v := r.URL.Query().Get("status"); v != "" {
		st := domainApproval.Status(strings.ToUpper(v))
		filter.Status = &st
	}
	if v := r.URL.Query().Get("entity_type"); v != "" {
		filter.EntityType = &v
	}
	if v := r.URL.Query().Get("entity_id"); v != "" {
		filter.EntityID = &v
	}
	if v := r.URL.Query().Get("operation"); v != "" {
		op := domainApproval.Operation(strings.ToUpper(v))
		filter.Operation = &op
	}
	if v := r.URL.Query().Get("requested_by"); v != "" {
		filter.RequestedBy = &v
	}
	items, err := s.approvalSvc.List(r.Context(), filter, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"approvals": items})
}

func (s *Server) getApproval(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "approvalId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid approvalId")
		return
	}
	item, err := s.approvalSvc.Get(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if item == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "approval not found")
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (s *Server) decideApproval(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "approvalId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid approvalId")
		return
	}
	var req approvalDecisionRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	decision := domainApproval.Decision(strings.ToUpper(req.Decision))
	if decision != domainApproval.DecisionApprove && decision != domainApproval.DecisionReject {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "decision must be APPROVE or REJECT")
		return
	}
	auth := authUserFromContext(r.Context())
	if auth == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth")
		return
	}
	actor := appApproval.Actor{
		UserID:   auth.UserID,
		Username: auth.Username,
		Role:     auth.Role,
		Type:     auth.Type,
	}
	item, err := s.approvalSvc.Decide(r.Context(), id, decision, actor, req.Comment)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, item)
}
