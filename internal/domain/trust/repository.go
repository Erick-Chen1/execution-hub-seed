package trust

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the interface for trust data persistence
type Repository interface {
	// Event operations
	InsertEvent(ctx context.Context, event *EventEvidence) error
	GetEvent(ctx context.Context, eventID uuid.UUID) (*EventEvidence, error)
	GetEvents(ctx context.Context, eventIDs []uuid.UUID) ([]EventEvidence, error)

	// Hash Chain operations
	InsertHashChainEntry(ctx context.Context, entry *HashChainEntry) error
	GetHashChainEntry(ctx context.Context, eventID uuid.UUID) (*HashChainEntry, error)
	GetLatestChainEntry(ctx context.Context, sourceID string) (*HashChainEntry, error)
	GetChainEntriesForSource(ctx context.Context, sourceID string, fromSeq, toSeq int64) ([]*HashChainEntry, error)
	GetChainEntriesForEvents(ctx context.Context, eventIDs []uuid.UUID) ([]*HashChainEntry, error)
	
	// Batch Signature operations
	InsertBatchSignature(ctx context.Context, sig *BatchSignature) error
	GetBatchSignature(ctx context.Context, batchID uuid.UUID) (*BatchSignature, error)
	GetBatchSignaturesBySource(ctx context.Context, sourceID string, limit int) ([]*BatchSignature, error)
	GetBatchSignatureForEvent(ctx context.Context, eventID uuid.UUID) (*BatchSignature, error)
	UpdateBatchSignatureStatus(ctx context.Context, batchID uuid.UUID, status VerificationStatus, errMsg *string) error
	
	// Trust Metadata operations
	GetTrustMetadata(ctx context.Context, eventID uuid.UUID) (*TrustMetadata, error)
	BatchGetTrustMetadata(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID]*TrustMetadata, error)
	UpdateEventTrustLevel(ctx context.Context, eventID uuid.UUID, level TrustLevel) error
	
	// Evidence Bundle operations (for audit/export)
	GetEvidenceBundleData(ctx context.Context, bundleType string, subjectID uuid.UUID) (*EvidenceBundleData, error)
}

// EvidenceBundleData contains raw data for constructing an evidence bundle
type EvidenceBundleData struct {
	Events       []EventEvidence
	HashChain    []*HashChainEntry
	Signatures   []*BatchSignature
	Rules        []RuleEvidence
	Actions      []ActionEvidence
	Evaluations  []EvaluationEvidence
}

// EvaluationEvidence represents evaluation data for evidence bundle
type EvaluationEvidence struct {
	EvaluationID uuid.UUID       `json:"evaluationId"`
	RuleID       uuid.UUID       `json:"ruleId"`
	RuleVersion  int             `json:"ruleVersion"`
	Matched      bool            `json:"matched"`
	EvaluatedAt  string          `json:"evaluatedAt"`
	Evidence     interface{}     `json:"evidence"`
}

// KeyStore defines the interface for managing signing keys
type KeyStore interface {
	// GetKey retrieves a signing key by ID
	GetKey(ctx context.Context, keyID string) ([]byte, error)
	
	// GetKeyForSource retrieves the current signing key for a source
	GetKeyForSource(ctx context.Context, sourceID string) (keyID string, key []byte, err error)
}

// HashChainFilter represents filters for querying hash chain entries
type HashChainFilter struct {
	SourceID      *string
	FromSequence  *int64
	ToSequence    *int64
	EventIDs      []uuid.UUID
}

// SignatureFilter represents filters for querying batch signatures
type SignatureFilter struct {
	SourceID           *string
	Status             *VerificationStatus
	SignedAfter        *string
	SignedBefore       *string
}
