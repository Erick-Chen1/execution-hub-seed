package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/collab"
)

// CollabRepository implements collab.Repository.
type CollabRepository struct {
	pool *pgxpool.Pool
}

func NewCollabRepository(pool *pgxpool.Pool) *CollabRepository {
	return &CollabRepository{pool: pool}
}

func (r *CollabRepository) CreateSession(ctx context.Context, session *collab.Session) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collab_sessions
		(session_id, task_id, workflow_id, workflow_version, name, status, context, trace_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, session.SessionID, session.TaskID, session.WorkflowID, session.WorkflowVersion, session.Name, session.Status, session.Context, session.TraceID, session.CreatedAt, session.UpdatedAt)
	return err
}

func (r *CollabRepository) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*collab.Session, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, session_id, task_id, workflow_id, workflow_version, name, status, context, trace_id, created_at, updated_at
		FROM collab_sessions
		WHERE session_id=$1
	`, sessionID)
	return scanCollabSession(row)
}

func (r *CollabRepository) UpdateSessionStatus(ctx context.Context, sessionID uuid.UUID, status collab.SessionStatus, updatedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE collab_sessions
		SET status=$1, updated_at=$2
		WHERE session_id=$3
	`, status, updatedAt, sessionID)
	return err
}

func (r *CollabRepository) CreateParticipant(ctx context.Context, participant *collab.Participant) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collab_participants
		(participant_id, session_id, type, ref, capabilities, trust_score, joined_at, last_seen_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, participant.ParticipantID, participant.SessionID, participant.Type, participant.Ref, participant.Capabilities, participant.TrustScore, participant.JoinedAt, participant.LastSeenAt)
	return err
}

func (r *CollabRepository) GetParticipantByID(ctx context.Context, participantID uuid.UUID) (*collab.Participant, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, participant_id, session_id, type, ref, capabilities, trust_score, joined_at, last_seen_at
		FROM collab_participants
		WHERE participant_id=$1
	`, participantID)
	return scanCollabParticipant(row)
}

func (r *CollabRepository) GetParticipantByRef(ctx context.Context, sessionID uuid.UUID, ref string) (*collab.Participant, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, participant_id, session_id, type, ref, capabilities, trust_score, joined_at, last_seen_at
		FROM collab_participants
		WHERE session_id=$1 AND ref=$2
	`, sessionID, ref)
	return scanCollabParticipant(row)
}

func (r *CollabRepository) TouchParticipant(ctx context.Context, participantID uuid.UUID, seenAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE collab_participants
		SET last_seen_at=$1
		WHERE participant_id=$2
	`, seenAt, participantID)
	return err
}

func (r *CollabRepository) ListParticipants(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]*collab.Participant, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, participant_id, session_id, type, ref, capabilities, trust_score, joined_at, last_seen_at
		FROM collab_participants
		WHERE session_id=$1
		ORDER BY joined_at ASC
		LIMIT $2 OFFSET $3
	`, sessionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*collab.Participant
	for rows.Next() {
		p, err := scanCollabParticipant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *CollabRepository) CreateStep(ctx context.Context, step *collab.Step) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collab_steps
		(step_id, session_id, step_key, name, status, required_capabilities, depends_on, lease_ttl_seconds, consensus_policy, created_at, updated_at, resolved_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, step.StepID, step.SessionID, step.StepKey, step.Name, step.Status, step.RequiredCapabilities, step.DependsOn, step.LeaseTTLSeconds, step.ConsensusPolicy, step.CreatedAt, step.UpdatedAt, step.ResolvedAt)
	return err
}

func (r *CollabRepository) GetStepByID(ctx context.Context, stepID uuid.UUID) (*collab.Step, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, step_id, session_id, step_key, name, status, required_capabilities, depends_on, lease_ttl_seconds, consensus_policy, created_at, updated_at, resolved_at
		FROM collab_steps
		WHERE step_id=$1
	`, stepID)
	return scanCollabStep(row)
}

func (r *CollabRepository) ListStepsBySession(ctx context.Context, sessionID uuid.UUID) ([]*collab.Step, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, step_id, session_id, step_key, name, status, required_capabilities, depends_on, lease_ttl_seconds, consensus_policy, created_at, updated_at, resolved_at
		FROM collab_steps
		WHERE session_id=$1
		ORDER BY created_at ASC, step_key ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*collab.Step
	for rows.Next() {
		s, err := scanCollabStep(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *CollabRepository) UpdateStep(ctx context.Context, step *collab.Step) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE collab_steps
		SET status=$1, required_capabilities=$2, depends_on=$3, lease_ttl_seconds=$4, consensus_policy=$5, updated_at=$6, resolved_at=$7
		WHERE step_id=$8
	`, step.Status, step.RequiredCapabilities, step.DependsOn, step.LeaseTTLSeconds, step.ConsensusPolicy, step.UpdatedAt, step.ResolvedAt, step.StepID)
	return err
}

func (r *CollabRepository) CountUnresolvedSteps(ctx context.Context, sessionID uuid.UUID) (int, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM collab_steps
		WHERE session_id=$1 AND status <> 'RESOLVED'
	`, sessionID)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *CollabRepository) CreateClaim(ctx context.Context, claim *collab.Claim) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collab_claims
		(claim_id, step_id, participant_id, status, lease_until, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, claim.ClaimID, claim.StepID, claim.ParticipantID, claim.Status, claim.LeaseUntil, claim.CreatedAt, claim.UpdatedAt)
	return err
}

func (r *CollabRepository) GetActiveClaimByStep(ctx context.Context, stepID uuid.UUID, now time.Time) (*collab.Claim, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, claim_id, step_id, participant_id, status, lease_until, created_at, updated_at
		FROM collab_claims
		WHERE step_id=$1 AND status='ACTIVE' AND lease_until > $2
		ORDER BY created_at DESC
		LIMIT 1
	`, stepID, now)
	return scanCollabClaim(row)
}

func (r *CollabRepository) GetActiveClaimByStepAndParticipant(ctx context.Context, stepID uuid.UUID, participantID uuid.UUID, now time.Time) (*collab.Claim, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, claim_id, step_id, participant_id, status, lease_until, created_at, updated_at
		FROM collab_claims
		WHERE step_id=$1 AND participant_id=$2 AND status='ACTIVE' AND lease_until > $3
		ORDER BY created_at DESC
		LIMIT 1
	`, stepID, participantID, now)
	return scanCollabClaim(row)
}

func (r *CollabRepository) UpdateClaimStatus(ctx context.Context, claimID uuid.UUID, status collab.ClaimStatus, updatedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE collab_claims
		SET status=$1, updated_at=$2
		WHERE claim_id=$3
	`, status, updatedAt, claimID)
	return err
}

func (r *CollabRepository) ListExpiredActiveClaims(ctx context.Context, now time.Time, limit int) ([]*collab.Claim, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, claim_id, step_id, participant_id, status, lease_until, created_at, updated_at
		FROM collab_claims
		WHERE status='ACTIVE' AND lease_until <= $1
		ORDER BY lease_until ASC
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*collab.Claim
	for rows.Next() {
		c, err := scanCollabClaim(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CollabRepository) CreateArtifact(ctx context.Context, artifact *collab.Artifact) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collab_artifacts
		(artifact_id, step_id, producer_id, kind, content, version, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, artifact.ArtifactID, artifact.StepID, artifact.ProducerID, artifact.Kind, artifact.Content, artifact.Version, artifact.CreatedAt)
	return err
}

func (r *CollabRepository) GetLatestArtifactVersion(ctx context.Context, stepID uuid.UUID) (int, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0)
		FROM collab_artifacts
		WHERE step_id=$1
	`, stepID)
	var version int
	if err := row.Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func (r *CollabRepository) ListArtifactsByStep(ctx context.Context, stepID uuid.UUID) ([]*collab.Artifact, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, artifact_id, step_id, producer_id, kind, content, version, created_at
		FROM collab_artifacts
		WHERE step_id=$1
		ORDER BY version ASC
	`, stepID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*collab.Artifact
	for rows.Next() {
		a, err := scanCollabArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *CollabRepository) CreateDecision(ctx context.Context, decision *collab.Decision) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collab_decisions
		(decision_id, step_id, policy, deadline, status, result, created_at, updated_at, decided_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, decision.DecisionID, decision.StepID, decision.Policy, decision.Deadline, decision.Status, decision.Result, decision.CreatedAt, decision.UpdatedAt, decision.DecidedAt)
	return err
}

func (r *CollabRepository) GetDecisionByID(ctx context.Context, decisionID uuid.UUID) (*collab.Decision, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, decision_id, step_id, policy, deadline, status, result, created_at, updated_at, decided_at
		FROM collab_decisions
		WHERE decision_id=$1
	`, decisionID)
	return scanCollabDecision(row)
}

func (r *CollabRepository) GetLatestDecisionByStep(ctx context.Context, stepID uuid.UUID) (*collab.Decision, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, decision_id, step_id, policy, deadline, status, result, created_at, updated_at, decided_at
		FROM collab_decisions
		WHERE step_id=$1
		ORDER BY created_at DESC
		LIMIT 1
	`, stepID)
	return scanCollabDecision(row)
}

func (r *CollabRepository) UpdateDecision(ctx context.Context, decision *collab.Decision) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE collab_decisions
		SET policy=$1, deadline=$2, status=$3, result=$4, updated_at=$5, decided_at=$6
		WHERE decision_id=$7
	`, decision.Policy, decision.Deadline, decision.Status, decision.Result, decision.UpdatedAt, decision.DecidedAt, decision.DecisionID)
	return err
}

func (r *CollabRepository) CreateVote(ctx context.Context, vote *collab.Vote) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collab_votes
		(vote_id, decision_id, participant_id, choice, comment, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (decision_id, participant_id) DO UPDATE
		SET choice=EXCLUDED.choice, comment=EXCLUDED.comment, created_at=EXCLUDED.created_at
	`, vote.VoteID, vote.DecisionID, vote.ParticipantID, vote.Choice, vote.Comment, vote.CreatedAt)
	return err
}

func (r *CollabRepository) ListVotesByDecision(ctx context.Context, decisionID uuid.UUID) ([]*collab.Vote, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, vote_id, decision_id, participant_id, choice, comment, created_at
		FROM collab_votes
		WHERE decision_id=$1
		ORDER BY created_at ASC
	`, decisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*collab.Vote
	for rows.Next() {
		v, err := scanCollabVote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *CollabRepository) CreateEvent(ctx context.Context, event *collab.Event) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collab_events
		(event_id, session_id, step_id, type, actor, payload, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, event.EventID, event.SessionID, event.StepID, event.Type, event.Actor, event.Payload, event.CreatedAt)
	return err
}

func (r *CollabRepository) ListEvents(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]*collab.Event, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, session_id, step_id, type, actor, payload, created_at
		FROM collab_events
		WHERE session_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, sessionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*collab.Event
	for rows.Next() {
		e, err := scanCollabEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanCollabSession(row pgx.Row) (*collab.Session, error) {
	var s collab.Session
	var contextData json.RawMessage
	if err := row.Scan(&s.ID, &s.SessionID, &s.TaskID, &s.WorkflowID, &s.WorkflowVersion, &s.Name, &s.Status, &contextData, &s.TraceID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(contextData) > 0 {
		s.Context = contextData
	}
	return &s, nil
}

func scanCollabParticipant(row pgx.Row) (*collab.Participant, error) {
	var p collab.Participant
	if err := row.Scan(&p.ID, &p.ParticipantID, &p.SessionID, &p.Type, &p.Ref, &p.Capabilities, &p.TrustScore, &p.JoinedAt, &p.LastSeenAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func scanCollabStep(row pgx.Row) (*collab.Step, error) {
	var s collab.Step
	var consensusPolicy json.RawMessage
	if err := row.Scan(&s.ID, &s.StepID, &s.SessionID, &s.StepKey, &s.Name, &s.Status, &s.RequiredCapabilities, &s.DependsOn, &s.LeaseTTLSeconds, &consensusPolicy, &s.CreatedAt, &s.UpdatedAt, &s.ResolvedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(consensusPolicy) > 0 {
		s.ConsensusPolicy = consensusPolicy
	}
	return &s, nil
}

func scanCollabClaim(row pgx.Row) (*collab.Claim, error) {
	var c collab.Claim
	if err := row.Scan(&c.ID, &c.ClaimID, &c.StepID, &c.ParticipantID, &c.Status, &c.LeaseUntil, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func scanCollabArtifact(row pgx.Row) (*collab.Artifact, error) {
	var a collab.Artifact
	var content json.RawMessage
	if err := row.Scan(&a.ID, &a.ArtifactID, &a.StepID, &a.ProducerID, &a.Kind, &content, &a.Version, &a.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(content) > 0 {
		a.Content = content
	}
	return &a, nil
}

func scanCollabDecision(row pgx.Row) (*collab.Decision, error) {
	var d collab.Decision
	var policy json.RawMessage
	if err := row.Scan(&d.ID, &d.DecisionID, &d.StepID, &policy, &d.Deadline, &d.Status, &d.Result, &d.CreatedAt, &d.UpdatedAt, &d.DecidedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(policy) > 0 {
		d.Policy = policy
	}
	return &d, nil
}

func scanCollabVote(row pgx.Row) (*collab.Vote, error) {
	var v collab.Vote
	if err := row.Scan(&v.ID, &v.VoteID, &v.DecisionID, &v.ParticipantID, &v.Choice, &v.Comment, &v.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

func scanCollabEvent(row pgx.Row) (*collab.Event, error) {
	var e collab.Event
	var payload json.RawMessage
	if err := row.Scan(&e.ID, &e.EventID, &e.SessionID, &e.StepID, &e.Type, &e.Actor, &payload, &e.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(payload) > 0 {
		e.Payload = payload
	}
	return &e, nil
}
