package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/workflow"
)

// WorkflowRepository implements workflow.Repository.
type WorkflowRepository struct {
	pool *pgxpool.Pool
}

func NewWorkflowRepository(pool *pgxpool.Pool) *WorkflowRepository {
	return &WorkflowRepository{pool: pool}
}

func (r *WorkflowRepository) Create(ctx context.Context, def *workflow.Definition) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workflow_definitions
		(workflow_id, name, version, description, status, definition, created_at, created_by, updated_at, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, def.WorkflowID, def.Name, def.Version, def.Description, def.Status, def.Definition, def.CreatedAt, def.CreatedBy, def.UpdatedAt, def.UpdatedBy)
	return err
}

func (r *WorkflowRepository) GetByID(ctx context.Context, workflowID uuid.UUID) (*workflow.Definition, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, workflow_id, name, version, description, status, definition, created_at, created_by, updated_at, updated_by
		FROM workflow_definitions
		WHERE workflow_id=$1
		ORDER BY version DESC
		LIMIT 1
	`, workflowID)
	return scanWorkflow(row)
}

func (r *WorkflowRepository) GetByIDAndVersion(ctx context.Context, workflowID uuid.UUID, version int) (*workflow.Definition, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, workflow_id, name, version, description, status, definition, created_at, created_by, updated_at, updated_by
		FROM workflow_definitions
		WHERE workflow_id=$1 AND version=$2
	`, workflowID, version)
	return scanWorkflow(row)
}

func (r *WorkflowRepository) List(ctx context.Context, limit, offset int) ([]*workflow.Definition, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workflow_id, name, version, description, status, definition, created_at, created_by, updated_at, updated_by
		FROM workflow_definitions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var defs []*workflow.Definition
	for rows.Next() {
		def, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, rows.Err()
}

func (r *WorkflowRepository) ListVersions(ctx context.Context, workflowID uuid.UUID) ([]*workflow.Definition, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workflow_id, name, version, description, status, definition, created_at, created_by, updated_at, updated_by
		FROM workflow_definitions
		WHERE workflow_id=$1
		ORDER BY version DESC
	`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var defs []*workflow.Definition
	for rows.Next() {
		def, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, rows.Err()
}

func (r *WorkflowRepository) UpdateStatus(ctx context.Context, workflowID uuid.UUID, version int, status workflow.Status, updatedBy *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE workflow_definitions
		SET status=$1, updated_at=NOW(), updated_by=$2
		WHERE workflow_id=$3 AND version=$4
	`, status, updatedBy, workflowID, version)
	return err
}

func scanWorkflow(row pgx.Row) (*workflow.Definition, error) {
	var def workflow.Definition
	var desc *string
	var createdBy *string
	var updatedBy *string
	var definition json.RawMessage
	if err := row.Scan(&def.ID, &def.WorkflowID, &def.Name, &def.Version, &desc, &def.Status, &definition, &def.CreatedAt, &createdBy, &def.UpdatedAt, &updatedBy); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if desc != nil {
		def.Description = *desc
	}
	def.Definition = definition
	def.CreatedBy = createdBy
	def.UpdatedBy = updatedBy
	return &def, nil
}
