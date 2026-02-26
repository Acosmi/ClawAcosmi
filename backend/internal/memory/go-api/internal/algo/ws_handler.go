package algo

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocket configuration constants.
const (
	wsWriteWait      = 10 * time.Second // max time to write a message
	wsPongWait       = 60 * time.Second // max time to wait for pong
	wsPingPeriod     = 15 * time.Second // ping interval (must be < pongWait)
	wsMaxMessageSize = 1 * 1024 * 1024  // 1 MB max message size
)

// upgrader handles HTTP → WebSocket upgrade with permissive CORS for local-proxy clients.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow any origin — protected by API key auth
	},
}

// WSHandler manages WebSocket connections for the algorithm API.
type WSHandler struct {
	svc       *Service
	validKeys map[string]bool
}

// NewWSHandler creates a WebSocket handler for the algorithm service.
func NewWSHandler(svc *Service, apiKeys []string) *WSHandler {
	keys := make(map[string]bool, len(apiKeys))
	for _, k := range apiKeys {
		if k != "" {
			keys[k] = true
		}
	}
	return &WSHandler{svc: svc, validKeys: keys}
}

// RegisterRoutes adds the WebSocket endpoint to the router.
// The WS endpoint does NOT use the standard middleware — it handles auth internally.
func (wh *WSHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/ws/algo", wh.handleWS)
}

// handleWS upgrades to WebSocket and runs the message loop.
func (wh *WSHandler) handleWS(c *gin.Context) {
	// Authenticate via query param or first message
	apiKey := c.Query("api_key")
	if apiKey == "" {
		// Also check header-based auth for upgrade request
		apiKey = c.GetHeader("X-API-Key")
		if apiKey == "" {
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}
	}

	// If keys configured, validate now (or defer to first message)
	authenticated := len(wh.validKeys) == 0 // dev mode: no keys = auto-auth
	if apiKey != "" {
		authenticated = wh.validateKey(apiKey)
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err, "remote", c.ClientIP())
		return
	}

	slog.Info("WebSocket client connected",
		"remote", c.ClientIP(),
		"authenticated", authenticated)

	// Run connection handler
	wh.serveConn(conn, authenticated)
}

// serveConn runs the read/write loops for a single WebSocket connection.
func (wh *WSHandler) serveConn(conn *websocket.Conn, authenticated bool) {
	var writeMu sync.Mutex

	// Configure connection
	conn.SetReadLimit(wsMaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// Ping ticker goroutine
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				writeMu.Lock()
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				writeMu.Unlock()
				if err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	defer func() {
		close(done)
		conn.Close()
		slog.Info("WebSocket client disconnected")
	}()

	// Read loop
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("WebSocket read error", "error", err)
			}
			return
		}

		// Handle authentication if not yet authenticated
		if !authenticated {
			var authMsg WSAuthMessage
			if err := json.Unmarshal(message, &authMsg); err == nil && authMsg.Type == "auth" {
				if wh.validateKey(authMsg.APIKey) {
					authenticated = true
					resp := WSResponse{JSONRPC: "2.0", ID: 0, Result: map[string]string{"status": "authenticated"}}
					writeMu.Lock()
					conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
					conn.WriteJSON(resp)
					writeMu.Unlock()
					continue
				} else {
					resp := WSResponse{JSONRPC: "2.0", ID: 0, Error: &WSError{Code: -1, Message: "invalid api key"}}
					writeMu.Lock()
					conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
					conn.WriteJSON(resp)
					writeMu.Unlock()
					return // Close on auth failure
				}
			}
			resp := WSResponse{JSONRPC: "2.0", ID: 0, Error: &WSError{Code: -1, Message: "authentication required"}}
			writeMu.Lock()
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			conn.WriteJSON(resp)
			writeMu.Unlock()
			return
		}

		// Parse JSON-RPC request
		var req WSRequest
		if err := json.Unmarshal(message, &req); err != nil {
			wh.sendError(&writeMu, conn, 0, WSErrParse, "parse error: "+err.Error())
			continue
		}

		if req.JSONRPC != "2.0" || req.Method == "" {
			wh.sendError(&writeMu, conn, req.ID, WSErrInvalidRequest, "invalid JSON-RPC 2.0 request")
			continue
		}

		// Dispatch method — run in goroutine for concurrency
		go wh.dispatch(&writeMu, conn, &req)
	}
}

// dispatch routes a JSON-RPC method to the corresponding service call.
func (wh *WSHandler) dispatch(writeMu *sync.Mutex, conn *websocket.Conn, req *WSRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var result interface{}
	var err error

	switch req.Method {
	case "algo.embed":
		var params EmbedRequest
		if e := json.Unmarshal(req.Params, &params); e != nil {
			wh.sendError(writeMu, conn, req.ID, WSErrInvalidParams, "invalid params: "+e.Error())
			return
		}
		result, err = wh.svc.Embed(ctx, &params)

	case "algo.classify":
		var params ClassifyRequest
		if e := json.Unmarshal(req.Params, &params); e != nil {
			wh.sendError(writeMu, conn, req.ID, WSErrInvalidParams, "invalid params: "+e.Error())
			return
		}
		result, err = wh.svc.Classify(ctx, &params)

	case "algo.rank":
		var params RankRequest
		if e := json.Unmarshal(req.Params, &params); e != nil {
			wh.sendError(writeMu, conn, req.ID, WSErrInvalidParams, "invalid params: "+e.Error())
			return
		}
		result, err = wh.svc.Rank(ctx, &params)

	case "algo.reflect":
		var params ReflectRequest
		if e := json.Unmarshal(req.Params, &params); e != nil {
			wh.sendError(writeMu, conn, req.ID, WSErrInvalidParams, "invalid params: "+e.Error())
			return
		}
		result, err = wh.svc.Reflect(ctx, &params)

	case "algo.extract":
		var params ExtractRequest
		if e := json.Unmarshal(req.Params, &params); e != nil {
			wh.sendError(writeMu, conn, req.ID, WSErrInvalidParams, "invalid params: "+e.Error())
			return
		}
		result, err = wh.svc.Extract(ctx, &params)

	case "algo.health":
		result = wh.svc.Health()

	default:
		wh.sendError(writeMu, conn, req.ID, WSErrMethodNotFound,
			fmt.Sprintf("method not found: %s", req.Method))
		return
	}

	if err != nil {
		wh.sendError(writeMu, conn, req.ID, WSErrInternal, err.Error())
		return
	}

	resp := WSResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	writeMu.Lock()
	defer writeMu.Unlock()
	conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	if writeErr := conn.WriteJSON(resp); writeErr != nil {
		slog.Warn("WebSocket write error", "error", writeErr, "method", req.Method)
	}
}

// sendError sends a JSON-RPC error response.
func (wh *WSHandler) sendError(writeMu *sync.Mutex, conn *websocket.Conn, id int64, code int, msg string) {
	resp := WSResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &WSError{Code: code, Message: msg},
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	conn.WriteJSON(resp)
}

// validateKey checks if the given API key is valid using constant-time comparison.
func (wh *WSHandler) validateKey(key string) bool {
	for validKey := range wh.validKeys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(validKey)) == 1 {
			return true
		}
	}
	return false
}
