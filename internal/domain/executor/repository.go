package executor

import (
	"context"
)

// Repository defines executor persistence.
type Repository interface {
	Create(ctx context.Context, exec *Executor) error
	GetByID(ctx context.Context, executorID string) (*Executor, error)
	List(ctx context.Context, limit, offset int) ([]*Executor, error)
	Update(ctx context.Context, exec *Executor) error
	UpdateStatus(ctx context.Context, executorID string, status Status) error
}
