package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/task"
)

// TaskRepository implements task.Repository.
type TaskRepository struct {
	pool *pgxpool.Pool
}

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{pool: pool}
}

func (r *TaskRepository) Create(ctx context.Context, t *task.Task) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tasks (task_id, workflow_id, workflow_version, title, status, context, trace_id, created_at, created_by, updated_at, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, t.TaskID, t.WorkflowID, t.WorkflowVersion, t.Title, t.Status, t.Context, t.TraceID, t.CreatedAt, t.CreatedBy, t.UpdatedAt, t.UpdatedBy)
	return err
}

func (r *TaskRepository) GetByID(ctx context.Context, taskID uuid.UUID) (*task.Task, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, task_id, workflow_id, workflow_version, title, status, context, trace_id, created_at, created_by, updated_at, updated_by
		FROM tasks WHERE task_id=$1
	`, taskID)
	var t task.Task
	var ctxData json.RawMessage
	var createdBy *string
	var updatedBy *string
	if err := row.Scan(&t.ID, &t.TaskID, &t.WorkflowID, &t.WorkflowVersion, &t.Title, &t.Status, &ctxData, &t.TraceID, &t.CreatedAt, &createdBy, &t.UpdatedAt, &updatedBy); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(ctxData) > 0 {
		t.Context = ctxData
	}
	t.CreatedBy = createdBy
	t.UpdatedBy = updatedBy
	return &t, nil
}

func (r *TaskRepository) List(ctx context.Context, status *task.Status, limit, offset int) ([]*task.Task, error) {
	query := `
		SELECT id, task_id, workflow_id, workflow_version, title, status, context, trace_id, created_at, created_by, updated_at, updated_by
		FROM tasks`
	args := []interface{}{}
	if status != nil {
		query += " WHERE status=$1"
		args = append(args, *status)
	}
	query += " ORDER BY created_at DESC LIMIT $" + itoa(len(args)+1) + " OFFSET $" + itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []*task.Task
	for rows.Next() {
		var t task.Task
		var ctxData json.RawMessage
		var createdBy *string
		var updatedBy *string
		if err := rows.Scan(&t.ID, &t.TaskID, &t.WorkflowID, &t.WorkflowVersion, &t.Title, &t.Status, &ctxData, &t.TraceID, &t.CreatedAt, &createdBy, &t.UpdatedAt, &updatedBy); err != nil {
			return nil, err
		}
		if len(ctxData) > 0 {
			t.Context = ctxData
		}
		t.CreatedBy = createdBy
		t.UpdatedBy = updatedBy
		tasks = append(tasks, &t)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) Update(ctx context.Context, t *task.Task) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tasks SET workflow_id=$1, workflow_version=$2, title=$3, status=$4, context=$5, trace_id=$6, updated_at=$7, updated_by=$8
		WHERE task_id=$9
	`, t.WorkflowID, t.WorkflowVersion, t.Title, t.Status, t.Context, t.TraceID, t.UpdatedAt, t.UpdatedBy, t.TaskID)
	return err
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, taskID uuid.UUID, status task.Status) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tasks SET status=$1, updated_at=NOW() WHERE task_id=$2
	`, status, taskID)
	return err
}

// StepRepository implements task.StepRepository.
type StepRepository struct {
	pool *pgxpool.Pool
}

func NewStepRepository(pool *pgxpool.Pool) *StepRepository {
	return &StepRepository{pool: pool}
}

func (r *StepRepository) Create(ctx context.Context, step *task.Step) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO task_steps
		(step_id, task_id, trace_id, step_key, name, status, executor_type, executor_ref, action_type, action_config, timeout_seconds, retry_count, max_retries, depends_on, action_id, on_fail, evidence, created_at, dispatched_at, acked_at, resolved_at, failed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		ON CONFLICT (task_id, step_key) DO NOTHING
	`, step.StepID, step.TaskID, step.TraceID, step.StepKey, step.Name, step.Status, step.ExecutorType, step.ExecutorRef, step.ActionType, step.ActionConfig, step.TimeoutSeconds, step.RetryCount, step.MaxRetries, step.DependsOn, step.ActionID, step.OnFail, step.Evidence, step.CreatedAt, step.DispatchedAt, step.AckedAt, step.ResolvedAt, step.FailedAt)
	return err
}

func (r *StepRepository) GetByID(ctx context.Context, stepID uuid.UUID) (*task.Step, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, step_id, task_id, trace_id, step_key, name, status, executor_type, executor_ref, action_type, action_config, timeout_seconds, retry_count, max_retries, depends_on, action_id, on_fail, evidence, created_at, dispatched_at, acked_at, resolved_at, failed_at
		FROM task_steps WHERE step_id=$1
	`, stepID)
	return scanStep(row)
}

func (r *StepRepository) GetByActionID(ctx context.Context, actionID uuid.UUID) (*task.Step, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, step_id, task_id, trace_id, step_key, name, status, executor_type, executor_ref, action_type, action_config, timeout_seconds, retry_count, max_retries, depends_on, action_id, on_fail, evidence, created_at, dispatched_at, acked_at, resolved_at, failed_at
		FROM task_steps WHERE action_id=$1
	`, actionID)
	return scanStep(row)
}

func (r *StepRepository) ListByTask(ctx context.Context, taskID uuid.UUID) ([]*task.Step, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, step_id, task_id, trace_id, step_key, name, status, executor_type, executor_ref, action_type, action_config, timeout_seconds, retry_count, max_retries, depends_on, action_id, on_fail, evidence, created_at, dispatched_at, acked_at, resolved_at, failed_at
		FROM task_steps WHERE task_id=$1 ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []*task.Step
	for rows.Next() {
		step, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *StepRepository) ListByStatus(ctx context.Context, status task.StepStatus, limit int) ([]*task.Step, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, step_id, task_id, trace_id, step_key, name, status, executor_type, executor_ref, action_type, action_config, timeout_seconds, retry_count, max_retries, depends_on, action_id, on_fail, evidence, created_at, dispatched_at, acked_at, resolved_at, failed_at
		FROM task_steps WHERE status=$1 ORDER BY created_at ASC LIMIT $2
	`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []*task.Step
	for rows.Next() {
		step, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *StepRepository) Update(ctx context.Context, step *task.Step) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE task_steps
		SET trace_id=$1, status=$2, executor_type=$3, executor_ref=$4, action_type=$5, action_config=$6, timeout_seconds=$7, retry_count=$8, max_retries=$9, depends_on=$10, action_id=$11, on_fail=$12, evidence=$13,
			dispatched_at=$14, acked_at=$15, resolved_at=$16, failed_at=$17
		WHERE step_id=$18
	`, step.TraceID, step.Status, step.ExecutorType, step.ExecutorRef, step.ActionType, step.ActionConfig, step.TimeoutSeconds, step.RetryCount, step.MaxRetries, step.DependsOn, step.ActionID, step.OnFail, step.Evidence,
		step.DispatchedAt, step.AckedAt, step.ResolvedAt, step.FailedAt, step.StepID)
	return err
}

func (r *StepRepository) UpdateStatus(ctx context.Context, stepID uuid.UUID, status task.StepStatus) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE task_steps SET status=$1 WHERE step_id=$2
	`, status, stepID)
	return err
}

func (r *StepRepository) ListTimedOut(ctx context.Context, now time.Time, limit int) ([]*task.Step, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, step_id, task_id, trace_id, step_key, name, status, executor_type, executor_ref, action_type, action_config, timeout_seconds, retry_count, max_retries, depends_on, action_id, on_fail, evidence, created_at, dispatched_at, acked_at, resolved_at, failed_at
		FROM task_steps
		WHERE status IN ('DISPATCHED','ACKED')
		AND timeout_seconds > 0
		AND dispatched_at IS NOT NULL
		AND dispatched_at + (timeout_seconds || ' seconds')::interval < $1
		ORDER BY dispatched_at ASC
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []*task.Step
	for rows.Next() {
		step, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func scanStep(row pgx.Row) (*task.Step, error) {
	var s task.Step
	var deps []uuid.UUID
	var actionConfig json.RawMessage
	var onFail json.RawMessage
	var evidence json.RawMessage
	if err := row.Scan(&s.ID, &s.StepID, &s.TaskID, &s.TraceID, &s.StepKey, &s.Name, &s.Status, &s.ExecutorType, &s.ExecutorRef, &s.ActionType, &actionConfig, &s.TimeoutSeconds, &s.RetryCount, &s.MaxRetries, &deps, &s.ActionID, &onFail, &evidence, &s.CreatedAt, &s.DispatchedAt, &s.AckedAt, &s.ResolvedAt, &s.FailedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.ActionConfig = actionConfig
	s.OnFail = onFail
	s.Evidence = evidence
	s.DependsOn = deps
	return &s, nil
}

func itoa(i int) string {
	return fmtInt(i)
}

func fmtInt(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	buf := make([]byte, 0, 12)
	for i > 0 {
		d := byte(i % 10)
		buf = append([]byte{d + '0'}, buf...)
		i /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
