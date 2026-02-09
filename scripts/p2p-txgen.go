package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/execution-hub/execution-hub/internal/p2p/protocol"
)

type options struct {
	op         string
	sessionID  string
	actor      string
	txID       string
	nonce      string
	timestamp  string
	privateKey string

	workflowID  string
	sessionName string
	contextJSON string
	stepsJSON   string

	defaultStepID           string
	defaultStepKey          string
	defaultStepName         string
	defaultStepCapabilities string
	defaultStepDependsOn    string
	leaseSeconds            int

	participantID           string
	participantType         string
	participantRef          string
	participantCapabilities string
	trustScore              int

	stepID string

	claimID string

	newClaimID        string
	fromParticipantID string
	toParticipantID   string

	artifactID   string
	producerID   string
	artifactKind string
	contentJSON  string
	externalURI  string
	contentHash  string
	contentBytes int64

	decisionID string
	policyJSON string
	deadline   string

	voteID  string
	choice  string
	comment string
}

func main() {
	var opt options

	flag.StringVar(&opt.op, "op", "", "operation: session-create|participant-join|step-claim|step-release|step-handoff|artifact-add|decision-open|vote-cast|step-resolve")
	flag.StringVar(&opt.sessionID, "session-id", "smoke-session", "session identifier")
	flag.StringVar(&opt.actor, "actor", "smoke", "actor string")
	flag.StringVar(&opt.txID, "tx-id", "", "tx identifier; auto-generated when empty")
	flag.StringVar(&opt.nonce, "nonce", "", "nonce; auto-generated when empty")
	flag.StringVar(&opt.timestamp, "timestamp", "", "RFC3339 timestamp; default now UTC")
	flag.StringVar(&opt.privateKey, "private-key", "", "base64 private key (32-byte seed or 64-byte private key); default random")

	flag.StringVar(&opt.workflowID, "workflow-id", "", "workflow identifier for session-create")
	flag.StringVar(&opt.sessionName, "session-name", "Smoke Session", "session name for session-create")
	flag.StringVar(&opt.contextJSON, "context-json", "", "session context JSON for session-create")
	flag.StringVar(&opt.stepsJSON, "steps-json", "", "session steps JSON array for session-create")

	flag.StringVar(&opt.defaultStepID, "default-step-id", "smoke-step-1", "default step_id when steps-json is empty")
	flag.StringVar(&opt.defaultStepKey, "default-step-key", "draft", "default step_key when steps-json is empty")
	flag.StringVar(&opt.defaultStepName, "default-step-name", "Draft", "default step name when steps-json is empty")
	flag.StringVar(&opt.defaultStepCapabilities, "default-step-capabilities", "draft", "comma-separated default required capabilities when steps-json is empty")
	flag.StringVar(&opt.defaultStepDependsOn, "default-step-depends-on", "", "comma-separated default depends_on when steps-json is empty")
	flag.IntVar(&opt.leaseSeconds, "lease-seconds", 300, "lease seconds for claim/handoff/default step")

	flag.StringVar(&opt.participantID, "participant-id", "smoke-participant", "participant identifier")
	flag.StringVar(&opt.participantType, "participant-type", "HUMAN", "participant type: HUMAN|AGENT")
	flag.StringVar(&opt.participantRef, "participant-ref", "user:smoke", "participant ref for participant-join")
	flag.StringVar(&opt.participantCapabilities, "participant-capabilities", "draft", "comma-separated participant capabilities for participant-join")
	flag.IntVar(&opt.trustScore, "trust-score", 100, "trust score for participant-join")

	flag.StringVar(&opt.stepID, "step-id", "", "step identifier")
	flag.StringVar(&opt.claimID, "claim-id", "", "claim identifier")
	flag.StringVar(&opt.newClaimID, "new-claim-id", "", "new claim identifier for step-handoff")
	flag.StringVar(&opt.fromParticipantID, "from-participant-id", "", "source participant for step-handoff")
	flag.StringVar(&opt.toParticipantID, "to-participant-id", "", "target participant for step-handoff")

	flag.StringVar(&opt.artifactID, "artifact-id", "", "artifact identifier")
	flag.StringVar(&opt.producerID, "producer-id", "", "artifact producer participant identifier")
	flag.StringVar(&opt.artifactKind, "artifact-kind", "generic", "artifact kind")
	flag.StringVar(&opt.contentJSON, "content-json", "", "artifact content JSON")
	flag.StringVar(&opt.externalURI, "external-uri", "", "artifact external URI when content is omitted")
	flag.StringVar(&opt.contentHash, "content-hash", "", "artifact content hash")
	flag.Int64Var(&opt.contentBytes, "content-bytes", 0, "artifact content bytes")

	flag.StringVar(&opt.decisionID, "decision-id", "", "decision identifier")
	flag.StringVar(&opt.policyJSON, "policy-json", "", "decision policy JSON")
	flag.StringVar(&opt.deadline, "deadline", "", "decision deadline RFC3339")

	flag.StringVar(&opt.voteID, "vote-id", "", "vote identifier")
	flag.StringVar(&opt.choice, "choice", "APPROVE", "vote choice: APPROVE|REJECT")
	flag.StringVar(&opt.comment, "comment", "", "comment for vote or handoff")
	flag.Parse()

	op, err := parseOperation(opt.op)
	if err != nil {
		log.Fatal(err)
	}
	opt.actor = strings.TrimSpace(opt.actor)
	if opt.actor == "" {
		log.Fatal("actor is required")
	}

	payload, txSessionID, err := buildPayload(op, opt)
	if err != nil {
		log.Fatal(err)
	}
	sessionID := strings.TrimSpace(opt.sessionID)
	if sessionID == "" {
		sessionID = txSessionID
	}

	privateKey, err := loadPrivateKey(opt.privateKey)
	if err != nil {
		log.Fatal(err)
	}
	ts, err := parseTimestamp(opt.timestamp)
	if err != nil {
		log.Fatal(err)
	}

	txID := strings.TrimSpace(opt.txID)
	if txID == "" {
		txID = autoID("tx", ts)
	}
	nonce := strings.TrimSpace(opt.nonce)
	if nonce == "" {
		nonce = autoID("n", ts)
	}
	tx := protocol.Tx{
		TxID:      txID,
		SessionID: sessionID,
		Nonce:     nonce,
		Timestamp: ts,
		Actor:     opt.actor,
		Op:        op,
		Payload:   payload,
	}
	if err := tx.Sign(privateKey); err != nil {
		log.Fatal(err)
	}

	out, err := json.Marshal(tx)
	if err != nil {
		log.Fatal(err)
	}
	_, _ = os.Stdout.Write(out)
}

func parseOperation(raw string) (protocol.Operation, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "session-create", "session_create":
		return protocol.OpSessionCreate, nil
	case "participant-join", "participant_join":
		return protocol.OpParticipantJoin, nil
	case "step-claim", "step_claim":
		return protocol.OpStepClaim, nil
	case "step-release", "step_release":
		return protocol.OpStepRelease, nil
	case "step-handoff", "step_handoff":
		return protocol.OpStepHandoff, nil
	case "artifact-add", "artifact_add":
		return protocol.OpArtifactAdd, nil
	case "decision-open", "decision_open":
		return protocol.OpDecisionOpen, nil
	case "vote-cast", "vote_cast":
		return protocol.OpVoteCast, nil
	case "step-resolve", "step_resolve":
		return protocol.OpStepResolve, nil
	default:
		return "", fmt.Errorf("unsupported op: %q", raw)
	}
}

func buildPayload(op protocol.Operation, opt options) (json.RawMessage, string, error) {
	switch op {
	case protocol.OpSessionCreate:
		sessionID := strings.TrimSpace(opt.sessionID)
		if sessionID == "" {
			return nil, "", errors.New("session-id is required for session-create")
		}
		sessionName := strings.TrimSpace(opt.sessionName)
		if sessionName == "" {
			return nil, "", errors.New("session-name is required for session-create")
		}
		contextRaw, err := parseOptionalJSON(opt.contextJSON, "context-json")
		if err != nil {
			return nil, "", err
		}

		var steps []protocol.SessionStep
		if strings.TrimSpace(opt.stepsJSON) != "" {
			if err := json.Unmarshal([]byte(opt.stepsJSON), &steps); err != nil {
				return nil, "", fmt.Errorf("invalid steps-json: %w", err)
			}
			if len(steps) == 0 {
				return nil, "", errors.New("steps-json must contain at least one step")
			}
		} else {
			stepID := strings.TrimSpace(opt.defaultStepID)
			stepKey := strings.TrimSpace(opt.defaultStepKey)
			stepName := strings.TrimSpace(opt.defaultStepName)
			if stepID == "" || stepKey == "" {
				return nil, "", errors.New("default-step-id and default-step-key are required when steps-json is empty")
			}
			if stepName == "" {
				stepName = stepKey
			}
			leaseSeconds := opt.leaseSeconds
			if leaseSeconds <= 0 {
				leaseSeconds = 300
			}
			steps = []protocol.SessionStep{
				{
					StepID:               stepID,
					StepKey:              stepKey,
					Name:                 stepName,
					RequiredCapabilities: splitCSV(opt.defaultStepCapabilities),
					DependsOn:            splitCSV(opt.defaultStepDependsOn),
					LeaseTTLSeconds:      leaseSeconds,
				},
			}
		}

		raw, err := json.Marshal(protocol.SessionCreatePayload{
			SessionID:  sessionID,
			WorkflowID: strings.TrimSpace(opt.workflowID),
			Name:       sessionName,
			Context:    contextRaw,
			Steps:      steps,
		})
		return raw, sessionID, err

	case protocol.OpParticipantJoin:
		sessionID := strings.TrimSpace(opt.sessionID)
		if sessionID == "" {
			return nil, "", errors.New("session-id is required for participant-join")
		}
		participantID := strings.TrimSpace(opt.participantID)
		if participantID == "" {
			return nil, "", errors.New("participant-id is required for participant-join")
		}
		participantType := strings.ToUpper(strings.TrimSpace(opt.participantType))
		if participantType == "" {
			participantType = "HUMAN"
		}
		ref := strings.TrimSpace(opt.participantRef)
		if ref == "" {
			if participantType == "AGENT" {
				ref = "agent:smoke"
			} else {
				ref = "user:smoke"
			}
		}
		raw, err := json.Marshal(protocol.ParticipantJoinPayload{
			ParticipantID: participantID,
			SessionID:     sessionID,
			Type:          participantType,
			Ref:           ref,
			Capabilities:  splitCSV(opt.participantCapabilities),
			TrustScore:    opt.trustScore,
		})
		return raw, sessionID, err

	case protocol.OpStepClaim:
		stepID := strings.TrimSpace(opt.stepID)
		participantID := strings.TrimSpace(opt.participantID)
		if stepID == "" || participantID == "" {
			return nil, "", errors.New("step-id and participant-id are required for step-claim")
		}
		claimID := strings.TrimSpace(opt.claimID)
		if claimID == "" {
			claimID = autoID("claim", time.Now().UTC())
		}
		raw, err := json.Marshal(protocol.StepClaimPayload{
			ClaimID:       claimID,
			StepID:        stepID,
			ParticipantID: participantID,
			LeaseSeconds:  opt.leaseSeconds,
		})
		return raw, strings.TrimSpace(opt.sessionID), err

	case protocol.OpStepRelease:
		stepID := strings.TrimSpace(opt.stepID)
		participantID := strings.TrimSpace(opt.participantID)
		if stepID == "" || participantID == "" {
			return nil, "", errors.New("step-id and participant-id are required for step-release")
		}
		raw, err := json.Marshal(protocol.StepReleasePayload{
			StepID:        stepID,
			ParticipantID: participantID,
		})
		return raw, strings.TrimSpace(opt.sessionID), err

	case protocol.OpStepHandoff:
		stepID := strings.TrimSpace(opt.stepID)
		fromID := strings.TrimSpace(opt.fromParticipantID)
		toID := strings.TrimSpace(opt.toParticipantID)
		if stepID == "" || fromID == "" || toID == "" {
			return nil, "", errors.New("step-id, from-participant-id and to-participant-id are required for step-handoff")
		}
		newClaimID := strings.TrimSpace(opt.newClaimID)
		if newClaimID == "" {
			newClaimID = autoID("claim", time.Now().UTC())
		}
		var comment *string
		if trimmed := strings.TrimSpace(opt.comment); trimmed != "" {
			comment = &trimmed
		}
		raw, err := json.Marshal(protocol.StepHandoffPayload{
			NewClaimID:        newClaimID,
			StepID:            stepID,
			FromParticipantID: fromID,
			ToParticipantID:   toID,
			LeaseSeconds:      opt.leaseSeconds,
			Comment:           comment,
		})
		return raw, strings.TrimSpace(opt.sessionID), err

	case protocol.OpArtifactAdd:
		stepID := strings.TrimSpace(opt.stepID)
		producerID := strings.TrimSpace(opt.producerID)
		if producerID == "" {
			producerID = strings.TrimSpace(opt.participantID)
		}
		if stepID == "" || producerID == "" {
			return nil, "", errors.New("step-id and producer-id (or participant-id) are required for artifact-add")
		}
		artifactID := strings.TrimSpace(opt.artifactID)
		if artifactID == "" {
			artifactID = autoID("artifact", time.Now().UTC())
		}
		content, err := parseOptionalJSON(opt.contentJSON, "content-json")
		if err != nil {
			return nil, "", err
		}
		externalURI := strings.TrimSpace(opt.externalURI)
		if len(content) == 0 && externalURI == "" {
			return nil, "", errors.New("content-json or external-uri is required for artifact-add")
		}
		kind := strings.TrimSpace(opt.artifactKind)
		if kind == "" {
			kind = "generic"
		}
		raw, err := json.Marshal(protocol.ArtifactAddPayload{
			ArtifactID:   artifactID,
			StepID:       stepID,
			ProducerID:   producerID,
			Kind:         kind,
			Content:      content,
			ContentHash:  strings.TrimSpace(opt.contentHash),
			ExternalURI:  externalURI,
			ContentBytes: opt.contentBytes,
		})
		return raw, strings.TrimSpace(opt.sessionID), err

	case protocol.OpDecisionOpen:
		stepID := strings.TrimSpace(opt.stepID)
		if stepID == "" {
			return nil, "", errors.New("step-id is required for decision-open")
		}
		decisionID := strings.TrimSpace(opt.decisionID)
		if decisionID == "" {
			decisionID = autoID("decision", time.Now().UTC())
		}
		policy, err := parseOptionalJSON(opt.policyJSON, "policy-json")
		if err != nil {
			return nil, "", err
		}
		var deadline *time.Time
		if strings.TrimSpace(opt.deadline) != "" {
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(opt.deadline))
			if err != nil {
				return nil, "", fmt.Errorf("invalid deadline: %w", err)
			}
			utc := parsed.UTC()
			deadline = &utc
		}
		raw, err := json.Marshal(protocol.DecisionOpenPayload{
			DecisionID: decisionID,
			StepID:     stepID,
			Policy:     policy,
			Deadline:   deadline,
		})
		return raw, strings.TrimSpace(opt.sessionID), err

	case protocol.OpVoteCast:
		decisionID := strings.TrimSpace(opt.decisionID)
		participantID := strings.TrimSpace(opt.participantID)
		if decisionID == "" || participantID == "" {
			return nil, "", errors.New("decision-id and participant-id are required for vote-cast")
		}
		voteID := strings.TrimSpace(opt.voteID)
		if voteID == "" {
			voteID = autoID("vote", time.Now().UTC())
		}
		choice := strings.ToUpper(strings.TrimSpace(opt.choice))
		if choice == "" {
			choice = "APPROVE"
		}
		var comment *string
		if trimmed := strings.TrimSpace(opt.comment); trimmed != "" {
			comment = &trimmed
		}
		raw, err := json.Marshal(protocol.VoteCastPayload{
			VoteID:        voteID,
			DecisionID:    decisionID,
			ParticipantID: participantID,
			Choice:        choice,
			Comment:       comment,
		})
		return raw, strings.TrimSpace(opt.sessionID), err

	case protocol.OpStepResolve:
		stepID := strings.TrimSpace(opt.stepID)
		if stepID == "" {
			return nil, "", errors.New("step-id is required for step-resolve")
		}
		var participantID *string
		if trimmed := strings.TrimSpace(opt.participantID); trimmed != "" {
			participantID = &trimmed
		}
		raw, err := json.Marshal(protocol.StepResolvePayload{
			StepID:        stepID,
			ParticipantID: participantID,
		})
		return raw, strings.TrimSpace(opt.sessionID), err
	}
	return nil, "", fmt.Errorf("unsupported op: %s", op)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func parseOptionalJSON(raw, fieldName string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if !json.Valid([]byte(trimmed)) {
		return nil, fmt.Errorf("invalid %s", fieldName)
	}
	return json.RawMessage(trimmed), nil
}

func parseTimestamp(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	return parsed.UTC(), nil
}

func loadPrivateKey(raw string) (ed25519.PrivateKey, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		_, key, err := ed25519.GenerateKey(rand.Reader)
		return key, err
	}
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid private-key base64: %w", err)
	}
	switch len(decoded) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), nil
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	default:
		return nil, fmt.Errorf("invalid private-key length: %d (expected 32 or 64 bytes)", len(decoded))
	}
}

func autoID(prefix string, ts time.Time) string {
	return fmt.Sprintf("%s-%d", prefix, ts.UnixNano())
}
