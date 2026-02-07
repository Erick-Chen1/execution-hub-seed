package sse

import (
	"context"
	"sync"

	"github.com/execution-hub/execution-hub/internal/domain/notification"
)

// Hub manages SSE clients.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*notification.SSEClient
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*notification.SSEClient),
	}
}

func (h *Hub) Register(client *notification.SSEClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client.ClientID] = client
}

func (h *Hub) Unregister(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.clients[clientID]; ok {
		c.Close()
		delete(h.clients, clientID)
	}
}

func (h *Hub) GetClient(clientID string) *notification.SSEClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[clientID]
}

func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) BroadcastToAll(message *notification.SSEMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		trySend(c, message)
	}
}

func (h *Hub) BroadcastToUser(userID string, message *notification.SSEMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		if c.UserID != nil && *c.UserID == userID {
			trySend(c, message)
		}
	}
}

func (h *Hub) BroadcastToGroup(group string, message *notification.SSEMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		for _, g := range c.Groups {
			if g == group {
				trySend(c, message)
				break
			}
		}
	}
}

func (h *Hub) SendToClient(clientID string, message *notification.SSEMessage) error {
	h.mu.RLock()
	c := h.clients[clientID]
	h.mu.RUnlock()
	if c == nil {
		return notification.ErrClientNotFound
	}
	if !trySend(c, message) {
		return notification.ErrChannelFull
	}
	return nil
}

func (h *Hub) Start(ctx context.Context) {
	_ = ctx
}

func (h *Hub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, c := range h.clients {
		c.Close()
		delete(h.clients, id)
	}
}

func trySend(c *notification.SSEClient, msg *notification.SSEMessage) bool {
	select {
	case c.MessageChan <- msg:
		return true
	default:
		return false
	}
}
