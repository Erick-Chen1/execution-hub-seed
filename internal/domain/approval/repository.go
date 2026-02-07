package approval

import (
	"context"

	"github.com/google/uuid"
)

// Filter controls approval listing.
type Filter struct {
	Status      *Status
	EntityType  *string
	EntityID    *string
	Operation   *Operation
	RequestedBy *string
}

// Repository defines persistence for approvals.
type Repository interface {
	Create(ctx context.Context, approval *Approval) error
	Update(ctx context.Context, approval *Approval) error
	GetByID(ctx context.Context, approvalID uuid.UUID) (*Approval, error)
	List(ctx context.Context, filter Filter, limit, offset int) ([]*Approval, error)
	CreateDecision(ctx context.Context, decision *DecisionRecord) error
	ListDecisions(ctx context.Context, approvalID uuid.UUID) ([]*DecisionRecord, error)
	CountDecisions(ctx context.Context, approvalID uuid.UUID) (approvals int, rejections int, err error)
	HasDecision(ctx context.Context, approvalID uuid.UUID, decidedBy string) (bool, error)
}
