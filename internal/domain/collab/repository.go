package collab

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines persistence for collaboration runtime.
type Repository interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*Session, error)
	UpdateSessionStatus(ctx context.Context, sessionID uuid.UUID, status SessionStatus, updatedAt time.Time) error

	CreateParticipant(ctx context.Context, participant *Participant) error
	GetParticipantByID(ctx context.Context, participantID uuid.UUID) (*Participant, error)
	GetParticipantByRef(ctx context.Context, sessionID uuid.UUID, ref string) (*Participant, error)
	TouchParticipant(ctx context.Context, participantID uuid.UUID, seenAt time.Time) error
	ListParticipants(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]*Participant, error)

	CreateStep(ctx context.Context, step *Step) error
	GetStepByID(ctx context.Context, stepID uuid.UUID) (*Step, error)
	ListStepsBySession(ctx context.Context, sessionID uuid.UUID) ([]*Step, error)
	UpdateStep(ctx context.Context, step *Step) error
	CountUnresolvedSteps(ctx context.Context, sessionID uuid.UUID) (int, error)

	CreateClaim(ctx context.Context, claim *Claim) error
	GetActiveClaimByStep(ctx context.Context, stepID uuid.UUID, now time.Time) (*Claim, error)
	GetActiveClaimByStepAndParticipant(ctx context.Context, stepID uuid.UUID, participantID uuid.UUID, now time.Time) (*Claim, error)
	UpdateClaimStatus(ctx context.Context, claimID uuid.UUID, status ClaimStatus, updatedAt time.Time) error
	ListExpiredActiveClaims(ctx context.Context, now time.Time, limit int) ([]*Claim, error)

	CreateArtifact(ctx context.Context, artifact *Artifact) error
	GetLatestArtifactVersion(ctx context.Context, stepID uuid.UUID) (int, error)
	ListArtifactsByStep(ctx context.Context, stepID uuid.UUID) ([]*Artifact, error)

	CreateDecision(ctx context.Context, decision *Decision) error
	GetDecisionByID(ctx context.Context, decisionID uuid.UUID) (*Decision, error)
	GetLatestDecisionByStep(ctx context.Context, stepID uuid.UUID) (*Decision, error)
	UpdateDecision(ctx context.Context, decision *Decision) error

	CreateVote(ctx context.Context, vote *Vote) error
	ListVotesByDecision(ctx context.Context, decisionID uuid.UUID) ([]*Vote, error)

	CreateEvent(ctx context.Context, event *Event) error
	ListEvents(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]*Event, error)
}
