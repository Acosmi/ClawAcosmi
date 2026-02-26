// cdp_session.go — Persistent CDP WebSocket session with event bus.
//
// Unlike WithCdpSocket() which opens/closes a connection per operation,
// CDPSession maintains a long-lived WebSocket connection, enabling:
//   - Event subscription (downloads, dialogs, navigation, crashes)
//   - Lower latency (no handshake overhead per command)
//   - Connection health monitoring and auto-reconnection
//
// Architecture inspired by: browser-use (event bus + watchdog pattern),
// Rod (persistent connection + state persistence), Chromedp (context-based API).
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// ---------- Event Bus ----------

// EventBus routes CDP events to subscribers.
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]eventHandler
	nextID   int64
}

type eventHandler struct {
	id int64
	fn func(json.RawMessage)
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]eventHandler),
	}
}

// Subscribe registers a handler for a CDP event method.
// Returns an unsubscribe function.
func (eb *EventBus) Subscribe(method string, fn func(json.RawMessage)) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.nextID++
	id := eb.nextID
	eb.handlers[method] = append(eb.handlers[method], eventHandler{id: id, fn: fn})
	return func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		handlers := eb.handlers[method]
		for i, h := range handlers {
			if h.id == id {
				eb.handlers[method] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}
}

// Emit dispatches a CDP event to all registered handlers.
func (eb *EventBus) Emit(method string, params json.RawMessage) {
	eb.mu.RLock()
	handlers := make([]eventHandler, len(eb.handlers[method]))
	copy(handlers, eb.handlers[method])
	eb.mu.RUnlock()

	for _, h := range handlers {
		h.fn(params) // synchronous dispatch — handlers should be fast
	}
}

// ---------- CDP Session ----------

// CDPSession maintains a persistent WebSocket connection to a Chrome CDP endpoint.
type CDPSession struct {
	mu     sync.RWMutex
	wsURL  string
	conn   *websocket.Conn
	logger *slog.Logger

	// Message routing.
	nextID  atomic.Int64
	pending sync.Map // map[int]chan sessionResult
	events  *EventBus
	done    chan struct{} // closed when read loop exits

	// State.
	closed     bool
	reconnects int
}

type sessionResult struct {
	result json.RawMessage
	err    error
}

// CDPSessionConfig configures a CDP session.
type CDPSessionConfig struct {
	WSURL  string
	Logger *slog.Logger
}

// NewCDPSession creates and connects a persistent CDP session.
func NewCDPSession(ctx context.Context, cfg CDPSessionConfig) (*CDPSession, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &CDPSession{
		wsURL:  cfg.WSURL,
		logger: logger,
		events: NewEventBus(),
		done:   make(chan struct{}),
	}
	s.nextID.Store(1)

	if err := s.connect(ctx); err != nil {
		return nil, fmt.Errorf("CDPSession connect: %w", err)
	}

	return s, nil
}

// connect establishes the WebSocket connection and starts the read loop.
func (s *CDPSession) connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: time.Duration(DefaultHandshakeTimeoutMs) * time.Millisecond,
	}
	conn, _, err := dialer.DialContext(ctx, s.wsURL, nil)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.conn = conn
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.readLoop()
	s.logger.Debug("CDPSession connected", "wsURL", s.wsURL)
	return nil
}

// readLoop reads messages from the WebSocket, routing responses to pending
// requests and CDP events to the event bus.
func (s *CDPSession) readLoop() {
	defer close(s.done)
	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			s.drainPending(fmt.Errorf("CDP session closed: %w", err))
			return
		}

		var envelope struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result,omitempty"`
			Error  *CdpError       `json:"error,omitempty"`
		}
		if err := json.Unmarshal(msg, &envelope); err != nil {
			continue
		}

		if envelope.ID != 0 {
			// Response to a command.
			if ch, ok := s.pending.LoadAndDelete(envelope.ID); ok {
				r := sessionResult{}
				if envelope.Error != nil && envelope.Error.Message != "" {
					r.err = fmt.Errorf("%s", envelope.Error.Message)
				} else {
					r.result = envelope.Result
				}
				ch.(chan sessionResult) <- r
			}
		} else if envelope.Method != "" {
			// CDP event — dispatch to event bus.
			s.events.Emit(envelope.Method, envelope.Params)
		}
	}
}

// drainPending closes all pending requests with the given error.
func (s *CDPSession) drainPending(err error) {
	s.pending.Range(func(key, value any) bool {
		ch := value.(chan sessionResult)
		ch <- sessionResult{err: err}
		s.pending.Delete(key)
		return true
	})
}

// Send sends a CDP command and waits for the response.
func (s *CDPSession) Send(method string, params map[string]any) (json.RawMessage, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("CDPSession is closed")
	}
	conn := s.conn
	s.mu.RUnlock()

	id := int(s.nextID.Add(1) - 1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	ch := make(chan sessionResult, 1)
	s.pending.Store(id, ch)

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		s.pending.Delete(id)
		return nil, fmt.Errorf("CDPSession write: %w", err)
	}

	select {
	case r := <-ch:
		return r.result, r.err
	case <-s.done:
		return nil, fmt.Errorf("CDPSession connection lost")
	}
}

// SendWithTimeout sends a CDP command with a deadline.
func (s *CDPSession) SendWithTimeout(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	type result struct {
		raw json.RawMessage
		err error
	}
	ch := make(chan result, 1)
	go func() {
		raw, err := s.Send(method, params)
		ch <- result{raw, err}
	}()

	select {
	case r := <-ch:
		return r.raw, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Events returns the event bus for subscribing to CDP events.
func (s *CDPSession) Events() *EventBus {
	return s.events
}

// WSURL returns the WebSocket URL of this session.
func (s *CDPSession) WSURL() string {
	return s.wsURL
}

// Reconnect attempts to re-establish the connection after a disconnect.
func (s *CDPSession) Reconnect(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("CDPSession is permanently closed")
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	s.mu.Unlock()

	// Wait for read loop to finish.
	<-s.done

	// Exponential backoff: 100ms, 200ms, 400ms, ..., max 5s.
	maxAttempts := 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		delay := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		if err := s.connect(ctx); err != nil {
			s.logger.Warn("CDPSession reconnect attempt failed",
				"attempt", attempt+1, "err", err)
			continue
		}

		s.mu.Lock()
		s.reconnects++
		s.mu.Unlock()
		s.logger.Info("CDPSession reconnected", "attempt", attempt+1)
		return nil
	}
	return fmt.Errorf("CDPSession reconnect failed after %d attempts", maxAttempts)
}

// IsConnected returns true if the session WebSocket is still open.
func (s *CDPSession) IsConnected() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

// Close permanently closes the CDP session.
func (s *CDPSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.conn != nil {
		err := s.conn.Close()
		<-s.done
		return err
	}
	return nil
}

// ---------- Session-aware CdpSendFn adapter ----------

// SessionSendFn returns a CdpSendFn backed by the persistent session.
// This bridges the existing WithCdpSocket-based code to use CDPSession.
func (s *CDPSession) SessionSendFn() CdpSendFn {
	return s.Send
}
