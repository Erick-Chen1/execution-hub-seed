package notification

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status represents the delivery status of a notification
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusSent      Status = "SENT"
	StatusDelivered Status = "DELIVERED"
	StatusFailed    Status = "FAILED"
	StatusExpired   Status = "EXPIRED"
)

// Channel represents the notification delivery channel
type Channel string

const (
	ChannelSSE     Channel = "SSE"
	ChannelWebhook Channel = "WEBHOOK"
	ChannelEmail   Channel = "EMAIL"
)

// Priority represents the notification priority
type Priority string

const (
	PriorityLow      Priority = "LOW"
	PriorityMedium   Priority = "MEDIUM"
	PriorityHigh     Priority = "HIGH"
	PriorityCritical Priority = "CRITICAL"
)

var (
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrAlreadyDelivered  = errors.New("notification already delivered")
	ErrExpired           = errors.New("notification has expired")
	ErrClientNotFound    = errors.New("SSE client not found")
	ErrChannelFull       = errors.New("SSE message channel full")
	ErrCannotRetry       = errors.New("cannot retry notification")
)

// Notification represents a notification to be sent to users
type Notification struct {
	ID               int64           `json:"id"`
	NotificationID   uuid.UUID       `json:"notificationId"`
	ActionID         uuid.UUID       `json:"actionId"`
	DedupeKey        *string         `json:"dedupeKey,omitempty"`
	Channel          Channel         `json:"channel"`
	Priority         Priority        `json:"priority"`
	Title            string          `json:"title"`
	Body             string          `json:"body"`
	Payload          json.RawMessage `json:"payload"`
	Status           Status          `json:"status"`
	TargetUserID     *string         `json:"targetUserId,omitempty"`
	TargetGroup      *string         `json:"targetGroup,omitempty"`
	RetryCount       int             `json:"retryCount"`
	MaxRetries       int             `json:"maxRetries"`
	LastError        *string         `json:"lastError,omitempty"`
	ExpiresAt        *time.Time      `json:"expiresAt,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	SentAt           *time.Time      `json:"sentAt,omitempty"`
	DeliveredAt      *time.Time      `json:"deliveredAt,omitempty"`
	FailedAt         *time.Time      `json:"failedAt,omitempty"`
	TraceID          *string         `json:"traceId,omitempty"`
}

// NewNotification creates a new notification
func NewNotification(
	actionID uuid.UUID,
	channel Channel,
	priority Priority,
	title string,
	body string,
	payload json.RawMessage,
) *Notification {
	return &Notification{
		NotificationID: uuid.New(),
		ActionID:       actionID,
		Channel:        channel,
		Priority:       priority,
		Title:          title,
		Body:           body,
		Payload:        payload,
		Status:         StatusPending,
		MaxRetries:     3,
		CreatedAt:      time.Now().UTC(),
	}
}

// SetTarget sets the notification target (user or group)
func (n *Notification) SetTarget(userID *string, group *string) {
	n.TargetUserID = userID
	n.TargetGroup = group
}

// SetExpiry sets the expiration time
func (n *Notification) SetExpiry(expiresAt time.Time) {
	n.ExpiresAt = &expiresAt
}

// SetTraceID sets the trace ID
func (n *Notification) SetTraceID(traceID string) {
	n.TraceID = &traceID
}

// IsExpired checks if the notification has expired
func (n *Notification) IsExpired() bool {
	if n.ExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*n.ExpiresAt)
}

// CanTransitionTo checks if a transition to the target status is valid
func (n *Notification) CanTransitionTo(target Status) bool {
	transitions := map[Status][]Status{
		StatusPending:   {StatusSent, StatusFailed, StatusExpired},
		StatusSent:      {StatusDelivered, StatusFailed},
		StatusDelivered: {},
		StatusFailed:    {StatusPending}, // Retry
		StatusExpired:   {},
	}

	allowed, ok := transitions[n.Status]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

// MarkSent marks the notification as sent
func (n *Notification) MarkSent() error {
	if n.IsExpired() {
		n.Status = StatusExpired
		return ErrExpired
	}
	if !n.CanTransitionTo(StatusSent) {
		return ErrInvalidTransition
	}
	n.Status = StatusSent
	now := time.Now().UTC()
	n.SentAt = &now
	return nil
}

// MarkDelivered marks the notification as delivered
func (n *Notification) MarkDelivered() error {
	if !n.CanTransitionTo(StatusDelivered) {
		return ErrInvalidTransition
	}
	n.Status = StatusDelivered
	now := time.Now().UTC()
	n.DeliveredAt = &now
	return nil
}

// MarkFailed marks the notification as failed
func (n *Notification) MarkFailed(errMsg string) error {
	// Check expiration first - expired notifications should transition to EXPIRED state
	if n.IsExpired() {
		n.Status = StatusExpired
		return ErrExpired
	}
	if !n.CanTransitionTo(StatusFailed) {
		return ErrInvalidTransition
	}
	n.Status = StatusFailed
	now := time.Now().UTC()
	n.FailedAt = &now
	n.LastError = &errMsg
	n.RetryCount++
	return nil
}

// MarkExpired marks the notification as expired
func (n *Notification) MarkExpired() error {
	if !n.CanTransitionTo(StatusExpired) {
		return ErrInvalidTransition
	}
	n.Status = StatusExpired
	return nil
}

// CanRetry checks if the notification can be retried
func (n *Notification) CanRetry() bool {
	return n.Status == StatusFailed && n.RetryCount < n.MaxRetries && !n.IsExpired()
}

// ResetForRetry resets the notification for retry
func (n *Notification) ResetForRetry() error {
	if !n.CanRetry() {
		return ErrCannotRetry
	}
	n.Status = StatusPending
	n.FailedAt = nil
	return nil
}

// IsTerminal returns true if the notification is in a terminal state
func (n *Notification) IsTerminal() bool {
	return n.Status == StatusDelivered ||
		n.Status == StatusExpired ||
		(n.Status == StatusFailed && !n.CanRetry())
}

// DeliveryAttempt represents a single delivery attempt
type DeliveryAttempt struct {
	ID             int64     `json:"id"`
	NotificationID uuid.UUID `json:"notificationId"`
	AttemptNumber  int       `json:"attemptNumber"`
	Status         Status    `json:"status"`
	AttemptedAt    time.Time `json:"attemptedAt"`
	ResponseCode   *int      `json:"responseCode,omitempty"`
	ResponseBody   *string   `json:"responseBody,omitempty"`
	ErrorMessage   *string   `json:"errorMessage,omitempty"`
	DurationMs     int       `json:"durationMs"`
}

// NewDeliveryAttempt creates a new delivery attempt record
func NewDeliveryAttempt(notificationID uuid.UUID, attemptNumber int) *DeliveryAttempt {
	return &DeliveryAttempt{
		NotificationID: notificationID,
		AttemptNumber:  attemptNumber,
		AttemptedAt:    time.Now().UTC(),
	}
}

// SSEClient represents an active SSE connection
type SSEClient struct {
	ClientID    string
	UserID      *string
	Groups      []string
	ConnectedAt time.Time
	LastEventAt *time.Time
	MessageChan chan *SSEMessage
}

// NewSSEClient creates a new SSE client
func NewSSEClient(clientID string, userID *string, groups []string) *SSEClient {
	return &SSEClient{
		ClientID:    clientID,
		UserID:      userID,
		Groups:      groups,
		ConnectedAt: time.Now().UTC(),
		MessageChan: make(chan *SSEMessage, 100),
	}
}

// Close closes the client's message channel
func (c *SSEClient) Close() {
	close(c.MessageChan)
}

// SSEMessage represents a message to be sent via SSE
type SSEMessage struct {
	ID        string          `json:"id"`
	Event     string          `json:"event"`
	Data      json.RawMessage `json:"data"`
	Retry     *int            `json:"retry,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewSSEMessage creates a new SSE message
func NewSSEMessage(event string, data json.RawMessage) *SSEMessage {
	return &SSEMessage{
		ID:        uuid.New().String(),
		Event:     event,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
}

// Filter represents filters for querying notifications
type Filter struct {
	ActionID     *uuid.UUID
	Channel      *Channel
	Status       *Status
	Priority     *Priority
	TargetUserID *string
	TargetGroup  *string
	Since        *time.Time
	Until        *time.Time
}
