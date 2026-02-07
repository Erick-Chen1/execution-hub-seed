package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/audit"
)

// AuditRepository implements audit.Repository.
type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) Create(ctx context.Context, entry *audit.AuditLog) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs
		(audit_id, entity_type, entity_id, action, actor, actor_roles, actor_ip, user_agent, old_values, new_values, diff, reason, risk_level, tags, signature, trace_id, session_id, request_method, request_path, response_status, duration_ms, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
	`, entry.AuditID, entry.EntityType, entry.EntityID, entry.Action, entry.Actor, entry.ActorRoles, entry.ActorIP, entry.UserAgent, entry.OldValues, entry.NewValues, entry.Diff, entry.Reason, entry.RiskLevel, entry.Tags, entry.Signature, entry.TraceID, entry.SessionID, entry.RequestMethod, entry.RequestPath, entry.ResponseStatus, entry.DurationMs, entry.CreatedAt)
	return err
}

func (r *AuditRepository) GetByID(ctx context.Context, auditID uuid.UUID) (*audit.AuditLog, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, audit_id, entity_type, entity_id, action, actor, actor_roles, actor_ip, user_agent, old_values, new_values, diff, reason, risk_level, tags, signature, trace_id, session_id, request_method, request_path, response_status, duration_ms, created_at
		FROM audit_logs WHERE audit_id=$1
	`, auditID)
	return scanAudit(row)
}

func (r *AuditRepository) Query(ctx context.Context, filter audit.QueryFilter, cursor *audit.Cursor, limit int) ([]*audit.AuditLog, *audit.Cursor, error) {
	query := `SELECT id, audit_id, entity_type, entity_id, action, actor, actor_roles, actor_ip, user_agent, old_values, new_values, diff, reason, risk_level, tags, signature, trace_id, session_id, request_method, request_path, response_status, duration_ms, created_at FROM audit_logs`
	args := []interface{}{}
	idx := 1
	if filter.EntityType != nil {
		query += " WHERE entity_type=$" + itoa(idx)
		args = append(args, *filter.EntityType)
		idx++
	}
	if filter.EntityID != nil {
		query += addWhere(query) + " entity_id=$" + itoa(idx)
		args = append(args, *filter.EntityID)
		idx++
	}
	if filter.Action != nil {
		query += addWhere(query) + " action=$" + itoa(idx)
		args = append(args, *filter.Action)
		idx++
	}
	if filter.Actor != nil {
		query += addWhere(query) + " actor=$" + itoa(idx)
		args = append(args, *filter.Actor)
		idx++
	}
	if filter.RiskLevel != nil {
		query += addWhere(query) + " risk_level=$" + itoa(idx)
		args = append(args, *filter.RiskLevel)
		idx++
	}
	if filter.StartTime != nil {
		query += addWhere(query) + " created_at >= $" + itoa(idx)
		args = append(args, *filter.StartTime)
		idx++
	}
	if filter.EndTime != nil {
		query += addWhere(query) + " created_at <= $" + itoa(idx)
		args = append(args, *filter.EndTime)
		idx++
	}
	if len(filter.Tags) > 0 {
		query += addWhere(query) + " tags @> $" + itoa(idx)
		args = append(args, filter.Tags)
		idx++
	}
	if filter.TraceID != nil {
		query += addWhere(query) + " trace_id=$" + itoa(idx)
		args = append(args, *filter.TraceID)
		idx++
	}
	if cursor != nil {
		query += addWhere(query) + " (created_at, id) < ($" + itoa(idx) + ", $" + itoa(idx+1) + ")"
		args = append(args, cursor.CreatedAt, cursor.ID)
		idx += 2
	}

	query += " ORDER BY created_at DESC, id DESC LIMIT $" + itoa(idx)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var logs []*audit.AuditLog
	for rows.Next() {
		log, err := scanAudit(rows)
		if err != nil {
			return nil, nil, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var nextCursor *audit.Cursor
	if len(logs) == limit {
		last := logs[len(logs)-1]
		nextCursor = &audit.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}

	return logs, nextCursor, nil
}

func (r *AuditRepository) GetByEntityID(ctx context.Context, entityType audit.EntityType, entityID string) ([]*audit.AuditLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, audit_id, entity_type, entity_id, action, actor, actor_roles, actor_ip, user_agent, old_values, new_values, diff, reason, risk_level, tags, signature, trace_id, session_id, request_method, request_path, response_status, duration_ms, created_at
		FROM audit_logs WHERE entity_type=$1 AND entity_id=$2 ORDER BY created_at DESC
	`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []*audit.AuditLog
	for rows.Next() {
		log, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (r *AuditRepository) Count(ctx context.Context, filter audit.QueryFilter) (int64, error) {
	query := `SELECT COUNT(1) FROM audit_logs`
	args := []interface{}{}
	idx := 1
	if filter.EntityType != nil {
		query += " WHERE entity_type=$" + itoa(idx)
		args = append(args, *filter.EntityType)
		idx++
	}
	if filter.EntityID != nil {
		query += addWhere(query) + " entity_id=$" + itoa(idx)
		args = append(args, *filter.EntityID)
		idx++
	}
	if filter.Action != nil {
		query += addWhere(query) + " action=$" + itoa(idx)
		args = append(args, *filter.Action)
		idx++
	}
	if filter.Actor != nil {
		query += addWhere(query) + " actor=$" + itoa(idx)
		args = append(args, *filter.Actor)
		idx++
	}
	if filter.RiskLevel != nil {
		query += addWhere(query) + " risk_level=$" + itoa(idx)
		args = append(args, *filter.RiskLevel)
		idx++
	}
	if filter.StartTime != nil {
		query += addWhere(query) + " created_at >= $" + itoa(idx)
		args = append(args, *filter.StartTime)
		idx++
	}
	if filter.EndTime != nil {
		query += addWhere(query) + " created_at <= $" + itoa(idx)
		args = append(args, *filter.EndTime)
		idx++
	}
	if len(filter.Tags) > 0 {
		query += addWhere(query) + " tags @> $" + itoa(idx)
		args = append(args, filter.Tags)
		idx++
	}
	if filter.TraceID != nil {
		query += addWhere(query) + " trace_id=$" + itoa(idx)
		args = append(args, *filter.TraceID)
		idx++
	}
	row := r.pool.QueryRow(ctx, query, args...)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *AuditRepository) VerifySignature(ctx context.Context, auditID uuid.UUID, key []byte) (bool, error) {
	row := r.pool.QueryRow(ctx, `SELECT signature FROM audit_logs WHERE audit_id=$1`, auditID)
	var sig []byte
	if err := row.Scan(&sig); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if len(sig) == 0 {
		return false, nil
	}
	log, err := r.GetByID(ctx, auditID)
	if err != nil {
		return false, err
	}
	if log == nil {
		return false, nil
	}
	return audit.VerifyAuditLogSignature(log, key)
}

func scanAudit(row pgx.Row) (*audit.AuditLog, error) {
	var log audit.AuditLog
	if err := row.Scan(&log.ID, &log.AuditID, &log.EntityType, &log.EntityID, &log.Action, &log.Actor, &log.ActorRoles, &log.ActorIP, &log.UserAgent, &log.OldValues, &log.NewValues, &log.Diff, &log.Reason, &log.RiskLevel, &log.Tags, &log.Signature, &log.TraceID, &log.SessionID, &log.RequestMethod, &log.RequestPath, &log.ResponseStatus, &log.DurationMs, &log.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}
