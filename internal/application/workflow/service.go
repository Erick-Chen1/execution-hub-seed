package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/execution-hub/execution-hub/internal/domain/workflow"
)

// Service handles workflow definition operations.
type Service struct {
	repo   workflow.Repository
	logger zerolog.Logger
}

// NewService creates a workflow service.
func NewService(repo workflow.Repository, logger zerolog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger.With().Str("service", "workflow").Logger(),
	}
}

// CreateDefinition creates a new workflow definition version.
func (s *Service) CreateDefinition(ctx context.Context, spec workflow.Spec, createdBy *string) (*workflow.Definition, error) {
	if err := workflow.ValidateSpec(&spec); err != nil {
		return nil, err
	}

	data, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow spec: %w", err)
	}

	def := &workflow.Definition{
		WorkflowID:  spec.WorkflowID,
		Name:        spec.Name,
		Version:     spec.Version,
		Description: spec.Description,
		Status:      workflow.StatusInactive,
		Definition:  data,
		CreatedAt:   time.Now().UTC(),
		CreatedBy:   createdBy,
		UpdatedAt:   time.Now().UTC(),
		UpdatedBy:   createdBy,
	}

	if def.WorkflowID == uuid.Nil {
		def.WorkflowID = uuid.New()
		spec.WorkflowID = def.WorkflowID
	}
	if def.Version == 0 {
		// auto-increment if workflow exists
		existing, err := s.repo.GetByID(ctx, def.WorkflowID)
		if err == nil && existing != nil {
			def.Version = existing.Version + 1
		} else {
			def.Version = 1
		}
		spec.Version = def.Version
	}

	if err := s.repo.Create(ctx, def); err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	s.logger.Info().
		Str("workflow_id", def.WorkflowID.String()).
		Int("version", def.Version).
		Msg("workflow definition created")

	return def, nil
}

// ListDefinitions lists workflows.
func (s *Service) ListDefinitions(ctx context.Context, limit, offset int) ([]*workflow.Definition, error) {
	return s.repo.List(ctx, limit, offset)
}

// GetDefinition retrieves a workflow by id (latest active/inactive).
func (s *Service) GetDefinition(ctx context.Context, workflowID uuid.UUID) (*workflow.Definition, error) {
	def, err := s.repo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	if def == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	return def, nil
}

// ListVersions lists all versions for a workflow.
func (s *Service) ListVersions(ctx context.Context, workflowID uuid.UUID) ([]*workflow.Definition, error) {
	return s.repo.ListVersions(ctx, workflowID)
}

// Activate sets a workflow version to ACTIVE.
func (s *Service) Activate(ctx context.Context, workflowID uuid.UUID, version int, updatedBy *string) error {
	if err := s.repo.UpdateStatus(ctx, workflowID, version, workflow.StatusActive, updatedBy); err != nil {
		return fmt.Errorf("failed to activate workflow: %w", err)
	}
	return nil
}

// Deactivate sets a workflow version to INACTIVE.
func (s *Service) Deactivate(ctx context.Context, workflowID uuid.UUID, version int, updatedBy *string) error {
	if err := s.repo.UpdateStatus(ctx, workflowID, version, workflow.StatusInactive, updatedBy); err != nil {
		return fmt.Errorf("failed to deactivate workflow: %w", err)
	}
	return nil
}
