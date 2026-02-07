package notification

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNotification(t *testing.T) {
	actionID := uuid.New()
	payload := json.RawMessage(`{"key": "value"}`)

	notification := NewNotification(actionID, ChannelSSE, PriorityHigh, "Test Title", "Test Body", payload)

	require.NotNil(t, notification)
	assert.NotEqual(t, uuid.Nil, notification.NotificationID)
	assert.Equal(t, actionID, notification.ActionID)
	assert.Equal(t, ChannelSSE, notification.Channel)
	assert.Equal(t, PriorityHigh, notification.Priority)
	assert.Equal(t, "Test Title", notification.Title)
	assert.Equal(t, "Test Body", notification.Body)
	assert.Equal(t, payload, notification.Payload)
	assert.Equal(t, StatusPending, notification.Status)
	assert.Equal(t, 3, notification.MaxRetries)
	assert.Equal(t, 0, notification.RetryCount)
	assert.False(t, notification.CreatedAt.IsZero())
	assert.Nil(t, notification.TargetUserID)
	assert.Nil(t, notification.TargetGroup)
	assert.Nil(t, notification.ExpiresAt)
}

func TestNotification_SetTarget(t *testing.T) {
	t.Run("set user target", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		userID := "user123"

		notification.SetTarget(&userID, nil)

		require.NotNil(t, notification.TargetUserID)
		assert.Equal(t, "user123", *notification.TargetUserID)
		assert.Nil(t, notification.TargetGroup)
	})

	t.Run("set group target", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		group := "admin-group"

		notification.SetTarget(nil, &group)

		assert.Nil(t, notification.TargetUserID)
		require.NotNil(t, notification.TargetGroup)
		assert.Equal(t, "admin-group", *notification.TargetGroup)
	})

	t.Run("set both targets", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		userID := "user123"
		group := "admin-group"

		notification.SetTarget(&userID, &group)

		require.NotNil(t, notification.TargetUserID)
		require.NotNil(t, notification.TargetGroup)
		assert.Equal(t, "user123", *notification.TargetUserID)
		assert.Equal(t, "admin-group", *notification.TargetGroup)
	})
}

func TestNotification_SetExpiry(t *testing.T) {
	notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
	assert.Nil(t, notification.ExpiresAt)

	expiryTime := time.Now().Add(24 * time.Hour)
	notification.SetExpiry(expiryTime)

	require.NotNil(t, notification.ExpiresAt)
	assert.Equal(t, expiryTime, *notification.ExpiresAt)
}

func TestNotification_SetTraceID(t *testing.T) {
	notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
	assert.Nil(t, notification.TraceID)

	notification.SetTraceID("trace-abc-123")

	require.NotNil(t, notification.TraceID)
	assert.Equal(t, "trace-abc-123", *notification.TraceID)
}

func TestNotification_IsExpired(t *testing.T) {
	t.Run("not expired when ExpiresAt is nil", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		assert.Nil(t, notification.ExpiresAt)

		assert.False(t, notification.IsExpired())
	})

	t.Run("not expired when ExpiresAt is in the future", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		futureTime := time.Now().Add(1 * time.Hour)
		notification.SetExpiry(futureTime)

		assert.False(t, notification.IsExpired())
	})

	t.Run("expired when ExpiresAt is in the past", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		pastTime := time.Now().Add(-1 * time.Hour)
		notification.SetExpiry(pastTime)

		assert.True(t, notification.IsExpired())
	})
}

func TestNotification_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     Status
		to       Status
		expected bool
	}{
		// PENDING transitions
		{name: "PENDING -> SENT", from: StatusPending, to: StatusSent, expected: true},
		{name: "PENDING -> FAILED", from: StatusPending, to: StatusFailed, expected: true},
		{name: "PENDING -> EXPIRED", from: StatusPending, to: StatusExpired, expected: true},
		{name: "PENDING -> DELIVERED (invalid)", from: StatusPending, to: StatusDelivered, expected: false},

		// SENT transitions
		{name: "SENT -> DELIVERED", from: StatusSent, to: StatusDelivered, expected: true},
		{name: "SENT -> FAILED", from: StatusSent, to: StatusFailed, expected: true},
		{name: "SENT -> PENDING (invalid)", from: StatusSent, to: StatusPending, expected: false},
		{name: "SENT -> EXPIRED (invalid)", from: StatusSent, to: StatusExpired, expected: false},

		// DELIVERED transitions (terminal)
		{name: "DELIVERED -> PENDING (invalid)", from: StatusDelivered, to: StatusPending, expected: false},
		{name: "DELIVERED -> FAILED (invalid)", from: StatusDelivered, to: StatusFailed, expected: false},

		// FAILED transitions (retry)
		{name: "FAILED -> PENDING (retry)", from: StatusFailed, to: StatusPending, expected: true},
		{name: "FAILED -> SENT (invalid)", from: StatusFailed, to: StatusSent, expected: false},

		// EXPIRED transitions (terminal)
		{name: "EXPIRED -> PENDING (invalid)", from: StatusExpired, to: StatusPending, expected: false},
		{name: "EXPIRED -> FAILED (invalid)", from: StatusExpired, to: StatusFailed, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
			notification.Status = tt.from
			assert.Equal(t, tt.expected, notification.CanTransitionTo(tt.to))
		})
	}
}

func TestNotification_MarkSent(t *testing.T) {
	t.Run("success from PENDING", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		assert.Nil(t, notification.SentAt)

		err := notification.MarkSent()

		require.NoError(t, err)
		assert.Equal(t, StatusSent, notification.Status)
		require.NotNil(t, notification.SentAt)
		assert.False(t, notification.SentAt.IsZero())
	})

	t.Run("error when already expired", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		pastTime := time.Now().Add(-1 * time.Hour)
		notification.SetExpiry(pastTime)

		err := notification.MarkSent()

		assert.ErrorIs(t, err, ErrExpired)
		assert.Equal(t, StatusExpired, notification.Status)
	})

	t.Run("error from invalid state", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		notification.Status = StatusDelivered

		err := notification.MarkSent()

		assert.ErrorIs(t, err, ErrInvalidTransition)
		assert.Equal(t, StatusDelivered, notification.Status)
	})
}

func TestNotification_MarkDelivered(t *testing.T) {
	t.Run("success from SENT", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		notification.Status = StatusSent
		assert.Nil(t, notification.DeliveredAt)

		err := notification.MarkDelivered()

		require.NoError(t, err)
		assert.Equal(t, StatusDelivered, notification.Status)
		require.NotNil(t, notification.DeliveredAt)
		assert.False(t, notification.DeliveredAt.IsZero())
	})

	t.Run("error from invalid state", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		// Status is PENDING

		err := notification.MarkDelivered()

		assert.ErrorIs(t, err, ErrInvalidTransition)
		assert.Equal(t, StatusPending, notification.Status)
	})
}

func TestNotification_MarkFailed(t *testing.T) {
	t.Run("success from PENDING", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		assert.Equal(t, 0, notification.RetryCount)
		assert.Nil(t, notification.FailedAt)
		assert.Nil(t, notification.LastError)

		err := notification.MarkFailed("connection refused")

		require.NoError(t, err)
		assert.Equal(t, StatusFailed, notification.Status)
		assert.Equal(t, 1, notification.RetryCount)
		require.NotNil(t, notification.FailedAt)
		require.NotNil(t, notification.LastError)
		assert.Equal(t, "connection refused", *notification.LastError)
	})

	t.Run("success from SENT", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		notification.Status = StatusSent

		err := notification.MarkFailed("delivery timeout")

		require.NoError(t, err)
		assert.Equal(t, StatusFailed, notification.Status)
	})

	t.Run("error from invalid state", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		notification.Status = StatusDelivered

		err := notification.MarkFailed("error")

		assert.ErrorIs(t, err, ErrInvalidTransition)
		assert.Equal(t, StatusDelivered, notification.Status)
	})

	t.Run("error when already expired", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		pastTime := time.Now().Add(-1 * time.Hour)
		notification.SetExpiry(pastTime)

		err := notification.MarkFailed("connection refused")

		assert.ErrorIs(t, err, ErrExpired)
		assert.Equal(t, StatusExpired, notification.Status)
	})

	t.Run("increments retry count on each fail", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		assert.Equal(t, 0, notification.RetryCount)

		// First failure
		_ = notification.MarkFailed("error 1")
		assert.Equal(t, 1, notification.RetryCount)

		// Reset for retry
		notification.Status = StatusPending
		notification.FailedAt = nil

		// Second failure
		_ = notification.MarkFailed("error 2")
		assert.Equal(t, 2, notification.RetryCount)
	})
}

func TestNotification_MarkExpired(t *testing.T) {
	t.Run("success from PENDING", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)

		err := notification.MarkExpired()

		require.NoError(t, err)
		assert.Equal(t, StatusExpired, notification.Status)
	})

	t.Run("error from invalid state", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		notification.Status = StatusDelivered

		err := notification.MarkExpired()

		assert.ErrorIs(t, err, ErrInvalidTransition)
		assert.Equal(t, StatusDelivered, notification.Status)
	})
}

func TestNotification_CanRetry(t *testing.T) {
	tests := []struct {
		name        string
		status      Status
		retryCount  int
		maxRetries  int
		expired     bool
		neverExpire bool // when true, ExpiresAt is not set (nil)
		expected    bool
	}{
		{
			name:        "can retry - failed with retries remaining and not expired",
			status:      StatusFailed,
			retryCount:  1,
			maxRetries:  3,
			expired:     false,
			neverExpire: false,
			expected:    true,
		},
		{
			name:        "can retry - failed with retries remaining and never expires",
			status:      StatusFailed,
			retryCount:  1,
			maxRetries:  3,
			expired:     false,
			neverExpire: true, // ExpiresAt is nil (never expires)
			expected:    true,
		},
		{
			name:       "cannot retry - max retries reached",
			status:     StatusFailed,
			retryCount: 3,
			maxRetries: 3,
			expired:    false,
			expected:   false,
		},
		{
			name:       "cannot retry - expired",
			status:     StatusFailed,
			retryCount: 1,
			maxRetries: 3,
			expired:    true,
			expected:   false,
		},
		{
			name:       "cannot retry - not in failed state",
			status:     StatusPending,
			retryCount: 0,
			maxRetries: 3,
			expired:    false,
			expected:   false,
		},
		{
			name:       "cannot retry - delivered state",
			status:     StatusDelivered,
			retryCount: 0,
			maxRetries: 3,
			expired:    false,
			expected:   false,
		},
		{
			name:        "can retry - zero retries used",
			status:      StatusFailed,
			retryCount:  0,
			maxRetries:  3,
			expired:     false,
			neverExpire: false,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
			notification.Status = tt.status
			notification.RetryCount = tt.retryCount
			notification.MaxRetries = tt.maxRetries
			if tt.expired {
				pastTime := time.Now().Add(-1 * time.Hour)
				notification.SetExpiry(pastTime)
			} else if !tt.neverExpire {
				// Only set expiry if not testing "never expire" scenario
				futureTime := time.Now().Add(1 * time.Hour)
				notification.SetExpiry(futureTime)
			}
			// When neverExpire is true, ExpiresAt remains nil (default)

			assert.Equal(t, tt.expected, notification.CanRetry())
		})
	}
}

func TestNotification_ResetForRetry(t *testing.T) {
	t.Run("success when can retry", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		notification.Status = StatusFailed
		notification.RetryCount = 1
		now := time.Now()
		notification.FailedAt = &now
		futureTime := time.Now().Add(1 * time.Hour)
		notification.SetExpiry(futureTime)

		err := notification.ResetForRetry()

		require.NoError(t, err)
		assert.Equal(t, StatusPending, notification.Status)
		assert.Nil(t, notification.FailedAt)
	})

	t.Run("error when max retries exceeded", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		notification.Status = StatusFailed
		notification.RetryCount = 3
		notification.MaxRetries = 3

		err := notification.ResetForRetry()

		assert.ErrorIs(t, err, ErrCannotRetry)
		assert.Equal(t, StatusFailed, notification.Status)
	})

	t.Run("error when expired", func(t *testing.T) {
		notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
		notification.Status = StatusFailed
		notification.RetryCount = 1
		pastTime := time.Now().Add(-1 * time.Hour)
		notification.SetExpiry(pastTime)

		err := notification.ResetForRetry()

		assert.ErrorIs(t, err, ErrCannotRetry)
		assert.Equal(t, StatusFailed, notification.Status)
	})
}

func TestNotification_IsTerminal(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		retryCount int
		maxRetries int
		expired    bool
		expected   bool
	}{
		{
			name:     "DELIVERED is terminal",
			status:   StatusDelivered,
			expected: true,
		},
		{
			name:     "EXPIRED is terminal",
			status:   StatusExpired,
			expected: true,
		},
		{
			name:       "FAILED with no retries left is terminal",
			status:     StatusFailed,
			retryCount: 3,
			maxRetries: 3,
			expired:    false,
			expected:   true,
		},
		{
			name:       "FAILED when expired is terminal",
			status:     StatusFailed,
			retryCount: 1,
			maxRetries: 3,
			expired:    true,
			expected:   true,
		},
		{
			name:       "FAILED with retries remaining is not terminal",
			status:     StatusFailed,
			retryCount: 1,
			maxRetries: 3,
			expired:    false,
			expected:   false,
		},
		{
			name:     "PENDING is not terminal",
			status:   StatusPending,
			expected: false,
		},
		{
			name:     "SENT is not terminal",
			status:   StatusSent,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notification := NewNotification(uuid.New(), ChannelSSE, PriorityMedium, "Title", "Body", nil)
			notification.Status = tt.status
			notification.RetryCount = tt.retryCount
			notification.MaxRetries = tt.maxRetries
			if tt.expired {
				pastTime := time.Now().Add(-1 * time.Hour)
				notification.SetExpiry(pastTime)
			}

			assert.Equal(t, tt.expected, notification.IsTerminal())
		})
	}
}

func TestNewDeliveryAttempt(t *testing.T) {
	notificationID := uuid.New()
	attemptNumber := 2

	attempt := NewDeliveryAttempt(notificationID, attemptNumber)

	require.NotNil(t, attempt)
	assert.Equal(t, notificationID, attempt.NotificationID)
	assert.Equal(t, attemptNumber, attempt.AttemptNumber)
	assert.False(t, attempt.AttemptedAt.IsZero())
	assert.Nil(t, attempt.ResponseCode)
	assert.Nil(t, attempt.ResponseBody)
	assert.Nil(t, attempt.ErrorMessage)
	assert.Equal(t, 0, attempt.DurationMs)
}

func TestNewSSEClient(t *testing.T) {
	t.Run("with user and groups", func(t *testing.T) {
		clientID := "client-123"
		userID := "user-456"
		groups := []string{"group1", "group2"}

		client := NewSSEClient(clientID, &userID, groups)

		require.NotNil(t, client)
		assert.Equal(t, clientID, client.ClientID)
		require.NotNil(t, client.UserID)
		assert.Equal(t, userID, *client.UserID)
		assert.Equal(t, groups, client.Groups)
		assert.False(t, client.ConnectedAt.IsZero())
		assert.Nil(t, client.LastEventAt)
		assert.NotNil(t, client.MessageChan)
	})

	t.Run("with nil user", func(t *testing.T) {
		client := NewSSEClient("client-123", nil, nil)

		require.NotNil(t, client)
		assert.Nil(t, client.UserID)
		assert.Nil(t, client.Groups)
	})
}

func TestSSEClient_Close(t *testing.T) {
	client := NewSSEClient("client-123", nil, nil)
	require.NotNil(t, client.MessageChan)

	// Should not panic
	client.Close()

	// Channel should be closed - sending should panic
	assert.Panics(t, func() {
		client.MessageChan <- &SSEMessage{}
	})
}

func TestNewSSEMessage(t *testing.T) {
	eventType := "notification"
	data := json.RawMessage(`{"title": "Test"}`)

	message := NewSSEMessage(eventType, data)

	require.NotNil(t, message)
	assert.NotEmpty(t, message.ID)
	assert.Equal(t, eventType, message.Event)
	assert.Equal(t, data, message.Data)
	assert.Nil(t, message.Retry)
	assert.False(t, message.Timestamp.IsZero())
}

func TestChannel_Constants(t *testing.T) {
	assert.Equal(t, Channel("SSE"), ChannelSSE)
	assert.Equal(t, Channel("WEBHOOK"), ChannelWebhook)
	assert.Equal(t, Channel("EMAIL"), ChannelEmail)
}

func TestStatus_Constants(t *testing.T) {
	assert.Equal(t, Status("PENDING"), StatusPending)
	assert.Equal(t, Status("SENT"), StatusSent)
	assert.Equal(t, Status("DELIVERED"), StatusDelivered)
	assert.Equal(t, Status("FAILED"), StatusFailed)
	assert.Equal(t, Status("EXPIRED"), StatusExpired)
}

func TestPriority_Constants(t *testing.T) {
	assert.Equal(t, Priority("LOW"), PriorityLow)
	assert.Equal(t, Priority("MEDIUM"), PriorityMedium)
	assert.Equal(t, Priority("HIGH"), PriorityHigh)
	assert.Equal(t, Priority("CRITICAL"), PriorityCritical)
}

func TestError_Constants(t *testing.T) {
	assert.NotNil(t, ErrInvalidTransition)
	assert.NotNil(t, ErrAlreadyDelivered)
	assert.NotNil(t, ErrExpired)
	assert.NotNil(t, ErrClientNotFound)
	assert.NotNil(t, ErrChannelFull)
}
