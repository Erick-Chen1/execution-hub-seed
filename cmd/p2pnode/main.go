package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	p2papi "github.com/execution-hub/execution-hub/internal/p2p/api"
	"github.com/execution-hub/execution-hub/internal/p2p/consensus"
)

type runtimeConfig struct {
	NodeID            string
	RaftAddr          string
	HTTPAddr          string
	DataDir           string
	Bootstrap         bool
	ApplyTimeout      time.Duration
	JoinEndpoint      string
	JoinRetries       int
	JoinRetryDelay    time.Duration
	StartupWaitLeader time.Duration
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	node, err := consensus.NewNode(consensus.Config{
		NodeID:         cfg.NodeID,
		RaftAddr:       cfg.RaftAddr,
		DataDir:        cfg.DataDir,
		Bootstrap:      cfg.Bootstrap,
		SnapshotRetain: 2,
		ApplyTimeout:   cfg.ApplyTimeout,
	})
	if err != nil {
		log.Fatalf("create raft node: %v", err)
	}
	defer func() {
		_ = node.Shutdown()
	}()

	if !cfg.Bootstrap && cfg.JoinEndpoint != "" {
		if err := joinCluster(cfg); err != nil {
			log.Printf("join cluster failed: %v", err)
		} else {
			log.Printf("joined cluster via %s", cfg.JoinEndpoint)
		}
	}

	if cfg.StartupWaitLeader > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.StartupWaitLeader)
		_, _ = node.WaitForLeader(ctx, 150*time.Millisecond)
		cancel()
	}

	apiServer := p2papi.NewServer(node)
	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      apiServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("p2p http listening on %s (node_id=%s raft_addr=%s bootstrap=%t)", cfg.HTTPAddr, cfg.NodeID, cfg.RaftAddr, cfg.Bootstrap)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = node.Shutdown()
}

func loadConfig() (*runtimeConfig, error) {
	hostname, _ := os.Hostname()
	nodeID := getenv("P2P_NODE_ID", strings.TrimSpace(hostname))
	if nodeID == "" {
		nodeID = "node-1"
	}
	raftAddr := getenv("P2P_RAFT_ADDR", "127.0.0.1:17000")
	httpAddr := getenv("P2P_HTTP_ADDR", "0.0.0.0:18080")
	bootstrap := parseBool(getenv("P2P_BOOTSTRAP", "false"), false)
	applyTimeout := parseDuration(getenv("P2P_APPLY_TIMEOUT", "5s"), 5*time.Second)
	joinEndpoint := strings.TrimSpace(getenv("P2P_JOIN_ENDPOINT", ""))
	joinRetries := parseInt(getenv("P2P_JOIN_RETRIES", "30"), 30)
	joinRetryDelay := parseDuration(getenv("P2P_JOIN_RETRY_DELAY", "1s"), time.Second)
	startupWait := parseDuration(getenv("P2P_STARTUP_WAIT_LEADER", "4s"), 4*time.Second)

	dataDir := strings.TrimSpace(getenv("P2P_DATA_DIR", ""))
	if dataDir == "" {
		dataDir = filepath.Join("tmp", "p2pnode", nodeID)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	return &runtimeConfig{
		NodeID:            nodeID,
		RaftAddr:          raftAddr,
		HTTPAddr:          httpAddr,
		DataDir:           dataDir,
		Bootstrap:         bootstrap,
		ApplyTimeout:      applyTimeout,
		JoinEndpoint:      joinEndpoint,
		JoinRetries:       joinRetries,
		JoinRetryDelay:    joinRetryDelay,
		StartupWaitLeader: startupWait,
	}, nil
}

func joinCluster(cfg *runtimeConfig) error {
	endpoint := strings.TrimRight(cfg.JoinEndpoint, "/") + "/v1/p2p/raft/join"
	payload := map[string]string{
		"node_id":   cfg.NodeID,
		"raft_addr": cfg.RaftAddr,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	var lastErr error
	for i := 0; i < cfg.JoinRetries; i++ {
		req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(cfg.JoinRetryDelay)
			continue
		}
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("join returned status %d", resp.StatusCode)
		time.Sleep(cfg.JoinRetryDelay)
	}
	if lastErr == nil {
		lastErr = errors.New("join failed")
	}
	return lastErr
}

func getenv(key, def string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	return val
}

func parseBool(raw string, def bool) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return v
}

func parseInt(raw string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return v
}

func parseDuration(raw string, def time.Duration) time.Duration {
	v, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return v
}
