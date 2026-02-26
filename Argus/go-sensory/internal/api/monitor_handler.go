package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"Argus-compound/go-sensory/internal/pipeline"
)

// SetMonitor attaches the VLM monitor to the server.
func (s *Server) SetMonitor(m *pipeline.Monitor) {
	s.monitor = m
}

// RegisterMonitorRoutes adds GET/PUT /api/monitor endpoints.
func (s *Server) RegisterMonitorRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/monitor", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// GET /api/monitor — return status + recent observations
			json.NewEncoder(w).Encode(s.monitor.Status())

		case http.MethodPut:
			// PUT /api/monitor — update interval
			var req struct {
				IntervalSec int   `json:"interval_sec"`
				Running     *bool `json:"running"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			if req.IntervalSec > 0 {
				s.monitor.SetInterval(time.Duration(req.IntervalSec) * time.Second)
			}
			if req.Running != nil {
				if *req.Running {
					s.monitor.Start()
				} else {
					s.monitor.Stop()
				}
			}
			json.NewEncoder(w).Encode(s.monitor.Status())

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/monitor/observations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		status := s.monitor.Status()

		// Optional ?limit=N query param
		limit := len(status.Observations)
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n < limit {
				limit = n
			}
		}

		// Return most recent N observations
		start := len(status.Observations) - limit
		if start < 0 {
			start = 0
		}
		json.NewEncoder(w).Encode(status.Observations[start:])
	})
}
