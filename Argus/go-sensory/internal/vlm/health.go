package vlm

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HealthStatus represents the health state of a single provider.
type HealthStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Model     string `json:"model"`
	Active    bool   `json:"active"`
	Healthy   bool   `json:"healthy"`
	Latency   int64  `json:"latency_ms"` // last check latency in milliseconds
	LastCheck string `json:"last_check"` // ISO 8601 timestamp
	Error     string `json:"error,omitempty"`
}

// HealthChecker periodically pings VLM provider endpoints.
type HealthChecker struct {
	router   *Router
	interval time.Duration
	client   *http.Client

	mu       sync.RWMutex
	statuses map[string]*HealthStatus

	stopCh chan struct{}
	done   chan struct{}
}

// NewHealthChecker creates a health checker that pings providers at the given interval.
func NewHealthChecker(router *Router, interval time.Duration) *HealthChecker {
	// Use a transport that bypasses proxy for local endpoints (e.g. Ollama)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	hasLocal := false
	for _, pc := range router.config.ListProviders() {
		if isLocalEndpoint(pc.Endpoint) {
			hasLocal = true
			break
		}
	}
	if hasLocal {
		transport.Proxy = nil
	}

	hc := &HealthChecker{
		router:   router,
		interval: interval,
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		statuses: make(map[string]*HealthStatus),
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}

	// Initialize statuses from config
	for _, pc := range router.config.ListProviders() {
		hc.statuses[pc.Name] = &HealthStatus{
			Name:    pc.Name,
			Type:    pc.Type,
			Model:   pc.Model,
			Active:  pc.Active,
			Healthy: false,
			Error:   "pending first check",
		}
	}

	return hc
}

// Start begins the periodic health checking loop.
func (hc *HealthChecker) Start() {
	log.Printf("[VLM Health] Starting health checker (interval=%s)", hc.interval)

	// Run initial check immediately
	hc.checkAll()

	go func() {
		defer close(hc.done)
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hc.checkAll()
			case <-hc.stopCh:
				log.Printf("[VLM Health] Health checker stopped")
				return
			}
		}
	}()
}

// Stop stops the health checker.
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
	<-hc.done
}

// Statuses returns the current health statuses for all providers.
func (hc *HealthChecker) Statuses() []HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make([]HealthStatus, 0, len(hc.statuses))
	for _, s := range hc.statuses {
		result = append(result, *s)
	}
	return result
}

// Refresh re-initializes statuses from the current config and checks immediately.
// Called after CRUD operations on providers.
func (hc *HealthChecker) Refresh() {
	hc.mu.Lock()
	// Rebuild statuses map from current config
	newStatuses := make(map[string]*HealthStatus)
	for _, pc := range hc.router.config.ListProviders() {
		if existing, ok := hc.statuses[pc.Name]; ok {
			// Keep existing status but update metadata
			existing.Active = pc.Active
			existing.Type = pc.Type
			existing.Model = pc.Model
			newStatuses[pc.Name] = existing
		} else {
			newStatuses[pc.Name] = &HealthStatus{
				Name:    pc.Name,
				Type:    pc.Type,
				Model:   pc.Model,
				Active:  pc.Active,
				Healthy: false,
				Error:   "pending first check",
			}
		}
	}
	hc.statuses = newStatuses
	hc.mu.Unlock()

	// Check all immediately in background
	go hc.checkAll()
}

// checkAll pings every registered provider.
func (hc *HealthChecker) checkAll() {
	providers := hc.router.config.ListProviders()

	var wg sync.WaitGroup
	for _, pc := range providers {
		wg.Add(1)
		go func(pc ProviderConfig) {
			defer wg.Done()
			hc.checkOne(pc)
		}(pc)
	}
	wg.Wait()
}

// checkOne pings a single provider endpoint.
func (hc *HealthChecker) checkOne(pc ProviderConfig) {
	start := time.Now()
	healthy := true
	errMsg := ""

	// Build a simple probe request based on provider type
	probeURL := buildProbeURL(pc)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		healthy = false
		errMsg = fmt.Sprintf("build request: %v", err)
	} else {
		if pc.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+pc.APIKey)
		}

		resp, err := hc.client.Do(req)
		if err != nil {
			healthy = false
			errMsg = fmt.Sprintf("connection failed: %v", err)
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 500 {
				healthy = false
				errMsg = fmt.Sprintf("server error: HTTP %d", resp.StatusCode)
			}
			// 2xx, 3xx, 4xx (like 401/404) all indicate the endpoint is reachable
		}
	}

	latency := time.Since(start).Milliseconds()

	hc.mu.Lock()
	if s, ok := hc.statuses[pc.Name]; ok {
		s.Healthy = healthy
		s.Latency = latency
		s.LastCheck = time.Now().Format(time.RFC3339)
		s.Error = errMsg
		s.Active = pc.Active
	}
	hc.mu.Unlock()

	if !healthy {
		log.Printf("[VLM Health] %s: unhealthy (%s) latency=%dms", pc.Name, errMsg, latency)
	}
}

// buildProbeURL returns a lightweight URL to ping for the given provider type.
func buildProbeURL(pc ProviderConfig) string {
	switch pc.Type {
	case "openai":
		// GET /models is lightweight and verifies auth
		return pc.Endpoint + "/models"
	case "gemini":
		// GET models list endpoint
		return pc.Endpoint + "/models?key=" + pc.APIKey
	case "ollama":
		// GET /api/tags lists models and verifies Ollama is running
		return strings.TrimSuffix(pc.Endpoint, "/v1") + "/api/tags"
	default:
		return pc.Endpoint
	}
}
