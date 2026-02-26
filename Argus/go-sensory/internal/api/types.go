package api

import (
	"encoding/json"

	"Argus-compound/go-sensory/internal/capture"
)

// --- WebSocket message types ---

// FrameMessage is sent to clients with each new frame.
type FrameMessage struct {
	Type      string `json:"type"`
	FrameNo   uint64 `json:"frame_no"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Timestamp int64  `json:"timestamp"`
	ImageB64  string `json:"image_b64"` // JPEG thumbnail base64
}

// CommandMessage is received from clients.
type CommandMessage struct {
	Type   string          `json:"type"`
	Action string          `json:"action"`
	Params json.RawMessage `json:"params"`
}

// ClickParams for click action.
type ClickParams struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Button int `json:"button"` // 0=left, 1=right, 2=middle
}

// TypeParams for type action.
type TypeParams struct {
	Text string `json:"text"`
}

// HotkeyParams for hotkey action.
type HotkeyParams struct {
	Keys []uint16 `json:"keys"` // CGKeyCode values
}

// MoveParams for mouse move.
type MoveParams struct {
	X          int  `json:"x"`
	Y          int  `json:"y"`
	Smooth     bool `json:"smooth"`
	DurationMs int  `json:"duration_ms"`
}

// ScrollParams for scroll action.
type ScrollParams struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	DeltaX int `json:"delta_x"`
	DeltaY int `json:"delta_y"`
}

// StatusResponse is the health/status endpoint response.
type StatusResponse struct {
	Status    string              `json:"status"`
	Backend   string              `json:"backend"` // "sck" or "cg"
	Uptime    string              `json:"uptime"`
	Capturing bool                `json:"capturing"`
	Display   capture.DisplayInfo `json:"display"`
	FrameNo   uint64              `json:"frame_no"`
	Clients   int                 `json:"connected_clients"`
}
