package notification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domainAction "github.com/industrial-data-source/internal/domain/action"
	actionMocks "github.com/industrial-data-source/internal/domain/action/mocks"
	"github.com/industrial-data-source/internal/domain/notification"
	notificationMocks "github.com/industrial-data-source/internal/domain/notification/mocks"
	"github.com/industrial-data-source/internal/domain/rule"
	ruleMocks "github.com/industrial-data-source/internal/domain/rule/mocks"
)

func TestNewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notificationRepo := notificationMocks.NewMockRepository(ctrl)
	actionRepo := actionMocks.NewMockRepository(ctrl)
	ruleRepo := ruleMocks.NewMockRepository(ctrl)
	sseHub := notificationMocks.NewMockSSEHub(ctrl)
	logger := zerolog.Nop()

	service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

	require.NotNil(t, service)
}

func TestService_CreateFromAction(t *testing.T) {
	t.Run("success with defaults", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		actionConfig := json.RawMessage(`{}`)
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, actionConfig)
		action.Priority = domainAction.PriorityHigh

		notificationRepo.EXPECT().
			Create(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, n *notification.Notification) error {
				assert.Equal(t, action.ActionID, n.ActionID)
				assert.Equal(t, notification.ChannelSSE, n.Channel)
				assert.Equal(t, "Alert Notification", n.Title)
				assert.Equal(t, "An alert has been triggered.", n.Body)
				return nil
			})

		result, err := service.CreateFromAction(ctx, action)

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("success with custom config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		actionConfig := json.RawMessage(`{
			"title": "Custom Title",
			"body": "Custom Body",
			"channel": "WEBHOOK",
			"userId": "user123",
			"group": "admins",
			"ttlHours": 24
		}`)
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, actionConfig)

		notificationRepo.EXPECT().
			Create(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, n *notification.Notification) error {
				assert.Equal(t, "Custom Title", n.Title)
				assert.Equal(t, "Custom Body", n.Body)
				assert.Equal(t, notification.ChannelWebhook, n.Channel)
				require.NotNil(t, n.TargetUserID)
				assert.Equal(t, "user123", *n.TargetUserID)
				require.NotNil(t, n.TargetGroup)
				assert.Equal(t, "admins", *n.TargetGroup)
				require.NotNil(t, n.ExpiresAt)
				return nil
			})

		result, err := service.CreateFromAction(ctx, action)

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("with trace id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, json.RawMessage(`{}`))
		action.SetTraceID("trace-abc")

		notificationRepo.EXPECT().
			Create(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, n *notification.Notification) error {
				require.NotNil(t, n.TraceID)
				assert.Equal(t, "trace-abc", *n.TraceID)
				return nil
			})

		_, err := service.CreateFromAction(ctx, action)
		require.NoError(t, err)
	})

	t.Run("repository error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, json.RawMessage(`{}`))

		notificationRepo.EXPECT().
			Create(ctx, gomock.Any()).
			Return(errors.New("database error"))

		result, err := service.CreateFromAction(ctx, action)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
		assert.Nil(t, result)
	})

	t.Run("invalid action config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		action := domainAction.NewAction(uuid.New(), 1, uuid.New(), domainAction.TypeNotify, json.RawMessage(`{invalid}`))

		result, err := service.CreateFromAction(ctx, action)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse action config")
		assert.Nil(t, result)
	})
}

func TestService_SendNotification(t *testing.T) {
	t.Run("success via SSE to user", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()
		userID := "user123"
		n := notification.NewNotification(uuid.New(), notification.ChannelSSE, notification.PriorityMedium, "Title", "Body", nil)
		n.NotificationID = notificationID
		n.SetTarget(&userID, nil)

		notificationRepo.EXPECT().
			GetByID(ctx, notificationID).
			Return(n, nil)

		notificationRepo.EXPECT().
			Update(ctx, gomock.Any()).
			Return(nil).Times(2) // Once for MarkSent, once for MarkDelivered

		sseHub.EXPECT().
			BroadcastToUser("user123", gomock.Any())

		notificationRepo.EXPECT().
			RecordAttempt(ctx, gomock.Any()).
			Return(nil)

		err := service.SendNotification(ctx, notificationID)

		require.NoError(t, err)
	})

	t.Run("success via SSE to group", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()
		group := "admins"
		n := notification.NewNotification(uuid.New(), notification.ChannelSSE, notification.PriorityMedium, "Title", "Body", nil)
		n.NotificationID = notificationID
		n.SetTarget(nil, &group)

		notificationRepo.EXPECT().
			GetByID(ctx, notificationID).
			Return(n, nil)

		notificationRepo.EXPECT().
			Update(ctx, gomock.Any()).
			Return(nil).Times(2)

		sseHub.EXPECT().
			BroadcastToGroup("admins", gomock.Any())

		notificationRepo.EXPECT().
			RecordAttempt(ctx, gomock.Any()).
			Return(nil)

		err := service.SendNotification(ctx, notificationID)

		require.NoError(t, err)
	})

	t.Run("success via SSE to all", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()
		n := notification.NewNotification(uuid.New(), notification.ChannelSSE, notification.PriorityMedium, "Title", "Body", nil)
		n.NotificationID = notificationID

		notificationRepo.EXPECT().
			GetByID(ctx, notificationID).
			Return(n, nil)

		notificationRepo.EXPECT().
			Update(ctx, gomock.Any()).
			Return(nil).Times(2)

		sseHub.EXPECT().
			BroadcastToAll(gomock.Any())

		notificationRepo.EXPECT().
			RecordAttempt(ctx, gomock.Any()).
			Return(nil)

		err := service.SendNotification(ctx, notificationID)

		require.NoError(t, err)
	})

	t.Run("notification not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()

		notificationRepo.EXPECT().
			GetByID(ctx, notificationID).
			Return(nil, nil)

		err := service.SendNotification(ctx, notificationID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "notification not found")
	})

	t.Run("notification expired", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()
		n := notification.NewNotification(uuid.New(), notification.ChannelSSE, notification.PriorityMedium, "Title", "Body", nil)
		n.NotificationID = notificationID
		pastTime := time.Now().Add(-1 * time.Hour)
		n.SetExpiry(pastTime)

		notificationRepo.EXPECT().
			GetByID(ctx, notificationID).
			Return(n, nil)

		notificationRepo.EXPECT().
			Update(ctx, gomock.Any()).
			Return(nil)

		err := service.SendNotification(ctx, notificationID)

		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrExpired)
	})
}

func TestService_GetNotification(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()
		expectedNotification := notification.NewNotification(uuid.New(), notification.ChannelSSE, notification.PriorityMedium, "Title", "Body", nil)
		expectedNotification.NotificationID = notificationID

		notificationRepo.EXPECT().
			GetByID(ctx, notificationID).
			Return(expectedNotification, nil)

		result, err := service.GetNotification(ctx, notificationID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, notificationID, result.NotificationID)
	})

	t.Run("not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()

		notificationRepo.EXPECT().
			GetByID(ctx, notificationID).
			Return(nil, nil)

		result, err := service.GetNotification(ctx, notificationID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "notification not found")
		assert.Nil(t, result)
	})
}

func TestService_ListNotifications(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		filter := notification.Filter{}
		limit := 50
		offset := 0

		expectedNotifications := []*notification.Notification{
			notification.NewNotification(uuid.New(), notification.ChannelSSE, notification.PriorityMedium, "Title 1", "Body 1", nil),
			notification.NewNotification(uuid.New(), notification.ChannelSSE, notification.PriorityHigh, "Title 2", "Body 2", nil),
		}

		notificationRepo.EXPECT().
			List(ctx, filter, limit, offset).
			Return(expectedNotifications, nil)

		result, err := service.ListNotifications(ctx, filter, limit, offset)

		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

func TestService_GetNotificationsByAction(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		actionID := uuid.New()

		expectedNotifications := []*notification.Notification{
			notification.NewNotification(actionID, notification.ChannelSSE, notification.PriorityMedium, "Title", "Body", nil),
		}

		notificationRepo.EXPECT().
			GetByActionID(ctx, actionID).
			Return(expectedNotifications, nil)

		result, err := service.GetNotificationsByAction(ctx, actionID)

		require.NoError(t, err)
		assert.Len(t, result, 1)
	})
}

func TestService_GetDeliveryAttempts(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()

		expectedAttempts := []*notification.DeliveryAttempt{
			notification.NewDeliveryAttempt(notificationID, 1),
			notification.NewDeliveryAttempt(notificationID, 2),
		}

		notificationRepo.EXPECT().
			GetAttempts(ctx, notificationID).
			Return(expectedAttempts, nil)

		result, err := service.GetDeliveryAttempts(ctx, notificationID)

		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

func TestService_ProcessPendingNotifications(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		limit := 10

		n1 := notification.NewNotification(uuid.New(), notification.ChannelSSE, notification.PriorityMedium, "Title 1", "Body 1", nil)
		n2 := notification.NewNotification(uuid.New(), notification.ChannelSSE, notification.PriorityMedium, "Title 2", "Body 2", nil)

		notificationRepo.EXPECT().
			ListPendingNotifications(ctx, limit).
			Return([]*notification.Notification{n1, n2}, nil)

		// For each notification, expect GetByID, Update (x2), BroadcastToAll, RecordAttempt
		notificationRepo.EXPECT().
			GetByID(ctx, n1.NotificationID).
			Return(n1, nil)
		notificationRepo.EXPECT().
			Update(ctx, gomock.Any()).
			Return(nil).Times(2)
		sseHub.EXPECT().
			BroadcastToAll(gomock.Any())
		notificationRepo.EXPECT().
			RecordAttempt(ctx, gomock.Any()).
			Return(nil)

		notificationRepo.EXPECT().
			GetByID(ctx, n2.NotificationID).
			Return(n2, nil)
		notificationRepo.EXPECT().
			Update(ctx, gomock.Any()).
			Return(nil).Times(2)
		sseHub.EXPECT().
			BroadcastToAll(gomock.Any())
		notificationRepo.EXPECT().
			RecordAttempt(ctx, gomock.Any()).
			Return(nil)

		processed, err := service.ProcessPendingNotifications(ctx, limit)

		require.NoError(t, err)
		assert.Equal(t, 2, processed)
	})
}

func TestService_ExpireNotifications(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()

		notificationRepo.EXPECT().
			ExpireNotifications(ctx).
			Return(int64(5), nil)

		count, err := service.ExpireNotifications(ctx)

		require.NoError(t, err)
		assert.Equal(t, int64(5), count)
	})
}

func TestService_GetSSEClientCount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		sseHub.EXPECT().
			GetClientCount().
			Return(10)

		count := service.GetSSEClientCount()

		assert.Equal(t, 10, count)
	})
}

func TestService_GetNotificationEvidence(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()
		actionID := uuid.New()
		ruleID := uuid.New()
		evaluationID := uuid.New()

		n := notification.NewNotification(actionID, notification.ChannelSSE, notification.PriorityMedium, "Title", "Body", nil)
		n.NotificationID = notificationID

		action := domainAction.NewAction(ruleID, 1, evaluationID, domainAction.TypeNotify, nil)
		action.ActionID = actionID

		r := rule.NewRule("Test Rule", rule.RuleTypeThreshold, json.RawMessage(`{}`), rule.ActionTypeNotify, json.RawMessage(`{}`))
		r.RuleID = ruleID

		eval := rule.NewEvaluation(r, true, nil, nil)
		eval.EvaluationID = evaluationID

		notificationRepo.EXPECT().
			GetByID(ctx, notificationID).
			Return(n, nil)

		actionRepo.EXPECT().
			GetByID(ctx, actionID).
			Return(action, nil)

		ruleRepo.EXPECT().
			GetEvaluationByID(ctx, evaluationID).
			Return(eval, nil)

		ruleRepo.EXPECT().
			GetByRuleIDAndVersion(ctx, ruleID, 1).
			Return(r, nil)

		result, err := service.GetNotificationEvidence(ctx, notificationID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, n, result.Notification)
		assert.Equal(t, action, result.Action)
		assert.Equal(t, eval, result.Evaluation)
		assert.Equal(t, r, result.Rule)
	})

	t.Run("notification not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		notificationRepo := notificationMocks.NewMockRepository(ctrl)
		actionRepo := actionMocks.NewMockRepository(ctrl)
		ruleRepo := ruleMocks.NewMockRepository(ctrl)
		sseHub := notificationMocks.NewMockSSEHub(ctrl)
		logger := zerolog.Nop()
		service := NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)

		ctx := context.Background()
		notificationID := uuid.New()

		notificationRepo.EXPECT().
			GetByID(ctx, notificationID).
			Return(nil, nil)

		result, err := service.GetNotificationEvidence(ctx, notificationID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "notification not found")
		assert.Nil(t, result)
	})
}
