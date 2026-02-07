package user

import (
	"context"

	"github.com/google/uuid"
)

// Filter controls user listing.
type Filter struct {
	Role        *Role
	Type        *Type
	Status      *Status
	OwnerUserID *uuid.UUID
	Username    *string
}

// Repository defines persistence for users.
type Repository interface {
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	GetByID(ctx context.Context, userID uuid.UUID) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	List(ctx context.Context, filter Filter, limit, offset int) ([]*User, error)
	Count(ctx context.Context) (int, error)
}
