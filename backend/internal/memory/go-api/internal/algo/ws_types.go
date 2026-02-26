package algo

import "encoding/json"

// ============================================================================
// WebSocket JSON-RPC 2.0 message types for the Algorithm API
// ============================================================================

// WSRequest is a JSON-RPC 2.0 request over WebSocket.
// Methods: "algo.embed", "algo.classify", "algo.rank", "algo.reflect", "algo.extract", "algo.health"
type WSRequest struct {
	JSONRPC string          `json:"jsonrpc"` // must be "2.0"
	ID      int64           `json:"id"`      // request correlation ID
	Method  string          `json:"method"`  // e.g. "algo.embed"
	Params  json.RawMessage `json:"params"`  // method-specific request body
}

// WSResponse is a JSON-RPC 2.0 response over WebSocket.
type WSResponse struct {
	JSONRPC string      `json:"jsonrpc"` // "2.0"
	ID      int64       `json:"id"`      // matches request ID
	Result  interface{} `json:"result,omitempty"`
	Error   *WSError    `json:"error,omitempty"`
}

// WSError represents a JSON-RPC 2.0 error object.
type WSError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC 2.0 standard error codes.
const (
	WSErrParse          = -32700 // parse error
	WSErrInvalidRequest = -32600 // invalid request
	WSErrMethodNotFound = -32601 // method not found
	WSErrInvalidParams  = -32602 // invalid params
	WSErrInternal       = -32603 // internal error
)

// WSAuthMessage is an optional first message for authentication.
type WSAuthMessage struct {
	Type   string `json:"type"`    // "auth"
	APIKey string `json:"api_key"` // API key for authentication
}
