package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/executor"
)

// ExecutorRepository implements executor.Repository.
type ExecutorRepository struct {
	pool *pgxpool.Pool
}

func NewExecutorRepository(pool *pgxpool.Pool) *ExecutorRepository {
	return &ExecutorRepository{pool: pool}
}

func (r *ExecutorRepository) Create(ctx context.Context, exec *executor.Executor) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO executors (executor_id, executor_type, display_name, capability_tags, status, metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, exec.ExecutorID, exec.ExecutorType, exec.DisplayName, exec.Capabilities, exec.Status, exec.Metadata, exec.CreatedAt, exec.UpdatedAt)
	return err
}

func (r *ExecutorRepository) GetByID(ctx context.Context, executorID string) (*executor.Executor, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT executor_id, executor_type, display_name, capability_tags, status, metadata, created_at, updated_at
		FROM executors WHERE executor_id=$1
	`, executorID)
	return scanExecutor(row)
}

func (r *ExecutorRepository) List(ctx context.Context, limit, offset int) ([]*executor.Executor, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT executor_id, executor_type, display_name, capability_tags, status, metadata, created_at, updated_at
		FROM executors ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*executor.Executor
	for rows.Next() {
		exec, err := scanExecutor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, exec)
	}
	return out, rows.Err()
}

func (r *ExecutorRepository) Update(ctx context.Context, exec *executor.Executor) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE executors SET executor_type=$1, display_name=$2, capability_tags=$3, status=$4, metadata=$5, updated_at=$6
		WHERE executor_id=$7
	`, exec.ExecutorType, exec.DisplayName, exec.Capabilities, exec.Status, exec.Metadata, exec.UpdatedAt, exec.ExecutorID)
	return err
}

func (r *ExecutorRepository) UpdateStatus(ctx context.Context, executorID string, status executor.Status) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE executors SET status=$1, updated_at=NOW() WHERE executor_id=$2
	`, status, executorID)
	return err
}

func scanExecutor(row pgx.Row) (*executor.Executor, error) {
	var exec executor.Executor
	if err := row.Scan(&exec.ExecutorID, &exec.ExecutorType, &exec.DisplayName, &exec.Capabilities, &exec.Status, &exec.Metadata, &exec.CreatedAt, &exec.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &exec, nil
}
