package rule

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Evaluation represents a rule evaluation record with evidence
type Evaluation struct {
	ID           int64           `json:"id"`
	EvaluationID uuid.UUID       `json:"evaluationId"`
	RuleID       uuid.UUID       `json:"ruleId"`
	RuleVersion  int             `json:"ruleVersion"`
	RuleType     RuleType        `json:"ruleType"`
	Matched      bool            `json:"matched"`
	EvaluatedAt  time.Time       `json:"evaluatedAt"`
	Evidence     json.RawMessage `json:"evidence"`
	EventIDs     []uuid.UUID     `json:"eventIds,omitempty"`
	TraceID      *string         `json:"traceId,omitempty"`
}

// NewEvaluation creates a new Evaluation from a rule
func NewEvaluation(r *Rule, matched bool, evidence json.RawMessage, eventIDs []uuid.UUID) *Evaluation {
	return &Evaluation{
		EvaluationID: uuid.New(),
		RuleID:       r.RuleID,
		RuleVersion:  r.Version,
		RuleType:     r.RuleType,
		Matched:      matched,
		EvaluatedAt:  time.Now().UTC(),
		Evidence:     evidence,
		EventIDs:     eventIDs,
	}
}

// SetTraceID sets the trace ID for distributed tracing
func (e *Evaluation) SetTraceID(traceID string) {
	e.TraceID = &traceID
}

// Evidence structures for different rule types

// ThresholdEvidence represents evidence for threshold rule evaluation
type ThresholdEvidence struct {
	EventID           uuid.UUID `json:"eventId"`
	Field             string    `json:"field"`
	ActualValue       float64   `json:"actualValue"`
	ExpectedCondition string    `json:"expectedCondition"`
	Matched           bool      `json:"matched"`
}

// MissingFieldEvidence represents evidence for missing field rule evaluation
type MissingFieldEvidence struct {
	EventID      uuid.UUID `json:"eventId"`
	Field        string    `json:"field"`
	FieldPresent bool      `json:"fieldPresent"`
	Matched      bool      `json:"matched"`
}

// RepeatedEvidence represents evidence for repeated event rule evaluation
type RepeatedEvidence struct {
	Field       string      `json:"field"`
	Value       interface{} `json:"value"`
	Count       int         `json:"count"`
	Threshold   int         `json:"threshold"`
	WindowStart time.Time   `json:"windowStart"`
	WindowEnd   time.Time   `json:"windowEnd"`
	EventIDs    []uuid.UUID `json:"eventIds"`
	Matched     bool        `json:"matched"`
}

// StreamDisconnectEvidence represents evidence for stream disconnect rule evaluation
type StreamDisconnectEvidence struct {
	SourceID       string    `json:"sourceId"`
	LastEventAt    time.Time `json:"lastEventAt"`
	TimeoutSeconds int       `json:"timeoutSeconds"`
	ActualGapSecs  float64   `json:"actualGapSeconds"`
	Matched        bool      `json:"matched"`
}

// TimeWindowEvidence represents evidence for time window rule evaluation
type TimeWindowEvidence struct {
	WindowStart  time.Time   `json:"windowStart"`
	WindowEnd    time.Time   `json:"windowEnd"`
	EventCount   int         `json:"eventCount"`
	FailureCount int         `json:"failureCount,omitempty"`
	FailureRate  float64     `json:"failureRate,omitempty"`
	Condition    string      `json:"condition"`
	EventSample  []uuid.UUID `json:"eventSample,omitempty"`
	Matched      bool        `json:"matched"`
}

// EvaluationFilter represents filters for querying evaluations
type EvaluationFilter struct {
	RuleID    *uuid.UUID
	Matched   *bool
	Since     *time.Time
	Until     *time.Time
}
