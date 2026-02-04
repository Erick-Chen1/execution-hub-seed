package rule

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvaluation(t *testing.T) {
	rule := NewRule("Test Rule", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
	evidence := json.RawMessage(`{"field": "temperature", "actualValue": 105}`)
	eventIDs := []uuid.UUID{uuid.New(), uuid.New()}

	evaluation := NewEvaluation(rule, true, evidence, eventIDs)

	require.NotNil(t, evaluation)
	assert.NotEqual(t, uuid.Nil, evaluation.EvaluationID)
	assert.Equal(t, rule.RuleID, evaluation.RuleID)
	assert.Equal(t, rule.Version, evaluation.RuleVersion)
	assert.Equal(t, rule.RuleType, evaluation.RuleType)
	assert.True(t, evaluation.Matched)
	assert.Equal(t, evidence, evaluation.Evidence)
	assert.Equal(t, eventIDs, evaluation.EventIDs)
	assert.False(t, evaluation.EvaluatedAt.IsZero())
	assert.Nil(t, evaluation.TraceID)
}

func TestNewEvaluation_NotMatched(t *testing.T) {
	rule := NewRule("Test Rule", RuleTypeMissingField, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))

	evaluation := NewEvaluation(rule, false, nil, nil)

	require.NotNil(t, evaluation)
	assert.False(t, evaluation.Matched)
	assert.Nil(t, evaluation.Evidence)
	assert.Nil(t, evaluation.EventIDs)
}

func TestEvaluation_SetTraceID(t *testing.T) {
	rule := NewRule("Test Rule", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
	evaluation := NewEvaluation(rule, true, nil, nil)
	assert.Nil(t, evaluation.TraceID)

	evaluation.SetTraceID("trace-xyz-789")

	require.NotNil(t, evaluation.TraceID)
	assert.Equal(t, "trace-xyz-789", *evaluation.TraceID)
}

func TestThresholdEvidence_Serialization(t *testing.T) {
	evidence := ThresholdEvidence{
		EventID:           uuid.New(),
		Field:             "temperature",
		ActualValue:       105.5,
		ExpectedCondition: "> 100",
		Matched:           true,
	}

	data, err := json.Marshal(evidence)
	require.NoError(t, err)

	var parsed ThresholdEvidence
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, evidence.EventID, parsed.EventID)
	assert.Equal(t, evidence.Field, parsed.Field)
	assert.Equal(t, evidence.ActualValue, parsed.ActualValue)
	assert.Equal(t, evidence.ExpectedCondition, parsed.ExpectedCondition)
	assert.Equal(t, evidence.Matched, parsed.Matched)
}

func TestMissingFieldEvidence_Serialization(t *testing.T) {
	evidence := MissingFieldEvidence{
		EventID:      uuid.New(),
		Field:        "device_id",
		FieldPresent: false,
		Matched:      true,
	}

	data, err := json.Marshal(evidence)
	require.NoError(t, err)

	var parsed MissingFieldEvidence
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, evidence, parsed)
}

func TestRepeatedEvidence_Serialization(t *testing.T) {
	now := time.Now().UTC()
	evidence := RepeatedEvidence{
		Field:       "error_code",
		Value:       "E001",
		Count:       5,
		Threshold:   3,
		WindowStart: now.Add(-5 * time.Minute),
		WindowEnd:   now,
		EventIDs:    []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
		Matched:     true,
	}

	data, err := json.Marshal(evidence)
	require.NoError(t, err)

	var parsed RepeatedEvidence
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, evidence.Field, parsed.Field)
	assert.Equal(t, evidence.Value, parsed.Value)
	assert.Equal(t, evidence.Count, parsed.Count)
	assert.Equal(t, evidence.Threshold, parsed.Threshold)
	assert.Len(t, parsed.EventIDs, 3)
	assert.True(t, parsed.Matched)
}

func TestStreamDisconnectEvidence_Serialization(t *testing.T) {
	evidence := StreamDisconnectEvidence{
		SourceID:       "gateway-01",
		LastEventAt:    time.Now().Add(-2 * time.Minute).UTC(),
		TimeoutSeconds: 60,
		ActualGapSecs:  120.5,
		Matched:        true,
	}

	data, err := json.Marshal(evidence)
	require.NoError(t, err)

	var parsed StreamDisconnectEvidence
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, evidence.SourceID, parsed.SourceID)
	assert.Equal(t, evidence.TimeoutSeconds, parsed.TimeoutSeconds)
	assert.Equal(t, evidence.ActualGapSecs, parsed.ActualGapSecs)
	assert.True(t, parsed.Matched)
}

func TestTimeWindowEvidence_Serialization(t *testing.T) {
	now := time.Now().UTC()
	evidence := TimeWindowEvidence{
		WindowStart:  now.Add(-5 * time.Minute),
		WindowEnd:    now,
		EventCount:   100,
		FailureCount: 85,
		FailureRate:  0.85,
		Condition:    "failure_rate > 0.8",
		EventSample:  []uuid.UUID{uuid.New(), uuid.New()},
		Matched:      true,
	}

	data, err := json.Marshal(evidence)
	require.NoError(t, err)

	var parsed TimeWindowEvidence
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, evidence.EventCount, parsed.EventCount)
	assert.Equal(t, evidence.FailureCount, parsed.FailureCount)
	assert.Equal(t, evidence.FailureRate, parsed.FailureRate)
	assert.Equal(t, evidence.Condition, parsed.Condition)
	assert.Len(t, parsed.EventSample, 2)
	assert.True(t, parsed.Matched)
}

func TestEvaluationFilter(t *testing.T) {
	ruleID := uuid.New()
	matched := true
	since := time.Now().Add(-24 * time.Hour)
	until := time.Now()

	filter := EvaluationFilter{
		RuleID:  &ruleID,
		Matched: &matched,
		Since:   &since,
		Until:   &until,
	}

	assert.Equal(t, ruleID, *filter.RuleID)
	assert.True(t, *filter.Matched)
	assert.Equal(t, since, *filter.Since)
	assert.Equal(t, until, *filter.Until)
}

func TestEvaluationFilter_Empty(t *testing.T) {
	filter := EvaluationFilter{}

	assert.Nil(t, filter.RuleID)
	assert.Nil(t, filter.Matched)
	assert.Nil(t, filter.Since)
	assert.Nil(t, filter.Until)
}

func TestEvaluation_InheritsRuleInfo(t *testing.T) {
	// Create a rule with specific version
	rule := NewRule("Test Rule", RuleTypeRepeated, json.RawMessage(`{}`), ActionTypeWebhook, json.RawMessage(`{}`))
	rule.Version = 5

	evaluation := NewEvaluation(rule, true, nil, nil)

	assert.Equal(t, rule.RuleID, evaluation.RuleID)
	assert.Equal(t, 5, evaluation.RuleVersion)
	assert.Equal(t, RuleTypeRepeated, evaluation.RuleType)
}

func TestNewEvaluation_WithMultipleEventIDs(t *testing.T) {
	rule := NewRule("Test Rule", RuleTypeRepeated, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
	eventIDs := make([]uuid.UUID, 10)
	for i := range eventIDs {
		eventIDs[i] = uuid.New()
	}

	evaluation := NewEvaluation(rule, true, nil, eventIDs)

	assert.Len(t, evaluation.EventIDs, 10)
	for i, id := range evaluation.EventIDs {
		assert.Equal(t, eventIDs[i], id)
	}
}
