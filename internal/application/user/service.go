package user

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	domain "github.com/execution-hub/execution-hub/internal/domain/user"
)

// Service handles user management.
type Service struct {
	repo   domain.Repository
	logger zerolog.Logger
}

// NewService creates a user service.
func NewService(repo domain.Repository, logger zerolog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger.With().Str("service", "user").Logger(),
	}
}

// CreateInput defines user creation input.
type CreateInput struct {
	Username    string
	Password    string
	Role        domain.Role
	Type        domain.Type
	OwnerUserID *uuid.UUID
	Status      domain.Status
}

// UpdateInput defines user update input.
type UpdateInput struct {
	Username    *string
	Role        *domain.Role
	Status      *domain.Status
	OwnerUserID *uuid.UUID
}

func (s *Service) CreateUser(ctx context.Context, input CreateInput) (*domain.User, error) {
	username := domain.NormalizeUsername(input.Username)
	if err := domain.ValidateUsername(username); err != nil {
		return nil, err
	}
	if err := domain.ValidatePassword(input.Password, username); err != nil {
		return nil, err
	}
	if err := domain.ValidateRole(input.Role); err != nil {
		return nil, err
	}
	if err := domain.ValidateType(input.Type); err != nil {
		return nil, err
	}
	if input.Status == "" {
		input.Status = domain.StatusActive
	}
	if err := domain.ValidateStatus(input.Status); err != nil {
		return nil, err
	}
	if input.Type == domain.TypeAgent && input.OwnerUserID == nil {
		return nil, fmt.Errorf("owner_user_id is required for agent")
	}
	if input.Type == domain.TypeHuman && input.OwnerUserID != nil {
		return nil, fmt.Errorf("owner_user_id is not allowed for human user")
	}

	hash, err := domain.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	u := &domain.User{
		UserID:       uuid.New(),
		Username:     username,
		PasswordHash: hash,
		Role:         input.Role,
		Type:         input.Type,
		OwnerUserID:  input.OwnerUserID,
		Status:       input.Status,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	s.logger.Info().Str("user_id", u.UserID.String()).Str("username", u.Username).Msg("user created")
	return u, nil
}

func (s *Service) UpdateUser(ctx context.Context, userID uuid.UUID, input UpdateInput) (*domain.User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("user not found: %s", userID)
	}
	if input.Username != nil {
		username := domain.NormalizeUsername(*input.Username)
		if err := domain.ValidateUsername(username); err != nil {
			return nil, err
		}
		u.Username = username
	}
	if input.Role != nil {
		if err := domain.ValidateRole(*input.Role); err != nil {
			return nil, err
		}
		u.Role = *input.Role
	}
	if input.Status != nil {
		if err := domain.ValidateStatus(*input.Status); err != nil {
			return nil, err
		}
		u.Status = *input.Status
	}
	if u.Type == domain.TypeAgent {
		if input.OwnerUserID != nil {
			u.OwnerUserID = input.OwnerUserID
		}
		if u.OwnerUserID == nil {
			return nil, fmt.Errorf("owner_user_id is required for agent")
		}
	} else if input.OwnerUserID != nil {
		return nil, fmt.Errorf("owner_user_id is not allowed for human user")
	}
	u.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) SetPassword(ctx context.Context, userID uuid.UUID, password string) error {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u == nil {
		return fmt.Errorf("user not found: %s", userID)
	}
	if err := domain.ValidatePassword(password, u.Username); err != nil {
		return err
	}
	hash, err := domain.HashPassword(password)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	u.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, u)
}

func (s *Service) GetUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.repo.GetByID(ctx, userID)
}

func (s *Service) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return s.repo.GetByUsername(ctx, domain.NormalizeUsername(username))
}

func (s *Service) ListUsers(ctx context.Context, filter domain.Filter, limit, offset int) ([]*domain.User, error) {
	return s.repo.List(ctx, filter, limit, offset)
}

func (s *Service) Count(ctx context.Context) (int, error) {
	return s.repo.Count(ctx)
}
