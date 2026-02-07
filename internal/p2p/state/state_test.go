package state

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/execution-hub/execution-hub/internal/p2p/protocol"
)

func TestMachineEndToEnd(t *testing.T) {
	m := NewMachine()
	_, priv := mustKey(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mustApply(t, m, signedTx(t, priv, "tx-001", "session-1", "actor:admin", base,
		protocol.OpSessionCreate, protocol.SessionCreatePayload{
			SessionID: "session-1",
			Name:      "P2P Session",
			Steps: []protocol.SessionStep{
				{StepID: "step-1", StepKey: "draft", Name: "Draft", LeaseTTLSeconds: 60},
				{StepID: "step-2", StepKey: "review", Name: "Review", DependsOn: []string{"step-1"}, RequiredCapabilities: []string{"review"}, LeaseTTLSeconds: 60},
			},
		}))
	mustApply(t, m, signedTx(t, priv, "tx-002", "session-1", "actor:alice", base.Add(1*time.Second),
		protocol.OpParticipantJoin, protocol.ParticipantJoinPayload{ParticipantID: "p-alice", SessionID: "session-1", Type: "HUMAN", Ref: "user:alice", Capabilities: []string{"draft"}}))
	mustApply(t, m, signedTx(t, priv, "tx-003", "session-1", "actor:bob", base.Add(2*time.Second),
		protocol.OpParticipantJoin, protocol.ParticipantJoinPayload{ParticipantID: "p-bob", SessionID: "session-1", Type: "HUMAN", Ref: "user:bob", Capabilities: []string{"review"}}))

	open, err := m.ListOpenSteps("session-1", ptr("p-bob"), base.Add(3*time.Second), 100, 0)
	if err != nil {
		t.Fatalf("list open steps: %v", err)
	}
	if len(open) != 1 || open[0].StepID != "step-1" {
		t.Fatalf("unexpected initial open steps: %+v", open)
	}

	mustApply(t, m, signedTx(t, priv, "tx-004", "session-1", "actor:alice", base.Add(3*time.Second),
		protocol.OpStepClaim, protocol.StepClaimPayload{ClaimID: "claim-1", StepID: "step-1", ParticipantID: "p-alice"}))
	mustApply(t, m, signedTx(t, priv, "tx-005", "session-1", "actor:alice", base.Add(4*time.Second),
		protocol.OpArtifactAdd, protocol.ArtifactAddPayload{ArtifactID: "artifact-1", StepID: "step-1", ProducerID: "p-alice", Kind: "draft", Content: rawJSON(`{"text":"done"}`)}))
	mustApply(t, m, signedTx(t, priv, "tx-006", "session-1", "actor:alice", base.Add(5*time.Second),
		protocol.OpDecisionOpen, protocol.DecisionOpenPayload{DecisionID: "decision-1", StepID: "step-1"}))
	mustApply(t, m, signedTx(t, priv, "tx-007", "session-1", "actor:bob", base.Add(6*time.Second),
		protocol.OpVoteCast, protocol.VoteCastPayload{VoteID: "vote-1", DecisionID: "decision-1", ParticipantID: "p-bob", Choice: VoteChoiceApprove}))
	mustApply(t, m, signedTx(t, priv, "tx-008", "session-1", "actor:alice", base.Add(7*time.Second),
		protocol.OpStepResolve, protocol.StepResolvePayload{StepID: "step-1", ParticipantID: ptr("p-alice")}))

	open2, err := m.ListOpenSteps("session-1", ptr("p-bob"), base.Add(8*time.Second), 100, 0)
	if err != nil {
		t.Fatalf("list open steps after resolve: %v", err)
	}
	if len(open2) != 1 || open2[0].StepID != "step-2" {
		t.Fatalf("expected step-2 claimable after step-1 resolve, got %+v", open2)
	}

	mustApply(t, m, signedTx(t, priv, "tx-009", "session-1", "actor:bob", base.Add(8*time.Second),
		protocol.OpStepClaim, protocol.StepClaimPayload{ClaimID: "claim-2", StepID: "step-2", ParticipantID: "p-bob"}))
	mustApply(t, m, signedTx(t, priv, "tx-010", "session-1", "actor:bob", base.Add(9*time.Second),
		protocol.OpStepResolve, protocol.StepResolvePayload{StepID: "step-2", ParticipantID: ptr("p-bob")}))

	session, ok := m.GetSession("session-1")
	if !ok {
		t.Fatalf("session not found")
	}
	if session.Status != SessionStatusCompleted {
		t.Fatalf("expected completed session, got %s", session.Status)
	}

	events := m.ListEvents("session-1", 100, 0)
	if len(events) < 10 {
		t.Fatalf("expected timeline events, got %d", len(events))
	}
}

func TestMachineClaimExpiresOnNextTx(t *testing.T) {
	m := NewMachine()
	_, priv := mustKey(t)
	base := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)

	mustApply(t, m, signedTx(t, priv, "tx-a1", "session-exp", "actor:admin", base,
		protocol.OpSessionCreate, protocol.SessionCreatePayload{
			SessionID: "session-exp",
			Name:      "Expiry Session",
			Steps:     []protocol.SessionStep{{StepID: "s1", StepKey: "build", Name: "Build", LeaseTTLSeconds: 1}},
		}))
	mustApply(t, m, signedTx(t, priv, "tx-a2", "session-exp", "actor:a", base.Add(1*time.Second),
		protocol.OpParticipantJoin, protocol.ParticipantJoinPayload{ParticipantID: "p1", SessionID: "session-exp", Type: "HUMAN", Ref: "user:a"}))
	mustApply(t, m, signedTx(t, priv, "tx-a3", "session-exp", "actor:b", base.Add(2*time.Second),
		protocol.OpParticipantJoin, protocol.ParticipantJoinPayload{ParticipantID: "p2", SessionID: "session-exp", Type: "HUMAN", Ref: "user:b"}))
	mustApply(t, m, signedTx(t, priv, "tx-a4", "session-exp", "actor:a", base.Add(3*time.Second),
		protocol.OpStepClaim, protocol.StepClaimPayload{ClaimID: "claim-a", StepID: "s1", ParticipantID: "p1", LeaseSeconds: 1}))

	mustApply(t, m, signedTx(t, priv, "tx-a5", "session-exp", "actor:b", base.Add(6*time.Second),
		protocol.OpStepClaim, protocol.StepClaimPayload{ClaimID: "claim-b", StepID: "s1", ParticipantID: "p2", LeaseSeconds: 30}))

	step, ok := m.GetStep("s1")
	if !ok {
		t.Fatalf("step not found")
	}
	if step.Status != StepStatusClaimed {
		t.Fatalf("expected claimed step, got %s", step.Status)
	}

	stats := m.StateStats(base.Add(6 * time.Second))
	if stats.ActiveClaims != 1 {
		t.Fatalf("expected exactly one active claim after expiry rollover, got %d", stats.ActiveClaims)
	}
}

func TestListOpenStepsRequiresExistingSession(t *testing.T) {
	m := NewMachine()
	_, err := m.ListOpenSteps("missing-session", nil, time.Now().UTC(), 100, 0)
	if err == nil {
		t.Fatalf("expected session not found error")
	}
}

func mustKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func rawJSON(s string) json.RawMessage {
	return json.RawMessage([]byte(s))
}

func ptr[T any](v T) *T {
	return &v
}

func mustApply(t *testing.T, m *Machine, tx protocol.Tx) {
	t.Helper()
	if err := m.ApplyTx(tx); err != nil {
		t.Fatalf("apply tx %s: %v", tx.TxID, err)
	}
}

func signedTx(t *testing.T, priv ed25519.PrivateKey, txID, sessionID, actor string, at time.Time, op protocol.Operation, payload any) protocol.Tx {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	tx := protocol.Tx{
		TxID:      txID,
		SessionID: sessionID,
		Nonce:     txID,
		Timestamp: at,
		Actor:     actor,
		Op:        op,
		Payload:   raw,
	}
	if err := tx.Sign(priv); err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	return tx
}
