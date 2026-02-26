package api

import (
	"log"
	"sync"
	"time"

	"Argus-compound/go-sensory/internal/capture"
)

// Hub manages WebSocket client connections and frame broadcasting.
// It provides an efficient fan-out mechanism from a single frame
// producer to N connected clients, with backpressure handling
// for slow consumers.
type Hub struct {
	capturer capture.Capturer

	// clients maps each connected WebSocket client's read-only
	// frame channel to its metadata.
	clients   map[<-chan *capture.Frame]ClientInfo
	clientsMu sync.RWMutex
}

// ClientInfo holds metadata about a connected client.
type ClientInfo struct {
	RemoteAddr  string `json:"remote_addr"`
	ConnectedAt int64  `json:"connected_at"` // Unix timestamp
}

// NewHub creates a new broadcast hub.
func NewHub(capturer capture.Capturer) *Hub {
	return &Hub{
		capturer: capturer,
		clients:  make(map[<-chan *capture.Frame]ClientInfo),
	}
}

// Register adds a new client and returns a dedicated frame channel.
func (h *Hub) Register(remoteAddr string) <-chan *capture.Frame {
	ch := h.capturer.Subscribe()

	h.clientsMu.Lock()
	h.clients[ch] = ClientInfo{
		RemoteAddr:  remoteAddr,
		ConnectedAt: captureTimeNow(),
	}
	h.clientsMu.Unlock()

	log.Printf("[Hub] Client registered: %s (total: %d)", remoteAddr, h.ClientCount())
	return ch
}

// Unregister removes a client and cleans up its frame channel.
func (h *Hub) Unregister(ch <-chan *capture.Frame) {
	h.clientsMu.Lock()
	info, ok := h.clients[ch]
	if ok {
		delete(h.clients, ch)
	}
	h.clientsMu.Unlock()

	h.capturer.Unsubscribe(ch)
	if ok {
		log.Printf("[Hub] Client unregistered: %s (total: %d)", info.RemoteAddr, h.ClientCount())
	}
}

// ClientCount returns the number of currently connected clients.
func (h *Hub) ClientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// Stats returns broadcast hub statistics.
func (h *Hub) Stats() HubStats {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	clients := make([]ClientInfo, 0, len(h.clients))
	for _, info := range h.clients {
		clients = append(clients, info)
	}

	return HubStats{
		ConnectedClients: len(h.clients),
		Clients:          clients,
	}
}

// HubStats holds snapshot stats of the hub.
type HubStats struct {
	ConnectedClients int          `json:"connected_clients"`
	Clients          []ClientInfo `json:"clients"`
}

// captureTimeNow returns Unix timestamp. Separated for testability.
func captureTimeNow() int64 {
	return time.Now().Unix()
}
