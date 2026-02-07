package httpapi

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	appUser "github.com/execution-hub/execution-hub/internal/application/user"
	domainUser "github.com/execution-hub/execution-hub/internal/domain/user"
)

type userCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type agentCreateRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	OwnerUserID string `json:"owner_user_id,omitempty"`
}

type userUpdateRequest struct {
	Username    *string `json:"username,omitempty"`
	Role        *string `json:"role,omitempty"`
	Status      *string `json:"status,omitempty"`
	OwnerUserID *string `json:"owner_user_id,omitempty"`
}

type passwordUpdateRequest struct {
	Password string `json:"password"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req userCreateRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	role, err := parseRole(req.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	u, err := s.userSvc.CreateUser(r.Context(), appUser.CreateInput{
		Username: req.Username,
		Password: req.Password,
		Role:     role,
		Type:     domainUser.TypeHuman,
		Status:   domainUser.StatusActive,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, u)
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	var req agentCreateRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	role, err := parseRole(req.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	var ownerID *uuid.UUID
	if req.OwnerUserID != "" {
		id, err := uuid.Parse(req.OwnerUserID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid owner_user_id")
			return
		}
		ownerID = &id
	}
	auth := authUserFromContext(r.Context())
	if auth == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth")
		return
	}
	if auth.Role != domainUser.RoleAdmin {
		ownerID = &auth.UserID
	}
	if ownerID == nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "owner_user_id required")
		return
	}

	u, err := s.userSvc.CreateUser(r.Context(), appUser.CreateInput{
		Username:    req.Username,
		Password:    req.Password,
		Role:        role,
		Type:        domainUser.TypeAgent,
		OwnerUserID: ownerID,
		Status:      domainUser.StatusActive,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, u)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := parseLimitOffset(r, 100, 200)
	filter := domainUser.Filter{}
	if v := r.URL.Query().Get("role"); v != "" {
		role, err := parseRole(v)
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
			return
		}
		filter.Role = &role
	}
	if v := r.URL.Query().Get("type"); v != "" {
		t := domainUser.Type(strings.ToUpper(v))
		filter.Type = &t
	}
	if v := r.URL.Query().Get("status"); v != "" {
		st := domainUser.Status(strings.ToUpper(v))
		filter.Status = &st
	}
	if v := r.URL.Query().Get("owner_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid owner_id")
			return
		}
		filter.OwnerUserID = &id
	}
	users, err := s.userSvc.ListUsers(r.Context(), filter, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"users": users})
}

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	limit, offset := parseLimitOffset(r, 100, 200)
	filter := domainUser.Filter{}
	agentType := domainUser.TypeAgent
	filter.Type = &agentType
	if v := r.URL.Query().Get("owner_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid owner_id")
			return
		}
		filter.OwnerUserID = &id
	} else {
		auth := authUserFromContext(r.Context())
		if auth != nil && auth.Role != domainUser.RoleAdmin {
			filter.OwnerUserID = &auth.UserID
		}
	}
	users, err := s.userSvc.ListUsers(r.Context(), filter, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"agents": users})
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "userId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid userId")
		return
	}
	u, err := s.userSvc.GetUser(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if u == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	auth := authUserFromContext(r.Context())
	if auth == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth")
		return
	}
	if auth.Role != domainUser.RoleAdmin && auth.UserID != id {
		if u.Type != domainUser.TypeAgent || u.OwnerUserID == nil || *u.OwnerUserID != auth.UserID {
			respondError(w, http.StatusForbidden, "FORBIDDEN", "insufficient role")
			return
		}
	}
	respondJSON(w, http.StatusOK, u)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "userId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid userId")
		return
	}
	var req userUpdateRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	var role *domainUser.Role
	if req.Role != nil {
		parsed, err := parseRole(*req.Role)
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
			return
		}
		role = &parsed
	}
	var status *domainUser.Status
	if req.Status != nil {
		st := domainUser.Status(strings.ToUpper(*req.Status))
		if err := domainUser.ValidateStatus(st); err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
			return
		}
		status = &st
	}
	var ownerID *uuid.UUID
	if req.OwnerUserID != nil && *req.OwnerUserID != "" {
		oid, err := uuid.Parse(*req.OwnerUserID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid owner_user_id")
			return
		}
		ownerID = &oid
	}
	u, err := s.userSvc.UpdateUser(r.Context(), id, appUser.UpdateInput{
		Username:    req.Username,
		Role:        role,
		Status:      status,
		OwnerUserID: ownerID,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, u)
}

func (s *Server) setUserPassword(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "userId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "invalid userId")
		return
	}
	auth := authUserFromContext(r.Context())
	if auth == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth")
		return
	}
	if auth.Role != domainUser.RoleAdmin && auth.UserID != id {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "insufficient role")
		return
	}
	var req passwordUpdateRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	if err := s.userSvc.SetPassword(r.Context(), id, req.Password); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "OK"})
}

func parseRole(role string) (domainUser.Role, error) {
	r := domainUser.Role(strings.ToUpper(role))
	if err := domainUser.ValidateRole(r); err != nil {
		return "", err
	}
	return r, nil
}
