package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// WSClient maintains a persistent WebSocket connection to the cloud algo API.
// It provides automatic reconnection with exponential backoff and Ping/Pong heartbeat.
//
// Usage:
//
//	ws := NewWSClient(cloudURL, apiKey)
//	ws.Start(ctx)
//	defer ws.Close()
//
//	resp, err := ws.Call(ctx, "algo.embed", &EmbedRequest{Texts: texts})
type WSClient struct {
	wsURL  string // e.g. "ws://localhost:19001/api/v1/ws/algo?api_key=..."
	apiKey string

	conn    *websocket.Conn
	connMu  sync.Mutex   // protects conn read/write
	writeMu sync.Mutex   // serializes writes
	nextID  atomic.Int64 // monotonic request IDs
	online  atomic.Bool

	// Pending request/response routing
	pending   map[int64]chan *wsResponse
	pendingMu sync.Mutex

	done chan struct{} // closed on shutdown
}

// wsRequest mirrors the algo.WSRequest JSON-RPC 2.0 format.
type wsRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// wsResponse mirrors the algo.WSResponse JSON-RPC 2.0 format.
type wsResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *wsError        `json:"error,omitempty"`
}

type wsError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// WebSocket configuration.
const (
	wscPingPeriod    = 15 * time.Second
	wscPongWait      = 60 * time.Second
	wscWriteWait     = 10 * time.Second
	wscCallTimeout   = 30 * time.Second
	wscMinBackoff    = 1 * time.Second
	wscMaxBackoff    = 60 * time.Second
	wscBackoffFactor = 2.0
)

// NewWSClient creates a WebSocket client for the cloud algo API.
// The cloudURL should be the base HTTP URL (e.g. "http://localhost:19001/api/v1").
func NewWSClient(cloudURL, apiKey string) *WSClient {
	// Convert HTTP URL to WebSocket URL
	wsURL := httpToWSURL(cloudURL) + "/ws/algo"
	if apiKey != "" {
		wsURL += "?api_key=" + url.QueryEscape(apiKey)
	}

	return &WSClient{
		wsURL:   wsURL,
		apiKey:  apiKey,
		pending: make(map[int64]chan *wsResponse),
		done:    make(chan struct{}),
	}
}

// Start begins the connection loop in a background goroutine.
// Call Close() to stop.
func (ws *WSClient) Start(ctx context.Context) {
	go ws.connectLoop(ctx)
}

// IsOnline returns true if the WebSocket connection is established and healthy.
func (ws *WSClient) IsOnline() bool {
	return ws.online.Load()
}

// Call sends a JSON-RPC 2.0 request and waits for the response.
// Returns an error if the connection is down or the request times out.
func (ws *WSClient) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if !ws.online.Load() {
		return nil, fmt.Errorf("ws: connection offline")
	}

	id := ws.nextID.Add(1)

	// Create response channel
	ch := make(chan *wsResponse, 1)
	ws.pendingMu.Lock()
	ws.pending[id] = ch
	ws.pendingMu.Unlock()

	defer func() {
		ws.pendingMu.Lock()
		delete(ws.pending, id)
		ws.pendingMu.Unlock()
	}()

	// Send request
	req := wsRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	ws.writeMu.Lock()
	ws.connMu.Lock()
	conn := ws.conn
	ws.connMu.Unlock()

	if conn == nil {
		ws.writeMu.Unlock()
		return nil, fmt.Errorf("ws: no connection")
	}

	conn.SetWriteDeadline(time.Now().Add(wscWriteWait))
	err := conn.WriteJSON(req)
	ws.writeMu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("ws: write failed: %w", err)
	}

	// Wait for response with timeout
	callCtx, cancel := context.WithTimeout(ctx, wscCallTimeout)
	defer cancel()

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("ws: rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-callCtx.Done():
		return nil, fmt.Errorf("ws: call timeout for %s", method)
	case <-ws.done:
		return nil, fmt.Errorf("ws: client closed")
	}
}

// Embed is a convenience wrapper for algo.embed via WebSocket.
func (ws *WSClient) Embed(ctx context.Context, texts []string) (*EmbedResponse, error) {
	result, err := ws.Call(ctx, "algo.embed", &EmbedRequest{Texts: texts})
	if err != nil {
		return nil, err
	}
	var resp EmbedResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("ws: decode embed response: %w", err)
	}
	return &resp, nil
}

// Close shuts down the WebSocket client.
func (ws *WSClient) Close() error {
	select {
	case <-ws.done:
		// Already closed
	default:
		close(ws.done)
	}

	ws.connMu.Lock()
	defer ws.connMu.Unlock()
	if ws.conn != nil {
		ws.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		return ws.conn.Close()
	}
	return nil
}

// connectLoop manages the connection lifecycle with exponential backoff.
func (ws *WSClient) connectLoop(ctx context.Context) {
	attempt := 0
	for {
		select {
		case <-ws.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		if err := ws.connect(ctx); err != nil {
			attempt++
			backoff := ws.calcBackoff(attempt)
			slog.Warn("WebSocket connection failed, retrying",
				"error", err, "attempt", attempt, "backoff", backoff)

			select {
			case <-time.After(backoff):
			case <-ws.done:
				return
			case <-ctx.Done():
				return
			}
			continue
		}

		// Connected — reset backoff
		attempt = 0
		ws.online.Store(true)
		slog.Info("WebSocket connected to cloud", "url", ws.wsURL)

		// Run read loop (blocks until disconnect)
		ws.readLoop()

		// Disconnected
		ws.online.Store(false)
		slog.Warn("WebSocket disconnected from cloud")

		// Fail all pending requests
		ws.failPending(fmt.Errorf("ws: disconnected"))
	}
}

// connect establishes a WebSocket connection.
func (ws *WSClient) connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	header := http.Header{}
	if ws.apiKey != "" {
		header.Set("X-API-Key", ws.apiKey)
	}

	conn, _, err := dialer.DialContext(ctx, ws.wsURL, header)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	// Configure connection
	conn.SetReadLimit(1 * 1024 * 1024) // 1MB
	conn.SetReadDeadline(time.Now().Add(wscPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wscPongWait))
		return nil
	})

	ws.connMu.Lock()
	ws.conn = conn
	ws.connMu.Unlock()

	// Start ping goroutine
	go ws.pingLoop(conn)

	return nil
}

// readLoop reads messages and routes responses to pending callers.
func (ws *WSClient) readLoop() {
	for {
		ws.connMu.Lock()
		conn := ws.conn
		ws.connMu.Unlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("WebSocket read error", "error", err)
			}
			return
		}

		var resp wsResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			slog.Warn("WebSocket invalid message", "error", err)
			continue
		}

		// Route to pending caller
		ws.pendingMu.Lock()
		ch, ok := ws.pending[resp.ID]
		if ok {
			delete(ws.pending, resp.ID)
		}
		ws.pendingMu.Unlock()

		if ok {
			ch <- &resp
		}
	}
}

// pingLoop sends periodic ping frames to keep the connection alive.
func (ws *WSClient) pingLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(wscPingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ws.writeMu.Lock()
			conn.SetWriteDeadline(time.Now().Add(wscWriteWait))
			err := conn.WriteMessage(websocket.PingMessage, nil)
			ws.writeMu.Unlock()
			if err != nil {
				return
			}
		case <-ws.done:
			return
		}
	}
}

// failPending fails all pending requests with the given error.
func (ws *WSClient) failPending(err error) {
	ws.pendingMu.Lock()
	defer ws.pendingMu.Unlock()

	for id, ch := range ws.pending {
		ch <- &wsResponse{
			ID:    id,
			Error: &wsError{Code: -1, Message: err.Error()},
		}
		delete(ws.pending, id)
	}
}

// calcBackoff computes exponential backoff with jitter.
func (ws *WSClient) calcBackoff(attempt int) time.Duration {
	backoff := float64(wscMinBackoff) * math.Pow(wscBackoffFactor, float64(attempt-1))
	if backoff > float64(wscMaxBackoff) {
		backoff = float64(wscMaxBackoff)
	}
	return time.Duration(backoff)
}

// httpToWSURL converts an HTTP(S) URL to WS(S).
func httpToWSURL(httpURL string) string {
	u := strings.TrimRight(httpURL, "/")
	u = strings.Replace(u, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	return u
}
