package protocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"
)

func TestTxSignAndVerify(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	payload, _ := json.Marshal(SessionCreatePayload{
		SessionID: "session-1",
		Name:      "demo",
		Steps: []SessionStep{{
			StepID:  "step-1",
			StepKey: "draft",
			Name:    "Draft",
		}},
	})
	tx := Tx{
		TxID:      "tx-1",
		SessionID: "session-1",
		Nonce:     "n1",
		Timestamp: time.Now().UTC(),
		Actor:     "user:alice",
		Op:        OpSessionCreate,
		Payload:   payload,
	}
	if err := tx.Sign(priv); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := tx.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}

	tx.Actor = "user:bob"
	if err := tx.Verify(); err == nil {
		t.Fatalf("expected verify failure after tamper")
	}
}
