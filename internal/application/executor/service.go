package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/execution-hub/execution-hub/internal/domain/executor"
)

// Service handles executor operations.
type Service struct {
	repo   executor.Repository
	logger zerolog.Logger
}

func NewService(repo executor.Repository, logger zerolog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger.With().Str("service", "executor").Logger(),
	}
}

func (s *Service) Create(ctx context.Context, exec *executor.Executor) error {
	if exec.ExecutorID == "" {
		return fmt.Errorf("executorId is required")
	}
	if exec.ExecutorType == "" {
		return fmt.Errorf("executorType is required")
	}
	if exec.Status == "" {
		exec.Status = executor.StatusActive
	}
	now := time.Now().UTC()
	if exec.CreatedAt.IsZero() {
		exec.CreatedAt = now
	}
	exec.UpdatedAt = now
	return s.repo.Create(ctx, exec)
}

func (s *Service) Get(ctx context.Context, executorID string) (*executor.Executor, error) {
	exec, err := s.repo.GetByID(ctx, executorID)
	if err != nil {
		return nil, err
	}
	if exec == nil {
		return nil, fmt.Errorf("executor not found: %s", executorID)
	}
	return exec, nil
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]*executor.Executor, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *Service) Activate(ctx context.Context, executorID string) error {
	return s.repo.UpdateStatus(ctx, executorID, executor.StatusActive)
}

func (s *Service) Deactivate(ctx context.Context, executorID string) error {
	return s.repo.UpdateStatus(ctx, executorID, executor.StatusInactive)
}
