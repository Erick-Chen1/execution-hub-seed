package trust

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TrustLevel represents the trust level of an event
type TrustLevel int

const (
	// T0 - Raw data, no verification
	TrustLevelT0 TrustLevel = 0
	// T1 - Server timestamp only
	TrustLevelT1 TrustLevel = 1
	// T2 - Hash chain verified
	TrustLevelT2 TrustLevel = 2
	// T3 - Gateway signed + server verified
	TrustLevelT3 TrustLevel = 3
)

// String returns the string representation of TrustLevel
func (t TrustLevel) String() string {
	switch t {
	case TrustLevelT0:
		return "T0"
	case TrustLevelT1:
		return "T1"
	case TrustLevelT2:
		return "T2"
	case TrustLevelT3:
		return "T3"
	default:
		return "UNKNOWN"
	}
}

// ParseTrustLevel parses a string to TrustLevel
func ParseTrustLevel(s string) (TrustLevel, error) {
	switch s {
	case "T0":
		return TrustLevelT0, nil
	case "T1":
		return TrustLevelT1, nil
	case "T2":
		return TrustLevelT2, nil
	case "T3":
		return TrustLevelT3, nil
	default:
		return TrustLevelT0, fmt.Errorf("invalid trust level: %s", s)
	}
}

// VerificationStatus represents the verification status
type VerificationStatus string

const (
	VerificationStatusPending  VerificationStatus = "PENDING"
	VerificationStatusVerified VerificationStatus = "VERIFIED"
	VerificationStatusFailed   VerificationStatus = "FAILED"
	VerificationStatusSkipped  VerificationStatus = "SKIPPED"
)

// HashChainEntry represents an entry in the hash chain
type HashChainEntry struct {
	ID          int64      `json:"id"`
	EventID     uuid.UUID  `json:"eventId"`
	SourceID    string     `json:"sourceId"`
	SequenceNum int64      `json:"sequenceNum"` // Sequence number within the chain
	EventHash   string     `json:"eventHash"`   // SHA-256 hash of the event content
	PrevHash    string     `json:"prevHash"`    // Hash of the previous entry (empty for genesis)
	ChainHash   string     `json:"chainHash"`   // hash(eventHash + prevHash)
	TrustLevel  TrustLevel `json:"trustLevel"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// EventHashInput represents the input for computing event hash
type EventHashInput struct {
	ClientRecordID string          `json:"clientRecordId"`
	SourceType     string          `json:"sourceType"`
	SourceID       string          `json:"sourceId"`
	TsDevice       *time.Time      `json:"tsDevice,omitempty"`
	TsGateway      *time.Time      `json:"tsGateway,omitempty"`
	Key            *string         `json:"key,omitempty"`
	EventType      string          `json:"eventType"`
	Payload        json.RawMessage `json:"payload"`
	SchemaVersion  string          `json:"schemaVersion"`
}

// ComputeEventHash computes the SHA-256 hash of an event
func ComputeEventHash(input *EventHashInput) (string, error) {
	// Serialize to canonical JSON (sorted keys)
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to serialize event for hashing: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ComputeChainHash computes the chain hash from event hash and previous hash
func ComputeChainHash(eventHash, prevHash string) string {
	combined := eventHash + prevHash
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// NewHashChainEntry creates a new hash chain entry
func NewHashChainEntry(eventID uuid.UUID, sourceID string, sequenceNum int64, eventHash, prevHash string) *HashChainEntry {
	return &HashChainEntry{
		EventID:     eventID,
		SourceID:    sourceID,
		SequenceNum: sequenceNum,
		EventHash:   eventHash,
		PrevHash:    prevHash,
		ChainHash:   ComputeChainHash(eventHash, prevHash),
		TrustLevel:  TrustLevelT2, // Hash chain provides T2
		CreatedAt:   time.Now().UTC(),
	}
}

// Verify verifies the hash chain entry
func (e *HashChainEntry) Verify() bool {
	expectedChainHash := ComputeChainHash(e.EventHash, e.PrevHash)
	return e.ChainHash == expectedChainHash
}

// BatchSignature represents a batch of events with signature
type BatchSignature struct {
	ID                 int64              `json:"id"`
	BatchID            uuid.UUID          `json:"batchId"`
	SourceID           string             `json:"sourceId"`
	EventIDs           []uuid.UUID        `json:"eventIds"`
	BatchHash          string             `json:"batchHash"`    // Hash of all events in batch
	Signature          string             `json:"signature"`    // HMAC-SHA256 signature
	SignatureAlg       string             `json:"signatureAlg"` // Algorithm: HMAC-SHA256
	KeyID              string             `json:"keyId"`        // Key identifier used for signing
	SignedAt           time.Time          `json:"signedAt"`     // When the signature was created (gateway time)
	VerifiedAt         *time.Time         `json:"verifiedAt,omitempty"`
	VerificationStatus VerificationStatus `json:"verificationStatus"`
	VerificationError  *string            `json:"verificationError,omitempty"`
	CreatedAt          time.Time          `json:"createdAt"`
}

// NewBatchSignature creates a new batch signature record
func NewBatchSignature(sourceID string, eventIDs []uuid.UUID, batchHash, signature, signatureAlg, keyID string, signedAt time.Time) *BatchSignature {
	return &BatchSignature{
		BatchID:            uuid.New(),
		SourceID:           sourceID,
		EventIDs:           eventIDs,
		BatchHash:          batchHash,
		Signature:          signature,
		SignatureAlg:       signatureAlg,
		KeyID:              keyID,
		SignedAt:           signedAt,
		VerificationStatus: VerificationStatusPending,
		CreatedAt:          time.Now().UTC(),
	}
}

// ComputeBatchHash computes the hash of a batch of event hashes
func ComputeBatchHash(eventHashes []string) string {
	combined := ""
	for _, h := range eventHashes {
		combined += h
	}
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// VerifyHMAC verifies the HMAC signature
func VerifyHMAC(batchHash, signature string, key []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(batchHash))
	expectedMAC := mac.Sum(nil)
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(signatureBytes, expectedMAC)
}

// CreateHMAC creates an HMAC signature
func CreateHMAC(batchHash string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(batchHash))
	return hex.EncodeToString(mac.Sum(nil))
}

// MarkVerified marks the batch signature as verified
func (b *BatchSignature) MarkVerified() {
	now := time.Now().UTC()
	b.VerifiedAt = &now
	b.VerificationStatus = VerificationStatusVerified
}

// MarkFailed marks the batch signature verification as failed
// Note: VerifiedAt is NOT set on failure to maintain semantic clarity
// (VerifiedAt indicates successful verification time only)
func (b *BatchSignature) MarkFailed(err string) {
	b.VerificationStatus = VerificationStatusFailed
	b.VerificationError = &err
}

// EvidenceBundle represents a complete evidence package for an event or action
type EvidenceBundle struct {
	BundleID    uuid.UUID `json:"bundleId"`
	BundleType  string    `json:"bundleType"` // EVENT | ACTION | BATCH | TASK
	SubjectID   uuid.UUID `json:"subjectId"`  // Event ID, Action ID, or Batch ID
	GeneratedAt time.Time `json:"generatedAt"`

	// Event evidence
	Events []EventEvidence `json:"events,omitempty"`

	// Hash chain evidence
	HashChain []HashChainEntry `json:"hashChain,omitempty"`

	// Signature evidence
	Signatures []BatchSignature `json:"signatures,omitempty"`

	// Rule & action evidence (for ACTION bundle)
	Rules   []RuleEvidence   `json:"rules,omitempty"`
	Actions []ActionEvidence `json:"actions,omitempty"`

	// Verification summary
	Verification VerificationSummary `json:"verification"`

	// Bundle integrity
	BundleHash string `json:"bundleHash"`
}

// EventEvidence represents event data in evidence bundle
type EventEvidence struct {
	EventID        uuid.UUID       `json:"eventId"`
	ClientRecordID string          `json:"clientRecordId"`
	SourceType     string          `json:"sourceType"`
	SourceID       string          `json:"sourceId"`
	TsDevice       *time.Time      `json:"tsDevice,omitempty"`
	TsGateway      *time.Time      `json:"tsGateway,omitempty"`
	TsServer       time.Time       `json:"tsServer"`
	Key            *string         `json:"key,omitempty"`
	EventType      string          `json:"eventType"`
	Payload        json.RawMessage `json:"payload"`
	SchemaVersion  string          `json:"schemaVersion"`
	TrustLevel     TrustLevel      `json:"trustLevel"`
}

// RuleEvidence represents rule data in evidence bundle
type RuleEvidence struct {
	RuleID         uuid.UUID       `json:"ruleId"`
	Version        int             `json:"version"`
	Name           string          `json:"name"`
	RuleType       string          `json:"ruleType"`
	Config         json.RawMessage `json:"config"`
	EffectiveFrom  time.Time       `json:"effectiveFrom"`
	EffectiveUntil *time.Time      `json:"effectiveUntil,omitempty"`
}

// ActionEvidence represents action data in evidence bundle
type ActionEvidence struct {
	ActionID     uuid.UUID       `json:"actionId"`
	RuleID       uuid.UUID       `json:"ruleId"`
	RuleVersion  int             `json:"ruleVersion"`
	EvaluationID uuid.UUID       `json:"evaluationId"`
	ActionType   string          `json:"actionType"`
	ActionConfig json.RawMessage `json:"actionConfig"`
	Status       string          `json:"status"`
	CreatedAt    time.Time       `json:"createdAt"`
	Evidence     json.RawMessage `json:"evidence"`
}

// VerificationSummary summarizes the verification results
type VerificationSummary struct {
	OverallTrustLevel  TrustLevel   `json:"overallTrustLevel"`
	HashChainValid     bool         `json:"hashChainValid"`
	SignatureValid     *bool        `json:"signatureValid,omitempty"`
	ChainBreaks        []ChainBreak `json:"chainBreaks,omitempty"`
	VerificationErrors []string     `json:"verificationErrors,omitempty"`
}

// ChainBreak represents a break in the hash chain
type ChainBreak struct {
	SourceID     string    `json:"sourceId"`
	BreakAt      int64     `json:"breakAt"` // Sequence number where break occurred
	ExpectedHash string    `json:"expectedHash"`
	ActualHash   string    `json:"actualHash"`
	DetectedAt   time.Time `json:"detectedAt"`
}

// NewEvidenceBundle creates a new evidence bundle
func NewEvidenceBundle(bundleType string, subjectID uuid.UUID) *EvidenceBundle {
	return &EvidenceBundle{
		BundleID:    uuid.New(),
		BundleType:  bundleType,
		SubjectID:   subjectID,
		GeneratedAt: time.Now().UTC(),
	}
}

// ComputeBundleHash computes the integrity hash of the bundle
func (b *EvidenceBundle) ComputeBundleHash() (string, error) {
	// Create a copy without the bundle hash for hashing
	copy := *b
	copy.BundleHash = ""

	data, err := json.Marshal(copy)
	if err != nil {
		return "", fmt.Errorf("failed to serialize bundle for hashing: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// Finalize finalizes the bundle by computing its hash
func (b *EvidenceBundle) Finalize() error {
	hash, err := b.ComputeBundleHash()
	if err != nil {
		return err
	}
	b.BundleHash = hash
	return nil
}

// VerifyIntegrity verifies the bundle integrity
func (b *EvidenceBundle) VerifyIntegrity() bool {
	savedHash := b.BundleHash
	b.BundleHash = ""

	expectedHash, err := b.ComputeBundleHash()
	b.BundleHash = savedHash

	if err != nil {
		return false
	}
	return savedHash == expectedHash
}

// TrustMetadata represents trust metadata attached to an event
type TrustMetadata struct {
	EventID      uuid.UUID  `json:"eventId"`
	TrustLevel   TrustLevel `json:"trustLevel"`
	ChainEntryID *int64     `json:"chainEntryId,omitempty"`
	BatchID      *uuid.UUID `json:"batchId,omitempty"`
	VerifiedAt   *time.Time `json:"verifiedAt,omitempty"`
	Flags        []string   `json:"flags,omitempty"` // e.g., ["CHAIN_VERIFIED", "SIGNATURE_VERIFIED"]
}

// TrustVerificationResult represents the result of trust verification
type TrustVerificationResult struct {
	EventID        uuid.UUID  `json:"eventId"`
	TrustLevel     TrustLevel `json:"trustLevel"`
	ChainValid     bool       `json:"chainValid"`
	SignatureValid *bool      `json:"signatureValid,omitempty"`
	Errors         []string   `json:"errors,omitempty"`
	VerifiedAt     time.Time  `json:"verifiedAt"`
}

// Errors
var (
	ErrEventNotFound          = errors.New("event not found")
	ErrChainBroken            = errors.New("hash chain is broken")
	ErrSignatureInvalid       = errors.New("signature verification failed")
	ErrKeyNotFound            = errors.New("signing key not found")
	ErrInvalidBundleType      = errors.New("invalid bundle type")
	ErrBatchSignatureNotFound = errors.New("batch signature not found")
)
