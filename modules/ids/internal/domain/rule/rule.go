package rule

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// RuleType represents the type of rule
type RuleType string

const (
	RuleTypeThreshold        RuleType = "THRESHOLD"
	RuleTypeRepeated         RuleType = "REPEATED"
	RuleTypeMissingField     RuleType = "MISSING_FIELD"
	RuleTypeStreamDisconnect RuleType = "STREAM_DISCONNECT"
	RuleTypeTimeWindow       RuleType = "TIME_WINDOW"
)

// RuleStatus represents the status of a rule
type RuleStatus string

const (
	RuleStatusActive   RuleStatus = "ACTIVE"
	RuleStatusInactive RuleStatus = "INACTIVE"
	RuleStatusArchived RuleStatus = "ARCHIVED"
)

// ActionType represents the type of action to take when a rule matches
type ActionType string

const (
	ActionTypeNotify   ActionType = "NOTIFY"
	ActionTypeWebhook  ActionType = "WEBHOOK"
	ActionTypeEscalate ActionType = "ESCALATE"
)

// Rule represents a versioned rule definition
type Rule struct {
	ID             int64           `json:"id"`
	RuleID         uuid.UUID       `json:"ruleId"`
	Version        int             `json:"version"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	RuleType       RuleType        `json:"ruleType"`
	Config         json.RawMessage `json:"config"`
	ActionType     ActionType      `json:"actionType"`
	ActionConfig   json.RawMessage `json:"actionConfig"`
	ScopeFactoryID *string         `json:"scopeFactoryId,omitempty"`
	ScopeLineID    *string         `json:"scopeLineId,omitempty"`
	EffectiveFrom  time.Time       `json:"effectiveFrom"`
	EffectiveUntil *time.Time      `json:"effectiveUntil,omitempty"`
	Status         RuleStatus      `json:"status"`
	CreatedAt      time.Time       `json:"createdAt"`
	CreatedBy      *string         `json:"createdBy,omitempty"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	UpdatedBy      *string         `json:"updatedBy,omitempty"`
}

// Rule configuration structures

// ThresholdConfig defines configuration for threshold-based rules
type ThresholdConfig struct {
	Field     string  `json:"field"`
	Operator  string  `json:"operator"` // >, >=, <, <=, ==, !=
	Threshold float64 `json:"threshold"`
	EventType string  `json:"eventType"`
}

// RepeatedConfig defines configuration for repeated event detection rules
type RepeatedConfig struct {
	Field         string `json:"field"`
	Count         int    `json:"count"`
	WindowSeconds int    `json:"windowSeconds"`
	EventType     string `json:"eventType"`
}

// MissingFieldConfig defines configuration for missing field detection rules
type MissingFieldConfig struct {
	Field     string `json:"field"`
	EventType string `json:"eventType"`
}

// StreamDisconnectConfig defines configuration for stream disconnect detection rules
type StreamDisconnectConfig struct {
	SourceID       string `json:"sourceId"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

// TimeWindowConfig defines configuration for time window aggregation rules
type TimeWindowConfig struct {
	WindowSize string `json:"windowSize"` // e.g., "5m", "1h"
	Condition  string `json:"condition"`  // e.g., "failure_rate > 0.8"
	EventType  string `json:"eventType"`
}

// Action configuration structures

// NotifyConfig defines configuration for notification actions
type NotifyConfig struct {
	Severity string   `json:"severity"` // LOW, MEDIUM, HIGH, CRITICAL
	Title    string   `json:"title"`
	Message  string   `json:"message"`
	Targets  []string `json:"targets,omitempty"`
	Dedupe   *DedupeConfig `json:"dedupe,omitempty"`
}

// DedupeConfig defines deduplication configuration
type DedupeConfig struct {
	Key             string `json:"key"`
	CooldownSeconds int    `json:"cooldownSeconds"`
}

// WebhookConfig defines configuration for webhook actions
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout int               `json:"timeout"` // in seconds
}

// NewRule creates a new Rule with default values
func NewRule(name string, ruleType RuleType, config json.RawMessage, actionType ActionType, actionConfig json.RawMessage) *Rule {
	now := time.Now().UTC()
	return &Rule{
		RuleID:        uuid.New(),
		Version:       1,
		Name:          name,
		RuleType:      ruleType,
		Config:        config,
		ActionType:    actionType,
		ActionConfig:  actionConfig,
		EffectiveFrom: now,
		Status:        RuleStatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// CreateNewVersion creates a new version of the rule
func (r *Rule) CreateNewVersion() *Rule {
	now := time.Now().UTC()
	return &Rule{
		RuleID:         r.RuleID,
		Version:        r.Version + 1,
		Name:           r.Name,
		Description:    r.Description,
		RuleType:       r.RuleType,
		Config:         r.Config,
		ActionType:     r.ActionType,
		ActionConfig:   r.ActionConfig,
		ScopeFactoryID: r.ScopeFactoryID,
		ScopeLineID:    r.ScopeLineID,
		EffectiveFrom:  now,
		Status:         RuleStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// IsEffective checks if the rule is effective at the given time
func (r *Rule) IsEffective(t time.Time) bool {
	if r.Status != RuleStatusActive {
		return false
	}
	if t.Before(r.EffectiveFrom) {
		return false
	}
	if r.EffectiveUntil != nil && t.After(*r.EffectiveUntil) {
		return false
	}
	return true
}

// Activate activates the rule
func (r *Rule) Activate() {
	r.Status = RuleStatusActive
	r.UpdatedAt = time.Now().UTC()
}

// Deactivate deactivates the rule
func (r *Rule) Deactivate() {
	r.Status = RuleStatusInactive
	r.UpdatedAt = time.Now().UTC()
}

// Archive archives the rule
func (r *Rule) Archive() {
	r.Status = RuleStatusArchived
	r.UpdatedAt = time.Now().UTC()
}

// Validate validates the rule
func (r *Rule) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	if len(r.Config) == 0 {
		return errors.New("config is required")
	}
	if len(r.ActionConfig) == 0 {
		return errors.New("actionConfig is required")
	}

	// Validate rule type
	switch r.RuleType {
	case RuleTypeThreshold, RuleTypeRepeated, RuleTypeMissingField, RuleTypeStreamDisconnect, RuleTypeTimeWindow:
		// Valid
	default:
		return errors.New("invalid ruleType")
	}

	// Validate action type
	switch r.ActionType {
	case ActionTypeNotify, ActionTypeWebhook, ActionTypeEscalate:
		// Valid
	default:
		return errors.New("invalid actionType")
	}

	// Validate config is valid JSON
	var js json.RawMessage
	if err := json.Unmarshal(r.Config, &js); err != nil {
		return errors.New("config must be valid JSON")
	}

	// Validate actionConfig is valid JSON
	if err := json.Unmarshal(r.ActionConfig, &js); err != nil {
		return errors.New("actionConfig must be valid JSON")
	}

	return nil
}

// Filter represents filters for querying rules
type Filter struct {
	RuleType       *RuleType
	Status         *RuleStatus
	ScopeFactoryID *string
	ScopeLineID    *string
}
