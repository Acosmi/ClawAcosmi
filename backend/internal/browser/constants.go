// Package browser provides browser automation via Chrome DevTools Protocol.
// It handles Chrome lifecycle management, CDP communication, extension relay,
// and browser session management.
package browser

// Constants for browser control infrastructure.
const (
	// GatewayPort is the fixed port for the Gateway WebSocket.
	GatewayPort = 19001
	// BridgePort is the fixed port for the Bridge service.
	BridgePort = 18790
	// BrowserControlPort is the fixed port for the browser control server.
	BrowserControlPort = 18791
	// CanvasPort is reserved for the canvas service.
	CanvasPort = 18793

	// CDPPortRangeStart is the start of the CDP port range.
	CDPPortRangeStart = 18800
	// CDPPortRangeEnd is the end of the CDP port range.
	CDPPortRangeEnd = 18899

	// DefaultHandshakeTimeoutMs is the default WebSocket handshake timeout.
	DefaultHandshakeTimeoutMs = 5000
	// DefaultFetchTimeoutMs is the default timeout for HTTP fetches.
	DefaultFetchTimeoutMs = 1500
)
