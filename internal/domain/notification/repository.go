package notification

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/mock_repository.go -package=mocks . Repository,SSEHub

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines the interface for notification persistence
type Repository interface {
	// Notification operations
	Create(ctx context.Context, notification *Notification) error
	GetByID(ctx context.Context, notificationID uuid.UUID) (*Notification, error)
	GetByActionID(ctx context.Context, actionID uuid.UUID) ([]*Notification, error)
	FindByDedupeKey(ctx context.Context, dedupeKey string, since time.Time) (*Notification, error)
	List(ctx context.Context, filter Filter, limit, offset int) ([]*Notification, error)
	Update(ctx context.Context, notification *Notification) error
	UpdateStatus(ctx context.Context, notificationID uuid.UUID, status Status) error

	// Delivery attempts
	RecordAttempt(ctx context.Context, attempt *DeliveryAttempt) error
	GetAttempts(ctx context.Context, notificationID uuid.UUID) ([]*DeliveryAttempt, error)

	// DLQ operations for delivery attempts
	SaveAttemptToDLQ(ctx context.Context, attempt *DeliveryAttempt, originalErr error) error

	// Retry support
	ListPendingNotifications(ctx context.Context, limit int) ([]*Notification, error)
	ListRetryableNotifications(ctx context.Context, limit int) ([]*Notification, error)

	// Expiration
	ExpireNotifications(ctx context.Context) (int64, error)
}

// SSEHub defines the interface for managing SSE connections
type SSEHub interface {
	// Client management
	Register(client *SSEClient)
	Unregister(clientID string)
	GetClient(clientID string) *SSEClient
	GetClientCount() int

	// Broadcasting
	BroadcastToAll(message *SSEMessage)
	BroadcastToUser(userID string, message *SSEMessage)
	BroadcastToGroup(group string, message *SSEMessage)
	SendToClient(clientID string, message *SSEMessage) error

	// Lifecycle
	Start(ctx context.Context)
	Stop()
}
