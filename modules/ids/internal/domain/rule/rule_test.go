package rule

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRule(t *testing.T) {
	name := "Test Rule"
	ruleType := RuleTypeThreshold
	config := json.RawMessage(`{"field": "temperature", "operator": ">", "threshold": 100}`)
	actionType := ActionTypeNotify
	actionConfig := json.RawMessage(`{"title": "Alert"}`)

	rule := NewRule(name, ruleType, config, actionType, actionConfig)

	require.NotNil(t, rule)
	assert.NotEmpty(t, rule.RuleID)
	assert.Equal(t, 1, rule.Version)
	assert.Equal(t, name, rule.Name)
	assert.Equal(t, ruleType, rule.RuleType)
	assert.Equal(t, config, rule.Config)
	assert.Equal(t, actionType, rule.ActionType)
	assert.Equal(t, actionConfig, rule.ActionConfig)
	assert.Equal(t, RuleStatusActive, rule.Status)
	assert.False(t, rule.CreatedAt.IsZero())
	assert.False(t, rule.UpdatedAt.IsZero())
	assert.False(t, rule.EffectiveFrom.IsZero())
	assert.Nil(t, rule.EffectiveUntil)
	assert.Nil(t, rule.ScopeFactoryID)
	assert.Nil(t, rule.ScopeLineID)
	assert.Empty(t, rule.Description)
}

func TestRule_CreateNewVersion(t *testing.T) {
	original := NewRule("Test Rule", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
	original.Description = "Original description"
	factoryID := "factory-1"
	lineID := "line-1"
	original.ScopeFactoryID = &factoryID
	original.ScopeLineID = &lineID
	original.Status = RuleStatusInactive

	// Wait a tiny bit to ensure timestamps are different
	time.Sleep(1 * time.Millisecond)

	newVersion := original.CreateNewVersion()

	require.NotNil(t, newVersion)
	// Same RuleID
	assert.Equal(t, original.RuleID, newVersion.RuleID)
	// Version incremented
	assert.Equal(t, 2, newVersion.Version)
	// Inherited fields
	assert.Equal(t, original.Name, newVersion.Name)
	assert.Equal(t, original.Description, newVersion.Description)
	assert.Equal(t, original.RuleType, newVersion.RuleType)
	assert.Equal(t, original.Config, newVersion.Config)
	assert.Equal(t, original.ActionType, newVersion.ActionType)
	assert.Equal(t, original.ActionConfig, newVersion.ActionConfig)
	assert.Equal(t, original.ScopeFactoryID, newVersion.ScopeFactoryID)
	assert.Equal(t, original.ScopeLineID, newVersion.ScopeLineID)
	// Reset fields
	assert.Equal(t, RuleStatusActive, newVersion.Status) // New version is always active
	assert.True(t, newVersion.CreatedAt.After(original.CreatedAt) || newVersion.CreatedAt.Equal(original.CreatedAt))
	assert.True(t, newVersion.EffectiveFrom.After(original.EffectiveFrom) || newVersion.EffectiveFrom.Equal(original.EffectiveFrom))
}

func TestRule_IsEffective(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name           string
		status         RuleStatus
		effectiveFrom  time.Time
		effectiveUntil *time.Time
		checkTime      time.Time
		expected       bool
	}{
		{
			name:          "active and within range",
			status:        RuleStatusActive,
			effectiveFrom: past,
			checkTime:     now,
			expected:      true,
		},
		{
			name:          "inactive status",
			status:        RuleStatusInactive,
			effectiveFrom: past,
			checkTime:     now,
			expected:      false,
		},
		{
			name:          "archived status",
			status:        RuleStatusArchived,
			effectiveFrom: past,
			checkTime:     now,
			expected:      false,
		},
		{
			name:          "before effective from",
			status:        RuleStatusActive,
			effectiveFrom: future,
			checkTime:     now,
			expected:      false,
		},
		{
			name:           "after effective until",
			status:         RuleStatusActive,
			effectiveFrom:  past,
			effectiveUntil: &past,
			checkTime:      now,
			expected:       false,
		},
		{
			name:           "within effective range with until",
			status:         RuleStatusActive,
			effectiveFrom:  past,
			effectiveUntil: &future,
			checkTime:      now,
			expected:       true,
		},
		{
			name:           "nil effective until (permanent)",
			status:         RuleStatusActive,
			effectiveFrom:  past,
			effectiveUntil: nil,
			checkTime:      now,
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewRule("Test", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
			rule.Status = tt.status
			rule.EffectiveFrom = tt.effectiveFrom
			rule.EffectiveUntil = tt.effectiveUntil

			assert.Equal(t, tt.expected, rule.IsEffective(tt.checkTime))
		})
	}
}

func TestRule_Activate(t *testing.T) {
	rule := NewRule("Test", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
	rule.Status = RuleStatusInactive
	originalUpdatedAt := rule.UpdatedAt

	time.Sleep(1 * time.Millisecond)
	rule.Activate()

	assert.Equal(t, RuleStatusActive, rule.Status)
	assert.True(t, rule.UpdatedAt.After(originalUpdatedAt))
}

func TestRule_Deactivate(t *testing.T) {
	rule := NewRule("Test", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
	originalUpdatedAt := rule.UpdatedAt

	time.Sleep(1 * time.Millisecond)
	rule.Deactivate()

	assert.Equal(t, RuleStatusInactive, rule.Status)
	assert.True(t, rule.UpdatedAt.After(originalUpdatedAt))
}

func TestRule_Archive(t *testing.T) {
	rule := NewRule("Test", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
	originalUpdatedAt := rule.UpdatedAt

	time.Sleep(1 * time.Millisecond)
	rule.Archive()

	assert.Equal(t, RuleStatusArchived, rule.Status)
	assert.True(t, rule.UpdatedAt.After(originalUpdatedAt))
}

func TestRule_Validate(t *testing.T) {
	tests := []struct {
		name        string
		rule        func() *Rule
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid rule",
			rule: func() *Rule {
				return NewRule("Test", RuleTypeThreshold, json.RawMessage(`{"field": "temp"}`), ActionTypeNotify, json.RawMessage(`{"title": "Alert"}`))
			},
			expectError: false,
		},
		{
			name: "empty name",
			rule: func() *Rule {
				r := NewRule("", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
				return r
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "empty config",
			rule: func() *Rule {
				r := NewRule("Test", RuleTypeThreshold, nil, ActionTypeNotify, json.RawMessage(`{}`))
				return r
			},
			expectError: true,
			errorMsg:    "config is required",
		},
		{
			name: "empty actionConfig",
			rule: func() *Rule {
				r := NewRule("Test", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, nil)
				return r
			},
			expectError: true,
			errorMsg:    "actionConfig is required",
		},
		{
			name: "invalid ruleType",
			rule: func() *Rule {
				r := NewRule("Test", RuleType("INVALID"), json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
				return r
			},
			expectError: true,
			errorMsg:    "invalid ruleType",
		},
		{
			name: "invalid actionType",
			rule: func() *Rule {
				r := NewRule("Test", RuleTypeThreshold, json.RawMessage(`{}`), ActionType("INVALID"), json.RawMessage(`{}`))
				return r
			},
			expectError: true,
			errorMsg:    "invalid actionType",
		},
		{
			name: "invalid config JSON",
			rule: func() *Rule {
				r := NewRule("Test", RuleTypeThreshold, json.RawMessage(`{invalid json}`), ActionTypeNotify, json.RawMessage(`{}`))
				return r
			},
			expectError: true,
			errorMsg:    "config must be valid JSON",
		},
		{
			name: "invalid actionConfig JSON",
			rule: func() *Rule {
				r := NewRule("Test", RuleTypeThreshold, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{invalid}`))
				return r
			},
			expectError: true,
			errorMsg:    "actionConfig must be valid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := tt.rule()
			err := rule.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRule_Validate_AllRuleTypes(t *testing.T) {
	validRuleTypes := []RuleType{
		RuleTypeThreshold,
		RuleTypeRepeated,
		RuleTypeMissingField,
		RuleTypeStreamDisconnect,
		RuleTypeTimeWindow,
	}

	for _, rt := range validRuleTypes {
		t.Run(string(rt), func(t *testing.T) {
			rule := NewRule("Test", rt, json.RawMessage(`{}`), ActionTypeNotify, json.RawMessage(`{}`))
			err := rule.Validate()
			require.NoError(t, err)
		})
	}
}

func TestRule_Validate_AllActionTypes(t *testing.T) {
	validActionTypes := []ActionType{
		ActionTypeNotify,
		ActionTypeWebhook,
		ActionTypeEscalate,
	}

	for _, at := range validActionTypes {
		t.Run(string(at), func(t *testing.T) {
			rule := NewRule("Test", RuleTypeThreshold, json.RawMessage(`{}`), at, json.RawMessage(`{}`))
			err := rule.Validate()
			require.NoError(t, err)
		})
	}
}

func TestRuleType_Constants(t *testing.T) {
	assert.Equal(t, RuleType("THRESHOLD"), RuleTypeThreshold)
	assert.Equal(t, RuleType("REPEATED"), RuleTypeRepeated)
	assert.Equal(t, RuleType("MISSING_FIELD"), RuleTypeMissingField)
	assert.Equal(t, RuleType("STREAM_DISCONNECT"), RuleTypeStreamDisconnect)
	assert.Equal(t, RuleType("TIME_WINDOW"), RuleTypeTimeWindow)
}

func TestRuleStatus_Constants(t *testing.T) {
	assert.Equal(t, RuleStatus("ACTIVE"), RuleStatusActive)
	assert.Equal(t, RuleStatus("INACTIVE"), RuleStatusInactive)
	assert.Equal(t, RuleStatus("ARCHIVED"), RuleStatusArchived)
}

func TestActionType_Constants(t *testing.T) {
	assert.Equal(t, ActionType("NOTIFY"), ActionTypeNotify)
	assert.Equal(t, ActionType("WEBHOOK"), ActionTypeWebhook)
	assert.Equal(t, ActionType("ESCALATE"), ActionTypeEscalate)
}

func TestThresholdConfig_Serialization(t *testing.T) {
	config := ThresholdConfig{
		Field:     "temperature",
		Operator:  ">",
		Threshold: 100.5,
		EventType: "sensor_reading",
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var parsed ThresholdConfig
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, config, parsed)
}

func TestRepeatedConfig_Serialization(t *testing.T) {
	config := RepeatedConfig{
		Field:         "error_code",
		Count:         5,
		WindowSeconds: 300,
		EventType:     "error",
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var parsed RepeatedConfig
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, config, parsed)
}

func TestMissingFieldConfig_Serialization(t *testing.T) {
	config := MissingFieldConfig{
		Field:     "device_id",
		EventType: "telemetry",
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var parsed MissingFieldConfig
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, config, parsed)
}

func TestStreamDisconnectConfig_Serialization(t *testing.T) {
	config := StreamDisconnectConfig{
		SourceID:       "gateway-01",
		TimeoutSeconds: 60,
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var parsed StreamDisconnectConfig
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, config, parsed)
}

func TestTimeWindowConfig_Serialization(t *testing.T) {
	config := TimeWindowConfig{
		WindowSize: "5m",
		Condition:  "failure_rate > 0.8",
		EventType:  "transaction",
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var parsed TimeWindowConfig
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, config, parsed)
}

func TestNotifyConfig_Serialization(t *testing.T) {
	config := NotifyConfig{
		Severity: "HIGH",
		Title:    "Alert Title",
		Message:  "Alert message body",
		Targets:  []string{"user1", "user2"},
		Dedupe: &DedupeConfig{
			Key:             "source_device",
			CooldownSeconds: 300,
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var parsed NotifyConfig
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, config, parsed)
}

func TestWebhookConfig_Serialization(t *testing.T) {
	config := WebhookConfig{
		URL:     "https://example.com/webhook",
		Method:  "POST",
		Headers: map[string]string{"Authorization": "Bearer token"},
		Timeout: 30,
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var parsed WebhookConfig
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, config, parsed)
}
