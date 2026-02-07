package collab

import (
	"encoding/json"
	"testing"

	"github.com/execution-hub/execution-hub/internal/domain/collab"
)

func TestEvaluateDecisionPassed(t *testing.T) {
	policy, _ := json.Marshal(map[string]int{
		"min_approvals": 2,
	})
	votes := []*collab.Vote{
		{Choice: collab.VoteChoiceApprove},
		{Choice: collab.VoteChoiceApprove},
	}
	status, reason := evaluateDecision(policy, votes)
	if status != collab.DecisionStatusPassed {
		t.Fatalf("expected PASSED, got %s", status)
	}
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestEvaluateDecisionRejected(t *testing.T) {
	policy, _ := json.Marshal(map[string]int{
		"min_approvals":    2,
		"reject_threshold": 1,
	})
	votes := []*collab.Vote{
		{Choice: collab.VoteChoiceReject},
	}
	status, _ := evaluateDecision(policy, votes)
	if status != collab.DecisionStatusRejected {
		t.Fatalf("expected REJECTED, got %s", status)
	}
}

func TestEvaluateDecisionPendingWithQuorum(t *testing.T) {
	policy, _ := json.Marshal(map[string]int{
		"min_approvals": 1,
		"quorum":        3,
	})
	votes := []*collab.Vote{
		{Choice: collab.VoteChoiceApprove},
		{Choice: collab.VoteChoiceApprove},
	}
	status, _ := evaluateDecision(policy, votes)
	if status != collab.DecisionStatusPending {
		t.Fatalf("expected PENDING, got %s", status)
	}
}
