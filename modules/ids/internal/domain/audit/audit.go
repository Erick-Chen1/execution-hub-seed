package audit

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/google/uuid"
)

// EntityType represents the type of entity being audited
type EntityType string

const (
	EntityTypeSource      EntityType = "SOURCE"
	EntityTypeSchema      EntityType = "SCHEMA"
	EntityTypeRule        EntityType = "RULE"
	EntityTypeConfig      EntityType = "CONFIG"
	EntityTypeUser        EntityType = "USER"
	EntityTypeCorrection  EntityType = "CORRECTION"
	EntityTypeSecret      EntityType = "SECRET"
	EntityTypeBackup      EntityType = "BACKUP"
	EntityTypeNotification EntityType = "NOTIFICATION"
)

// Action represents the type of action being audited
type Action string

const (
	ActionCreate   Action = "CREATE"
	ActionUpdate   Action = "UPDATE"
	ActionDelete   Action = "DELETE"
	ActionLogin    Action = "LOGIN"
	ActionLogout   Action = "LOGOUT"
	ActionExport   Action = "EXPORT"
	ActionCorrect  Action = "CORRECT"
	ActionRotate   Action = "ROTATE"
	ActionRevoke   Action = "REVOKE"
	ActionApprove  Action = "APPROVE"
	ActionReject   Action = "REJECT"
	ActionActivate Action = "ACTIVATE"
	ActionSuspend  Action = "SUSPEND"
)

// RiskLevel represents the risk classification of an operation
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "LOW"
	RiskLevelMedium   RiskLevel = "MEDIUM"
	RiskLevelHigh     RiskLevel = "HIGH"
	RiskLevelCritical RiskLevel = "CRITICAL"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID             int64           `json:"id"`
	AuditID        uuid.UUID       `json:"auditId"`
	EntityType     EntityType      `json:"entityType"`
	EntityID       string          `json:"entityId"`
	Action         Action          `json:"action"`
	Actor          string          `json:"actor"`
	ActorRoles     []string        `json:"actorRoles,omitempty"`
	ActorIP        net.IP          `json:"actorIp,omitempty"`
	UserAgent      string          `json:"userAgent,omitempty"`
	OldValues      json.RawMessage `json:"oldValues,omitempty"`
	NewValues      json.RawMessage `json:"newValues,omitempty"`
	Diff           json.RawMessage `json:"diff,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	RiskLevel      RiskLevel       `json:"riskLevel"`
	Tags           []string        `json:"tags,omitempty"`
	Signature      []byte          `json:"signature,omitempty"`
	TraceID        string          `json:"traceId,omitempty"`
	SessionID      string          `json:"sessionId,omitempty"`
	RequestMethod  string          `json:"requestMethod,omitempty"`
	RequestPath    string          `json:"requestPath,omitempty"`
	ResponseStatus int             `json:"responseStatus,omitempty"`
	DurationMs     int             `json:"durationMs,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}

// AuditEntry represents an entry to be logged (input for creating audit logs)
type AuditEntry struct {
	EntityType     EntityType
	EntityID       string
	Action         Action
	Actor          string
	ActorRoles     []string
	ActorIP        net.IP
	UserAgent      string
	OldValues      interface{}
	NewValues      interface{}
	Reason         string
	Tags           []string
	TraceID        string
	SessionID      string
	RequestMethod  string
	RequestPath    string
	ResponseStatus int
	DurationMs     int
}

// QueryFilter represents filters for querying audit logs
type QueryFilter struct {
	EntityType     *EntityType
	EntityID       *string
	Action         *Action
	Actor          *string
	RiskLevel      *RiskLevel
	StartTime      *time.Time
	EndTime        *time.Time
	Tags           []string
	TraceID        *string
}

// Cursor represents a pagination cursor for audit logs
type Cursor struct {
	CreatedAt time.Time `json:"ts"`
	ID        int64     `json:"id"`
}

// Repository defines the interface for audit log persistence
type Repository interface {
	// Create creates a new audit log entry
	Create(ctx context.Context, entry *AuditLog) error

	// GetByID retrieves an audit log by its audit ID
	GetByID(ctx context.Context, auditID uuid.UUID) (*AuditLog, error)

	// Query retrieves audit logs based on filters with cursor-based pagination
	Query(ctx context.Context, filter QueryFilter, cursor *Cursor, limit int) ([]*AuditLog, *Cursor, error)

	// GetByEntityID retrieves all audit logs for a specific entity
	GetByEntityID(ctx context.Context, entityType EntityType, entityID string) ([]*AuditLog, error)

	// Count returns the total number of audit logs matching the filter
	Count(ctx context.Context, filter QueryFilter) (int64, error)

	// VerifySignature verifies the signature of an audit log entry
	VerifySignature(ctx context.Context, auditID uuid.UUID, key []byte) (bool, error)
}

// DetermineRiskLevel determines the risk level based on entity type and action
func DetermineRiskLevel(entityType EntityType, action Action) RiskLevel {
	// Critical: Key/credential operations
	if entityType == EntityTypeSecret {
		if action == ActionRotate || action == ActionRevoke || action == ActionCreate || action == ActionDelete {
			return RiskLevelCritical
		}
		return RiskLevelHigh
	}

	// High: Security-related changes
	if entityType == EntityTypeUser {
		return RiskLevelHigh
	}

	// High: Deletion operations
	if action == ActionDelete {
		return RiskLevelHigh
	}

	// Medium: Configuration changes
	if entityType == EntityTypeConfig || entityType == EntityTypeSchema || entityType == EntityTypeRule {
		if action == ActionCreate || action == ActionUpdate {
			return RiskLevelMedium
		}
	}

	// Medium: Corrections
	if entityType == EntityTypeCorrection {
		return RiskLevelMedium
	}

	// Default: Low
	return RiskLevelLow
}

// NewAuditLog creates a new AuditLog from an AuditEntry
func NewAuditLog(entry *AuditEntry) (*AuditLog, error) {
	log := &AuditLog{
		AuditID:        uuid.New(),
		EntityType:     entry.EntityType,
		EntityID:       entry.EntityID,
		Action:         entry.Action,
		Actor:          entry.Actor,
		ActorRoles:     entry.ActorRoles,
		ActorIP:        entry.ActorIP,
		UserAgent:      entry.UserAgent,
		Reason:         entry.Reason,
		Tags:           entry.Tags,
		TraceID:        entry.TraceID,
		SessionID:      entry.SessionID,
		RequestMethod:  entry.RequestMethod,
		RequestPath:    entry.RequestPath,
		ResponseStatus: entry.ResponseStatus,
		DurationMs:     entry.DurationMs,
		RiskLevel:      DetermineRiskLevel(entry.EntityType, entry.Action),
		CreatedAt:      time.Now().UTC(),
	}

	// Marshal old values if present
	if entry.OldValues != nil {
		data, err := json.Marshal(entry.OldValues)
		if err != nil {
			return nil, err
		}
		log.OldValues = data
	}

	// Marshal new values if present
	if entry.NewValues != nil {
		data, err := json.Marshal(entry.NewValues)
		if err != nil {
			return nil, err
		}
		log.NewValues = data
	}

	return log, nil
}
