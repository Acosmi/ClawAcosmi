package vlm

// Provider configuration CRUD HTTP handlers.
// Split from router.go for single-responsibility compliance.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// handleConfigProviders handles GET (list) and POST (create) on /api/config/providers.
func (r *Router) handleConfigProviders(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch req.Method {
	case http.MethodGet:
		r.handleListProviders(w)
	case http.MethodPost:
		r.handleCreateProvider(w, req)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleConfigProviderByName handles PUT, DELETE, and PATCH on /api/config/providers/{name}.
func (r *Router) handleConfigProviderByName(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Extract provider name from URL path: /api/config/providers/{name}
	name := strings.TrimPrefix(req.URL.Path, "/api/config/providers/")
	if name == "" {
		http.Error(w, `{"error":"provider name required"}`, http.StatusBadRequest)
		return
	}

	switch req.Method {
	case http.MethodPut:
		r.handleUpdateProvider(w, req, name)
	case http.MethodDelete:
		r.handleDeleteProvider(w, name)
	case http.MethodPatch:
		r.handleSetActiveProvider(w, name)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleListProviders returns all provider configs (with API keys masked).
func (r *Router) handleListProviders(w http.ResponseWriter) {
	type providerView struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Endpoint string `json:"endpoint"`
		Model    string `json:"model"`
		Active   bool   `json:"active"`
		HasKey   bool   `json:"has_key"`
	}

	providers := r.config.ListProviders()
	views := make([]providerView, len(providers))
	for i, pc := range providers {
		views[i] = providerView{
			Name:     pc.Name,
			Type:     pc.Type,
			Endpoint: pc.Endpoint,
			Model:    pc.Model,
			Active:   pc.Active,
			HasKey:   pc.APIKey != "",
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"providers": views,
		"count":     len(views),
	})
}

// handleCreateProvider creates a new provider from JSON body.
func (r *Router) handleCreateProvider(w http.ResponseWriter, req *http.Request) {
	var pc ProviderConfig
	if err := json.NewDecoder(req.Body).Decode(&pc); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid JSON: %s"}`, err), http.StatusBadRequest)
		return
	}

	if pc.Name == "" || pc.Type == "" || pc.Endpoint == "" {
		http.Error(w, `{"error":"name, type, and endpoint are required"}`, http.StatusBadRequest)
		return
	}

	if err := r.config.AddProvider(pc); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusConflict)
		return
	}

	r.ReloadProviders()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created", "name": pc.Name})
}

// handleUpdateProvider updates an existing provider.
func (r *Router) handleUpdateProvider(w http.ResponseWriter, req *http.Request, name string) {
	var pc ProviderConfig
	if err := json.NewDecoder(req.Body).Decode(&pc); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid JSON: %s"}`, err), http.StatusBadRequest)
		return
	}

	if err := r.config.UpdateProvider(name, pc); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	r.ReloadProviders()

	json.NewEncoder(w).Encode(map[string]string{"status": "updated", "name": name})
}

// handleDeleteProvider deletes a provider by name.
func (r *Router) handleDeleteProvider(w http.ResponseWriter, name string) {
	if err := r.config.DeleteProvider(name); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	r.ReloadProviders()

	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "name": name})
}

// handleSetActiveProvider sets a provider as active (PATCH).
func (r *Router) handleSetActiveProvider(w http.ResponseWriter, name string) {
	if err := r.config.SetActive(name); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	r.ReloadProviders()

	json.NewEncoder(w).Encode(map[string]string{"status": "activated", "name": name})
}
