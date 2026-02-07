package httpapi

import (
	"net"
	"net/http"
	"time"

	appUser "github.com/execution-hub/execution-hub/internal/application/user"
	"github.com/execution-hub/execution-hub/internal/domain/audit"
	domainUser "github.com/execution-hub/execution-hub/internal/domain/user"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	User         interface{} `json:"user"`
	SessionID    string      `json:"session_id"`
	ExpiresAt    string      `json:"expires_at"`
	SessionToken string      `json:"session_token"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	userAgent := r.UserAgent()
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = host
	}
	res, err := s.authSvc.Login(r.Context(), req.Username, req.Password, &userAgent, &ip)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	actorPrefix := "user"
	if res.User.Type == domainUser.TypeAgent {
		actorPrefix = "agent"
	}
	s.auditSvc.Log(r.Context(), &audit.AuditEntry{
		EntityType: audit.EntityTypeUser,
		EntityID:   res.User.UserID.String(),
		Action:     audit.ActionLogin,
		Actor:      actorPrefix + ":" + res.User.Username,
		ActorRoles: []string{string(res.User.Role)},
		Reason:     "login",
	})

	cookie := &http.Cookie{
		Name:     s.sessionCookieName,
		Value:    res.Token,
		Path:     "/",
		Expires:  res.Session.ExpiresAt,
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	respondJSON(w, http.StatusOK, loginResponse{
		User:         res.User,
		SessionID:    res.Session.SessionID.String(),
		ExpiresAt:    res.Session.ExpiresAt.Format(time.RFC3339),
		SessionToken: res.Token,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r, s.sessionCookieName)
	if auth := authUserFromContext(r.Context()); auth != nil {
		s.auditSvc.Log(r.Context(), &audit.AuditEntry{
			EntityType: audit.EntityTypeUser,
			EntityID:   auth.UserID.String(),
			Action:     audit.ActionLogout,
			Actor:      auth.ActorString(),
			ActorRoles: []string{string(auth.Role)},
			Reason:     "logout",
		})
	}
	_ = s.authSvc.Logout(r.Context(), token)

	cookie := &http.Cookie{
		Name:     s.sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   s.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	http.SetCookie(w, cookie)
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "OK"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u := authUserFromContext(r.Context())
	if u == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth")
		return
	}
	user, err := s.userSvc.GetUser(r.Context(), u.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, user)
}

func (s *Server) bootstrapAdmin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	count, err := s.userSvc.Count(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if count > 0 {
		respondError(w, http.StatusBadRequest, "INVALID_STATE", "bootstrap already completed")
		return
	}
	u, err := s.userSvc.CreateUser(r.Context(), appUser.CreateInput{
		Username: req.Username,
		Password: req.Password,
		Role:     domainUser.RoleAdmin,
		Type:     domainUser.TypeHuman,
		Status:   domainUser.StatusActive,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, u)
}
