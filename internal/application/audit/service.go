package audit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/execution-hub/execution-hub/internal/domain/audit"
)

// Service handles audit log operations
type Service struct {
	repo   audit.Repository
	logger zerolog.Logger
	signKey []byte
}

// NewService creates a new audit service
func NewService(repo audit.Repository, logger zerolog.Logger, signKey []byte) *Service {
	return &Service{
		repo:    repo,
		signKey: signKey,
		logger:  logger.With().Str("service", "audit").Logger(),
	}
}

// Log creates a new audit log entry asynchronously
func (s *Service) Log(ctx context.Context, entry *audit.AuditEntry) {
	go func() {
		if err := s.LogSync(context.Background(), entry); err != nil {
			s.logger.Error().Err(err).
				Str("entityType", string(entry.EntityType)).
				Str("entityId", entry.EntityID).
				Str("action", string(entry.Action)).
				Msg("failed to create audit log")
		}
	}()
}

// LogSync creates a new audit log entry synchronously
func (s *Service) LogSync(ctx context.Context, entry *audit.AuditEntry) error {
	auditLog, err := audit.NewAuditLog(entry)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	if len(s.signKey) > 0 {
		sig, err := audit.SignAuditLog(auditLog, s.signKey)
		if err != nil {
			return fmt.Errorf("failed to sign audit log: %w", err)
		}
		auditLog.Signature = sig
	}

	if err := s.repo.Create(ctx, auditLog); err != nil {
		return fmt.Errorf("failed to save audit log: %w", err)
	}

	s.logger.Debug().
		Str("auditId", auditLog.AuditID.String()).
		Str("entityType", string(auditLog.EntityType)).
		Str("entityId", auditLog.EntityID).
		Str("action", string(auditLog.Action)).
		Str("actor", auditLog.Actor).
		Str("riskLevel", string(auditLog.RiskLevel)).
		Msg("audit log created")

	// Alert on high-risk operations
	if auditLog.RiskLevel == audit.RiskLevelHigh || auditLog.RiskLevel == audit.RiskLevelCritical {
		s.logger.Warn().
			Str("auditId", auditLog.AuditID.String()).
			Str("entityType", string(auditLog.EntityType)).
			Str("entityId", auditLog.EntityID).
			Str("action", string(auditLog.Action)).
			Str("actor", auditLog.Actor).
			Str("riskLevel", string(auditLog.RiskLevel)).
			Msg("high-risk operation detected")
	}

	return nil
}

// QueryParams represents query parameters for audit logs
type QueryParams struct {
	EntityType *string
	EntityID   *string
	Action     *string
	Actor      *string
	RiskLevel  *string
	StartTime  *time.Time
	EndTime    *time.Time
	Tags       []string
	TraceID    *string
	Cursor     *string
	Limit      int
}

// QueryResult represents the result of an audit log query
type QueryResult struct {
	Logs       []*audit.AuditLog `json:"logs"`
	Pagination Pagination        `json:"pagination"`
	TraceID    string            `json:"traceId"`
}

// Pagination holds pagination information
type Pagination struct {
	Cursor  *string `json:"cursor,omitempty"`
	HasMore bool    `json:"hasMore"`
	Count   int     `json:"count"`
	Total   int64   `json:"total,omitempty"`
}

// Query retrieves audit logs based on parameters
func (s *Service) Query(ctx context.Context, params QueryParams, traceID string) (*QueryResult, error) {
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 200 {
		params.Limit = 200
	}

	// Decode cursor if provided
	var cursor *audit.Cursor
	if params.Cursor != nil && *params.Cursor != "" {
		c, err := decodeCursor(*params.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		cursor = c
	}

	// Build filter
	filter := audit.QueryFilter{
		Tags:    params.Tags,
	}

	if params.EntityType != nil {
		et := audit.EntityType(*params.EntityType)
		filter.EntityType = &et
	}
	if params.EntityID != nil {
		filter.EntityID = params.EntityID
	}
	if params.Action != nil {
		a := audit.Action(*params.Action)
		filter.Action = &a
	}
	if params.Actor != nil {
		filter.Actor = params.Actor
	}
	if params.RiskLevel != nil {
		rl := audit.RiskLevel(*params.RiskLevel)
		filter.RiskLevel = &rl
	}
	if params.StartTime != nil {
		filter.StartTime = params.StartTime
	}
	if params.EndTime != nil {
		filter.EndTime = params.EndTime
	}
	if params.TraceID != nil {
		filter.TraceID = params.TraceID
	}

	// Execute query
	logs, nextCursor, err := s.repo.Query(ctx, filter, cursor, params.Limit)
	if err != nil {
		s.logger.Error().Err(err).Str("traceId", traceID).Msg("failed to query audit logs")
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}

	result := &QueryResult{
		Logs:    logs,
		TraceID: traceID,
		Pagination: Pagination{
			Count:   len(logs),
			HasMore: nextCursor != nil,
		},
	}

	// Encode next cursor
	if nextCursor != nil {
		encoded, err := encodeCursor(nextCursor)
		if err != nil {
			s.logger.Warn().Err(err).Msg("failed to encode cursor")
		} else {
			result.Pagination.Cursor = &encoded
		}
	}

	return result, nil
}

// GetByID retrieves an audit log by its ID
func (s *Service) GetByID(ctx context.Context, auditID uuid.UUID, traceID string) (*audit.AuditLog, error) {
	log, err := s.repo.GetByID(ctx, auditID)
	if err != nil {
		s.logger.Error().Err(err).
			Str("traceId", traceID).
			Str("auditId", auditID.String()).
			Msg("failed to get audit log")
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}
	return log, nil
}

// GetEntityHistory retrieves the complete audit history for an entity
func (s *Service) GetEntityHistory(ctx context.Context, entityType string, entityID string, traceID string) ([]*audit.AuditLog, error) {
	et := audit.EntityType(entityType)
	logs, err := s.repo.GetByEntityID(ctx, et, entityID)
	if err != nil {
		s.logger.Error().Err(err).
			Str("traceId", traceID).
			Str("entityType", entityType).
			Str("entityId", entityID).
			Msg("failed to get entity history")
		return nil, fmt.Errorf("failed to get entity history: %w", err)
	}
	return logs, nil
}

// VerifyIntegrity verifies the signature of an audit log entry
type VerifyResult struct {
	AuditID   uuid.UUID `json:"auditId"`
	Verified  bool      `json:"verified"`
	Message   string    `json:"message"`
}

func (s *Service) VerifyIntegrity(ctx context.Context, auditID uuid.UUID, signKey []byte, traceID string) (*VerifyResult, error) {
	verified, err := s.repo.VerifySignature(ctx, auditID, signKey)
	if err != nil {
		s.logger.Error().Err(err).
			Str("traceId", traceID).
			Str("auditId", auditID.String()).
			Msg("failed to verify audit log signature")
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}

	result := &VerifyResult{
		AuditID:  auditID,
		Verified: verified,
	}

	if verified {
		result.Message = "Audit log integrity verified"
	} else {
		result.Message = "Audit log signature mismatch - possible tampering detected"
		s.logger.Warn().
			Str("auditId", auditID.String()).
			Msg("audit log signature verification failed")
	}

	return result, nil
}

// encodeCursor encodes a cursor to base64 string
func encodeCursor(c *audit.Cursor) (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

// decodeCursor decodes a base64 string to cursor
func decodeCursor(s string) (*audit.Cursor, error) {
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	var c audit.Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}

	return &c, nil
}
