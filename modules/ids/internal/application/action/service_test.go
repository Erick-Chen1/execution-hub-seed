package action

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domainAction "github.com/industrial-data-source/internal/domain/action"
	actionMocks "github.com/industrial-data-source/internal/domain/action/mocks"
	"github.com/industrial-data-source/internal/domain/rule"
	ruleMocks "github.com/industrial-data-source/internal/domain/rule/mocks"
)

func TestNewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	actionRepo := actionMocks.NewMockRepository(ctrl)
	ruleRepo := ruleMocks.NewMockRepository(ctrl)
	logger := zerolog.Nop()

	service := NewService(actionRepo, ruleRepo, logger)

	require.NotNil(t, service)
}

func TestService_CreateFromEvaluation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		r := rule.NewRule("Test Rule", rule.RuleTypeThreshold, json.RawMessage(`{}`), rule.ActionTypeNotify, json.RawMessage(`{"severity": "HIGH"}`))
		eval := rule.NewEvaluation(r, true, nil, nil)

		actionRepo.EXPECT().
			Create(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, action *domainAction.Action) error {
				assert.Equal(t, eval.RuleID, action.RuleID)
				assert.Equal(t, eval.EvaluationID, action.EvaluationID)
				assert.Equal(t, domainAction.TypeNotify, action.ActionType)
				assert.Equal(t, domainAction.PriorityHigh, action.Priority)
				return nil
			})

		actionRepo.EXPECT().
			RecordTransition(ctx, gomock.Any()).
			Return(nil)

		result, err := service.CreateFromEvaluation(ctx, eval, r)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, domainAction.StatusCreated, result.Status)
	})

	t.Run("with trace id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		r := rule.NewRule("Test Rule", rule.RuleTypeThreshold, json.RawMessage(`{}`), rule.ActionTypeNotify, json.RawMessage(`{}`))
		eval := rule.NewEvaluation(r, true, nil, nil)
		eval.SetTraceID("trace-123")

		actionRepo.EXPECT().
			Create(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, action *domainAction.Action) error {
				require.NotNil(t, action.TraceID)
				assert.Equal(t, "trace-123", *action.TraceID)
				return nil
			})

		actionRepo.EXPECT().
			RecordTransition(ctx, gomock.Any()).
			Return(nil)

		result, err := service.CreateFromEvaluation(ctx, eval, r)

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("with dedupe key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionConfig := json.RawMessage(`{"severity": "MEDIUM", "dedupe": {"key": "source_device", "cooldownSeconds": 300}}`)
		r := rule.NewRule("Test Rule", rule.RuleTypeThreshold, json.RawMessage(`{}`), rule.ActionTypeNotify, actionConfig)
		eval := rule.NewEvaluation(r, true, nil, nil)

		actionRepo.EXPECT().
			Create(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, action *domainAction.Action) error {
				require.NotNil(t, action.DedupeKey)
				assert.Equal(t, "source_device", *action.DedupeKey)
				return nil
			})

		actionRepo.EXPECT().
			RecordTransition(ctx, gomock.Any()).
			Return(nil)

		_, err := service.CreateFromEvaluation(ctx, eval, r)
		require.NoError(t, err)
	})

	t.Run("repository error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		r := rule.NewRule("Test Rule", rule.RuleTypeThreshold, json.RawMessage(`{}`), rule.ActionTypeNotify, json.RawMessage(`{}`))
		eval := rule.NewEvaluation(r, true, nil, nil)

		actionRepo.EXPECT().
			Create(ctx, gomock.Any()).
			Return(errors.New("database error"))

		result, err := service.CreateFromEvaluation(ctx, eval, r)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
		assert.Nil(t, result)
	})
}

func TestService_GetAction(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		expectedAction := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, nil)
		expectedAction.ActionID = actionID

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(expectedAction, nil)

		result, err := service.GetAction(ctx, actionID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, actionID, result.ActionID)
	})

	t.Run("not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(nil, nil)

		result, err := service.GetAction(ctx, actionID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "action not found")
		assert.Nil(t, result)
	})

	t.Run("repository error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(nil, errors.New("database error"))

		result, err := service.GetAction(ctx, actionID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
		assert.Nil(t, result)
	})
}

func TestService_ListActions(t *testing.T) {
	t.Run("success with filter", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		ruleID := uuid.New()
		status := domainAction.StatusCreated
		filter := domainAction.Filter{
			RuleID: &ruleID,
			Status: &status,
		}
		limit := 50
		offset := 0

		expectedActions := []*domainAction.Action{
			domainAction.NewAction(ruleID, 1, uuid.New(), domainAction.TypeNotify, nil),
			domainAction.NewAction(ruleID, 1, uuid.New(), domainAction.TypeNotify, nil),
		}

		actionRepo.EXPECT().
			List(ctx, filter, limit, offset).
			Return(expectedActions, nil)

		result, err := service.ListActions(ctx, filter, limit, offset)

		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

func TestService_AcknowledgeAction(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, nil)
		action.ActionID = actionID
		action.Status = domainAction.StatusDispatched

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		actionRepo.EXPECT().
			Update(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, a *domainAction.Action) error {
				assert.Equal(t, domainAction.StatusAcked, a.Status)
				require.NotNil(t, a.AckedBy)
				assert.Equal(t, "user@example.com", *a.AckedBy)
				return nil
			})

		actionRepo.EXPECT().
			RecordTransition(ctx, gomock.Any()).
			Return(nil)

		err := service.AcknowledgeAction(ctx, actionID, "user@example.com")

		require.NoError(t, err)
	})

	t.Run("action not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(nil, nil)

		err := service.AcknowledgeAction(ctx, actionID, "user@example.com")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "action not found")
	})

	t.Run("invalid state transition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, nil)
		action.ActionID = actionID
		action.Status = domainAction.StatusCreated // Can't acknowledge from CREATED

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		err := service.AcknowledgeAction(ctx, actionID, "user@example.com")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to acknowledge")
	})
}

func TestService_ResolveAction(t *testing.T) {
	t.Run("success from dispatched", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, nil)
		action.ActionID = actionID
		action.Status = domainAction.StatusDispatched

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		actionRepo.EXPECT().
			Update(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, a *domainAction.Action) error {
				assert.Equal(t, domainAction.StatusResolved, a.Status)
				return nil
			})

		actionRepo.EXPECT().
			RecordTransition(ctx, gomock.Any()).
			Return(nil)

		err := service.ResolveAction(ctx, actionID, "user@example.com")

		require.NoError(t, err)
	})

	t.Run("success from acked", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, nil)
		action.ActionID = actionID
		action.Status = domainAction.StatusAcked

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		actionRepo.EXPECT().
			Update(ctx, gomock.Any()).
			Return(nil)

		actionRepo.EXPECT().
			RecordTransition(ctx, gomock.Any()).
			Return(nil)

		err := service.ResolveAction(ctx, actionID, "user@example.com")

		require.NoError(t, err)
	})

	t.Run("action not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(nil, nil)

		err := service.ResolveAction(ctx, actionID, "user@example.com")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "action not found")
	})
}

func TestService_GetActionEvidence(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		ruleID := uuid.New()
		evaluationID := uuid.New()

		action := domainAction.NewAction(ruleID, 1, evaluationID, domainAction.TypeNotify, nil)
		action.ActionID = actionID

		r := rule.NewRule("Test Rule", rule.RuleTypeThreshold, json.RawMessage(`{}`), rule.ActionTypeNotify, json.RawMessage(`{}`))
		r.RuleID = ruleID

		eval := rule.NewEvaluation(r, true, nil, nil)
		eval.EvaluationID = evaluationID

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		ruleRepo.EXPECT().
			GetEvaluationByID(ctx, evaluationID).
			Return(eval, nil)

		ruleRepo.EXPECT().
			GetByRuleIDAndVersion(ctx, ruleID, 1).
			Return(r, nil)

		result, err := service.GetActionEvidence(ctx, actionID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, action, result.Action)
		assert.Equal(t, eval, result.Evaluation)
		assert.Equal(t, r, result.Rule)
	})

	t.Run("action not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(nil, nil)

		result, err := service.GetActionEvidence(ctx, actionID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "action not found")
		assert.Nil(t, result)
	})

	t.Run("evaluation not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		evaluationID := uuid.New()

		action := domainAction.NewAction(uuid.New(), 1, evaluationID, domainAction.TypeNotify, nil)
		action.ActionID = actionID

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		ruleRepo.EXPECT().
			GetEvaluationByID(ctx, evaluationID).
			Return(nil, errors.New("evaluation not found"))

		result, err := service.GetActionEvidence(ctx, actionID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "evaluation")
		assert.Nil(t, result)
	})
}

func TestService_GetActionTransitions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		fromStatus := domainAction.StatusCreated
		expectedTransitions := []*domainAction.StateTransition{
			domainAction.NewStateTransition(actionID, nil, domainAction.StatusCreated, nil),
			domainAction.NewStateTransition(actionID, &fromStatus, domainAction.StatusDispatching, nil),
		}

		actionRepo.EXPECT().
			GetTransitions(ctx, actionID).
			Return(expectedTransitions, nil)

		result, err := service.GetActionTransitions(ctx, actionID)

		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

func TestService_DispatchAction(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, nil)
		action.ActionID = actionID
		action.Status = domainAction.StatusCreated

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		actionRepo.EXPECT().
			Update(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, a *domainAction.Action) error {
				assert.Equal(t, domainAction.StatusDispatching, a.Status)
				return nil
			})

		actionRepo.EXPECT().
			RecordTransition(ctx, gomock.Any()).
			Return(nil)

		err := service.DispatchAction(ctx, actionID)

		require.NoError(t, err)
	})
}

func TestService_ConfirmDispatched(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, nil)
		action.ActionID = actionID
		action.Status = domainAction.StatusDispatching

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		actionRepo.EXPECT().
			Update(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, a *domainAction.Action) error {
				assert.Equal(t, domainAction.StatusDispatched, a.Status)
				return nil
			})

		actionRepo.EXPECT().
			RecordTransition(ctx, gomock.Any()).
			Return(nil)

		err := service.ConfirmDispatched(ctx, actionID)

		require.NoError(t, err)
	})
}

func TestService_FailAction(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		logger := zerolog.Nop()
		service := NewService(actionRepo, ruleRepo, logger)

		ctx := context.Background()
		actionID := uuid.New()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, nil)
		action.ActionID = actionID
		action.Status = domainAction.StatusCreated

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		actionRepo.EXPECT().
			Update(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, a *domainAction.Action) error {
				assert.Equal(t, domainAction.StatusFailed, a.Status)
				require.NotNil(t, a.LastError)
				assert.Equal(t, "connection timeout", *a.LastError)
				return nil
			})

		actionRepo.EXPECT().
			RecordTransition(ctx, gomock.Any()).
			Return(nil)

		err := service.FailAction(ctx, actionID, "connection timeout")

		require.NoError(t, err)
	})
}
