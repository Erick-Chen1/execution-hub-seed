package session

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines persistence for sessions.
type Repository interface {
	Create(ctx context.Context, session *Session) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	DeleteByID(ctx context.Context, sessionID uuid.UUID) error
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	UpdateLastSeen(ctx context.Context, sessionID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int, error)
}
