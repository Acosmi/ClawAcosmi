package api

import (
	"encoding/json"
	"log"
	"net/http"

	"Argus-compound/go-sensory/internal/capture"
)

// ── Window management HTTP handlers ──────────────────────────

// registerWindowRoutes adds window listing and exclusion endpoints.
func registerWindowRoutes(mux *http.ServeMux, capturer capture.Capturer) {
	mux.HandleFunc("/api/windows", func(w http.ResponseWriter, r *http.Request) {
		handleListWindows(w, r, capturer)
	})
	mux.HandleFunc("/api/windows/exclude", func(w http.ResponseWriter, r *http.Request) {
		handleWindowExclude(w, r, capturer)
	})
	mux.HandleFunc("/api/windows/exclude/app", func(w http.ResponseWriter, r *http.Request) {
		handleExcludeApp(w, r, capturer)
	})
}

// GET /api/windows — list all visible windows
func handleListWindows(w http.ResponseWriter, r *http.Request, capturer capture.Capturer) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	windows, err := capturer.ListWindows()
	if err != nil {
		log.Printf("[Windows] ListWindows error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"windows": windows,
		"count":   len(windows),
	})
}

// GET    /api/windows/exclude — get current exclusion list
// POST   /api/windows/exclude — set excluded window IDs
// DELETE /api/windows/exclude — clear all exclusions
func handleWindowExclude(w http.ResponseWriter, r *http.Request, capturer capture.Capturer) {
	switch r.Method {
	case http.MethodGet:
		ids := capturer.GetExcludedWindows()
		writeJSON(w, http.StatusOK, map[string]any{
			"excluded_window_ids": ids,
		})

	case http.MethodPost:
		var req struct {
			WindowIDs []uint32 `json:"window_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON: " + err.Error(),
			})
			return
		}
		if err := capturer.SetExcludedWindows(req.WindowIDs); err != nil {
			log.Printf("[Windows] SetExcludedWindows error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}
		log.Printf("[Windows] Excluded %d window(s)", len(req.WindowIDs))
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "updated",
		})

	case http.MethodDelete:
		if err := capturer.SetExcludedWindows(nil); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}
		log.Printf("[Windows] Cleared all exclusions")
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "cleared",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// POST /api/windows/exclude/app — exclude all windows from a bundle ID
func handleExcludeApp(w http.ResponseWriter, r *http.Request, capturer capture.Capturer) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BundleID string `json:"bundle_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON: " + err.Error(),
		})
		return
	}
	if req.BundleID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "bundle_id is required",
		})
		return
	}

	if err := capturer.ExcludeApp(req.BundleID); err != nil {
		log.Printf("[Windows] ExcludeApp(%s) error: %v", req.BundleID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}
	log.Printf("[Windows] Excluded all windows from %s", req.BundleID)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "excluded",
		"bundle_id": req.BundleID,
	})
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
