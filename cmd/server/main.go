package main

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/execution-hub/execution-hub/internal/api/http"
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
	"github.com/execution-hub/execution-hub/internal/config"
	"github.com/execution-hub/execution-hub/internal/infrastructure/keystore"
	"github.com/execution-hub/execution-hub/internal/infrastructure/postgres"
	"github.com/execution-hub/execution-hub/internal/infrastructure/sse"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer pool.Close()

	if err := postgres.RunMigrations(ctx, pool, "internal/migrations"); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	// repositories
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

	// infrastructure
	sseHub := sse.NewHub()
	keyStore, _ := keystore.NewFromEnv()
	if keyStore == nil {
		keyStore = &keystore.StaticKeyStore{}
	}

	// services
	actionSvc := action.NewService(actionRepo, ruleRepo, logger)
	notificationSvc := notification.NewService(notificationRepo, actionRepo, ruleRepo, sseHub, logger)
	auditKey := loadHexKey(os.Getenv("AUDIT_SIGNING_KEY"))
	auditSvc := audit.NewService(auditRepo, logger, auditKey)
	trustSvc := trust.NewService(trustRepo, keyStore, logger)
	workflowSvc := workflow.NewService(workflowRepo, logger)
	executorSvc := executor.NewService(execRepo, logger)
	userSvc := user.NewService(userRepo, logger)
	authSvc := auth.NewService(userRepo, sessionRepo, cfg.SessionTTL, logger)

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

	// API server
	apiServer := httpapi.NewServer(workflowSvc, taskSvc, executorSvc, actionSvc, notificationSvc, auditSvc, trustSvc, authSvc, userSvc, approvalSvc, sseHub, cfg.SessionCookieName, cfg.SessionCookieSecure)

	httpServer := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      apiServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// background loops
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			_, _ = notificationSvc.ProcessPendingNotifications(context.Background(), 50)
			_, _ = notificationSvc.ProcessRetryableNotifications(context.Background(), 50)
			_, _ = notificationSvc.ExpireNotifications(context.Background())
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			_, _ = orchestratorSvc.ProcessTimeouts(context.Background(), 50)
		}
	}()

	// start server
	go func() {
		logger.Info().Str("addr", cfg.ServerAddr).Msg("http server started")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("http server failed")
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctxShutdown)
}

func loadHexKey(hexStr string) []byte {
	if hexStr == "" {
		return nil
	}
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil
	}
	return b
}
