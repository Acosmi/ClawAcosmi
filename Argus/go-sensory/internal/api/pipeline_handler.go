package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"Argus-compound/go-sensory/internal/pipeline"
)

// PipelineHandler exposes pipeline stats and keyframe retrieval via REST.
type PipelineHandler struct {
	pipe      *pipeline.Pipeline
	extractor *pipeline.KeyframeExtractor
}

// RegisterPipelineRoutes adds pipeline-related routes to the mux.
func (s *Server) RegisterPipelineRoutes(mux *http.ServeMux) {
	if s.pipeline == nil {
		return
	}
	h := &PipelineHandler{pipe: s.pipeline, extractor: s.keyframeExtractor}

	mux.HandleFunc("/api/pipeline/stats", h.handleStats)
	mux.HandleFunc("/api/pipeline/keyframes", h.handleKeyframes)
}

// SetPipeline attaches the pipeline and extractor to the server.
func (s *Server) SetPipeline(p *pipeline.Pipeline, ke *pipeline.KeyframeExtractor) {
	s.pipeline = p
	s.keyframeExtractor = ke
}

// handleStats returns pipeline statistics.
func (h *PipelineHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.pipe.Stats())
}

// handleKeyframes returns the N most recent keyframes with optional base64 thumbnails.
func (h *PipelineHandler) handleKeyframes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	n := 10
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}

	keyframes := h.extractor.RecentKeyframes(n)

	// Build response with base64 thumbnails (ThumbnailJPEG is json:"-")
	type kfResponse struct {
		FrameNo       int64          `json:"frame_no"`
		Timestamp     float64        `json:"timestamp"`
		ChangeRatio   float64        `json:"change_ratio"`
		TriggerReason string         `json:"trigger_reason"`
		ThumbnailB64  string         `json:"thumbnail_b64,omitempty"`
		Metadata      map[string]any `json:"metadata,omitempty"`
	}

	out := make([]kfResponse, len(keyframes))
	for i, kf := range keyframes {
		out[i] = kfResponse{
			FrameNo:       kf.FrameNo,
			Timestamp:     kf.Timestamp,
			ChangeRatio:   kf.ChangeRatio,
			TriggerReason: kf.TriggerReason,
			Metadata:      kf.Metadata,
		}
		if len(kf.ThumbnailJPEG) > 0 {
			out[i].ThumbnailB64 = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(kf.ThumbnailJPEG)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count":     len(out),
		"keyframes": out,
	})
}
