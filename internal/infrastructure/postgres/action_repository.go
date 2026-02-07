package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainAction "github.com/execution-hub/execution-hub/internal/domain/action"
)

// ActionRepository implements action.Repository.
type ActionRepository struct {
	pool *pgxpool.Pool
}

func NewActionRepository(pool *pgxpool.Pool) *ActionRepository {
	return &ActionRepository{pool: pool}
}

func (r *ActionRepository) Create(ctx context.Context, action *domainAction.Action) error {
	var ttlSeconds *int
	if action.TTL != nil {
		t := int(action.TTL.Seconds())
		ttlSeconds = &t
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO actions
		(action_id, rule_id, rule_version, evaluation_id, action_type, action_config, status, dedupe_key, cooldown_until, priority, ttl_seconds, retry_count, max_retries, last_error, created_at, dispatched_at, acked_at, acked_by, resolved_at, resolved_by, failed_at, trace_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
	`, action.ActionID, action.RuleID, action.RuleVersion, action.EvaluationID, action.ActionType, action.ActionConfig, action.Status, action.DedupeKey, action.CooldownUntil, action.Priority, ttlSeconds, action.RetryCount, action.MaxRetries, action.LastError, action.CreatedAt, action.DispatchedAt, action.AckedAt, action.AckedBy, action.ResolvedAt, action.ResolvedBy, action.FailedAt, action.TraceID)
	return err
}

func (r *ActionRepository) GetByID(ctx context.Context, actionID uuid.UUID) (*domainAction.Action, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, action_id, rule_id, rule_version, evaluation_id, action_type, action_config, status, dedupe_key, cooldown_until, priority, ttl_seconds, retry_count, max_retries, last_error, created_at, dispatched_at, acked_at, acked_by, resolved_at, resolved_by, failed_at, trace_id
		FROM actions WHERE action_id=$1
	`, actionID)
	return scanAction(row)
}

func (r *ActionRepository) GetByEvaluationID(ctx context.Context, evaluationID uuid.UUID) (*domainAction.Action, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, action_id, rule_id, rule_version, evaluation_id, action_type, action_config, status, dedupe_key, cooldown_until, priority, ttl_seconds, retry_count, max_retries, last_error, created_at, dispatched_at, acked_at, acked_by, resolved_at, resolved_by, failed_at, trace_id
		FROM actions WHERE evaluation_id=$1
	`, evaluationID)
	return scanAction(row)
}

func (r *ActionRepository) List(ctx context.Context, filter domainAction.Filter, limit, offset int) ([]*domainAction.Action, error) {
	query := `SELECT id, action_id, rule_id, rule_version, evaluation_id, action_type, action_config, status, dedupe_key, cooldown_until, priority, ttl_seconds, retry_count, max_retries, last_error, created_at, dispatched_at, acked_at, acked_by, resolved_at, resolved_by, failed_at, trace_id FROM actions`
	args := []interface{}{}
	idx := 1
	if filter.RuleID != nil {
		query += " WHERE rule_id=$" + itoa(idx)
		args = append(args, *filter.RuleID)
		idx++
	}
	if filter.EvaluationID != nil {
		query += addWhere(query) + " evaluation_id=$" + itoa(idx)
		args = append(args, *filter.EvaluationID)
		idx++
	}
	if filter.Status != nil {
		query += addWhere(query) + " status=$" + itoa(idx)
		args = append(args, *filter.Status)
		idx++
	}
	if filter.Priority != nil {
		query += addWhere(query) + " priority=$" + itoa(idx)
		args = append(args, *filter.Priority)
		idx++
	}
	if filter.Since != nil {
		query += addWhere(query) + " created_at >= $" + itoa(idx)
		args = append(args, *filter.Since)
		idx++
	}
	if filter.Until != nil {
		query += addWhere(query) + " created_at <= $" + itoa(idx)
		args = append(args, *filter.Until)
		idx++
	}
	query += " ORDER BY created_at DESC LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var actions []*domainAction.Action
	for rows.Next() {
		action, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}

func (r *ActionRepository) Update(ctx context.Context, action *domainAction.Action) error {
	var ttlSeconds *int
	if action.TTL != nil {
		t := int(action.TTL.Seconds())
		ttlSeconds = &t
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE actions
		SET rule_id=$1, rule_version=$2, evaluation_id=$3, action_type=$4, action_config=$5, status=$6, dedupe_key=$7, cooldown_until=$8, priority=$9, ttl_seconds=$10, retry_count=$11, max_retries=$12, last_error=$13, dispatched_at=$14, acked_at=$15, acked_by=$16, resolved_at=$17, resolved_by=$18, failed_at=$19, trace_id=$20
		WHERE action_id=$21
	`, action.RuleID, action.RuleVersion, action.EvaluationID, action.ActionType, action.ActionConfig, action.Status, action.DedupeKey, action.CooldownUntil, action.Priority, ttlSeconds, action.RetryCount, action.MaxRetries, action.LastError, action.DispatchedAt, action.AckedAt, action.AckedBy, action.ResolvedAt, action.ResolvedBy, action.FailedAt, action.TraceID, action.ActionID)
	return err
}

func (r *ActionRepository) UpdateStatus(ctx context.Context, actionID uuid.UUID, status domainAction.Status) error {
	_, err := r.pool.Exec(ctx, `UPDATE actions SET status=$1 WHERE action_id=$2`, status, actionID)
	return err
}

func (r *ActionRepository) FindByDedupeKey(ctx context.Context, dedupeKey string, since time.Time) (*domainAction.Action, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, action_id, rule_id, rule_version, evaluation_id, action_type, action_config, status, dedupe_key, cooldown_until, priority, ttl_seconds, retry_count, max_retries, last_error, created_at, dispatched_at, acked_at, acked_by, resolved_at, resolved_by, failed_at, trace_id
		FROM actions WHERE dedupe_key=$1 AND created_at >= $2
		ORDER BY created_at DESC LIMIT 1
	`, dedupeKey, since)
	return scanAction(row)
}

func (r *ActionRepository) RecordTransition(ctx context.Context, transition *domainAction.StateTransition) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO action_state_transitions (action_id, from_status, to_status, transitioned_at, reason, metadata)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, transition.ActionID, transition.FromStatus, transition.ToStatus, transition.TransitionedAt, transition.Reason, transition.Metadata)
	return err
}

func (r *ActionRepository) GetTransitions(ctx context.Context, actionID uuid.UUID) ([]*domainAction.StateTransition, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, action_id, from_status, to_status, transitioned_at, reason, metadata
		FROM action_state_transitions WHERE action_id=$1 ORDER BY transitioned_at ASC
	`, actionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var transitions []*domainAction.StateTransition
	for rows.Next() {
		var tr domainAction.StateTransition
		if err := rows.Scan(&tr.ID, &tr.ActionID, &tr.FromStatus, &tr.ToStatus, &tr.TransitionedAt, &tr.Reason, &tr.Metadata); err != nil {
			return nil, err
		}
		transitions = append(transitions, &tr)
	}
	return transitions, rows.Err()
}

func (r *ActionRepository) ListRetryableActions(ctx context.Context, limit int) ([]*domainAction.Action, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, action_id, rule_id, rule_version, evaluation_id, action_type, action_config, status, dedupe_key, cooldown_until, priority, ttl_seconds, retry_count, max_retries, last_error, created_at, dispatched_at, acked_at, acked_by, resolved_at, resolved_by, failed_at, trace_id
		FROM actions
		WHERE status='FAILED' AND retry_count < max_retries
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var actions []*domainAction.Action
	for rows.Next() {
		action, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}

func scanAction(row pgx.Row) (*domainAction.Action, error) {
	var a domainAction.Action
	var ttlSeconds *int
	if err := row.Scan(&a.ID, &a.ActionID, &a.RuleID, &a.RuleVersion, &a.EvaluationID, &a.ActionType, &a.ActionConfig, &a.Status, &a.DedupeKey, &a.CooldownUntil, &a.Priority, &ttlSeconds, &a.RetryCount, &a.MaxRetries, &a.LastError, &a.CreatedAt, &a.DispatchedAt, &a.AckedAt, &a.AckedBy, &a.ResolvedAt, &a.ResolvedBy, &a.FailedAt, &a.TraceID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if ttlSeconds != nil {
		t := time.Duration(*ttlSeconds) * time.Second
		a.TTL = &t
	}
	return &a, nil
}

func addWhere(query string) string {
	if strings.Contains(query, " WHERE ") {
		return " AND"
	}
	return " WHERE"
}
