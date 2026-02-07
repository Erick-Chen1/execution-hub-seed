package rule

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/mock_repository.go -package=mocks . Repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines the interface for rule persistence
type Repository interface {
	// Rule operations
	Create(ctx context.Context, rule *Rule) error
	GetByRuleID(ctx context.Context, ruleID uuid.UUID) (*Rule, error)
	GetByRuleIDAndVersion(ctx context.Context, ruleID uuid.UUID, version int) (*Rule, error)
	GetByID(ctx context.Context, id int64) (*Rule, error)
	ListActiveRules(ctx context.Context, filter Filter) ([]*Rule, error)
	ListVersions(ctx context.Context, ruleID uuid.UUID) ([]*Rule, error)
	UpdateStatus(ctx context.Context, id int64, status RuleStatus, updatedBy *string) error

	// Evaluation operations
	CreateEvaluation(ctx context.Context, eval *Evaluation) error
	GetEvaluationByID(ctx context.Context, evaluationID uuid.UUID) (*Evaluation, error)
	ListEvaluationsByRule(ctx context.Context, ruleID uuid.UUID, limit int) ([]*Evaluation, error)
	ListMatchedEvaluations(ctx context.Context, since time.Time, limit int) ([]*Evaluation, error)
}
