package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	domainAction "github.com/industrial-data-source/internal/domain/action"
	"github.com/industrial-data-source/internal/domain/notification"
	"github.com/industrial-data-source/internal/domain/rule"
)

// Service handles notification operations
type Service struct {
	notificationRepo notification.Repository
	actionRepo       domainAction.Repository
	ruleRepo         rule.Repository
	sseHub           notification.SSEHub
	logger           zerolog.Logger
}

// NewService creates a new notification service
func NewService(
	notificationRepo notification.Repository,
	actionRepo domainAction.Repository,
	ruleRepo rule.Repository,
	sseHub notification.SSEHub,
	logger zerolog.Logger,
) *Service {
	return &Service{
		notificationRepo: notificationRepo,
		actionRepo:       actionRepo,
		ruleRepo:         ruleRepo,
		sseHub:           sseHub,
		logger:           logger.With().Str("service", "notification").Logger(),
	}
}

// CreateFromAction creates a notification from an action
func (s *Service) CreateFromAction(ctx context.Context, action *domainAction.Action) (*notification.Notification, error) {
	// Parse action config for notification settings
	var actionCfg struct {
		Title    string  `json:"title"`
		Body     string  `json:"body"`
		Channel  string  `json:"channel"`
		UserID   *string `json:"userId,omitempty"`
		Group    *string `json:"group,omitempty"`
		TTLHours *int    `json:"ttlHours,omitempty"`
	}
	if err := json.Unmarshal(action.ActionConfig, &actionCfg); err != nil {
		return nil, fmt.Errorf("failed to parse action config: %w", err)
	}

	// Set defaults
	channel := notification.ChannelSSE
	if actionCfg.Channel != "" {
		channel = notification.Channel(actionCfg.Channel)
	}

	title := actionCfg.Title
	if title == "" {
		title = "Alert Notification"
	}

	body := actionCfg.Body
	if body == "" {
		body = "An alert has been triggered."
	}

	// Map action priority to notification priority
	priority := notification.Priority(action.Priority)

	// Build notification payload
	payload, err := json.Marshal(map[string]interface{}{
		"actionId":     action.ActionID.String(),
		"ruleId":       action.RuleID.String(),
		"evaluationId": action.EvaluationID.String(),
		"actionType":   string(action.ActionType),
		"priority":     string(action.Priority),
		"createdAt":    action.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		s.logger.Warn().Err(err).Str("action_id", action.ActionID.String()).Msg("failed to marshal notification payload, using empty")
		payload = []byte("{}")
	}

	// Create notification
	n := notification.NewNotification(
		action.ActionID,
		channel,
		priority,
		title,
		body,
		payload,
	)

	// Set target
	n.SetTarget(actionCfg.UserID, actionCfg.Group)

	// Set expiry if TTL specified
	if actionCfg.TTLHours != nil && *actionCfg.TTLHours > 0 {
		n.SetExpiry(time.Now().UTC().Add(time.Duration(*actionCfg.TTLHours) * time.Hour))
	}

	// Set trace ID from action
	if action.TraceID != nil {
		n.SetTraceID(*action.TraceID)
	}

	// Save notification
	if err := s.notificationRepo.Create(ctx, n); err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	s.logger.Info().
		Str("notification_id", n.NotificationID.String()).
		Str("action_id", action.ActionID.String()).
		Str("channel", string(n.Channel)).
		Str("priority", string(n.Priority)).
		Msg("notification created")

	return n, nil
}

// SendNotification sends a notification through its channel
func (s *Service) SendNotification(ctx context.Context, notificationID uuid.UUID) error {
	n, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return fmt.Errorf("failed to get notification: %w", err)
	}
	if n == nil {
		return fmt.Errorf("notification not found: %s", notificationID)
	}

	// Record attempt start
	attempt := notification.NewDeliveryAttempt(notificationID, n.RetryCount+1)
	startTime := time.Now()

	// Check if expired
	if n.IsExpired() {
		n.MarkExpired()
		if err := s.notificationRepo.Update(ctx, n); err != nil {
			s.logger.Warn().
				Str("notification_id", n.NotificationID.String()).
				Err(err).
				Msg("failed to persist expired status")
		}
		return notification.ErrExpired
	}

	// Mark as sent - this must be persisted before actual sending
	if err := n.MarkSent(); err != nil {
		return fmt.Errorf("failed to mark notification as sent: %w", err)
	}
	if err := s.notificationRepo.Update(ctx, n); err != nil {
		return fmt.Errorf("failed to persist sent status: %w", err)
	}

	// Send based on channel
	var sendErr error
	switch n.Channel {
	case notification.ChannelSSE:
		sendErr = s.sendViaSSE(n)
	case notification.ChannelWebhook:
		sendErr = s.sendViaWebhook(ctx, n)
	default:
		sendErr = fmt.Errorf("unsupported channel: %s", n.Channel)
	}

	// Record attempt result
	attempt.DurationMs = int(time.Since(startTime).Milliseconds())

	if sendErr != nil {
		attempt.Status = notification.StatusFailed
		errMsg := sendErr.Error()
		attempt.ErrorMessage = &errMsg
		n.MarkFailed(errMsg)

		s.logger.Warn().
			Str("notification_id", n.NotificationID.String()).
			Err(sendErr).
			Int("retry_count", n.RetryCount).
			Msg("notification send failed")
	} else {
		attempt.Status = notification.StatusDelivered
		n.MarkDelivered()

		s.logger.Info().
			Str("notification_id", n.NotificationID.String()).
			Str("channel", string(n.Channel)).
			Int("duration_ms", attempt.DurationMs).
			Msg("notification delivered")
	}

	// Save updated notification and attempt - track persistence errors
	var persistErr error
	if err := s.notificationRepo.Update(ctx, n); err != nil {
		s.logger.Error().
			Str("notification_id", n.NotificationID.String()).
			Err(err).
			Msg("failed to persist final notification state")
		persistErr = err
	}
	if err := s.notificationRepo.RecordAttempt(ctx, attempt); err != nil {
		s.logger.Warn().
			Str("notification_id", n.NotificationID.String()).
			Err(err).
			Msg("failed to record delivery attempt, saving to DLQ")
		// Save to DLQ for later retry
		if dlqErr := s.notificationRepo.SaveAttemptToDLQ(ctx, attempt, err); dlqErr != nil {
			s.logger.Error().
				Str("notification_id", n.NotificationID.String()).
				Err(dlqErr).
				Msg("CRITICAL: failed to save delivery attempt to DLQ")
		}
		persistErr = errors.Join(persistErr, err)
	}

	// Return combined errors: send error takes precedence, but include persistence errors
	if sendErr != nil {
		return errors.Join(sendErr, persistErr)
	}
	return persistErr
}

// sendViaSSE sends notification via Server-Sent Events
func (s *Service) sendViaSSE(n *notification.Notification) error {
	// Build SSE message
	msg := notification.NewSSEMessage("notification", n.Payload)

	// Route based on target
	if n.TargetUserID != nil {
		s.sseHub.BroadcastToUser(*n.TargetUserID, msg)
	} else if n.TargetGroup != nil {
		s.sseHub.BroadcastToGroup(*n.TargetGroup, msg)
	} else {
		s.sseHub.BroadcastToAll(msg)
	}

	return nil
}

// sendViaWebhook sends notification via webhook
func (s *Service) sendViaWebhook(ctx context.Context, n *notification.Notification) error {
	// Get the action to retrieve webhook configuration
	action, err := s.actionRepo.GetByID(ctx, n.ActionID)
	if err != nil {
		return fmt.Errorf("failed to get action for webhook: %w", err)
	}
	if action == nil {
		return fmt.Errorf("action not found: %s", n.ActionID)
	}

	// Parse webhook configuration from action config
	var webhookConfig struct {
		WebhookURL string            `json:"webhookUrl"`
		Headers    map[string]string `json:"headers"`
		Timeout    int               `json:"timeout"` // seconds, optional
	}
	if err := json.Unmarshal(action.ActionConfig, &webhookConfig); err != nil {
		return fmt.Errorf("failed to parse webhook config: %w", err)
	}

	if webhookConfig.WebhookURL == "" {
		return fmt.Errorf("webhook URL not configured in action")
	}

	// Set default timeout
	timeout := 30 * time.Second
	if webhookConfig.Timeout > 0 {
		timeout = time.Duration(webhookConfig.Timeout) * time.Second
	}

	// Build webhook payload
	webhookPayload := map[string]interface{}{
		"notification_id": n.NotificationID.String(),
		"action_id":       n.ActionID.String(),
		"channel":         string(n.Channel),
		"priority":        string(n.Priority),
		"title":           n.Title,
		"body":            n.Body,
		"created_at":      n.CreatedAt.Format(time.RFC3339),
	}

	// Include notification payload if present
	if len(n.Payload) > 0 {
		var payloadData interface{}
		if err := json.Unmarshal(n.Payload, &payloadData); err == nil {
			webhookPayload["payload"] = payloadData
		}
	}

	// Include target info if present
	if n.TargetUserID != nil {
		webhookPayload["target_user_id"] = *n.TargetUserID
	}
	if n.TargetGroup != nil {
		webhookPayload["target_group"] = *n.TargetGroup
	}
	if n.TraceID != nil {
		webhookPayload["trace_id"] = *n.TraceID
	}

	// Marshal payload
	body, err := json.Marshal(webhookPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{Timeout: timeout}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookConfig.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Industrial-Data-Source-Notification/1.0")
	req.Header.Set("X-Notification-ID", n.NotificationID.String())
	for key, value := range webhookConfig.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body (limited to 1KB for logging)
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	// Log the attempt
	s.logger.Debug().
		Str("notification_id", n.NotificationID.String()).
		Str("webhook_url", webhookConfig.WebhookURL).
		Int("status_code", resp.StatusCode).
		Str("response_body", string(respBody)).
		Msg("webhook delivery attempted")

	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.logger.Info().
			Str("notification_id", n.NotificationID.String()).
			Str("webhook_url", webhookConfig.WebhookURL).
			Int("status_code", resp.StatusCode).
			Msg("webhook delivery succeeded")
		return nil
	}

	// 4xx errors are permanent failures (client error, don't retry)
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return fmt.Errorf("webhook rejected with status %d: %s (permanent failure)", resp.StatusCode, string(respBody))
	}

	// 5xx errors are temporary failures (server error, can retry)
	return fmt.Errorf("webhook failed with status %d: %s (retryable)", resp.StatusCode, string(respBody))
}

// GetNotification retrieves a notification by ID
func (s *Service) GetNotification(ctx context.Context, notificationID uuid.UUID) (*notification.Notification, error) {
	n, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}
	if n == nil {
		return nil, fmt.Errorf("notification not found: %s", notificationID)
	}
	return n, nil
}

// ListNotifications lists notifications with filters
func (s *Service) ListNotifications(ctx context.Context, filter notification.Filter, limit, offset int) ([]*notification.Notification, error) {
	return s.notificationRepo.List(ctx, filter, limit, offset)
}

// GetNotificationsByAction retrieves notifications for an action
func (s *Service) GetNotificationsByAction(ctx context.Context, actionID uuid.UUID) ([]*notification.Notification, error) {
	return s.notificationRepo.GetByActionID(ctx, actionID)
}

// GetDeliveryAttempts retrieves delivery attempts for a notification
func (s *Service) GetDeliveryAttempts(ctx context.Context, notificationID uuid.UUID) ([]*notification.DeliveryAttempt, error) {
	return s.notificationRepo.GetAttempts(ctx, notificationID)
}

// ProcessPendingNotifications processes pending notifications
func (s *Service) ProcessPendingNotifications(ctx context.Context, limit int) (int, error) {
	notifications, err := s.notificationRepo.ListPendingNotifications(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to list pending notifications: %w", err)
	}

	processed := 0
	for _, n := range notifications {
		if err := s.SendNotification(ctx, n.NotificationID); err != nil {
			s.logger.Warn().
				Str("notification_id", n.NotificationID.String()).
				Err(err).
				Msg("failed to send pending notification")
			continue
		}
		processed++
	}

	return processed, nil
}

// ProcessRetryableNotifications processes notifications that can be retried
func (s *Service) ProcessRetryableNotifications(ctx context.Context, limit int) (int, error) {
	notifications, err := s.notificationRepo.ListRetryableNotifications(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to list retryable notifications: %w", err)
	}

	retried := 0
	for _, n := range notifications {
		// Reset for retry
		if err := n.ResetForRetry(); err != nil {
			s.logger.Warn().
				Str("notification_id", n.NotificationID.String()).
				Err(err).
				Msg("failed to reset notification for retry")
			continue
		}

		// Persist the reset state before attempting to send
		if err := s.notificationRepo.Update(ctx, n); err != nil {
			s.logger.Error().
				Str("notification_id", n.NotificationID.String()).
				Err(err).
				Msg("failed to persist notification reset state")
			continue // Skip sending to avoid state inconsistency
		}

		// Send
		if err := s.SendNotification(ctx, n.NotificationID); err != nil {
			s.logger.Warn().
				Str("notification_id", n.NotificationID.String()).
				Err(err).
				Int("retry_count", n.RetryCount).
				Msg("retry failed")
			continue
		}
		retried++
	}

	return retried, nil
}

// ExpireNotifications expires old notifications
func (s *Service) ExpireNotifications(ctx context.Context) (int64, error) {
	return s.notificationRepo.ExpireNotifications(ctx)
}

// GetSSEClientCount returns the number of connected SSE clients
func (s *Service) GetSSEClientCount() int {
	return s.sseHub.GetClientCount()
}

// NotificationEvidence contains the full evidence chain for a notification
type NotificationEvidence struct {
	Notification *notification.Notification
	Action       *domainAction.Action
	Evaluation   *rule.Evaluation
	Rule         *rule.Rule
}

// GetNotificationEvidence retrieves the full evidence chain for a notification
func (s *Service) GetNotificationEvidence(ctx context.Context, notificationID uuid.UUID) (*NotificationEvidence, error) {
	n, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}
	if n == nil {
		return nil, fmt.Errorf("notification not found: %s", notificationID)
	}

	action, err := s.actionRepo.GetByID(ctx, n.ActionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get action: %w", err)
	}

	eval, err := s.ruleRepo.GetEvaluationByID(ctx, action.EvaluationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get evaluation: %w", err)
	}

	r, err := s.ruleRepo.GetByRuleIDAndVersion(ctx, action.RuleID, action.RuleVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule: %w", err)
	}

	return &NotificationEvidence{
		Notification: n,
		Action:       action,
		Evaluation:   eval,
		Rule:         r,
	}, nil
}
