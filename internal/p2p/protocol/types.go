package protocol

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Operation defines supported P2P collaboration writes.
type Operation string

const (
	OpSessionCreate   Operation = "SESSION_CREATE"
	OpParticipantJoin Operation = "PARTICIPANT_JOIN"
	OpStepClaim       Operation = "STEP_CLAIM"
	OpStepRelease     Operation = "STEP_RELEASE"
	OpStepHandoff     Operation = "STEP_HANDOFF"
	OpArtifactAdd     Operation = "ARTIFACT_ADD"
	OpDecisionOpen    Operation = "DECISION_OPEN"
	OpVoteCast        Operation = "VOTE_CAST"
	OpStepResolve     Operation = "STEP_RESOLVE"
)

var validOps = map[Operation]struct{}{
	OpSessionCreate:   {},
	OpParticipantJoin: {},
	OpStepClaim:       {},
	OpStepRelease:     {},
	OpStepHandoff:     {},
	OpArtifactAdd:     {},
	OpDecisionOpen:    {},
	OpVoteCast:        {},
	OpStepResolve:     {},
}

// Tx is the signed, replicated command envelope.
type Tx struct {
	TxID      string          `json:"tx_id"`
	SessionID string          `json:"session_id,omitempty"`
	Nonce     string          `json:"nonce"`
	Timestamp time.Time       `json:"timestamp"`
	Actor     string          `json:"actor"`
	Op        Operation       `json:"op"`
	Payload   json.RawMessage `json:"payload"`
	PublicKey string          `json:"public_key"` // base64 raw ed25519 public key
	Signature string          `json:"signature"`  // base64 raw signature
}

type txSignable struct {
	TxID      string          `json:"tx_id"`
	SessionID string          `json:"session_id,omitempty"`
	Nonce     string          `json:"nonce"`
	Timestamp time.Time       `json:"timestamp"`
	Actor     string          `json:"actor"`
	Op        Operation       `json:"op"`
	Payload   json.RawMessage `json:"payload"`
	PublicKey string          `json:"public_key"`
}

// CanonicalBytes returns the deterministic signing payload.
func (t Tx) CanonicalBytes() ([]byte, error) {
	signable := txSignable{
		TxID:      strings.TrimSpace(t.TxID),
		SessionID: strings.TrimSpace(t.SessionID),
		Nonce:     strings.TrimSpace(t.Nonce),
		Timestamp: t.Timestamp.UTC(),
		Actor:     strings.TrimSpace(t.Actor),
		Op:        t.Op,
		Payload:   t.Payload,
		PublicKey: strings.TrimSpace(t.PublicKey),
	}
	return json.Marshal(signable)
}

// ValidateBasic checks required immutable tx fields.
func (t Tx) ValidateBasic() error {
	if strings.TrimSpace(t.TxID) == "" {
		return errors.New("tx_id is required")
	}
	if strings.TrimSpace(t.Nonce) == "" {
		return errors.New("nonce is required")
	}
	if strings.TrimSpace(t.Actor) == "" {
		return errors.New("actor is required")
	}
	if t.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	if _, ok := validOps[t.Op]; !ok {
		return fmt.Errorf("unsupported op: %s", t.Op)
	}
	if len(t.Payload) == 0 {
		return errors.New("payload is required")
	}
	if strings.TrimSpace(t.PublicKey) == "" {
		return errors.New("public_key is required")
	}
	if strings.TrimSpace(t.Signature) == "" {
		return errors.New("signature is required")
	}
	return nil
}

// Sign sets tx public key/signature for the given private key.
func (t *Tx) Sign(privateKey ed25519.PrivateKey) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("invalid private key")
	}
	t.PublicKey = base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))
	payload, err := t.CanonicalBytes()
	if err != nil {
		return err
	}
	sig := ed25519.Sign(privateKey, payload)
	t.Signature = base64.StdEncoding.EncodeToString(sig)
	return nil
}

// Verify validates tx signature using included public key.
func (t Tx) Verify() error {
	if err := t.ValidateBasic(); err != nil {
		return err
	}
	pubRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(t.PublicKey))
	if err != nil {
		return fmt.Errorf("invalid public_key: %w", err)
	}
	if len(pubRaw) != ed25519.PublicKeySize {
		return errors.New("invalid public_key size")
	}
	sigRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(t.Signature))
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}
	if len(sigRaw) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	payload, err := t.CanonicalBytes()
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pubRaw), payload, sigRaw) {
		return errors.New("signature verification failed")
	}
	return nil
}

// DecodePayload decodes operation payloads.
func DecodePayload[T any](raw json.RawMessage) (T, error) {
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}

// SessionStep defines a deterministic step definition in session create tx.
type SessionStep struct {
	StepID               string   `json:"step_id"`
	StepKey              string   `json:"step_key"`
	Name                 string   `json:"name"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	DependsOn            []string `json:"depends_on,omitempty"`
	LeaseTTLSeconds      int      `json:"lease_ttl_seconds,omitempty"`
}

type SessionCreatePayload struct {
	SessionID  string          `json:"session_id"`
	WorkflowID string          `json:"workflow_id,omitempty"`
	Name       string          `json:"name"`
	Context    json.RawMessage `json:"context,omitempty"`
	Steps      []SessionStep   `json:"steps"`
}

type ParticipantJoinPayload struct {
	ParticipantID string   `json:"participant_id"`
	SessionID     string   `json:"session_id"`
	Type          string   `json:"type"`
	Ref           string   `json:"ref"`
	Capabilities  []string `json:"capabilities,omitempty"`
	TrustScore    int      `json:"trust_score,omitempty"`
}

type StepClaimPayload struct {
	ClaimID       string `json:"claim_id"`
	StepID        string `json:"step_id"`
	ParticipantID string `json:"participant_id"`
	LeaseSeconds  int    `json:"lease_seconds,omitempty"`
}

type StepReleasePayload struct {
	StepID        string `json:"step_id"`
	ParticipantID string `json:"participant_id"`
}

type StepHandoffPayload struct {
	NewClaimID        string  `json:"new_claim_id"`
	StepID            string  `json:"step_id"`
	FromParticipantID string  `json:"from_participant_id"`
	ToParticipantID   string  `json:"to_participant_id"`
	LeaseSeconds      int     `json:"lease_seconds,omitempty"`
	Comment           *string `json:"comment,omitempty"`
}

type ArtifactAddPayload struct {
	ArtifactID   string          `json:"artifact_id"`
	StepID       string          `json:"step_id"`
	ProducerID   string          `json:"producer_id"`
	Kind         string          `json:"kind"`
	Content      json.RawMessage `json:"content"`
	ContentHash  string          `json:"content_hash,omitempty"`
	ExternalURI  string          `json:"external_uri,omitempty"`
	ContentBytes int64           `json:"content_bytes,omitempty"`
}

type DecisionOpenPayload struct {
	DecisionID string          `json:"decision_id"`
	StepID     string          `json:"step_id"`
	Policy     json.RawMessage `json:"policy,omitempty"`
	Deadline   *time.Time      `json:"deadline,omitempty"`
}

type VoteCastPayload struct {
	VoteID        string  `json:"vote_id"`
	DecisionID    string  `json:"decision_id"`
	ParticipantID string  `json:"participant_id"`
	Choice        string  `json:"choice"`
	Comment       *string `json:"comment,omitempty"`
}

type StepResolvePayload struct {
	StepID        string  `json:"step_id"`
	ParticipantID *string `json:"participant_id,omitempty"`
}
