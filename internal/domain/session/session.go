package session

import (
	"time"

	"github.com/google/uuid"
)

// Session represents an authenticated session.
type Session struct {
	ID         int64      `json:"id"`
	SessionID  uuid.UUID  `json:"sessionId"`
	TokenHash  string     `json:"-"`
	UserID     uuid.UUID  `json:"userId"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
	UserAgent  *string    `json:"userAgent,omitempty"`
	IPAddress  *string    `json:"ipAddress,omitempty"`
}

func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}
