package executor

import (
	"time"
)

// Type represents executor type.
type Type string

const (
	TypeHuman Type = "HUMAN"
	TypeAgent Type = "AGENT"
)

// Status represents executor status.
type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
)

// Executor represents a human or agent executor.
type Executor struct {
	ExecutorID    string    `json:"executorId"`
	ExecutorType  Type      `json:"executorType"`
	DisplayName   string    `json:"displayName"`
	Capabilities  []string  `json:"capabilityTags,omitempty"`
	Status        Status    `json:"status"`
	Metadata      []byte    `json:"metadata,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
