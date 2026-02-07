//go:build integration
// +build integration

package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	httpapi "github.com/execution-hub/execution-hub/internal/api/http"
	"github.com/execution-hub/execution-hub/internal/application/action"
	"github.com/execution-hub/execution-hub/internal/application/approval"
	"github.com/execution-hub/execution-hub/internal/application/audit"
	"github.com/execution-hub/execution-hub/internal/application/auth"
	"github.com/execution-hub/execution-hub/internal/application/executor"
	"github.com/execution-hub/execution-hub/internal/application/notification"
	"github.com/execution-hub/execution-hub/internal/application/orchestrator"
	"github.com/execution-hub/execution-hub/internal/application/task"
	"github.com/execution-hub/execution-hub/internal/application/trust"
	"github.com/execution-hub/execution-hub/internal/application/user"
	"github.com/execution-hub/execution-hub/internal/application/workflow"
	"github.com/execution-hub/execution-hub/internal/infrastructure/keystore"
	"github.com/execution-hub/execution-hub/internal/infrastructure/postgres"
	"github.com/execution-hub/execution-hub/internal/infrastructure/sse"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

const auditKeyHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
const testUsername = "alice"
const testPassword = "S3cure!Passw0rd"

func TestWebhookDeliveryIntegration(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	hit := make(chan *http.Request, 1)
	bodyCh := make(chan []byte, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		bodyCh <- body
		hit <- r
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook.Close()

	client := newAuthedClient(t, server.URL)

	wfReq := map[string]interface{}{
		"name": "webhook_flow",
		"steps": []map[string]interface{}{
			{
				"step_key":      "s1",
				"name":          "WebhookStep",
				"executor_type": "HUMAN",
				"executor_ref":  "user:alice",
				"action_type":   "WEBHOOK",
				"action_config": map[string]interface{}{
					"title":      "Webhook Alert",
					"body":       "Webhook Body",
					"channel":    "WEBHOOK",
					"webhookUrl": webhook.URL,
					"headers": map[string]string{
						"X-Test": "true",
					},
					"timeout": 3,
				},
				"timeout_seconds": 0,
				"max_retries":     1,
			},
		},
	}

	var wfResp createWorkflowResponse
	postJSON(t, client, server.URL+"/v1/workflows", wfReq, &wfResp)

	taskReq := map[string]interface{}{
		"title":       "webhook task",
		"workflow_id": wfResp.WorkflowID,
		"context": map[string]interface{}{
			"flag": true,
		},
	}
	var taskResp createTaskResponse
	postJSON(t, client, server.URL+"/v1/tasks", taskReq, &taskResp)

	startReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/tasks/"+taskResp.TaskID+"/start", nil)
	if err != nil {
		t.Fatalf("start request: %v", err)
	}
	if _, err := client.Do(startReq); err != nil {
		t.Fatalf("start task: %v", err)
	}

	select {
	case req := <-hit:
		if req.Header.Get("X-Notification-ID") == "" {
			t.Fatalf("missing X-Notification-ID header")
		}
		if req.Header.Get("X-Test") != "true" {
			t.Fatalf("missing webhook header")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("webhook not received")
	}

	select {
	case body := <-bodyCh:
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("invalid webhook payload: %v", err)
		}
		if payload["notification_id"] == "" {
			t.Fatalf("missing notification_id in webhook payload")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("webhook body not received")
	}
}

func TestSSEDeliveryIntegration(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	client := newAuthedClient(t, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/v1/notifications/sse?client_id=test-client&user_id=alice", nil)
	if err != nil {
		t.Fatalf("sse request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("sse connect: %v", err)
	}
	defer resp.Body.Close()

	msgCh := make(chan map[string]interface{}, 1)
	go func() {
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "data: ") {
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
				var msg map[string]interface{}
				if err := json.Unmarshal([]byte(payload), &msg); err == nil {
					msgCh <- msg
					return
				}
			}
		}
	}()

	wfReq := map[string]interface{}{
		"name": "sse_flow",
		"steps": []map[string]interface{}{
			{
				"step_key":      "s1",
				"name":          "NotifyStep",
				"executor_type": "HUMAN",
				"executor_ref":  "user:alice",
				"action_type":   "NOTIFY",
				"action_config": map[string]interface{}{
					"title":   "Hello",
					"body":    "SSE Body",
					"channel": "SSE",
					"userId":  "alice",
				},
				"timeout_seconds": 0,
				"max_retries":     1,
			},
		},
	}

	var wfResp createWorkflowResponse
	postJSON(t, client, server.URL+"/v1/workflows", wfReq, &wfResp)

	taskReq := map[string]interface{}{
		"title":       "sse task",
		"workflow_id": wfResp.WorkflowID,
	}
	var taskResp createTaskResponse
	postJSON(t, client, server.URL+"/v1/tasks", taskReq, &taskResp)

	startReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/tasks/"+taskResp.TaskID+"/start", nil)
	if err != nil {
		t.Fatalf("start request: %v", err)
	}
	if _, err := client.Do(startReq); err != nil {
		t.Fatalf("start task: %v", err)
	}

	select {
	case msg := <-msgCh:
		if msg["event"] != "notification" {
			t.Fatalf("unexpected event: %v", msg["event"])
		}
		data, ok := msg["data"].(map[string]interface{})
		if !ok || data["actionId"] == "" {
			t.Fatalf("missing actionId in SSE payload")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("SSE message not received")
	}
}

type createWorkflowResponse struct {
	WorkflowID string `json:"workflow_id"`
	Version    int    `json:"version"`
}

type createTaskResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

func postJSON(t *testing.T, client *http.Client, url string, body interface{}, out interface{}) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("post %s status %d: %s", url, resp.StatusCode, string(bodyBytes))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}

func newAuthedClient(t *testing.T, baseURL string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}
	bootstrapUser(t, client, baseURL)
	loginUser(t, client, baseURL)
	return client
}

func bootstrapUser(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	payload := map[string]string{"username": testUsername, "password": testPassword}
	data, _ := json.Marshal(payload)
	resp, err := client.Post(baseURL+"/v1/auth/bootstrap", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest {
		return
	}
	body, _ := io.ReadAll(resp.Body)
	t.Fatalf("bootstrap status %d: %s", resp.StatusCode, string(body))
}

func loginUser(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	var out map[string]interface{}
	postJSON(t, client, baseURL+"/v1/auth/login", map[string]string{
		"username": testUsername,
		"password": testPassword,
	}, &out)
}

func newTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	dsn := testDatabaseURL(t)
	t.Setenv("AUDIT_SIGNING_KEY", auditKeyHex)

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("db pool: %v", err)
	}

	root := repoRoot(t)
	if err := postgres.RunMigrations(ctx, pool, filepath.Join(root, "internal", "migrations")); err != nil {
		pool.Close()
		t.Fatalf("migrations: %v", err)
	}
	if err := resetDatabase(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("reset db: %v", err)
	}

	logger := zerolog.Nop()
	workflowRepo := postgres.NewWorkflowRepository(pool)
	taskRepo := postgres.NewTaskRepository(pool)
	stepRepo := postgres.NewStepRepository(pool)
	actionRepo := postgres.NewActionRepository(pool)
	notificationRepo := postgres.NewNotificationRepository(pool)
	auditRepo := postgres.NewAuditRepository(pool)
	ruleRepo := postgres.NewRuleRepository(pool)
	trustRepo := postgres.NewTrustRepository(pool)
	execRepo := postgres.NewExecutorRepository(pool)
	userRepo := postgres.NewUserRepository(pool)
	sessionRepo := postgres.NewSessionRepository(pool)
	approvalRepo := postgres.NewApprovalRepository(pool)

	sseHub := sse.NewHub()
	keyStore := &keystore.StaticKeyStore{}

	actionSvc := action.NewService(actionRepo, ruleRepo, logger)
	notificationSvc := notification.NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)
	auditSvc := audit.NewService(auditRepo, logger, mustDecodeHex(t, auditKeyHex))
	trustSvc := trust.NewService(trustRepo, keyStore, logger)
	workflowSvc := workflow.NewService(workflowRepo, logger)
	executorSvc := executor.NewService(execRepo, logger)
	userSvc := user.NewService(userRepo, logger)
	authSvc := auth.NewService(userRepo, sessionRepo, 24*time.Hour, logger)

	orchestratorSvc := orchestrator.NewOrchestrator(
		workflowRepo,
		taskRepo,
		stepRepo,
		actionSvc,
		notificationSvc,
		&orchestrator.DefaultAgentRunner{},
		logger,
	)

	taskSvc := task.NewService(taskRepo, stepRepo, workflowRepo, actionSvc, auditSvc, orchestratorSvc, logger)
	approvalSvc := approval.NewService(approvalRepo, userRepo, taskRepo, stepRepo, workflowRepo, taskSvc, actionSvc, auditSvc, sseHub, logger)
	apiServer := httpapi.NewServer(workflowSvc, taskSvc, executorSvc, actionSvc, notificationSvc, auditSvc, trustSvc, authSvc, userSvc, approvalSvc, sseHub, "exec_hub_session", false)
	server := httptest.NewServer(apiServer.Router())

	cleanup := func() {
		server.Close()
		pool.Close()
	}

	return server, cleanup
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	return ""
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func resetDatabase(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		TRUNCATE TABLE
			notification_attempts,
			notifications,
			action_state_transitions,
			actions,
			task_steps,
			tasks,
			workflow_definitions,
			executors,
			audit_logs,
			rule_evaluations,
			rules,
			trust_hash_chain_entries,
			trust_batch_signatures,
			trust_metadata,
			events,
			approval_decisions,
			approvals,
			sessions,
			users
		RESTART IDENTITY CASCADE
	`)
	return err
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	b, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("invalid hex: %v", err)
	}
	return b
}
