package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/session"
)

// SessionRepository implements session.Repository.
type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, s *session.Session) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sessions
		(session_id, token_hash, user_id, created_at, expires_at, last_seen_at, user_agent, ip_address)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, s.SessionID, s.TokenHash, s.UserID, s.CreatedAt, s.ExpiresAt, s.LastSeenAt, s.UserAgent, s.IPAddress)
	return err
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*session.Session, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, session_id, token_hash, user_id, created_at, expires_at, last_seen_at, user_agent, ip_address::text
		FROM sessions WHERE token_hash=$1
	`, tokenHash)
	return scanSession(row)
}

func (r *SessionRepository) DeleteByID(ctx context.Context, sessionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE session_id=$1`, sessionID)
	return err
}

func (r *SessionRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, tokenHash)
	return err
}

func (r *SessionRepository) UpdateLastSeen(ctx context.Context, sessionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET last_seen_at=$1 WHERE session_id=$2`, time.Now().UTC(), sessionID)
	return err
}

func (r *SessionRepository) DeleteExpired(ctx context.Context) (int, error) {
	res, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < $1`, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return int(res.RowsAffected()), nil
}

func scanSession(row pgx.Row) (*session.Session, error) {
	var s session.Session
	var lastSeen *time.Time
	var userAgent *string
	var ipAddress *string
	if err := row.Scan(&s.ID, &s.SessionID, &s.TokenHash, &s.UserID, &s.CreatedAt, &s.ExpiresAt, &lastSeen, &userAgent, &ipAddress); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.LastSeenAt = lastSeen
	s.UserAgent = userAgent
	s.IPAddress = ipAddress
	return &s, nil
}
