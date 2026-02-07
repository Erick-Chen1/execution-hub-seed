package workflow

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines workflow definition persistence.
type Repository interface {
	Create(ctx context.Context, def *Definition) error
	GetByID(ctx context.Context, workflowID uuid.UUID) (*Definition, error)
	GetByIDAndVersion(ctx context.Context, workflowID uuid.UUID, version int) (*Definition, error)
	List(ctx context.Context, limit, offset int) ([]*Definition, error)
	ListVersions(ctx context.Context, workflowID uuid.UUID) ([]*Definition, error)
	UpdateStatus(ctx context.Context, workflowID uuid.UUID, version int, status Status, updatedBy *string) error
}
