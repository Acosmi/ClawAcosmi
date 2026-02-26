// Package handler — SSE events endpoint.
// Mirrors Python api/routes/events.py — real-time processing events via Server-Sent Events.
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// --- Event Types ---

// EventType enumerates SSE event types.
type EventType string

const (
	EventConnected  EventType = "connected"
	EventRetrieving EventType = "retrieving"
	EventRetrieved  EventType = "retrieved"
	EventExtracting EventType = "extracting"
	EventExtracted  EventType = "extracted"
	EventReflecting EventType = "reflecting"
	EventReflected  EventType = "reflected"
	EventCommitting EventType = "committing"
	EventCommitted  EventType = "committed"
	EventError      EventType = "error"
	EventKeepalive  EventType = "keepalive"
)

// StreamEvent represents an SSE event.
type StreamEvent struct {
	EventType EventType      `json:"event_type"`
	Data      map[string]any `json:"data"`
	Timestamp time.Time      `json:"timestamp"`
}

// --- Event Bus (in-memory, can be swapped to Redis pub/sub) ---

var (
	eventBusMu sync.RWMutex
	eventBus   = make(map[string][]chan StreamEvent) // user_id → channels
)

// Subscribe creates a new channel for a user's events.
func Subscribe(userID string) chan StreamEvent {
	ch := make(chan StreamEvent, 16)
	eventBusMu.Lock()
	eventBus[userID] = append(eventBus[userID], ch)
	eventBusMu.Unlock()
	return ch
}

// Unsubscribe removes and closes a channel.
func Unsubscribe(userID string, ch chan StreamEvent) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	channels := eventBus[userID]
	for i, c := range channels {
		if c == ch {
			eventBus[userID] = append(channels[:i], channels[i+1:]...)
			close(ch)
			break
		}
	}
	if len(eventBus[userID]) == 0 {
		delete(eventBus, userID)
	}
}

// PublishEvent sends an event to all subscribers of a user.
func PublishEvent(userID string, event StreamEvent) {
	event.Timestamp = time.Now().UTC()
	eventBusMu.RLock()
	channels := eventBus[userID]
	eventBusMu.RUnlock()
	for _, ch := range channels {
		select {
		case ch <- event:
		default:
			// Channel full, skip (non-blocking)
		}
	}
}

// --- Convenience emitters ---

func EmitRetrieving(userID, query string) {
	PublishEvent(userID, StreamEvent{
		EventType: EventRetrieving,
		Data:      map[string]any{"query": query},
	})
}

func EmitRetrieved(userID string, count int) {
	PublishEvent(userID, StreamEvent{
		EventType: EventRetrieved,
		Data:      map[string]any{"count": count},
	})
}

func EmitExtracting(userID, content string) {
	if len(content) > 100 {
		content = content[:100] + "..."
	}
	PublishEvent(userID, StreamEvent{
		EventType: EventExtracting,
		Data:      map[string]any{"content": content},
	})
}

func EmitError(userID, errMsg string) {
	PublishEvent(userID, StreamEvent{
		EventType: EventError,
		Data:      map[string]any{"error": errMsg},
	})
}

func EmitSessionCommitting(userID, sessionID string) {
	PublishEvent(userID, StreamEvent{
		EventType: EventCommitting,
		Data:      map[string]any{"session_id": sessionID},
	})
}

func EmitSessionCommitted(userID, sessionID string, extracted, created, merged int) {
	PublishEvent(userID, StreamEvent{
		EventType: EventCommitted,
		Data: map[string]any{
			"session_id": sessionID,
			"extracted":  extracted,
			"created":    created,
			"merged":     merged,
		},
	})
}

// --- SSE Handler ---

// EventsHandler handles SSE event streaming.
type EventsHandler struct{}

// NewEventsHandler creates a new EventsHandler.
func NewEventsHandler() *EventsHandler { return &EventsHandler{} }

// RegisterRoutes registers event routes.
func (h *EventsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/events/stream", h.StreamEvents)
}

// StreamEvents handles GET /events/stream — SSE endpoint.
func (h *EventsHandler) StreamEvents(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch := Subscribe(userID)
	defer Unsubscribe(userID, ch)

	// Send connection event
	writeSSE(c, EventConnected, map[string]any{
		"user_id":   userID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})

	clientGone := c.Request.Context().Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-clientGone:
			slog.Debug("SSE client disconnected", "user_id", userID)
			return

		case event := <-ch:
			data := event.Data
			if data == nil {
				data = make(map[string]any)
			}
			data["timestamp"] = event.Timestamp.Format(time.RFC3339)
			writeSSE(c, event.EventType, data)

		case <-ticker.C:
			writeSSE(c, EventKeepalive, map[string]any{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
}

func writeSSE(c *gin.Context, eventType EventType, data map[string]any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, _ = io.WriteString(c.Writer, fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, jsonData))
	c.Writer.Flush()
}
