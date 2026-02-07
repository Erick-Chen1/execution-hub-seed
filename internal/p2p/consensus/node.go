package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"

	"github.com/execution-hub/execution-hub/internal/p2p/protocol"
	"github.com/execution-hub/execution-hub/internal/p2p/state"
)

// Config defines one Raft node runtime.
type Config struct {
	NodeID         string
	RaftAddr       string
	DataDir        string
	Bootstrap      bool
	SnapshotRetain int
	ApplyTimeout   time.Duration
}

// Node wraps Raft + deterministic state machine.
type Node struct {
	id           string
	raftAddr     string
	applyTimeout time.Duration

	raft      *raft.Raft
	transport *raft.NetworkTransport
	machine   *state.Machine
}

func (c Config) normalized() (Config, error) {
	c.NodeID = strings.TrimSpace(c.NodeID)
	c.RaftAddr = strings.TrimSpace(c.RaftAddr)
	c.DataDir = strings.TrimSpace(c.DataDir)
	if c.NodeID == "" {
		return c, errors.New("node_id is required")
	}
	if c.RaftAddr == "" {
		return c, errors.New("raft_addr is required")
	}
	if c.DataDir == "" {
		return c, errors.New("data_dir is required")
	}
	if c.SnapshotRetain <= 0 {
		c.SnapshotRetain = 2
	}
	if c.ApplyTimeout <= 0 {
		c.ApplyTimeout = 5 * time.Second
	}
	return c, nil
}

// NewNode creates a Raft node.
func NewNode(cfg Config) (*Node, error) {
	cfg, err := cfg.normalized()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}

	machine := state.NewMachine()
	fsm := &fsm{machine: machine}

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(cfg.DataDir, "raft-log.bolt"))
	if err != nil {
		return nil, err
	}
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(cfg.DataDir, "raft-stable.bolt"))
	if err != nil {
		return nil, err
	}
	snapshotStore, err := raft.NewFileSnapshotStore(cfg.DataDir, cfg.SnapshotRetain, os.Stderr)
	if err != nil {
		return nil, err
	}
	transport, err := raft.NewTCPTransport(cfg.RaftAddr, nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, err
	}

	raftCfg := raft.DefaultConfig()
	raftCfg.LocalID = raft.ServerID(cfg.NodeID)
	r, err := raft.NewRaft(raftCfg, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, err
	}

	n := &Node{
		id:           cfg.NodeID,
		raftAddr:     cfg.RaftAddr,
		applyTimeout: cfg.ApplyTimeout,
		raft:         r,
		transport:    transport,
		machine:      machine,
	}

	if cfg.Bootstrap {
		hasState, err := raft.HasExistingState(logStore, stableStore, snapshotStore)
		if err != nil {
			return nil, err
		}
		if !hasState {
			future := r.BootstrapCluster(raft.Configuration{Servers: []raft.Server{{
				ID:      raft.ServerID(cfg.NodeID),
				Address: raft.ServerAddress(cfg.RaftAddr),
			}}})
			if err := future.Error(); err != nil && !errors.Is(err, raft.ErrCantBootstrap) {
				return nil, err
			}
		}
	}

	return n, nil
}

// ApplyTx replicates one signed transaction through Raft.
func (n *Node) ApplyTx(ctx context.Context, tx protocol.Tx) error {
	if err := tx.Verify(); err != nil {
		return err
	}
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	timeout := n.applyTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.DeadlineExceeded
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	future := n.raft.Apply(data, timeout)
	if err := future.Error(); err != nil {
		return err
	}
	if applyErr, ok := future.Response().(error); ok && applyErr != nil {
		return applyErr
	}
	return nil
}

// AddVoter joins or updates one voter in the cluster config.
func (n *Node) AddVoter(ctx context.Context, nodeID, raftAddr string) error {
	nodeID = strings.TrimSpace(nodeID)
	raftAddr = strings.TrimSpace(raftAddr)
	if nodeID == "" || raftAddr == "" {
		return errors.New("node_id and raft_addr are required")
	}
	cfgFuture := n.raft.GetConfiguration()
	if err := cfgFuture.Error(); err != nil {
		return err
	}
	for _, srv := range cfgFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) && srv.Address == raft.ServerAddress(raftAddr) {
			return nil
		}
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(raftAddr) {
			if err := n.raft.RemoveServer(srv.ID, 0, n.raftTimeout(ctx)).Error(); err != nil {
				return err
			}
		}
	}
	return n.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(raftAddr), 0, n.raftTimeout(ctx)).Error()
}

// RemoveServer removes one server by node ID.
func (n *Node) RemoveServer(ctx context.Context, nodeID string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return errors.New("node_id is required")
	}
	return n.raft.RemoveServer(raft.ServerID(nodeID), 0, n.raftTimeout(ctx)).Error()
}

func (n *Node) raftTimeout(ctx context.Context) time.Duration {
	timeout := 10 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	return timeout
}

// WaitForLeader waits until any leader is elected.
func (n *Node) WaitForLeader(ctx context.Context, pollInterval time.Duration) (string, error) {
	if pollInterval <= 0 {
		pollInterval = 100 * time.Millisecond
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		leader := strings.TrimSpace(string(n.raft.Leader()))
		if leader != "" {
			return leader, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func (n *Node) ID() string              { return n.id }
func (n *Node) RaftAddr() string        { return n.raftAddr }
func (n *Node) Machine() *state.Machine { return n.machine }
func (n *Node) IsLeader() bool          { return n.raft.State() == raft.Leader }
func (n *Node) LeaderAddr() string      { return strings.TrimSpace(string(n.raft.Leader())) }

// LeaderNodeID returns leader ID if available.
func (n *Node) LeaderNodeID() string {
	_, leaderID := n.raft.LeaderWithID()
	return strings.TrimSpace(string(leaderID))
}

func (n *Node) State() string {
	return n.raft.State().String()
}

func (n *Node) Stats() map[string]string {
	stats := n.raft.Stats()
	out := make(map[string]string, len(stats))
	for k, v := range stats {
		out[k] = v
	}
	return out
}

// Shutdown stops Raft and transport.
func (n *Node) Shutdown() error {
	var shutdownErr error
	if n.raft != nil {
		if err := n.raft.Shutdown().Error(); err != nil {
			shutdownErr = err
		}
	}
	if n.transport != nil {
		_ = n.transport.Close()
	}
	return shutdownErr
}

// fsm wires raft log entries into the state machine.
type fsm struct {
	machine *state.Machine
}

func (f *fsm) Apply(log *raft.Log) interface{} {
	var tx protocol.Tx
	if err := json.Unmarshal(log.Data, &tx); err != nil {
		return fmt.Errorf("decode tx: %w", err)
	}
	if err := f.machine.ApplyTx(tx); err != nil {
		return err
	}
	return nil
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	data, err := f.machine.Marshal()
	if err != nil {
		return nil, err
	}
	return &fsmSnapshot{data: data}, nil
}

func (f *fsm) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return f.machine.Unmarshal(data)
}

type fsmSnapshot struct {
	data []byte
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	if len(s.data) == 0 {
		return sink.Close()
	}
	if _, err := sink.Write(s.data); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {}
