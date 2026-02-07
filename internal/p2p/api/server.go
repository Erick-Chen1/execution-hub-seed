package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/raft"

	"github.com/execution-hub/execution-hub/internal/p2p/consensus"
	"github.com/execution-hub/execution-hub/internal/p2p/protocol"
)

// Server provides HTTP endpoints for P2P runtime.
type Server struct {
	node *consensus.Node
}

func NewServer(node *consensus.Node) *Server {
	return &Server{node: node}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", s.healthz)
	r.Route("/v1/p2p", func(r chi.Router) {
		r.Post("/tx", s.submitTx)
		r.Get("/stats", s.stateStats)
		r.Get("/raft", s.raftStatus)
		r.Post("/raft/join", s.raftJoin)
		r.Post("/raft/remove", s.raftRemove)

		r.Get("/sessions/{sessionId}", s.getSession)
		r.Get("/sessions/{sessionId}/participants", s.listParticipants)
		r.Get("/sessions/{sessionId}/steps/open", s.listOpenSteps)
		r.Get("/sessions/{sessionId}/events", s.listEvents)

		r.Get("/steps/{stepId}", s.getStep)
		r.Get("/steps/{stepId}/artifacts", s.listArtifacts)
	})

	return r
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"nodeId":   s.node.ID(),
		"state":    s.node.State(),
		"leader":   s.node.LeaderAddr(),
		"leaderId": s.node.LeaderNodeID(),
	})
}

func (s *Server) submitTx(w http.ResponseWriter, r *http.Request) {
	if !s.node.IsLeader() {
		respondError(w, http.StatusConflict, "NOT_LEADER", "submit to leader", map[string]any{
			"leader":    s.node.LeaderAddr(),
			"leader_id": s.node.LeaderNodeID(),
		})
		return
	}
	var tx protocol.Tx
	if err := decodeBody(r, &tx); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), nil)
		return
	}
	if err := s.node.ApplyTx(r.Context(), tx); err != nil {
		if isLeadershipErr(err) {
			respondError(w, http.StatusConflict, "NOT_LEADER", err.Error(), map[string]any{
				"leader":    s.node.LeaderAddr(),
				"leader_id": s.node.LeaderNodeID(),
			})
			return
		}
		respondError(w, http.StatusBadRequest, "TX_REJECTED", err.Error(), nil)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"tx_id":      tx.TxID,
		"session_id": tx.SessionID,
		"status":     "APPLIED",
	})
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	session, ok := s.node.Machine().GetSession(sessionID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "session not found", nil)
		return
	}
	respondJSON(w, http.StatusOK, session)
}

func (s *Server) listParticipants(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if _, ok := s.node.Machine().GetSession(sessionID); !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "session not found", nil)
		return
	}
	limit, offset := parseLimitOffset(r, 100, 500)
	participants := s.node.Machine().ListParticipants(sessionID, limit, offset)
	respondJSON(w, http.StatusOK, map[string]any{
		"session_id":   sessionID,
		"participants": participants,
	})
}

func (s *Server) listOpenSteps(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if _, ok := s.node.Machine().GetSession(sessionID); !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "session not found", nil)
		return
	}
	limit, offset := parseLimitOffset(r, 100, 500)
	var participantID *string
	if raw := strings.TrimSpace(r.URL.Query().Get("participant_id")); raw != "" {
		participantID = &raw
	}
	steps, err := s.node.Machine().ListOpenSteps(sessionID, participantID, time.Now().UTC(), limit, offset)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), nil)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"steps":      steps,
	})
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if _, ok := s.node.Machine().GetSession(sessionID); !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "session not found", nil)
		return
	}
	limit, offset := parseLimitOffset(r, 100, 500)
	events := s.node.Machine().ListEvents(sessionID, limit, offset)
	respondJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"events":     events,
	})
}

func (s *Server) getStep(w http.ResponseWriter, r *http.Request) {
	stepID := strings.TrimSpace(chi.URLParam(r, "stepId"))
	step, ok := s.node.Machine().GetStep(stepID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "step not found", nil)
		return
	}
	respondJSON(w, http.StatusOK, step)
}

func (s *Server) listArtifacts(w http.ResponseWriter, r *http.Request) {
	stepID := strings.TrimSpace(chi.URLParam(r, "stepId"))
	_, ok := s.node.Machine().GetStep(stepID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "step not found", nil)
		return
	}
	artifacts := s.node.Machine().ListArtifacts(stepID)
	respondJSON(w, http.StatusOK, map[string]any{
		"step_id":   stepID,
		"artifacts": artifacts,
	})
}

func (s *Server) stateStats(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, s.node.Machine().StateStats(time.Now().UTC()))
}

func (s *Server) raftStatus(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"node_id":    s.node.ID(),
		"raft_addr":  s.node.RaftAddr(),
		"state":      s.node.State(),
		"leader":     s.node.LeaderAddr(),
		"leader_id":  s.node.LeaderNodeID(),
		"is_leader":  s.node.IsLeader(),
		"raft_stats": s.node.Stats(),
	})
}

type raftJoinRequest struct {
	NodeID   string `json:"node_id"`
	RaftAddr string `json:"raft_addr"`
}

func (s *Server) raftJoin(w http.ResponseWriter, r *http.Request) {
	if !s.node.IsLeader() {
		respondError(w, http.StatusConflict, "NOT_LEADER", "submit to leader", map[string]any{
			"leader":    s.node.LeaderAddr(),
			"leader_id": s.node.LeaderNodeID(),
		})
		return
	}
	var req raftJoinRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), nil)
		return
	}
	if err := s.node.AddVoter(r.Context(), req.NodeID, req.RaftAddr); err != nil {
		if isLeadershipErr(err) {
			respondError(w, http.StatusConflict, "NOT_LEADER", err.Error(), map[string]any{
				"leader":    s.node.LeaderAddr(),
				"leader_id": s.node.LeaderNodeID(),
			})
			return
		}
		respondError(w, http.StatusBadRequest, "JOIN_FAILED", err.Error(), nil)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "OK"})
}

type raftRemoveRequest struct {
	NodeID string `json:"node_id"`
}

func (s *Server) raftRemove(w http.ResponseWriter, r *http.Request) {
	if !s.node.IsLeader() {
		respondError(w, http.StatusConflict, "NOT_LEADER", "submit to leader", map[string]any{
			"leader":    s.node.LeaderAddr(),
			"leader_id": s.node.LeaderNodeID(),
		})
		return
	}
	var req raftRemoveRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", err.Error(), nil)
		return
	}
	if err := s.node.RemoveServer(r.Context(), req.NodeID); err != nil {
		if isLeadershipErr(err) {
			respondError(w, http.StatusConflict, "NOT_LEADER", err.Error(), map[string]any{
				"leader":    s.node.LeaderAddr(),
				"leader_id": s.node.LeaderNodeID(),
			})
			return
		}
		respondError(w, http.StatusBadRequest, "REMOVE_FAILED", err.Error(), nil)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "OK"})
}

func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func parseLimitOffset(r *http.Request, defaultLimit, maxLimit int) (int, int) {
	limit := defaultLimit
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			offset = parsed
		}
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	out := map[string]any{
		"error":   code,
		"message": message,
	}
	for k, v := range extra {
		out[k] = v
	}
	respondJSON(w, status, out)
}

func isLeadershipErr(err error) bool {
	return errors.Is(err, raft.ErrNotLeader) ||
		errors.Is(err, raft.ErrLeadershipLost) ||
		errors.Is(err, raft.ErrLeadershipTransferInProgress)
}
