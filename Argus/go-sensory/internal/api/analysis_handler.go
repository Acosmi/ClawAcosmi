package api

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"

	"Argus-compound/go-sensory/internal/analysis"
	"Argus-compound/go-sensory/internal/vlm"
)

// AnalysisHandler handles temporal analysis REST endpoints.
// REST handlers for temporal analysis endpoints.
type AnalysisHandler struct {
	analyzer *analysis.TemporalAnalyzer
}

// analyzeRequest is the request body for POST /api/analyze.
type analyzeRequest struct {
	ImagesB64  []string  `json:"images_b64"`
	Timestamps []float64 `json:"timestamps"`
	Query      string    `json:"query"`
	Context    string    `json:"context"`
}

// RegisterAnalysisRoutes adds the /api/analyze endpoint.
func (s *Server) RegisterAnalysisRoutes(mux *http.ServeMux) {
	if s.vlmRouter == nil || s.vlmRouter.ActiveProvider() == nil {
		log.Printf("[AnalysisHandler] VLM not configured, /api/analyze disabled")
		return
	}

	analyzer := analysis.NewTemporalAnalyzer(s.vlmRouter.ActiveProvider(), "", s.scaler)
	h := &AnalysisHandler{analyzer: analyzer}
	mux.HandleFunc("/api/analyze", h.handleAnalyze)
}

// handleAnalyze handles POST /api/analyze.
// Bug fix from Python version: properly handles async flow
// (Python had missing `await` on analyze_sequence call).
func (h *AnalysisHandler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if len(req.ImagesB64) != len(req.Timestamps) {
		http.Error(w, `{"error":"images_b64 and timestamps must have same length"}`, http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, `{"error":"query is required"}`, http.StatusBadRequest)
		return
	}

	// Convert base64 images to JPEG bytes
	frames := make([]analysis.FrameInput, 0, len(req.ImagesB64))
	for i, b64 := range req.ImagesB64 {
		jpegBytes, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			http.Error(w, `{"error":"invalid base64 image at index `+string(rune('0'+i))+`"}`, http.StatusBadRequest)
			return
		}
		frames = append(frames, analysis.FrameInput{
			JPEG:      jpegBytes,
			Timestamp: req.Timestamps[i],
			FrameNo:   i,
		})
	}

	result, err := h.analyzer.AnalyzeSequence(r.Context(), frames, req.Query, req.Context)
	if err != nil {
		log.Printf("[AnalysisHandler] analyze error: %v", err)
		http.Error(w, `{"error":"analysis failed"}`, http.StatusInternalServerError)
		return
	}

	// Format response matching Python's output
	events := make([]map[string]any, 0, len(result.Events))
	for _, e := range result.Events {
		events = append(events, map[string]any{
			"timestamp":   e.Timestamp,
			"frame_no":    e.FrameNo,
			"type":        e.EventType,
			"description": e.Description,
			"severity":    e.Severity,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"summary":        result.Summary,
		"events":         events,
		"root_cause":     result.RootCause,
		"recommendation": result.Recommendation,
		"confidence":     result.Confidence,
	})
}

// RegisterHealthRoutes adds the unified /api/health endpoint.
// REST handler for system health/status endpoint.
// Bug fix from Python version: removed reference to deleted vlm_service.
func (s *Server) RegisterHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		vlmReady := false
		vlmProvider := ""
		if s.vlmRouter != nil && s.vlmRouter.ActiveProvider() != nil {
			vlmReady = true
			vlmProvider = s.vlmRouter.ActiveProvider().Name()
		}

		pipelineStats := map[string]any{}
		if s.pipeline != nil {
			pipelineStats = s.pipeline.Stats()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":         "ok",
			"vlm_ready":      vlmReady,
			"vlm_provider":   vlmProvider,
			"pipeline_stats": pipelineStats,
			"version":        "go-sensory/p5",
		})
	})
}

// Ensure vlm package is imported.
var _ vlm.Provider
