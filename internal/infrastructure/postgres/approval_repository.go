package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/approval"
)

// ApprovalRepository implements approval.Repository.
type ApprovalRepository struct {
	pool *pgxpool.Pool
}

func NewApprovalRepository(pool *pgxpool.Pool) *ApprovalRepository {
	return &ApprovalRepository{pool: pool}
}

func (r *ApprovalRepository) Create(ctx context.Context, a *approval.Approval) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO approvals
		(approval_id, entity_type, entity_id, operation, status, request_payload, requested_by, requested_by_type, required_roles, applies_to, min_approvals, approvals_count, rejections_count, requested_at, decided_at, executed_at, execution_error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`, a.ApprovalID, a.EntityType, a.EntityID, a.Operation, a.Status, a.RequestPayload, a.RequestedBy, a.RequestedByType, a.RequiredRoles, a.AppliesTo, a.MinApprovals, a.ApprovalsCount, a.RejectionsCount, a.RequestedAt, a.DecidedAt, a.ExecutedAt, a.ExecutionError)
	return err
}

func (r *ApprovalRepository) Update(ctx context.Context, a *approval.Approval) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE approvals
		SET status=$1, approvals_count=$2, rejections_count=$3, decided_at=$4, executed_at=$5, execution_error=$6
		WHERE approval_id=$7
	`, a.Status, a.ApprovalsCount, a.RejectionsCount, a.DecidedAt, a.ExecutedAt, a.ExecutionError, a.ApprovalID)
	return err
}

func (r *ApprovalRepository) GetByID(ctx context.Context, approvalID uuid.UUID) (*approval.Approval, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, approval_id, entity_type, entity_id, operation, status, request_payload, requested_by, requested_by_type, required_roles, applies_to, min_approvals, approvals_count, rejections_count, requested_at, decided_at, executed_at, execution_error
		FROM approvals WHERE approval_id=$1
	`, approvalID)
	return scanApproval(row)
}

func (r *ApprovalRepository) List(ctx context.Context, filter approval.Filter, limit, offset int) ([]*approval.Approval, error) {
	query := `SELECT id, approval_id, entity_type, entity_id, operation, status, request_payload, requested_by, requested_by_type, required_roles, applies_to, min_approvals, approvals_count, rejections_count, requested_at, decided_at, executed_at, execution_error FROM approvals`
	args := []interface{}{}
	idx := 1
	if filter.Status != nil {
		query += " WHERE status=$" + itoa(idx)
		args = append(args, *filter.Status)
		idx++
	}
	if filter.EntityType != nil {
		query += addWhere(query) + " entity_type=$" + itoa(idx)
		args = append(args, *filter.EntityType)
		idx++
	}
	if filter.EntityID != nil {
		query += addWhere(query) + " entity_id=$" + itoa(idx)
		args = append(args, *filter.EntityID)
		idx++
	}
	if filter.Operation != nil {
		query += addWhere(query) + " operation=$" + itoa(idx)
		args = append(args, *filter.Operation)
		idx++
	}
	if filter.RequestedBy != nil {
		query += addWhere(query) + " requested_by=$" + itoa(idx)
		args = append(args, *filter.RequestedBy)
		idx++
	}
	query += " ORDER BY requested_at DESC LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var approvals []*approval.Approval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, a)
	}
	return approvals, rows.Err()
}

func (r *ApprovalRepository) CreateDecision(ctx context.Context, d *approval.DecisionRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO approval_decisions
		(decision_id, approval_id, decision, decided_by, decided_by_type, decided_by_role, comment, decided_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, d.DecisionID, d.ApprovalID, d.Decision, d.DecidedBy, d.DecidedByType, d.DecidedByRole, d.Comment, d.DecidedAt)
	return err
}

func (r *ApprovalRepository) ListDecisions(ctx context.Context, approvalID uuid.UUID) ([]*approval.DecisionRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, decision_id, approval_id, decision, decided_by, decided_by_type, decided_by_role, comment, decided_at
		FROM approval_decisions WHERE approval_id=$1 ORDER BY decided_at ASC
	`, approvalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var decisions []*approval.DecisionRecord
	for rows.Next() {
		var d approval.DecisionRecord
		if err := rows.Scan(&d.ID, &d.DecisionID, &d.ApprovalID, &d.Decision, &d.DecidedBy, &d.DecidedByType, &d.DecidedByRole, &d.Comment, &d.DecidedAt); err != nil {
			return nil, err
		}
		decisions = append(decisions, &d)
	}
	return decisions, rows.Err()
}

func (r *ApprovalRepository) CountDecisions(ctx context.Context, approvalID uuid.UUID) (int, int, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE decision='APPROVE') AS approvals,
			COUNT(*) FILTER (WHERE decision='REJECT') AS rejections
		FROM approval_decisions WHERE approval_id=$1
	`, approvalID)
	var approvals int
	var rejections int
	if err := row.Scan(&approvals, &rejections); err != nil {
		return 0, 0, err
	}
	return approvals, rejections, nil
}

func (r *ApprovalRepository) HasDecision(ctx context.Context, approvalID uuid.UUID, decidedBy string) (bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT 1 FROM approval_decisions WHERE approval_id=$1 AND decided_by=$2 LIMIT 1
	`, approvalID, decidedBy)
	var v int
	if err := row.Scan(&v); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func scanApproval(row pgx.Row) (*approval.Approval, error) {
	var a approval.Approval
	if err := row.Scan(&a.ID, &a.ApprovalID, &a.EntityType, &a.EntityID, &a.Operation, &a.Status, &a.RequestPayload, &a.RequestedBy, &a.RequestedByType, &a.RequiredRoles, &a.AppliesTo, &a.MinApprovals, &a.ApprovalsCount, &a.RejectionsCount, &a.RequestedAt, &a.DecidedAt, &a.ExecutedAt, &a.ExecutionError); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}
