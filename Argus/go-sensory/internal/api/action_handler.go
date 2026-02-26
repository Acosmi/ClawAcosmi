package api

import (
	"encoding/json"
	"log"
	"net/http"

	"Argus-compound/go-sensory/internal/input"
)

// ActionRequest is the body for POST /api/action.
type ActionRequest struct {
	Action string          `json:"action"`
	Params json.RawMessage `json:"params"`
}

// ActionResponse is the response for POST /api/action.
type ActionResponse struct {
	OK     bool        `json:"ok"`
	Action string      `json:"action"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// handleAction is the REST endpoint for executing input actions.
// POST /api/action  { "action": "click", "params": {"x":100,"y":200} }
//
// Provides synchronous HTTP access to the same functionality as the
// WebSocket /ws/control endpoint, useful for one-shot request-response
// patterns in automated workflows.
func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ActionResponse{
			OK:    false,
			Error: "method not allowed, use POST",
		})
		return
	}

	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActionResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	// Safety check via guardrails
	if s.guardrails != nil {
		allowed, reason := s.guardrails.CheckAction(req.Action, req.Params, "rest-api")
		if !allowed {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(ActionResponse{
				OK:     false,
				Action: req.Action,
				Error:  reason,
			})
			return
		}
	}

	// Reuse the existing executeCommand logic from ws_control.go
	cmd := CommandMessage{
		Action: req.Action,
		Params: req.Params,
	}
	result, err := s.executeCommand(cmd)
	if err != nil {
		log.Printf("[Action] %s failed: %v", req.Action, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ActionResponse{
			OK:     false,
			Action: req.Action,
			Error:  err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(ActionResponse{
		OK:     true,
		Action: req.Action,
		Data:   result,
	})
}

// handleMousePosition is a GET convenience endpoint.
// GET /api/action/mouse → {"ok": true, "data": {"x": 123, "y": 456}}
func (s *Server) handleMousePosition(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	x, y, err := s.inputCtrl.GetMousePosition()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ActionResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(ActionResponse{
		OK:     true,
		Action: "mouse_position",
		Data:   map[string]int{"x": x, "y": y},
	})
}

// handleActionBatch executes multiple actions in sequence.
// POST /api/action/batch  [{"action":"click","params":...}, ...]
func (s *Server) handleActionBatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ActionResponse{OK: false, Error: "use POST"})
		return
	}

	var reqs []ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActionResponse{OK: false, Error: "invalid JSON array"})
		return
	}

	results := make([]ActionResponse, 0, len(reqs))
	for _, req := range reqs {
		// Safety check
		if s.guardrails != nil {
			allowed, reason := s.guardrails.CheckAction(req.Action, req.Params, "rest-api-batch")
			if !allowed {
				results = append(results, ActionResponse{
					OK:     false,
					Action: req.Action,
					Error:  reason,
				})
				continue
			}
		}

		cmd := CommandMessage{Action: req.Action, Params: req.Params}
		result, err := s.executeCommand(cmd)
		if err != nil {
			results = append(results, ActionResponse{
				OK:     false,
				Action: req.Action,
				Error:  err.Error(),
			})
		} else {
			results = append(results, ActionResponse{
				OK:     true,
				Action: req.Action,
				Data:   result,
			})
		}
	}

	json.NewEncoder(w).Encode(results)
}

// RegisterActionRoutes adds the action REST endpoints to the mux.
func (s *Server) RegisterActionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/action", s.handleAction)
	mux.HandleFunc("/api/action/mouse", s.handleMousePosition)
	mux.HandleFunc("/api/action/batch", s.handleActionBatch)
}

// SetGuardrails sets the action guardrails for safety checks.
func (s *Server) SetGuardrails(g *input.ActionGuardrails) {
	s.guardrails = g
}
