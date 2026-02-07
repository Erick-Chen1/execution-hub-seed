package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/execution-hub/execution-hub/internal/domain/user"
)

type authContextKey string

const (
	authUserKey    authContextKey = "authUser"
	authSessionKey authContextKey = "authSession"
)

// AuthUser represents the authenticated user in context.
type AuthUser struct {
	UserID    uuid.UUID
	Username  string
	Role      user.Role
	Type      user.Type
	SessionID uuid.UUID
}

func (u AuthUser) ActorString() string {
	prefix := "user"
	if u.Type == user.TypeAgent {
		prefix = "agent"
	}
	return prefix + ":" + u.Username
}

func withAuthUser(ctx context.Context, u *AuthUser) context.Context {
	if u == nil {
		return ctx
	}
	return context.WithValue(ctx, authUserKey, u)
}

func authUserFromContext(ctx context.Context) *AuthUser {
	val := ctx.Value(authUserKey)
	if v, ok := val.(*AuthUser); ok {
		return v
	}
	return nil
}
