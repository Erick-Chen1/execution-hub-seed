package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/notification"
)

// NotificationRepository implements notification.Repository.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

func (r *NotificationRepository) Create(ctx context.Context, n *notification.Notification) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notifications
		(notification_id, action_id, dedupe_key, channel, priority, title, body, payload, status, target_user_id, target_group, retry_count, max_retries, last_error, expires_at, created_at, sent_at, delivered_at, failed_at, trace_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
	`, n.NotificationID, n.ActionID, n.DedupeKey, n.Channel, n.Priority, n.Title, n.Body, n.Payload, n.Status, n.TargetUserID, n.TargetGroup, n.RetryCount, n.MaxRetries, n.LastError, n.ExpiresAt, n.CreatedAt, n.SentAt, n.DeliveredAt, n.FailedAt, n.TraceID)
	return err
}

func (r *NotificationRepository) GetByID(ctx context.Context, notificationID uuid.UUID) (*notification.Notification, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, notification_id, action_id, dedupe_key, channel, priority, title, body, payload, status, target_user_id, target_group, retry_count, max_retries, last_error, expires_at, created_at, sent_at, delivered_at, failed_at, trace_id
		FROM notifications WHERE notification_id=$1
	`, notificationID)
	return scanNotification(row)
}

func (r *NotificationRepository) GetByActionID(ctx context.Context, actionID uuid.UUID) ([]*notification.Notification, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, notification_id, action_id, dedupe_key, channel, priority, title, body, payload, status, target_user_id, target_group, retry_count, max_retries, last_error, expires_at, created_at, sent_at, delivered_at, failed_at, trace_id
		FROM notifications WHERE action_id=$1 ORDER BY created_at ASC
	`, actionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*notification.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NotificationRepository) FindByDedupeKey(ctx context.Context, dedupeKey string, since time.Time) (*notification.Notification, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, notification_id, action_id, dedupe_key, channel, priority, title, body, payload, status, target_user_id, target_group, retry_count, max_retries, last_error, expires_at, created_at, sent_at, delivered_at, failed_at, trace_id
		FROM notifications WHERE dedupe_key=$1 AND created_at >= $2
		ORDER BY created_at DESC LIMIT 1
	`, dedupeKey, since)
	return scanNotification(row)
}

func (r *NotificationRepository) List(ctx context.Context, filter notification.Filter, limit, offset int) ([]*notification.Notification, error) {
	query := `SELECT id, notification_id, action_id, dedupe_key, channel, priority, title, body, payload, status, target_user_id, target_group, retry_count, max_retries, last_error, expires_at, created_at, sent_at, delivered_at, failed_at, trace_id FROM notifications`
	args := []interface{}{}
	idx := 1
	if filter.ActionID != nil {
		query += " WHERE action_id=$" + itoa(idx)
		args = append(args, *filter.ActionID)
		idx++
	}
	if filter.Channel != nil {
		query += addWhere(query) + " channel=$" + itoa(idx)
		args = append(args, *filter.Channel)
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
	if filter.TargetUserID != nil {
		query += addWhere(query) + " target_user_id=$" + itoa(idx)
		args = append(args, *filter.TargetUserID)
		idx++
	}
	if filter.TargetGroup != nil {
		query += addWhere(query) + " target_group=$" + itoa(idx)
		args = append(args, *filter.TargetGroup)
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
	var out []*notification.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NotificationRepository) Update(ctx context.Context, n *notification.Notification) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notifications
		SET dedupe_key=$1, channel=$2, priority=$3, title=$4, body=$5, payload=$6, status=$7, target_user_id=$8, target_group=$9, retry_count=$10, max_retries=$11, last_error=$12, expires_at=$13, sent_at=$14, delivered_at=$15, failed_at=$16, trace_id=$17
		WHERE notification_id=$18
	`, n.DedupeKey, n.Channel, n.Priority, n.Title, n.Body, n.Payload, n.Status, n.TargetUserID, n.TargetGroup, n.RetryCount, n.MaxRetries, n.LastError, n.ExpiresAt, n.SentAt, n.DeliveredAt, n.FailedAt, n.TraceID, n.NotificationID)
	return err
}

func (r *NotificationRepository) UpdateStatus(ctx context.Context, notificationID uuid.UUID, status notification.Status) error {
	_, err := r.pool.Exec(ctx, `UPDATE notifications SET status=$1 WHERE notification_id=$2`, status, notificationID)
	return err
}

func (r *NotificationRepository) RecordAttempt(ctx context.Context, attempt *notification.DeliveryAttempt) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification_attempts
		(notification_id, attempt_number, status, attempted_at, response_code, response_body, error_message, duration_ms)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, attempt.NotificationID, attempt.AttemptNumber, attempt.Status, attempt.AttemptedAt, attempt.ResponseCode, attempt.ResponseBody, attempt.ErrorMessage, attempt.DurationMs)
	return err
}

func (r *NotificationRepository) GetAttempts(ctx context.Context, notificationID uuid.UUID) ([]*notification.DeliveryAttempt, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, notification_id, attempt_number, status, attempted_at, response_code, response_body, error_message, duration_ms
		FROM notification_attempts WHERE notification_id=$1 ORDER BY attempted_at ASC
	`, notificationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*notification.DeliveryAttempt
	for rows.Next() {
		var a notification.DeliveryAttempt
		if err := rows.Scan(&a.ID, &a.NotificationID, &a.AttemptNumber, &a.Status, &a.AttemptedAt, &a.ResponseCode, &a.ResponseBody, &a.ErrorMessage, &a.DurationMs); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

func (r *NotificationRepository) SaveAttemptToDLQ(ctx context.Context, attempt *notification.DeliveryAttempt, originalErr error) error {
	msg := ""
	if originalErr != nil {
		msg = originalErr.Error()
	}
	attempt.ErrorMessage = &msg
	return r.RecordAttempt(ctx, attempt)
}

func (r *NotificationRepository) ListPendingNotifications(ctx context.Context, limit int) ([]*notification.Notification, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, notification_id, action_id, dedupe_key, channel, priority, title, body, payload, status, target_user_id, target_group, retry_count, max_retries, last_error, expires_at, created_at, sent_at, delivered_at, failed_at, trace_id
		FROM notifications WHERE status='PENDING' ORDER BY created_at ASC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*notification.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NotificationRepository) ListRetryableNotifications(ctx context.Context, limit int) ([]*notification.Notification, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, notification_id, action_id, dedupe_key, channel, priority, title, body, payload, status, target_user_id, target_group, retry_count, max_retries, last_error, expires_at, created_at, sent_at, delivered_at, failed_at, trace_id
		FROM notifications
		WHERE status='FAILED' AND retry_count < max_retries AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at ASC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*notification.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NotificationRepository) ExpireNotifications(ctx context.Context) (int64, error) {
	res, err := r.pool.Exec(ctx, `
		UPDATE notifications
		SET status='EXPIRED'
		WHERE status NOT IN ('DELIVERED','EXPIRED') AND expires_at IS NOT NULL AND expires_at < NOW()
	`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

func scanNotification(row pgx.Row) (*notification.Notification, error) {
	var n notification.Notification
	var payload []byte
	if err := row.Scan(&n.ID, &n.NotificationID, &n.ActionID, &n.DedupeKey, &n.Channel, &n.Priority, &n.Title, &n.Body, &payload, &n.Status, &n.TargetUserID, &n.TargetGroup, &n.RetryCount, &n.MaxRetries, &n.LastError, &n.ExpiresAt, &n.CreatedAt, &n.SentAt, &n.DeliveredAt, &n.FailedAt, &n.TraceID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(payload) > 0 {
		n.Payload = payload
	}
	return &n, nil
}
