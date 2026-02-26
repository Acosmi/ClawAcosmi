package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// OllamaEngine calls the local Ollama HTTP API for embedding.
// Uses /api/embed (not the legacy /api/embeddings) which supports batch input.
// Does NOT depend on the Ollama Go SDK to avoid heavy transitive dependencies.
type OllamaEngine struct {
	baseURL string // e.g. "http://localhost:11434"
	model   string // e.g. "all-minilm"
	dim     int    // cached dimension after first successful call
	http    *http.Client

	// Cached availability — refreshed at most every 5 seconds.
	mu            sync.RWMutex
	available     bool
	lastCheckTime time.Time
}

const (
	ollamaCheckInterval = 5 * time.Second
	ollamaCheckTimeout  = 2 * time.Second
	ollamaEmbedTimeout  = 30 * time.Second
)

// NewOllamaEngine creates an Ollama embedding adapter.
//
//   - baseURL: Ollama API base (default http://localhost:11434)
//   - model: embedding model name (default "all-minilm", 384-dim, 22MB)
func NewOllamaEngine(baseURL, model string) *OllamaEngine {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "all-minilm"
	}
	return &OllamaEngine{
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{Timeout: ollamaEmbedTimeout},
	}
}

func (o *OllamaEngine) Name() string { return "ollama" }

// Available checks whether the Ollama daemon is reachable.
// Result is cached for 5 seconds to avoid hammering the server on every embed call.
func (o *OllamaEngine) Available() bool {
	o.mu.RLock()
	if time.Since(o.lastCheckTime) < ollamaCheckInterval {
		avail := o.available
		o.mu.RUnlock()
		return avail
	}
	o.mu.RUnlock()

	// Need to refresh
	o.mu.Lock()
	defer o.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(o.lastCheckTime) < ollamaCheckInterval {
		return o.available
	}

	ctx, cancel := context.WithTimeout(context.Background(), ollamaCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/", nil)
	if err != nil {
		o.available = false
		o.lastCheckTime = time.Now()
		return false
	}

	resp, err := o.http.Do(req)
	if err != nil {
		o.available = false
		o.lastCheckTime = time.Now()
		slog.Debug("Ollama not available", "url", o.baseURL, "error", err)
		return false
	}
	resp.Body.Close()

	o.available = resp.StatusCode == http.StatusOK
	o.lastCheckTime = time.Now()
	if o.available {
		slog.Debug("Ollama is available", "url", o.baseURL)
	}
	return o.available
}

// ollamaEmbedRequest is the POST body for /api/embed.
type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// ollamaEmbedResponse is the response from /api/embed.
type ollamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed calls Ollama's /api/embed endpoint to generate vectors.
func (o *OllamaEngine) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(ollamaEmbedRequest{
		Model: o.model,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.http.Do(req)
	if err != nil {
		// Mark as unavailable on connection failure
		o.mu.Lock()
		o.available = false
		o.lastCheckTime = time.Now()
		o.mu.Unlock()
		return nil, fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result ollamaEmbedResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}

	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama: expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}

	// Cache dimension from first successful response
	if o.dim == 0 && len(result.Embeddings) > 0 && len(result.Embeddings[0]) > 0 {
		o.dim = len(result.Embeddings[0])
		slog.Info("Ollama embedding dimension detected", "model", o.model, "dim", o.dim)
	}

	return result.Embeddings, nil
}

// Dimension returns the cached vector dimension. Returns 0 until the first successful Embed() call.
func (o *OllamaEngine) Dimension() int {
	return o.dim
}

// Close is a no-op for the HTTP-based Ollama adapter.
func (o *OllamaEngine) Close() error {
	return nil
}
