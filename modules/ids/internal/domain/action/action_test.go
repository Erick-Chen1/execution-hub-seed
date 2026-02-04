package action

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAction(t *testing.T) {
	ruleID := uuid.New()
	evaluationID := uuid.New()
	actionConfig := json.RawMessage(`{"title": "test"}`)

	action := NewAction(ruleID, 1, evaluationID, TypeNotify, actionConfig)

	require.NotNil(t, action)
	assert.NotEqual(t, uuid.Nil, action.ActionID)
	assert.Equal(t, ruleID, action.RuleID)
	assert.Equal(t, 1, action.RuleVersion)
	assert.Equal(t, evaluationID, action.EvaluationID)
	assert.Equal(t, TypeNotify, action.ActionType)
	assert.Equal(t, actionConfig, action.ActionConfig)
	assert.Equal(t, StatusCreated, action.Status)
	assert.Equal(t, PriorityMedium, action.Priority)
	assert.Equal(t, 3, action.MaxRetries)
	assert.Equal(t, 0, action.RetryCount)
	assert.False(t, action.CreatedAt.IsZero())
}

func TestAction_SetPriority(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		expected Priority
	}{
		{name: "critical", severity: "CRITICAL", expected: PriorityCritical},
		{name: "high", severity: "HIGH", expected: PriorityHigh},
		{name: "medium", severity: "MEDIUM", expected: PriorityMedium},
		{name: "low", severity: "LOW", expected: PriorityLow},
		{name: "unknown defaults to low", severity: "UNKNOWN", expected: PriorityLow},
		{name: "empty defaults to low", severity: "", expected: PriorityLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
			action.SetPriority(tt.severity)
			assert.Equal(t, tt.expected, action.Priority)
		})
	}
}

func TestAction_SetDedupeKey(t *testing.T) {
	action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
	assert.Nil(t, action.DedupeKey)

	action.SetDedupeKey("test-key")

	require.NotNil(t, action.DedupeKey)
	assert.Equal(t, "test-key", *action.DedupeKey)
}

func TestAction_SetCooldown(t *testing.T) {
	action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
	assert.Nil(t, action.CooldownUntil)

	cooldownTime := time.Now().Add(1 * time.Hour)
	action.SetCooldown(cooldownTime)

	require.NotNil(t, action.CooldownUntil)
	assert.Equal(t, cooldownTime, *action.CooldownUntil)
}

func TestAction_SetTraceID(t *testing.T) {
	action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
	assert.Nil(t, action.TraceID)

	action.SetTraceID("trace-123")

	require.NotNil(t, action.TraceID)
	assert.Equal(t, "trace-123", *action.TraceID)
}

func TestAction_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     Status
		to       Status
		expected bool
	}{
		// CREATED transitions
		{name: "CREATED -> DISPATCHING", from: StatusCreated, to: StatusDispatching, expected: true},
		{name: "CREATED -> FAILED", from: StatusCreated, to: StatusFailed, expected: true},
		{name: "CREATED -> DISPATCHED (invalid)", from: StatusCreated, to: StatusDispatched, expected: false},
		{name: "CREATED -> ACKED (invalid)", from: StatusCreated, to: StatusAcked, expected: false},
		{name: "CREATED -> RESOLVED (invalid)", from: StatusCreated, to: StatusResolved, expected: false},

		// DISPATCHING transitions
		{name: "DISPATCHING -> DISPATCHED", from: StatusDispatching, to: StatusDispatched, expected: true},
		{name: "DISPATCHING -> FAILED", from: StatusDispatching, to: StatusFailed, expected: true},
		{name: "DISPATCHING -> ACKED (invalid)", from: StatusDispatching, to: StatusAcked, expected: false},

		// DISPATCHED transitions
		{name: "DISPATCHED -> ACKED", from: StatusDispatched, to: StatusAcked, expected: true},
		{name: "DISPATCHED -> RESOLVED", from: StatusDispatched, to: StatusResolved, expected: true},
		{name: "DISPATCHED -> FAILED", from: StatusDispatched, to: StatusFailed, expected: true},
		{name: "DISPATCHED -> CREATED (invalid)", from: StatusDispatched, to: StatusCreated, expected: false},

		// ACKED transitions
		{name: "ACKED -> RESOLVED", from: StatusAcked, to: StatusResolved, expected: true},
		{name: "ACKED -> FAILED (invalid)", from: StatusAcked, to: StatusFailed, expected: false},

		// RESOLVED transitions (terminal)
		{name: "RESOLVED -> CREATED (invalid)", from: StatusResolved, to: StatusCreated, expected: false},
		{name: "RESOLVED -> FAILED (invalid)", from: StatusResolved, to: StatusFailed, expected: false},

		// FAILED transitions (retry)
		{name: "FAILED -> CREATED (retry)", from: StatusFailed, to: StatusCreated, expected: true},
		{name: "FAILED -> DISPATCHING (invalid)", from: StatusFailed, to: StatusDispatching, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
			action.Status = tt.from
			assert.Equal(t, tt.expected, action.CanTransitionTo(tt.to))
		})
	}
}

func TestAction_Dispatch(t *testing.T) {
	t.Run("success from CREATED", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		assert.Equal(t, StatusCreated, action.Status)

		err := action.Dispatch()

		require.NoError(t, err)
		assert.Equal(t, StatusDispatching, action.Status)
	})

	t.Run("error from invalid state", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusResolved

		err := action.Dispatch()

		assert.ErrorIs(t, err, ErrInvalidTransition)
		assert.Equal(t, StatusResolved, action.Status)
	})
}

func TestAction_ConfirmDispatched(t *testing.T) {
	t.Run("success from DISPATCHING", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusDispatching
		assert.Nil(t, action.DispatchedAt)

		err := action.ConfirmDispatched()

		require.NoError(t, err)
		assert.Equal(t, StatusDispatched, action.Status)
		require.NotNil(t, action.DispatchedAt)
		assert.False(t, action.DispatchedAt.IsZero())
	})

	t.Run("error from invalid state", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		// Status is CREATED

		err := action.ConfirmDispatched()

		assert.ErrorIs(t, err, ErrInvalidTransition)
		assert.Equal(t, StatusCreated, action.Status)
		assert.Nil(t, action.DispatchedAt)
	})
}

func TestAction_Acknowledge(t *testing.T) {
	t.Run("success from DISPATCHED", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusDispatched
		assert.Nil(t, action.AckedAt)
		assert.Nil(t, action.AckedBy)

		err := action.Acknowledge("user@example.com")

		require.NoError(t, err)
		assert.Equal(t, StatusAcked, action.Status)
		require.NotNil(t, action.AckedAt)
		require.NotNil(t, action.AckedBy)
		assert.Equal(t, "user@example.com", *action.AckedBy)
	})

	t.Run("error from invalid state", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		// Status is CREATED

		err := action.Acknowledge("user@example.com")

		assert.ErrorIs(t, err, ErrInvalidTransition)
		assert.Equal(t, StatusCreated, action.Status)
	})
}

func TestAction_Resolve(t *testing.T) {
	t.Run("success from DISPATCHED", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusDispatched
		assert.Nil(t, action.ResolvedAt)
		assert.Nil(t, action.ResolvedBy)

		err := action.Resolve("user@example.com")

		require.NoError(t, err)
		assert.Equal(t, StatusResolved, action.Status)
		require.NotNil(t, action.ResolvedAt)
		require.NotNil(t, action.ResolvedBy)
		assert.Equal(t, "user@example.com", *action.ResolvedBy)
	})

	t.Run("success from ACKED", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusAcked

		err := action.Resolve("user@example.com")

		require.NoError(t, err)
		assert.Equal(t, StatusResolved, action.Status)
	})

	t.Run("error from invalid state", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		// Status is CREATED

		err := action.Resolve("user@example.com")

		assert.ErrorIs(t, err, ErrInvalidTransition)
		assert.Equal(t, StatusCreated, action.Status)
	})
}

func TestAction_Fail(t *testing.T) {
	t.Run("success from CREATED", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		assert.Equal(t, 0, action.RetryCount)
		assert.Nil(t, action.FailedAt)
		assert.Nil(t, action.LastError)

		err := action.Fail("connection timeout")

		require.NoError(t, err)
		assert.Equal(t, StatusFailed, action.Status)
		assert.Equal(t, 1, action.RetryCount)
		require.NotNil(t, action.FailedAt)
		require.NotNil(t, action.LastError)
		assert.Equal(t, "connection timeout", *action.LastError)
	})

	t.Run("success from DISPATCHING", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusDispatching

		err := action.Fail("send failed")

		require.NoError(t, err)
		assert.Equal(t, StatusFailed, action.Status)
	})

	t.Run("success from DISPATCHED", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusDispatched

		err := action.Fail("acknowledgement timeout")

		require.NoError(t, err)
		assert.Equal(t, StatusFailed, action.Status)
	})

	t.Run("error from ACKED (invalid)", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusAcked

		err := action.Fail("error")

		assert.ErrorIs(t, err, ErrInvalidTransition)
		assert.Equal(t, StatusAcked, action.Status)
	})

	t.Run("increments retry count on each fail", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		assert.Equal(t, 0, action.RetryCount)

		// First failure
		_ = action.Fail("error 1")
		assert.Equal(t, 1, action.RetryCount)

		// Reset for retry
		action.Status = StatusCreated
		action.FailedAt = nil

		// Second failure
		_ = action.Fail("error 2")
		assert.Equal(t, 2, action.RetryCount)
	})
}

func TestAction_CanRetry(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		retryCount int
		maxRetries int
		expected   bool
	}{
		{
			name:       "can retry - failed with retries remaining",
			status:     StatusFailed,
			retryCount: 1,
			maxRetries: 3,
			expected:   true,
		},
		{
			name:       "cannot retry - max retries reached",
			status:     StatusFailed,
			retryCount: 3,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "cannot retry - exceeded max retries",
			status:     StatusFailed,
			retryCount: 4,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "cannot retry - not in failed state",
			status:     StatusCreated,
			retryCount: 0,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "cannot retry - resolved state",
			status:     StatusResolved,
			retryCount: 0,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "can retry - zero retries used",
			status:     StatusFailed,
			retryCount: 0,
			maxRetries: 3,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
			action.Status = tt.status
			action.RetryCount = tt.retryCount
			action.MaxRetries = tt.maxRetries

			assert.Equal(t, tt.expected, action.CanRetry())
		})
	}
}

func TestAction_ResetForRetry(t *testing.T) {
	t.Run("success when can retry", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusFailed
		action.RetryCount = 1
		now := time.Now()
		action.FailedAt = &now

		err := action.ResetForRetry()

		require.NoError(t, err)
		assert.Equal(t, StatusCreated, action.Status)
		assert.Nil(t, action.FailedAt)
		assert.Equal(t, 1, action.RetryCount) // RetryCount is not reset
	})

	t.Run("error when max retries exceeded", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusFailed
		action.RetryCount = 3
		action.MaxRetries = 3

		err := action.ResetForRetry()

		assert.ErrorIs(t, err, ErrCannotRetry)
		assert.Equal(t, StatusFailed, action.Status)
	})

	t.Run("error when not in failed state", func(t *testing.T) {
		action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
		action.Status = StatusCreated

		err := action.ResetForRetry()

		assert.ErrorIs(t, err, ErrCannotRetry)
	})
}

func TestAction_IsTerminal(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		retryCount int
		maxRetries int
		expected   bool
	}{
		{
			name:       "RESOLVED is terminal",
			status:     StatusResolved,
			retryCount: 0,
			maxRetries: 3,
			expected:   true,
		},
		{
			name:       "FAILED with no retries left is terminal",
			status:     StatusFailed,
			retryCount: 3,
			maxRetries: 3,
			expected:   true,
		},
		{
			name:       "FAILED with retries remaining is not terminal",
			status:     StatusFailed,
			retryCount: 1,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "CREATED is not terminal",
			status:     StatusCreated,
			retryCount: 0,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "DISPATCHING is not terminal",
			status:     StatusDispatching,
			retryCount: 0,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "DISPATCHED is not terminal",
			status:     StatusDispatched,
			retryCount: 0,
			maxRetries: 3,
			expected:   false,
		},
		{
			name:       "ACKED is not terminal",
			status:     StatusAcked,
			retryCount: 0,
			maxRetries: 3,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := NewAction(uuid.New(), 1, uuid.New(), TypeNotify, nil)
			action.Status = tt.status
			action.RetryCount = tt.retryCount
			action.MaxRetries = tt.maxRetries

			assert.Equal(t, tt.expected, action.IsTerminal())
		})
	}
}

func TestNewStateTransition(t *testing.T) {
	actionID := uuid.New()
	fromStatus := StatusCreated
	toStatus := StatusDispatching
	reason := "starting dispatch"

	transition := NewStateTransition(actionID, &fromStatus, toStatus, &reason)

	require.NotNil(t, transition)
	assert.Equal(t, actionID, transition.ActionID)
	require.NotNil(t, transition.FromStatus)
	assert.Equal(t, fromStatus, *transition.FromStatus)
	assert.Equal(t, toStatus, transition.ToStatus)
	require.NotNil(t, transition.Reason)
	assert.Equal(t, reason, *transition.Reason)
	assert.False(t, transition.TransitionedAt.IsZero())
}

func TestNewStateTransition_WithNilValues(t *testing.T) {
	actionID := uuid.New()
	toStatus := StatusCreated

	transition := NewStateTransition(actionID, nil, toStatus, nil)

	require.NotNil(t, transition)
	assert.Nil(t, transition.FromStatus)
	assert.Nil(t, transition.Reason)
}

func TestActionType_Constants(t *testing.T) {
	assert.Equal(t, Type("NOTIFY"), TypeNotify)
	assert.Equal(t, Type("WEBHOOK"), TypeWebhook)
	assert.Equal(t, Type("ESCALATE"), TypeEscalate)
}

func TestStatus_Constants(t *testing.T) {
	assert.Equal(t, Status("CREATED"), StatusCreated)
	assert.Equal(t, Status("DISPATCHING"), StatusDispatching)
	assert.Equal(t, Status("DISPATCHED"), StatusDispatched)
	assert.Equal(t, Status("ACKED"), StatusAcked)
	assert.Equal(t, Status("RESOLVED"), StatusResolved)
	assert.Equal(t, Status("FAILED"), StatusFailed)
}

func TestPriority_Constants(t *testing.T) {
	assert.Equal(t, Priority("LOW"), PriorityLow)
	assert.Equal(t, Priority("MEDIUM"), PriorityMedium)
	assert.Equal(t, Priority("HIGH"), PriorityHigh)
	assert.Equal(t, Priority("CRITICAL"), PriorityCritical)
}
