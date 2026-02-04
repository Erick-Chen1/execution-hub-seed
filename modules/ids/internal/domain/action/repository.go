package action

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/mock_repository.go -package=mocks . Repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines the interface for action persistence
type Repository interface {
	// Action operations
	Create(ctx context.Context, action *Action) error
	GetByID(ctx context.Context, actionID uuid.UUID) (*Action, error)
	GetByEvaluationID(ctx context.Context, evaluationID uuid.UUID) (*Action, error)
	List(ctx context.Context, filter Filter, limit, offset int) ([]*Action, error)
	Update(ctx context.Context, action *Action) error
	UpdateStatus(ctx context.Context, actionID uuid.UUID, status Status) error

	// Deduplication
	FindByDedupeKey(ctx context.Context, dedupeKey string, since time.Time) (*Action, error)

	// State transitions
	RecordTransition(ctx context.Context, transition *StateTransition) error
	GetTransitions(ctx context.Context, actionID uuid.UUID) ([]*StateTransition, error)

	// Retry support
	ListRetryableActions(ctx context.Context, limit int) ([]*Action, error)
}
