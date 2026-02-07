package httpapi

import (
	"net/http"
	"strings"
)

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r, s.sessionCookieName)
		u, sess, err := s.authSvc.Authenticate(r.Context(), token)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
			return
		}
		ctx := withAuthUser(r.Context(), &AuthUser{
			UserID:    u.UserID,
			Username:  u.Username,
			Role:      u.Role,
			Type:      u.Type,
			SessionID: sess.SessionID,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{})
	for _, r := range roles {
		allowed[strings.ToUpper(r)] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := authUserFromContext(r.Context())
			if user == nil {
				respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth")
				return
			}
			if _, ok := allowed[strings.ToUpper(string(user.Role))]; !ok {
				respondError(w, http.StatusForbidden, "FORBIDDEN", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractToken(r *http.Request, cookieName string) string {
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(authz, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	}
	if cookieName != "" {
		if c, err := r.Cookie(cookieName); err == nil {
			return c.Value
		}
	}
	return ""
}
