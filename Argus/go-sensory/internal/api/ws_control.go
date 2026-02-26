package api

import (
	"encoding/json"
	"fmt"
	"log"

	"Argus-compound/go-sensory/internal/input"

	"golang.org/x/net/websocket"
)

// handleControl processes input commands from clients.
func (s *Server) handleControl(ws *websocket.Conn) {
	log.Printf("Control client connected: %s", ws.RemoteAddr())
	defer ws.Close()

	for {
		var raw []byte
		if err := websocket.Message.Receive(ws, &raw); err != nil {
			return
		}

		var cmd CommandMessage
		if err := json.Unmarshal(raw, &cmd); err != nil {
			s.sendError(ws, "invalid JSON")
			continue
		}

		result, err := s.executeCommand(cmd)
		if err != nil {
			s.sendError(ws, err.Error())
			continue
		}

		resp, _ := json.Marshal(map[string]interface{}{
			"type":   "result",
			"action": cmd.Action,
			"ok":     true,
			"data":   result,
		})
		ws.Write(resp)
	}
}

// executeCommand dispatches a command to the input controller.
func (s *Server) executeCommand(cmd CommandMessage) (interface{}, error) {
	switch cmd.Action {
	case "click":
		var p ClickParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, fmt.Errorf("invalid click params: %w", err)
		}
		return nil, s.inputCtrl.Click(p.X, p.Y, input.MouseButton(p.Button))

	case "double_click":
		var p ClickParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return nil, s.inputCtrl.DoubleClick(p.X, p.Y)

	case "move":
		var p MoveParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		if p.Smooth {
			dur := p.DurationMs
			if dur == 0 {
				dur = 500
			}
			return nil, s.inputCtrl.MoveToSmooth(p.X, p.Y, dur)
		}
		return nil, s.inputCtrl.MoveTo(p.X, p.Y)

	case "type":
		var p TypeParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return nil, s.inputCtrl.Type(p.Text)

	case "hotkey":
		var p HotkeyParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		keys := make([]input.Key, len(p.Keys))
		for i, k := range p.Keys {
			keys[i] = input.Key(k)
		}
		return nil, s.inputCtrl.Hotkey(keys...)

	case "scroll":
		var p ScrollParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return nil, s.inputCtrl.Scroll(p.X, p.Y, p.DeltaX, p.DeltaY)

	case "mouse_position":
		x, y, err := s.inputCtrl.GetMousePosition()
		if err != nil {
			return nil, err
		}
		return map[string]int{"x": x, "y": y}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", cmd.Action)
	}
}

// sendError sends an error response via WebSocket.
func (s *Server) sendError(ws *websocket.Conn, msg string) {
	resp, _ := json.Marshal(map[string]interface{}{
		"type":  "error",
		"error": msg,
	})
	ws.Write(resp)
}
