package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	appApproval "github.com/execution-hub/execution-hub/internal/application/approval"
	domainUser "github.com/execution-hub/execution-hub/internal/domain/user"
)

func (s *Server) approvalActorFromRequest(r *http.Request) (appApproval.Actor, error) {
	auth := authUserFromContext(r.Context())
	if auth == nil {
		return appApproval.Actor{}, fmt.Errorf("missing auth")
	}
	requested := strings.TrimSpace(r.Header.Get("X-Actor"))
	if requested == "" || requested == auth.ActorString() {
		return appApproval.Actor{
			UserID:   auth.UserID,
			Username: auth.Username,
			Role:     auth.Role,
			Type:     auth.Type,
		}, nil
	}
	if strings.HasPrefix(requested, "agent:") {
		name := strings.TrimPrefix(requested, "agent:")
		agent, err := s.userSvc.GetByUsername(r.Context(), name)
		if err != nil || agent == nil {
			return appApproval.Actor{}, fmt.Errorf("agent not found")
		}
		if agent.Type != domainUser.TypeAgent {
			return appApproval.Actor{}, fmt.Errorf("not an agent user")
		}
		if auth.Role != domainUser.RoleAdmin {
			if agent.OwnerUserID == nil || *agent.OwnerUserID != auth.UserID {
				return appApproval.Actor{}, fmt.Errorf("agent not owned by user")
			}
		}
		return appApproval.Actor{
			UserID:   agent.UserID,
			Username: agent.Username,
			Role:     agent.Role,
			Type:     agent.Type,
		}, nil
	}
	// default to authenticated user for non-agent override
	return appApproval.Actor{
		UserID:   auth.UserID,
		Username: auth.Username,
		Role:     auth.Role,
		Type:     auth.Type,
	}, nil
}

func respondApprovalPending(w http.ResponseWriter, approval interface{}) {
	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":   "PENDING_APPROVAL",
		"approval": approval,
	})
}
