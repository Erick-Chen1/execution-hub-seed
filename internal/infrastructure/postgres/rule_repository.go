package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/rule"
)

// RuleRepository implements rule.Repository.
type RuleRepository struct {
	pool *pgxpool.Pool
}

func NewRuleRepository(pool *pgxpool.Pool) *RuleRepository {
	return &RuleRepository{pool: pool}
}

func (r *RuleRepository) Create(ctx context.Context, rl *rule.Rule) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO rules
		(rule_id, version, name, description, rule_type, config, action_type, action_config, scope_factory_id, scope_line_id, effective_from, effective_until, status, created_at, created_by, updated_at, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`, rl.RuleID, rl.Version, rl.Name, rl.Description, rl.RuleType, rl.Config, rl.ActionType, rl.ActionConfig, rl.ScopeFactoryID, rl.ScopeLineID, rl.EffectiveFrom, rl.EffectiveUntil, rl.Status, rl.CreatedAt, rl.CreatedBy, rl.UpdatedAt, rl.UpdatedBy)
	return err
}

func (r *RuleRepository) GetByRuleID(ctx context.Context, ruleID uuid.UUID) (*rule.Rule, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, rule_id, version, name, description, rule_type, config, action_type, action_config, scope_factory_id, scope_line_id, effective_from, effective_until, status, created_at, created_by, updated_at, updated_by
		FROM rules WHERE rule_id=$1 ORDER BY version DESC LIMIT 1
	`, ruleID)
	return scanRule(row)
}

func (r *RuleRepository) GetByRuleIDAndVersion(ctx context.Context, ruleID uuid.UUID, version int) (*rule.Rule, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, rule_id, version, name, description, rule_type, config, action_type, action_config, scope_factory_id, scope_line_id, effective_from, effective_until, status, created_at, created_by, updated_at, updated_by
		FROM rules WHERE rule_id=$1 AND version=$2
	`, ruleID, version)
	return scanRule(row)
}

func (r *RuleRepository) GetByID(ctx context.Context, id int64) (*rule.Rule, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, rule_id, version, name, description, rule_type, config, action_type, action_config, scope_factory_id, scope_line_id, effective_from, effective_until, status, created_at, created_by, updated_at, updated_by
		FROM rules WHERE id=$1
	`, id)
	return scanRule(row)
}

func (r *RuleRepository) ListActiveRules(ctx context.Context, filter rule.Filter) ([]*rule.Rule, error) {
	query := `SELECT id, rule_id, version, name, description, rule_type, config, action_type, action_config, scope_factory_id, scope_line_id, effective_from, effective_until, status, created_at, created_by, updated_at, updated_by FROM rules`
	args := []interface{}{}
	idx := 1
	if filter.RuleType != nil {
		query += " WHERE rule_type=$" + itoa(idx)
		args = append(args, *filter.RuleType)
		idx++
	}
	if filter.Status != nil {
		query += addWhere(query) + " status=$" + itoa(idx)
		args = append(args, *filter.Status)
		idx++
	} else {
		query += addWhere(query) + " status='ACTIVE'"
	}
	if filter.ScopeFactoryID != nil {
		query += addWhere(query) + " scope_factory_id=$" + itoa(idx)
		args = append(args, *filter.ScopeFactoryID)
		idx++
	}
	if filter.ScopeLineID != nil {
		query += addWhere(query) + " scope_line_id=$" + itoa(idx)
		args = append(args, *filter.ScopeLineID)
		idx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []*rule.Rule
	for rows.Next() {
		rl, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rl)
	}
	return rules, rows.Err()
}

func (r *RuleRepository) ListVersions(ctx context.Context, ruleID uuid.UUID) ([]*rule.Rule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, rule_id, version, name, description, rule_type, config, action_type, action_config, scope_factory_id, scope_line_id, effective_from, effective_until, status, created_at, created_by, updated_at, updated_by
		FROM rules WHERE rule_id=$1 ORDER BY version DESC
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []*rule.Rule
	for rows.Next() {
		rl, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rl)
	}
	return rules, rows.Err()
}

func (r *RuleRepository) UpdateStatus(ctx context.Context, id int64, status rule.RuleStatus, updatedBy *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE rules SET status=$1, updated_at=NOW(), updated_by=$2 WHERE id=$3
	`, status, updatedBy, id)
	return err
}

func (r *RuleRepository) CreateEvaluation(ctx context.Context, eval *rule.Evaluation) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO rule_evaluations
		(evaluation_id, rule_id, rule_version, rule_type, matched, evaluated_at, evidence, event_ids, trace_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, eval.EvaluationID, eval.RuleID, eval.RuleVersion, eval.RuleType, eval.Matched, eval.EvaluatedAt, eval.Evidence, eval.EventIDs, eval.TraceID)
	return err
}

func (r *RuleRepository) GetEvaluationByID(ctx context.Context, evaluationID uuid.UUID) (*rule.Evaluation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, evaluation_id, rule_id, rule_version, rule_type, matched, evaluated_at, evidence, event_ids, trace_id
		FROM rule_evaluations WHERE evaluation_id=$1
	`, evaluationID)
	return scanEvaluation(row)
}

func (r *RuleRepository) ListEvaluationsByRule(ctx context.Context, ruleID uuid.UUID, limit int) ([]*rule.Evaluation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, evaluation_id, rule_id, rule_version, rule_type, matched, evaluated_at, evidence, event_ids, trace_id
		FROM rule_evaluations WHERE rule_id=$1 ORDER BY evaluated_at DESC LIMIT $2
	`, ruleID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*rule.Evaluation
	for rows.Next() {
		eval, err := scanEvaluation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, eval)
	}
	return out, rows.Err()
}

func (r *RuleRepository) ListMatchedEvaluations(ctx context.Context, since time.Time, limit int) ([]*rule.Evaluation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, evaluation_id, rule_id, rule_version, rule_type, matched, evaluated_at, evidence, event_ids, trace_id
		FROM rule_evaluations WHERE matched=true AND evaluated_at >= $1 ORDER BY evaluated_at DESC LIMIT $2
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*rule.Evaluation
	for rows.Next() {
		eval, err := scanEvaluation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, eval)
	}
	return out, rows.Err()
}

func scanRule(row pgx.Row) (*rule.Rule, error) {
	var rl rule.Rule
	var desc *string
	var scopeFactory *string
	var scopeLine *string
	var effectiveUntil *time.Time
	var createdBy *string
	var updatedBy *string
	var cfg json.RawMessage
	var actionCfg json.RawMessage
	if err := row.Scan(&rl.ID, &rl.RuleID, &rl.Version, &rl.Name, &desc, &rl.RuleType, &cfg, &rl.ActionType, &actionCfg, &scopeFactory, &scopeLine, &rl.EffectiveFrom, &effectiveUntil, &rl.Status, &rl.CreatedAt, &createdBy, &rl.UpdatedAt, &updatedBy); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if desc != nil {
		rl.Description = *desc
	}
	rl.Config = cfg
	rl.ActionConfig = actionCfg
	rl.ScopeFactoryID = scopeFactory
	rl.ScopeLineID = scopeLine
	rl.EffectiveUntil = effectiveUntil
	rl.CreatedBy = createdBy
	rl.UpdatedBy = updatedBy
	return &rl, nil
}

func scanEvaluation(row pgx.Row) (*rule.Evaluation, error) {
	var eval rule.Evaluation
	var evidence json.RawMessage
	var eventIDs []uuid.UUID
	var traceID *string
	if err := row.Scan(&eval.ID, &eval.EvaluationID, &eval.RuleID, &eval.RuleVersion, &eval.RuleType, &eval.Matched, &eval.EvaluatedAt, &evidence, &eventIDs, &traceID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	eval.Evidence = evidence
	eval.EventIDs = eventIDs
	eval.TraceID = traceID
	return &eval, nil
}
