package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/execution-hub/execution-hub/internal/p2p/protocol"
)

const (
	SessionStatusActive    = "ACTIVE"
	SessionStatusCompleted = "COMPLETED"
	SessionStatusFailed    = "FAILED"
	SessionStatusCancelled = "CANCELLED"

	StepStatusOpen     = "OPEN"
	StepStatusClaimed  = "CLAIMED"
	StepStatusInReview = "IN_REVIEW"
	StepStatusResolved = "RESOLVED"
	StepStatusFailed   = "FAILED"

	ClaimStatusActive   = "ACTIVE"
	ClaimStatusExpired  = "EXPIRED"
	ClaimStatusReleased = "RELEASED"

	DecisionStatusPending  = "PENDING"
	DecisionStatusPassed   = "PASSED"
	DecisionStatusRejected = "REJECTED"

	VoteChoiceApprove = "APPROVE"
	VoteChoiceReject  = "REJECT"
)

type Session struct {
	SessionID   string          `json:"sessionId"`
	WorkflowID  string          `json:"workflowId,omitempty"`
	Name        string          `json:"name"`
	Status      string          `json:"status"`
	Context     json.RawMessage `json:"context,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	LastEventID string          `json:"lastEventId,omitempty"`
}

type Participant struct {
	ParticipantID string    `json:"participantId"`
	SessionID     string    `json:"sessionId"`
	Type          string    `json:"type"`
	Ref           string    `json:"ref"`
	Capabilities  []string  `json:"capabilities,omitempty"`
	TrustScore    int       `json:"trustScore"`
	JoinedAt      time.Time `json:"joinedAt"`
	LastSeenAt    time.Time `json:"lastSeenAt"`
}

type Step struct {
	StepID               string          `json:"stepId"`
	SessionID            string          `json:"sessionId"`
	StepKey              string          `json:"stepKey"`
	Name                 string          `json:"name"`
	Status               string          `json:"status"`
	RequiredCapabilities []string        `json:"requiredCapabilities,omitempty"`
	DependsOn            []string        `json:"dependsOn,omitempty"`
	LeaseTTLSeconds      int             `json:"leaseTtlSeconds"`
	ConsensusPolicy      json.RawMessage `json:"consensusPolicy,omitempty"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
	ResolvedAt           *time.Time      `json:"resolvedAt,omitempty"`
}

type Claim struct {
	ClaimID       string    `json:"claimId"`
	StepID        string    `json:"stepId"`
	ParticipantID string    `json:"participantId"`
	Status        string    `json:"status"`
	LeaseUntil    time.Time `json:"leaseUntil"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Artifact struct {
	ArtifactID   string          `json:"artifactId"`
	StepID       string          `json:"stepId"`
	ProducerID   string          `json:"producerId"`
	Kind         string          `json:"kind"`
	Content      json.RawMessage `json:"content,omitempty"`
	ContentHash  string          `json:"contentHash,omitempty"`
	ExternalURI  string          `json:"externalUri,omitempty"`
	ContentBytes int64           `json:"contentBytes,omitempty"`
	Version      int             `json:"version"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type Decision struct {
	DecisionID string          `json:"decisionId"`
	StepID     string          `json:"stepId"`
	Policy     json.RawMessage `json:"policy"`
	Deadline   *time.Time      `json:"deadline,omitempty"`
	Status     string          `json:"status"`
	Result     *string         `json:"result,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	DecidedAt  *time.Time      `json:"decidedAt,omitempty"`
}

type Vote struct {
	VoteID        string    `json:"voteId"`
	DecisionID    string    `json:"decisionId"`
	ParticipantID string    `json:"participantId"`
	Choice        string    `json:"choice"`
	Comment       *string   `json:"comment,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Event struct {
	EventID    string          `json:"eventId"`
	SessionID  string          `json:"sessionId"`
	StepID     *string         `json:"stepId,omitempty"`
	Type       string          `json:"type"`
	Actor      string          `json:"actor"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	TxID       string          `json:"txId"`
	CommitTime time.Time       `json:"commitTime"`
}

type snapshot struct {
	Sessions                map[string]Session             `json:"sessions"`
	Participants            map[string]Participant         `json:"participants"`
	ParticipantsBySession   map[string]string              `json:"participantsBySession"`
	Steps                   map[string]Step                `json:"steps"`
	StepOrderBySession      map[string][]string            `json:"stepOrderBySession"`
	Claims                  map[string]Claim               `json:"claims"`
	ArtifactsByStep         map[string][]Artifact          `json:"artifactsByStep"`
	Decisions               map[string]Decision            `json:"decisions"`
	DecisionByStep          map[string]string              `json:"decisionByStep"`
	VotesByDecision         map[string]map[string]Vote     `json:"votesByDecision"`
	EventsBySession         map[string][]Event             `json:"eventsBySession"`
	AppliedTx               map[string]bool                `json:"appliedTx"`
	StepKeysBySession       map[string]map[string]struct{} `json:"-"`
	DecisionByStepFinalized map[string]bool                `json:"decisionByStepFinalized,omitempty"`
}

// Machine is the deterministic collaboration state machine.
type Machine struct {
	mu sync.RWMutex
	s  snapshot
}

func NewMachine() *Machine {
	m := &Machine{}
	m.s = emptySnapshot()
	return m
}

func emptySnapshot() snapshot {
	return snapshot{
		Sessions:                map[string]Session{},
		Participants:            map[string]Participant{},
		ParticipantsBySession:   map[string]string{},
		Steps:                   map[string]Step{},
		StepOrderBySession:      map[string][]string{},
		Claims:                  map[string]Claim{},
		ArtifactsByStep:         map[string][]Artifact{},
		Decisions:               map[string]Decision{},
		DecisionByStep:          map[string]string{},
		VotesByDecision:         map[string]map[string]Vote{},
		EventsBySession:         map[string][]Event{},
		AppliedTx:               map[string]bool{},
		StepKeysBySession:       map[string]map[string]struct{}{},
		DecisionByStepFinalized: map[string]bool{},
	}
}

// Marshal serializes current machine snapshot.
func (m *Machine) Marshal() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.copySnapshotLocked()
	// StepKeysBySession is a runtime cache; it can be rebuilt.
	s.StepKeysBySession = nil
	return json.Marshal(s)
}

// Unmarshal restores machine state from snapshot payload.
func (m *Machine) Unmarshal(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty snapshot")
	}
	var s snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	m.normalizeSnapshot(&s)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.s = s
	return nil
}

func (m *Machine) normalizeSnapshot(s *snapshot) {
	if s.Sessions == nil {
		s.Sessions = map[string]Session{}
	}
	if s.Participants == nil {
		s.Participants = map[string]Participant{}
	}
	if s.ParticipantsBySession == nil {
		s.ParticipantsBySession = map[string]string{}
	}
	if s.Steps == nil {
		s.Steps = map[string]Step{}
	}
	if s.StepOrderBySession == nil {
		s.StepOrderBySession = map[string][]string{}
	}
	if s.Claims == nil {
		s.Claims = map[string]Claim{}
	}
	if s.ArtifactsByStep == nil {
		s.ArtifactsByStep = map[string][]Artifact{}
	}
	if s.Decisions == nil {
		s.Decisions = map[string]Decision{}
	}
	if s.DecisionByStep == nil {
		s.DecisionByStep = map[string]string{}
	}
	if s.VotesByDecision == nil {
		s.VotesByDecision = map[string]map[string]Vote{}
	}
	if s.EventsBySession == nil {
		s.EventsBySession = map[string][]Event{}
	}
	if s.AppliedTx == nil {
		s.AppliedTx = map[string]bool{}
	}
	s.StepKeysBySession = map[string]map[string]struct{}{}
	for _, step := range s.Steps {
		if _, ok := s.StepKeysBySession[step.SessionID]; !ok {
			s.StepKeysBySession[step.SessionID] = map[string]struct{}{}
		}
		s.StepKeysBySession[step.SessionID][step.StepKey] = struct{}{}
	}
	if s.DecisionByStepFinalized == nil {
		s.DecisionByStepFinalized = map[string]bool{}
	}
}

func (m *Machine) copySnapshotLocked() snapshot {
	out := emptySnapshot()
	for k, v := range m.s.Sessions {
		out.Sessions[k] = v
	}
	for k, v := range m.s.Participants {
		out.Participants[k] = cloneParticipant(v)
	}
	for k, v := range m.s.ParticipantsBySession {
		out.ParticipantsBySession[k] = v
	}
	for k, v := range m.s.Steps {
		out.Steps[k] = cloneStep(v)
	}
	for k, v := range m.s.StepOrderBySession {
		out.StepOrderBySession[k] = append([]string(nil), v...)
	}
	for k, v := range m.s.Claims {
		out.Claims[k] = v
	}
	for k, v := range m.s.ArtifactsByStep {
		out.ArtifactsByStep[k] = append([]Artifact(nil), v...)
	}
	for k, v := range m.s.Decisions {
		out.Decisions[k] = v
	}
	for k, v := range m.s.DecisionByStep {
		out.DecisionByStep[k] = v
	}
	for k, v := range m.s.VotesByDecision {
		cp := map[string]Vote{}
		for pid, vote := range v {
			cp[pid] = vote
		}
		out.VotesByDecision[k] = cp
	}
	for k, v := range m.s.EventsBySession {
		out.EventsBySession[k] = append([]Event(nil), v...)
	}
	for k, v := range m.s.AppliedTx {
		out.AppliedTx[k] = v
	}
	for k, v := range m.s.StepKeysBySession {
		cp := map[string]struct{}{}
		for key := range v {
			cp[key] = struct{}{}
		}
		out.StepKeysBySession[k] = cp
	}
	for k, v := range m.s.DecisionByStepFinalized {
		out.DecisionByStepFinalized[k] = v
	}
	return out
}

func cloneParticipant(in Participant) Participant {
	in.Capabilities = append([]string(nil), in.Capabilities...)
	return in
}

func cloneStep(in Step) Step {
	in.RequiredCapabilities = append([]string(nil), in.RequiredCapabilities...)
	in.DependsOn = append([]string(nil), in.DependsOn...)
	return in
}

// ApplyTx validates and applies one signed transaction.
func (m *Machine) ApplyTx(tx protocol.Tx) error {
	if err := tx.Verify(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.s.AppliedTx[tx.TxID] {
		return nil
	}
	at := tx.Timestamp.UTC()
	m.expireClaimsLocked(at, tx.TxID)

	var err error
	switch tx.Op {
	case protocol.OpSessionCreate:
		err = m.applySessionCreateLocked(tx, at)
	case protocol.OpParticipantJoin:
		err = m.applyParticipantJoinLocked(tx, at)
	case protocol.OpStepClaim:
		err = m.applyStepClaimLocked(tx, at)
	case protocol.OpStepRelease:
		err = m.applyStepReleaseLocked(tx, at)
	case protocol.OpStepHandoff:
		err = m.applyStepHandoffLocked(tx, at)
	case protocol.OpArtifactAdd:
		err = m.applyArtifactAddLocked(tx, at)
	case protocol.OpDecisionOpen:
		err = m.applyDecisionOpenLocked(tx, at)
	case protocol.OpVoteCast:
		err = m.applyVoteCastLocked(tx, at)
	case protocol.OpStepResolve:
		err = m.applyStepResolveLocked(tx, at)
	default:
		err = fmt.Errorf("unsupported op: %s", tx.Op)
	}
	if err != nil {
		return err
	}
	m.s.AppliedTx[tx.TxID] = true
	return nil
}

func (m *Machine) applySessionCreateLocked(tx protocol.Tx, at time.Time) error {
	payload, err := protocol.DecodePayload[protocol.SessionCreatePayload](tx.Payload)
	if err != nil {
		return err
	}
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		return errors.New("session_id is required")
	}
	if _, ok := m.s.Sessions[sessionID]; ok {
		return fmt.Errorf("session already exists: %s", sessionID)
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return errors.New("name is required")
	}
	if len(payload.Steps) == 0 {
		return errors.New("steps are required")
	}
	session := Session{
		SessionID:  sessionID,
		WorkflowID: strings.TrimSpace(payload.WorkflowID),
		Name:       name,
		Status:     SessionStatusActive,
		Context:    payload.Context,
		CreatedAt:  at,
		UpdatedAt:  at,
	}
	m.s.Sessions[sessionID] = session
	if _, ok := m.s.StepKeysBySession[sessionID]; !ok {
		m.s.StepKeysBySession[sessionID] = map[string]struct{}{}
	}
	stepOrder := make([]string, 0, len(payload.Steps))
	for _, raw := range payload.Steps {
		stepID := strings.TrimSpace(raw.StepID)
		if stepID == "" {
			return errors.New("step_id is required")
		}
		stepKey := strings.TrimSpace(raw.StepKey)
		if stepKey == "" {
			return errors.New("step_key is required")
		}
		if _, exists := m.s.Steps[stepID]; exists {
			return fmt.Errorf("step already exists: %s", stepID)
		}
		if _, exists := m.s.StepKeysBySession[sessionID][stepKey]; exists {
			return fmt.Errorf("duplicate step_key in session: %s", stepKey)
		}
		ttl := raw.LeaseTTLSeconds
		if ttl <= 0 {
			ttl = 900
		}
		step := Step{
			StepID:               stepID,
			SessionID:            sessionID,
			StepKey:              stepKey,
			Name:                 strings.TrimSpace(raw.Name),
			Status:               StepStatusOpen,
			RequiredCapabilities: uniqueNonEmpty(raw.RequiredCapabilities),
			DependsOn:            uniqueNonEmpty(raw.DependsOn),
			LeaseTTLSeconds:      ttl,
			CreatedAt:            at,
			UpdatedAt:            at,
		}
		if step.Name == "" {
			step.Name = step.StepKey
		}
		m.s.Steps[step.StepID] = step
		m.s.StepKeysBySession[sessionID][step.StepKey] = struct{}{}
		stepOrder = append(stepOrder, step.StepID)
	}
	m.s.StepOrderBySession[sessionID] = stepOrder
	m.appendEventLocked(sessionID, nil, string(protocol.OpSessionCreate), tx.Actor, payload, at, tx.TxID)
	return nil
}

func (m *Machine) applyParticipantJoinLocked(tx protocol.Tx, at time.Time) error {
	payload, err := protocol.DecodePayload[protocol.ParticipantJoinPayload](tx.Payload)
	if err != nil {
		return err
	}
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		return errors.New("session_id is required")
	}
	if _, ok := m.s.Sessions[sessionID]; !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	ref := strings.TrimSpace(payload.Ref)
	if ref == "" {
		return errors.New("ref is required")
	}
	participantType := strings.ToUpper(strings.TrimSpace(payload.Type))
	if participantType != "HUMAN" && participantType != "AGENT" {
		return errors.New("type must be HUMAN or AGENT")
	}
	sessionRefKey := sessionRef(sessionID, ref)
	if existingID, ok := m.s.ParticipantsBySession[sessionRefKey]; ok {
		existing := m.s.Participants[existingID]
		existing.LastSeenAt = at
		existing.Capabilities = uniqueNonEmpty(payload.Capabilities)
		existing.TrustScore = payload.TrustScore
		m.s.Participants[existingID] = existing
		m.appendEventLocked(sessionID, nil, "PARTICIPANT_TOUCH", tx.Actor, map[string]any{
			"participantId": existingID,
		}, at, tx.TxID)
		return nil
	}
	participantID := strings.TrimSpace(payload.ParticipantID)
	if participantID == "" {
		return errors.New("participant_id is required")
	}
	if _, exists := m.s.Participants[participantID]; exists {
		return fmt.Errorf("participant already exists: %s", participantID)
	}
	p := Participant{
		ParticipantID: participantID,
		SessionID:     sessionID,
		Type:          participantType,
		Ref:           ref,
		Capabilities:  uniqueNonEmpty(payload.Capabilities),
		TrustScore:    payload.TrustScore,
		JoinedAt:      at,
		LastSeenAt:    at,
	}
	m.s.Participants[participantID] = p
	m.s.ParticipantsBySession[sessionRefKey] = participantID
	m.appendEventLocked(sessionID, nil, string(protocol.OpParticipantJoin), tx.Actor, payload, at, tx.TxID)
	return nil
}

func (m *Machine) applyStepClaimLocked(tx protocol.Tx, at time.Time) error {
	payload, err := protocol.DecodePayload[protocol.StepClaimPayload](tx.Payload)
	if err != nil {
		return err
	}
	stepID := strings.TrimSpace(payload.StepID)
	participantID := strings.TrimSpace(payload.ParticipantID)
	claimID := strings.TrimSpace(payload.ClaimID)
	if stepID == "" || participantID == "" || claimID == "" {
		return errors.New("step_id, participant_id and claim_id are required")
	}
	step, ok := m.s.Steps[stepID]
	if !ok {
		return fmt.Errorf("step not found: %s", stepID)
	}
	participant, ok := m.s.Participants[participantID]
	if !ok {
		return fmt.Errorf("participant not found: %s", participantID)
	}
	if participant.SessionID != step.SessionID {
		return errors.New("participant does not belong to step session")
	}
	if !depsResolved(step, m.s.Steps) {
		return errors.New("step dependencies are not resolved")
	}
	if !hasCapabilities(participant.Capabilities, step.RequiredCapabilities) {
		return errors.New("participant capabilities do not satisfy requirements")
	}
	if step.Status != StepStatusOpen {
		return errors.New("step is not OPEN")
	}
	if existingID, _ := m.findActiveClaimByStepLocked(step.StepID, at); existingID != "" {
		return errors.New("step already claimed")
	}
	if _, exists := m.s.Claims[claimID]; exists {
		return fmt.Errorf("claim already exists: %s", claimID)
	}
	leaseSeconds := payload.LeaseSeconds
	if leaseSeconds <= 0 {
		leaseSeconds = step.LeaseTTLSeconds
	}
	if leaseSeconds <= 0 {
		leaseSeconds = 900
	}
	claim := Claim{
		ClaimID:       claimID,
		StepID:        step.StepID,
		ParticipantID: participant.ParticipantID,
		Status:        ClaimStatusActive,
		LeaseUntil:    at.Add(time.Duration(leaseSeconds) * time.Second),
		CreatedAt:     at,
		UpdatedAt:     at,
	}
	m.s.Claims[claimID] = claim
	step.Status = StepStatusClaimed
	step.UpdatedAt = at
	m.s.Steps[step.StepID] = step
	m.appendEventLocked(step.SessionID, &step.StepID, string(protocol.OpStepClaim), tx.Actor, payload, at, tx.TxID)
	return nil
}

func (m *Machine) applyStepReleaseLocked(tx protocol.Tx, at time.Time) error {
	payload, err := protocol.DecodePayload[protocol.StepReleasePayload](tx.Payload)
	if err != nil {
		return err
	}
	stepID := strings.TrimSpace(payload.StepID)
	participantID := strings.TrimSpace(payload.ParticipantID)
	if stepID == "" || participantID == "" {
		return errors.New("step_id and participant_id are required")
	}
	step, ok := m.s.Steps[stepID]
	if !ok {
		return fmt.Errorf("step not found: %s", stepID)
	}
	if _, ok := m.s.Participants[participantID]; !ok {
		return fmt.Errorf("participant not found: %s", participantID)
	}
	claimID, claim := m.findActiveClaimByStepAndParticipantLocked(stepID, participantID, at)
	if claimID == "" {
		return errors.New("active claim not found for participant")
	}
	claim.Status = ClaimStatusReleased
	claim.UpdatedAt = at
	m.s.Claims[claimID] = claim
	step.Status = StepStatusOpen
	step.UpdatedAt = at
	m.s.Steps[step.StepID] = step
	m.appendEventLocked(step.SessionID, &step.StepID, string(protocol.OpStepRelease), tx.Actor, payload, at, tx.TxID)
	return nil
}

func (m *Machine) applyStepHandoffLocked(tx protocol.Tx, at time.Time) error {
	payload, err := protocol.DecodePayload[protocol.StepHandoffPayload](tx.Payload)
	if err != nil {
		return err
	}
	newClaimID := strings.TrimSpace(payload.NewClaimID)
	stepID := strings.TrimSpace(payload.StepID)
	fromParticipantID := strings.TrimSpace(payload.FromParticipantID)
	toParticipantID := strings.TrimSpace(payload.ToParticipantID)
	if newClaimID == "" || stepID == "" || fromParticipantID == "" || toParticipantID == "" {
		return errors.New("new_claim_id, step_id, from_participant_id and to_participant_id are required")
	}
	step, ok := m.s.Steps[stepID]
	if !ok {
		return fmt.Errorf("step not found: %s", stepID)
	}
	fromParticipant, ok := m.s.Participants[fromParticipantID]
	if !ok {
		return fmt.Errorf("participant not found: %s", fromParticipantID)
	}
	toParticipant, ok := m.s.Participants[toParticipantID]
	if !ok {
		return fmt.Errorf("participant not found: %s", toParticipantID)
	}
	if fromParticipant.SessionID != step.SessionID || toParticipant.SessionID != step.SessionID {
		return errors.New("participant does not belong to step session")
	}
	if !hasCapabilities(toParticipant.Capabilities, step.RequiredCapabilities) {
		return errors.New("target participant capabilities do not satisfy requirements")
	}
	activeID, active := m.findActiveClaimByStepAndParticipantLocked(stepID, fromParticipantID, at)
	if activeID == "" {
		return errors.New("source participant has no active claim")
	}
	if _, exists := m.s.Claims[newClaimID]; exists {
		return fmt.Errorf("claim already exists: %s", newClaimID)
	}
	active.Status = ClaimStatusReleased
	active.UpdatedAt = at
	m.s.Claims[activeID] = active

	leaseSeconds := payload.LeaseSeconds
	if leaseSeconds <= 0 {
		leaseSeconds = step.LeaseTTLSeconds
	}
	if leaseSeconds <= 0 {
		leaseSeconds = 900
	}
	newClaim := Claim{
		ClaimID:       newClaimID,
		StepID:        step.StepID,
		ParticipantID: toParticipant.ParticipantID,
		Status:        ClaimStatusActive,
		LeaseUntil:    at.Add(time.Duration(leaseSeconds) * time.Second),
		CreatedAt:     at,
		UpdatedAt:     at,
	}
	m.s.Claims[newClaimID] = newClaim
	step.Status = StepStatusClaimed
	step.UpdatedAt = at
	m.s.Steps[step.StepID] = step
	m.appendEventLocked(step.SessionID, &step.StepID, string(protocol.OpStepHandoff), tx.Actor, payload, at, tx.TxID)
	return nil
}

func (m *Machine) applyArtifactAddLocked(tx protocol.Tx, at time.Time) error {
	payload, err := protocol.DecodePayload[protocol.ArtifactAddPayload](tx.Payload)
	if err != nil {
		return err
	}
	artifactID := strings.TrimSpace(payload.ArtifactID)
	stepID := strings.TrimSpace(payload.StepID)
	producerID := strings.TrimSpace(payload.ProducerID)
	kind := strings.TrimSpace(payload.Kind)
	if artifactID == "" || stepID == "" || producerID == "" {
		return errors.New("artifact_id, step_id and producer_id are required")
	}
	if len(payload.Content) == 0 && strings.TrimSpace(payload.ExternalURI) == "" {
		return errors.New("content or external_uri is required")
	}
	if kind == "" {
		kind = "generic"
	}
	if m.artifactExistsLocked(artifactID) {
		return fmt.Errorf("artifact already exists: %s", artifactID)
	}
	step, ok := m.s.Steps[stepID]
	if !ok {
		return fmt.Errorf("step not found: %s", stepID)
	}
	participant, ok := m.s.Participants[producerID]
	if !ok {
		return fmt.Errorf("participant not found: %s", producerID)
	}
	if participant.SessionID != step.SessionID {
		return errors.New("participant does not belong to step session")
	}
	activeID, _ := m.findActiveClaimByStepAndParticipantLocked(stepID, producerID, at)
	if activeID == "" {
		return errors.New("participant must hold active claim")
	}
	version := len(m.s.ArtifactsByStep[stepID]) + 1
	artifact := Artifact{
		ArtifactID:   artifactID,
		StepID:       stepID,
		ProducerID:   producerID,
		Kind:         kind,
		Content:      append([]byte(nil), payload.Content...),
		ContentHash:  strings.TrimSpace(payload.ContentHash),
		ExternalURI:  strings.TrimSpace(payload.ExternalURI),
		ContentBytes: payload.ContentBytes,
		Version:      version,
		CreatedAt:    at,
	}
	m.s.ArtifactsByStep[stepID] = append(m.s.ArtifactsByStep[stepID], artifact)
	step.Status = StepStatusInReview
	step.UpdatedAt = at
	m.s.Steps[step.StepID] = step
	m.appendEventLocked(step.SessionID, &step.StepID, string(protocol.OpArtifactAdd), tx.Actor, payload, at, tx.TxID)
	return nil
}

func (m *Machine) applyDecisionOpenLocked(tx protocol.Tx, at time.Time) error {
	payload, err := protocol.DecodePayload[protocol.DecisionOpenPayload](tx.Payload)
	if err != nil {
		return err
	}
	decisionID := strings.TrimSpace(payload.DecisionID)
	stepID := strings.TrimSpace(payload.StepID)
	if decisionID == "" || stepID == "" {
		return errors.New("decision_id and step_id are required")
	}
	if _, exists := m.s.Decisions[decisionID]; exists {
		return fmt.Errorf("decision already exists: %s", decisionID)
	}
	step, ok := m.s.Steps[stepID]
	if !ok {
		return fmt.Errorf("step not found: %s", stepID)
	}
	if step.Status != StepStatusClaimed && step.Status != StepStatusInReview {
		return errors.New("step must be CLAIMED or IN_REVIEW")
	}
	if latestID := strings.TrimSpace(m.s.DecisionByStep[stepID]); latestID != "" {
		latest := m.s.Decisions[latestID]
		if latest.Status == DecisionStatusPending {
			return errors.New("pending decision already exists")
		}
	}
	policy, _, err := normalizeDecisionPolicy(payload.Policy)
	if err != nil {
		return err
	}
	var deadline *time.Time
	if payload.Deadline != nil {
		d := payload.Deadline.UTC()
		deadline = &d
	}
	decision := Decision{
		DecisionID: decisionID,
		StepID:     stepID,
		Policy:     policy,
		Deadline:   deadline,
		Status:     DecisionStatusPending,
		CreatedAt:  at,
		UpdatedAt:  at,
	}
	m.s.Decisions[decisionID] = decision
	m.s.DecisionByStep[stepID] = decisionID
	m.s.DecisionByStepFinalized[stepID] = false
	m.appendEventLocked(step.SessionID, &step.StepID, string(protocol.OpDecisionOpen), tx.Actor, payload, at, tx.TxID)
	return nil
}

func (m *Machine) applyVoteCastLocked(tx protocol.Tx, at time.Time) error {
	payload, err := protocol.DecodePayload[protocol.VoteCastPayload](tx.Payload)
	if err != nil {
		return err
	}
	voteID := strings.TrimSpace(payload.VoteID)
	decisionID := strings.TrimSpace(payload.DecisionID)
	participantID := strings.TrimSpace(payload.ParticipantID)
	choice := strings.ToUpper(strings.TrimSpace(payload.Choice))
	if voteID == "" || decisionID == "" || participantID == "" || choice == "" {
		return errors.New("vote_id, decision_id, participant_id and choice are required")
	}
	if choice != VoteChoiceApprove && choice != VoteChoiceReject {
		return errors.New("choice must be APPROVE or REJECT")
	}
	if m.voteExistsLocked(voteID) {
		return fmt.Errorf("vote already exists: %s", voteID)
	}
	decision, ok := m.s.Decisions[decisionID]
	if !ok {
		return fmt.Errorf("decision not found: %s", decisionID)
	}
	if decision.Status != DecisionStatusPending {
		return errors.New("decision is not pending")
	}
	if decision.Deadline != nil && at.After(*decision.Deadline) {
		return errors.New("decision already expired")
	}
	step, ok := m.s.Steps[decision.StepID]
	if !ok {
		return fmt.Errorf("step not found: %s", decision.StepID)
	}
	participant, ok := m.s.Participants[participantID]
	if !ok {
		return fmt.Errorf("participant not found: %s", participantID)
	}
	if participant.SessionID != step.SessionID {
		return errors.New("participant not found in decision session")
	}
	if _, exists := m.s.VotesByDecision[decisionID]; !exists {
		m.s.VotesByDecision[decisionID] = map[string]Vote{}
	}
	if _, exists := m.s.VotesByDecision[decisionID][participantID]; exists {
		return errors.New("participant has already voted")
	}
	vote := Vote{
		VoteID:        voteID,
		DecisionID:    decisionID,
		ParticipantID: participantID,
		Choice:        choice,
		Comment:       payload.Comment,
		CreatedAt:     at,
	}
	m.s.VotesByDecision[decisionID][participantID] = vote
	decisionStatus, result := evaluateDecision(decision.Policy, m.s.VotesByDecision[decisionID])
	if decisionStatus != DecisionStatusPending {
		decision.Status = decisionStatus
		decision.Result = &result
		decidedAt := at
		decision.DecidedAt = &decidedAt
		m.s.DecisionByStepFinalized[decision.StepID] = true
	}
	decision.UpdatedAt = at
	m.s.Decisions[decision.DecisionID] = decision
	m.appendEventLocked(step.SessionID, &step.StepID, string(protocol.OpVoteCast), tx.Actor, payload, at, tx.TxID)
	return nil
}

func (m *Machine) applyStepResolveLocked(tx protocol.Tx, at time.Time) error {
	payload, err := protocol.DecodePayload[protocol.StepResolvePayload](tx.Payload)
	if err != nil {
		return err
	}
	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		return errors.New("step_id is required")
	}
	step, ok := m.s.Steps[stepID]
	if !ok {
		return fmt.Errorf("step not found: %s", stepID)
	}
	if step.Status != StepStatusClaimed && step.Status != StepStatusInReview {
		return errors.New("step must be CLAIMED or IN_REVIEW")
	}
	if decisionID := strings.TrimSpace(m.s.DecisionByStep[stepID]); decisionID != "" {
		decision := m.s.Decisions[decisionID]
		if decision.Status == DecisionStatusPending {
			return errors.New("pending decision must complete first")
		}
		if decision.Status == DecisionStatusRejected {
			return errors.New("decision rejected; cannot resolve step")
		}
	}
	if payload.ParticipantID != nil {
		participantID := strings.TrimSpace(*payload.ParticipantID)
		if participantID == "" {
			return errors.New("participant_id must not be empty")
		}
		if _, ok := m.s.Participants[participantID]; !ok {
			return fmt.Errorf("participant not found: %s", participantID)
		}
		activeID, _ := m.findActiveClaimByStepAndParticipantLocked(stepID, participantID, at)
		if activeID == "" {
			return errors.New("participant does not hold active claim")
		}
	}
	if activeID, active := m.findActiveClaimByStepLocked(stepID, at); activeID != "" {
		active.Status = ClaimStatusReleased
		active.UpdatedAt = at
		m.s.Claims[activeID] = active
	}
	resolvedAt := at
	step.Status = StepStatusResolved
	step.ResolvedAt = &resolvedAt
	step.UpdatedAt = at
	m.s.Steps[step.StepID] = step
	m.appendEventLocked(step.SessionID, &step.StepID, string(protocol.OpStepResolve), tx.Actor, payload, at, tx.TxID)

	if m.allStepsResolvedLocked(step.SessionID) {
		session := m.s.Sessions[step.SessionID]
		if session.Status == SessionStatusActive {
			session.Status = SessionStatusCompleted
			session.UpdatedAt = at
			m.s.Sessions[step.SessionID] = session
			m.appendEventLocked(step.SessionID, nil, "SESSION_COMPLETED", tx.Actor, map[string]any{
				"sessionId": step.SessionID,
			}, at, tx.TxID)
		}
	}
	return nil
}

func (m *Machine) expireClaimsLocked(at time.Time, txID string) {
	expiredIDs := make([]string, 0)
	for claimID, claim := range m.s.Claims {
		if claim.Status != ClaimStatusActive {
			continue
		}
		if !claim.LeaseUntil.After(at) {
			expiredIDs = append(expiredIDs, claimID)
		}
	}
	sort.Strings(expiredIDs)
	for _, claimID := range expiredIDs {
		claim := m.s.Claims[claimID]
		claim.Status = ClaimStatusExpired
		claim.UpdatedAt = at
		m.s.Claims[claimID] = claim

		step, ok := m.s.Steps[claim.StepID]
		if ok && step.Status == StepStatusClaimed {
			if activeID, _ := m.findActiveClaimByStepLocked(step.StepID, at); activeID == "" {
				step.Status = StepStatusOpen
				step.UpdatedAt = at
				m.s.Steps[step.StepID] = step
			}
		}
		if ok {
			m.appendEventLocked(step.SessionID, &step.StepID, "CLAIM_EXPIRED", "system", map[string]any{
				"claimId":       claim.ClaimID,
				"participantId": claim.ParticipantID,
				"leaseUntil":    claim.LeaseUntil,
			}, at, txID)
		}
	}
}

func (m *Machine) appendEventLocked(sessionID string, stepID *string, eventType, actor string, payload any, at time.Time, txID string) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system"
	}
	rawPayload := json.RawMessage(nil)
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			rawPayload = b
		}
	}
	seq := len(m.s.EventsBySession[sessionID]) + 1
	eventID := fmt.Sprintf("%s:%s:%06d", strings.TrimSpace(txID), sessionID, seq)
	if strings.TrimSpace(txID) == "" {
		eventID = fmt.Sprintf("%d:%s:%06d", at.UnixNano(), sessionID, seq)
	}
	var sid *string
	if stepID != nil {
		step := strings.TrimSpace(*stepID)
		if step != "" {
			sid = &step
		}
	}
	event := Event{
		EventID:    eventID,
		SessionID:  sessionID,
		StepID:     sid,
		Type:       strings.TrimSpace(eventType),
		Actor:      actor,
		Payload:    rawPayload,
		CreatedAt:  at,
		TxID:       txID,
		CommitTime: at,
	}
	m.s.EventsBySession[sessionID] = append(m.s.EventsBySession[sessionID], event)
	if session, ok := m.s.Sessions[sessionID]; ok {
		session.LastEventID = event.EventID
		session.UpdatedAt = at
		m.s.Sessions[sessionID] = session
	}
}

func (m *Machine) findActiveClaimByStepLocked(stepID string, at time.Time) (string, Claim) {
	ids := make([]string, 0, len(m.s.Claims))
	for claimID, claim := range m.s.Claims {
		if claim.StepID == stepID {
			ids = append(ids, claimID)
		}
	}
	sort.Strings(ids)
	for _, claimID := range ids {
		claim := m.s.Claims[claimID]
		if claim.Status != ClaimStatusActive {
			continue
		}
		if claim.LeaseUntil.After(at) {
			return claimID, claim
		}
	}
	return "", Claim{}
}

func (m *Machine) findActiveClaimByStepAndParticipantLocked(stepID, participantID string, at time.Time) (string, Claim) {
	ids := make([]string, 0, len(m.s.Claims))
	for claimID, claim := range m.s.Claims {
		if claim.StepID == stepID && claim.ParticipantID == participantID {
			ids = append(ids, claimID)
		}
	}
	sort.Strings(ids)
	for _, claimID := range ids {
		claim := m.s.Claims[claimID]
		if claim.Status != ClaimStatusActive {
			continue
		}
		if claim.LeaseUntil.After(at) {
			return claimID, claim
		}
	}
	return "", Claim{}
}

func (m *Machine) artifactExistsLocked(artifactID string) bool {
	for _, artifacts := range m.s.ArtifactsByStep {
		for _, artifact := range artifacts {
			if artifact.ArtifactID == artifactID {
				return true
			}
		}
	}
	return false
}

func (m *Machine) voteExistsLocked(voteID string) bool {
	for _, byParticipant := range m.s.VotesByDecision {
		for _, vote := range byParticipant {
			if vote.VoteID == voteID {
				return true
			}
		}
	}
	return false
}

func (m *Machine) allStepsResolvedLocked(sessionID string) bool {
	stepIDs := append([]string(nil), m.s.StepOrderBySession[sessionID]...)
	sort.Strings(stepIDs)
	if len(stepIDs) == 0 {
		return true
	}
	for _, stepID := range stepIDs {
		step, ok := m.s.Steps[stepID]
		if !ok {
			continue
		}
		if step.Status != StepStatusResolved {
			return false
		}
	}
	return true
}

func sessionRef(sessionID, ref string) string {
	return strings.ToLower(strings.TrimSpace(sessionID) + "::" + strings.TrimSpace(ref))
}

func uniqueNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func depsResolved(step Step, all map[string]Step) bool {
	if len(step.DependsOn) == 0 {
		return true
	}
	for _, dep := range step.DependsOn {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if st, ok := all[dep]; ok && st.SessionID == step.SessionID {
			if st.Status != StepStatusResolved {
				return false
			}
			continue
		}
		foundByKey := false
		for _, candidate := range all {
			if candidate.SessionID != step.SessionID {
				continue
			}
			if candidate.StepKey != dep {
				continue
			}
			foundByKey = true
			if candidate.Status != StepStatusResolved {
				return false
			}
			break
		}
		if !foundByKey {
			return false
		}
	}
	return true
}

func hasCapabilities(actual, required []string) bool {
	if len(required) == 0 {
		return true
	}
	if len(actual) == 0 {
		return false
	}
	have := map[string]struct{}{}
	for _, capability := range actual {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			continue
		}
		have[capability] = struct{}{}
	}
	for _, capability := range required {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			continue
		}
		if _, ok := have[capability]; !ok {
			return false
		}
	}
	return true
}

type decisionPolicy struct {
	MinApprovals    int `json:"min_approvals"`
	Quorum          int `json:"quorum"`
	RejectThreshold int `json:"reject_threshold"`
}

func normalizeDecisionPolicy(raw json.RawMessage) (json.RawMessage, decisionPolicy, error) {
	p := decisionPolicy{MinApprovals: 1}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, decisionPolicy{}, errors.New("policy must be valid JSON")
		}
	}
	if p.MinApprovals <= 0 {
		p.MinApprovals = 1
	}
	if p.Quorum < 0 {
		p.Quorum = 0
	}
	if p.RejectThreshold < 0 {
		p.RejectThreshold = 0
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, decisionPolicy{}, err
	}
	return b, p, nil
}

func evaluateDecision(policyRaw json.RawMessage, votes map[string]Vote) (string, string) {
	_, policy, err := normalizeDecisionPolicy(policyRaw)
	if err != nil {
		policy = decisionPolicy{MinApprovals: 1}
	}
	approves := 0
	rejects := 0
	for _, vote := range votes {
		switch strings.ToUpper(strings.TrimSpace(vote.Choice)) {
		case VoteChoiceApprove:
			approves++
		case VoteChoiceReject:
			rejects++
		}
	}
	if policy.RejectThreshold > 0 && rejects >= policy.RejectThreshold {
		return DecisionStatusRejected, "reject threshold reached"
	}
	if approves >= policy.MinApprovals {
		if policy.Quorum <= 0 || len(votes) >= policy.Quorum {
			return DecisionStatusPassed, "approval threshold reached"
		}
	}
	return DecisionStatusPending, ""
}

func pageWindow(total, limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return total, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return offset, end
}

func cloneSession(in Session) Session {
	if in.Context != nil {
		in.Context = append([]byte(nil), in.Context...)
	}
	return in
}

func cloneArtifact(in Artifact) Artifact {
	if in.Content != nil {
		in.Content = append([]byte(nil), in.Content...)
	}
	return in
}

func cloneDecision(in Decision) Decision {
	if in.Policy != nil {
		in.Policy = append([]byte(nil), in.Policy...)
	}
	return in
}

func cloneEvent(in Event) Event {
	if in.Payload != nil {
		in.Payload = append([]byte(nil), in.Payload...)
	}
	if in.StepID != nil {
		stepID := *in.StepID
		in.StepID = &stepID
	}
	return in
}

func (m *Machine) GetSession(sessionID string) (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.s.Sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return Session{}, false
	}
	return cloneSession(session), true
}

func (m *Machine) ListParticipants(sessionID string, limit, offset int) []Participant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessionID = strings.TrimSpace(sessionID)
	out := make([]Participant, 0)
	for _, participant := range m.s.Participants {
		if participant.SessionID != sessionID {
			continue
		}
		out = append(out, cloneParticipant(participant))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].JoinedAt.Equal(out[j].JoinedAt) {
			return out[i].ParticipantID < out[j].ParticipantID
		}
		return out[i].JoinedAt.Before(out[j].JoinedAt)
	})
	start, end := pageWindow(len(out), limit, offset)
	return append([]Participant(nil), out[start:end]...)
}

func (m *Machine) ListOpenSteps(sessionID string, participantID *string, at time.Time, limit, offset int) ([]Step, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session_id is required")
	}
	if _, ok := m.s.Sessions[sessionID]; !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	var participant *Participant
	if participantID != nil {
		pid := strings.TrimSpace(*participantID)
		if pid != "" {
			p, ok := m.s.Participants[pid]
			if !ok {
				return nil, fmt.Errorf("participant not found: %s", pid)
			}
			if p.SessionID != sessionID {
				return nil, errors.New("participant not found in session")
			}
			cp := cloneParticipant(p)
			participant = &cp
		}
	}
	stepIDs := append([]string(nil), m.s.StepOrderBySession[sessionID]...)
	out := make([]Step, 0, len(stepIDs))
	for _, stepID := range stepIDs {
		step, ok := m.s.Steps[stepID]
		if !ok {
			continue
		}
		stepStatus := step.Status
		if stepStatus == StepStatusClaimed {
			if activeID, _ := m.findActiveClaimByStepLocked(step.StepID, at); activeID == "" {
				stepStatus = StepStatusOpen
			}
		}
		if stepStatus != StepStatusOpen {
			continue
		}
		if !depsResolved(step, m.s.Steps) {
			continue
		}
		if participant != nil && !hasCapabilities(participant.Capabilities, step.RequiredCapabilities) {
			continue
		}
		if activeID, _ := m.findActiveClaimByStepLocked(step.StepID, at); activeID != "" {
			continue
		}
		out = append(out, cloneStep(step))
	}
	start, end := pageWindow(len(out), limit, offset)
	return append([]Step(nil), out[start:end]...), nil
}

func (m *Machine) GetStep(stepID string) (Step, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	step, ok := m.s.Steps[strings.TrimSpace(stepID)]
	if !ok {
		return Step{}, false
	}
	return cloneStep(step), true
}

func (m *Machine) ListArtifacts(stepID string) []Artifact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stepID = strings.TrimSpace(stepID)
	items := append([]Artifact(nil), m.s.ArtifactsByStep[stepID]...)
	out := make([]Artifact, 0, len(items))
	for _, artifact := range items {
		out = append(out, cloneArtifact(artifact))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Version == out[j].Version {
			if out[i].CreatedAt.Equal(out[j].CreatedAt) {
				return out[i].ArtifactID < out[j].ArtifactID
			}
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].Version < out[j].Version
	})
	return out
}

func (m *Machine) ListEvents(sessionID string, limit, offset int) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessionID = strings.TrimSpace(sessionID)
	items := append([]Event(nil), m.s.EventsBySession[sessionID]...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].EventID > items[j].EventID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	start, end := pageWindow(len(items), limit, offset)
	out := make([]Event, 0, end-start)
	for _, event := range items[start:end] {
		out = append(out, cloneEvent(event))
	}
	return out
}

type Stats struct {
	Sessions         int `json:"sessions"`
	Participants     int `json:"participants"`
	Steps            int `json:"steps"`
	OpenSteps        int `json:"openSteps"`
	Claims           int `json:"claims"`
	ActiveClaims     int `json:"activeClaims"`
	Artifacts        int `json:"artifacts"`
	Decisions        int `json:"decisions"`
	PendingDecisions int `json:"pendingDecisions"`
	Votes            int `json:"votes"`
	Events           int `json:"events"`
	AppliedTx        int `json:"appliedTx"`
}

func (m *Machine) StateStats(at time.Time) Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := Stats{
		Sessions:     len(m.s.Sessions),
		Participants: len(m.s.Participants),
		Steps:        len(m.s.Steps),
		Claims:       len(m.s.Claims),
		Decisions:    len(m.s.Decisions),
		AppliedTx:    len(m.s.AppliedTx),
	}
	for _, step := range m.s.Steps {
		if step.Status == StepStatusOpen {
			stats.OpenSteps++
			continue
		}
		if step.Status == StepStatusClaimed {
			if activeID, _ := m.findActiveClaimByStepLocked(step.StepID, at); activeID == "" {
				stats.OpenSteps++
			}
		}
	}
	for _, claim := range m.s.Claims {
		if claim.Status == ClaimStatusActive && claim.LeaseUntil.After(at) {
			stats.ActiveClaims++
		}
	}
	for _, artifacts := range m.s.ArtifactsByStep {
		stats.Artifacts += len(artifacts)
	}
	for _, decision := range m.s.Decisions {
		if decision.Status == DecisionStatusPending {
			stats.PendingDecisions++
		}
	}
	for _, byParticipant := range m.s.VotesByDecision {
		stats.Votes += len(byParticipant)
	}
	for _, events := range m.s.EventsBySession {
		stats.Events += len(events)
	}
	return stats
}
