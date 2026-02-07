package task

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines task persistence.
type Repository interface {
	Create(ctx context.Context, task *Task) error
	GetByID(ctx context.Context, taskID uuid.UUID) (*Task, error)
	List(ctx context.Context, status *Status, limit, offset int) ([]*Task, error)
	Update(ctx context.Context, task *Task) error
	UpdateStatus(ctx context.Context, taskID uuid.UUID, status Status) error
}

// StepRepository defines step persistence.
type StepRepository interface {
	Create(ctx context.Context, step *Step) error
	GetByID(ctx context.Context, stepID uuid.UUID) (*Step, error)
	GetByActionID(ctx context.Context, actionID uuid.UUID) (*Step, error)
	ListByTask(ctx context.Context, taskID uuid.UUID) ([]*Step, error)
	ListByStatus(ctx context.Context, status StepStatus, limit int) ([]*Step, error)
	Update(ctx context.Context, step *Step) error
	UpdateStatus(ctx context.Context, stepID uuid.UUID, status StepStatus) error
	ListTimedOut(ctx context.Context, now time.Time, limit int) ([]*Step, error)
}
