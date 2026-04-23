package ws

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/net/websocket"
)

// client represents a single WebSocket connection.
type client struct {
	orgID uuid.UUID
	conn  *websocket.Conn
	send  chan []byte
}

// Hub manages all active WebSocket connections.
// For multi-pod deployments, incoming messages are fan-out via Valkey pub/sub
// (not yet implemented here — single-process for now).
type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[*client]struct{} // orgID → set of clients
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uuid.UUID]map[*client]struct{})}
}

// Broadcast sends a message to all clients in a given org.
func (h *Hub) Broadcast(orgID uuid.UUID, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[orgID] {
		select {
		case c.send <- msg:
		default:
			// Drop if buffer full — client is too slow.
		}
	}
}

// ServeHTTP upgrades the connection and registers the client.
// The org ID must already be on the request context (set by auth middleware).
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value(orgIDKey{}).(uuid.UUID)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	websocket.Handler(func(conn *websocket.Conn) {
		c := &client{orgID: orgID, conn: conn, send: make(chan []byte, 64)}
		h.register(c)
		defer h.unregister(c)

		// Writer
		go func() {
			for msg := range c.send {
				if _, err := conn.Write(msg); err != nil {
					return
				}
			}
		}()

		// Reader: keep connection alive, discard client messages.
		buf := make([]byte, 256)
		for {
			if _, err := conn.Read(buf); err != nil {
				slog.Debug("ws read", "err", err)
				return
			}
		}
	}).ServeHTTP(w, r)
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.orgID] == nil {
		h.clients[c.orgID] = make(map[*client]struct{})
	}
	h.clients[c.orgID][c] = struct{}{}
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[c.orgID], c)
	close(c.send)
}

type orgIDKey struct{}
