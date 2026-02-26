package vlm

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Router manages VLM providers and handles HTTP routing.
type Router struct {
	providers     map[string]Provider
	active        Provider
	config        *VLMConfig
	healthChecker *HealthChecker
}

// NewRouter creates a VLM router from configuration.
func NewRouter(cfg *VLMConfig) (*Router, error) {
	r := &Router{
		providers: make(map[string]Provider),
		config:    cfg,
	}

	for _, pc := range cfg.Providers {
		provider, err := createProvider(pc)
		if err != nil {
			log.Printf("[VLM] Warning: failed to create provider %q: %v", pc.Name, err)
			continue
		}

		r.providers[pc.Name] = provider
		log.Printf("[VLM] Registered provider: %s (type=%s, model=%s)", pc.Name, pc.Type, pc.Model)

		if pc.Active {
			r.active = provider
			log.Printf("[VLM] Active provider: %s", pc.Name)
		}
	}

	// Fallback: use first provider as active
	if r.active == nil && len(r.providers) > 0 {
		for name, p := range r.providers {
			r.active = p
			log.Printf("[VLM] Auto-selected active provider: %s", name)
			break
		}
	}

	if len(r.providers) == 0 {
		log.Printf("[VLM] Warning: no providers configured — VLM routes will return 503")
	}

	return r, nil
}

// ActiveProvider returns the currently active VLM provider for in-process calls.
// Returns nil if no provider is configured.
func (r *Router) ActiveProvider() Provider {
	return r.active
}

// createProvider instantiates a Provider from config.
func createProvider(cfg ProviderConfig) (Provider, error) {
	switch strings.ToLower(cfg.Type) {
	case "openai":
		return NewOpenAIProvider(cfg), nil
	case "gemini":
		return NewGeminiProvider(cfg), nil
	case "ollama":
		return NewOllamaProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}

// RegisterRoutes registers VLM HTTP routes on the given ServeMux.
func (r *Router) RegisterRoutes(mux *http.ServeMux) {
	// Chat completions
	mux.HandleFunc("/v1/chat/completions", r.handleChatCompletions)
	mux.HandleFunc("/api/vlm/health", r.handleHealth)

	// Config CRUD
	mux.HandleFunc("/api/config/providers", r.handleConfigProviders)
	mux.HandleFunc("/api/config/providers/", r.handleConfigProviderByName)

	log.Printf("[VLM] Routes registered: /v1/chat/completions, /api/vlm/health, /api/config/providers")
}

// handleChatCompletions handles the OpenAI-compatible chat completions endpoint.
func (r *Router) handleChatCompletions(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if r.active == nil {
		http.Error(w, `{"error":"no VLM provider configured"}`, http.StatusServiceUnavailable)
		return
	}

	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"reading request body: %s"}`, err), http.StatusBadRequest)
		return
	}

	var chatReq ChatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request JSON: %s"}`, err), http.StatusBadRequest)
		return
	}

	// Resolve provider: use active provider by default
	provider := r.active

	// Log the request
	log.Printf("[VLM] %s request: model=%s messages=%d stream=%v provider=%s",
		req.Method, chatReq.Model, len(chatReq.Messages), chatReq.Stream, provider.Name())

	if chatReq.Stream {
		r.handleStreamResponse(w, req, provider, chatReq)
	} else {
		r.handleNonStreamResponse(w, req, provider, chatReq)
	}
}

// handleNonStreamResponse handles a non-streaming chat completion.
func (r *Router) handleNonStreamResponse(w http.ResponseWriter, req *http.Request, provider Provider, chatReq ChatRequest) {
	resp, err := provider.ChatCompletion(req.Context(), chatReq)
	if err != nil {
		log.Printf("[VLM] Error from provider %s: %v", provider.Name(), err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleStreamResponse handles a streaming chat completion using SSE.
func (r *Router) handleStreamResponse(w http.ResponseWriter, req *http.Request, provider Provider, chatReq ChatRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	ch, err := provider.ChatCompletionStream(req.Context(), chatReq)
	if err != nil {
		log.Printf("[VLM] Stream error from provider %s: %v", provider.Name(), err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	for chunk := range ch {
		if chunk.Error != nil {
			log.Printf("[VLM] Stream chunk error: %v", chunk.Error)
			break
		}

		data, err := json.Marshal(chunk)
		if err != nil {
			log.Printf("[VLM] Failed to marshal chunk: %v", err)
			continue
		}

		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// handleHealth returns VLM module health status.
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	activeName := ""
	if r.active != nil {
		activeName = r.active.Name()
	}

	resp := map[string]any{
		"status":          "ok",
		"active_provider": activeName,
		"provider_count":  len(r.providers),
	}

	// Include health checker data if available
	if r.healthChecker != nil {
		resp["providers"] = r.healthChecker.Statuses()
	} else {
		// Fallback: basic provider info without health data
		type providerInfo struct {
			Name   string `json:"name"`
			Type   string `json:"type"`
			Model  string `json:"model"`
			Active bool   `json:"active"`
		}
		var providers []providerInfo
		for _, pc := range r.config.ListProviders() {
			providers = append(providers, providerInfo{
				Name:   pc.Name,
				Type:   pc.Type,
				Model:  pc.Model,
				Active: pc.Active,
			})
		}
		resp["providers"] = providers
	}

	json.NewEncoder(w).Encode(resp)
}

// Close releases all provider resources.
func (r *Router) Close() {
	if r.healthChecker != nil {
		r.healthChecker.Stop()
	}
	for name, p := range r.providers {
		if err := p.Close(); err != nil {
			log.Printf("[VLM] Error closing provider %s: %v", name, err)
		}
	}
}

// SetHealthChecker attaches a health checker to this router.
func (r *Router) SetHealthChecker(hc *HealthChecker) {
	r.healthChecker = hc
}

// ReloadProviders re-creates provider instances from the current config.
// This is called after config CRUD operations to apply changes.
func (r *Router) ReloadProviders() error {
	// Close existing providers
	for _, p := range r.providers {
		p.Close()
	}

	r.providers = make(map[string]Provider)
	r.active = nil

	for _, pc := range r.config.ListProviders() {
		provider, err := createProvider(pc)
		if err != nil {
			log.Printf("[VLM] Warning: failed to create provider %q: %v", pc.Name, err)
			continue
		}
		r.providers[pc.Name] = provider
		if pc.Active {
			r.active = provider
		}
	}

	// Fallback: use first provider as active
	if r.active == nil && len(r.providers) > 0 {
		for _, p := range r.providers {
			r.active = p
			break
		}
	}

	log.Printf("[VLM] Providers reloaded: %d active", len(r.providers))

	// Refresh health checker if attached
	if r.healthChecker != nil {
		r.healthChecker.Refresh()
	}

	return nil
}
