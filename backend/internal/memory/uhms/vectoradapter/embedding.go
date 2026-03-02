package vectoradapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HTTPEmbeddingProvider implements uhms.EmbeddingProvider via HTTP API.
// Supports OpenAI, Ollama, Gemini, Qwen (DashScope), Voyage, and other OpenAI-compatible endpoints.
type HTTPEmbeddingProvider struct {
	provider  string // "openai" | "ollama" | "gemini" | "qwen" | "voyage"
	model     string // e.g. "text-embedding-3-small"
	baseURL   string
	apiKey    string
	dimension int
	dimOnce   sync.Once // guards one-time auto-detection of dimension
	client    *http.Client
}

// NewHTTPEmbeddingProvider creates a new HTTP embedding provider.
// provider: "openai", "ollama", "gemini", "voyage", etc.
// model: embedding model name (e.g. "text-embedding-3-small").
// baseURL: API base URL (empty = provider default).
// apiKey: API key (empty for local providers like Ollama).
func NewHTTPEmbeddingProvider(provider, model, baseURL, apiKey string) (*HTTPEmbeddingProvider, error) {
	if provider == "" {
		provider = "openai"
	}
	if model == "" {
		model = defaultModelForProvider(provider)
	}
	if baseURL == "" {
		baseURL = defaultBaseURLForProvider(provider)
	}

	dim := inferDimension(provider, model)

	return &HTTPEmbeddingProvider{
		provider:  provider,
		model:     model,
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		dimension: dim,
		client:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Embed generates an embedding vector for the given text.
// Retries once on transient errors (5xx, timeout) with 1s backoff.
func (e *HTTPEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("vectoradapter/embedding: text must not be empty")
	}

	vec, err := e.doEmbed(ctx, text)
	if err != nil && isRetryable(err) {
		// Single retry with 1s backoff.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
		}
		vec, err = e.doEmbed(ctx, text)
	}
	return vec, err
}

func (e *HTTPEmbeddingProvider) doEmbed(ctx context.Context, text string) ([]float32, error) {
	switch e.provider {
	case "ollama":
		return e.embedOllama(ctx, text)
	case "gemini":
		return e.embedGemini(ctx, text)
	default:
		return e.embedOpenAI(ctx, text)
	}
}

// isRetryable returns true for transient HTTP errors (5xx, timeout, connection reset).
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// 5xx server errors
	if strings.Contains(s, "HTTP 5") {
		return true
	}
	// Network errors
	for _, keyword := range []string{"timeout", "connection reset", "connection refused", "EOF", "broken pipe"} {
		if strings.Contains(s, keyword) {
			return true
		}
	}
	return false
}

// Dimension returns the embedding vector dimension.
func (e *HTTPEmbeddingProvider) Dimension() int {
	return e.dimension
}

// embedOpenAI calls the OpenAI-compatible /v1/embeddings endpoint.
// Works with OpenAI, Voyage, and other compatible providers.
func (e *HTTPEmbeddingProvider) embedOpenAI(ctx context.Context, text string) ([]float32, error) {
	body := map[string]interface{}{
		"input": text,
		"model": e.model,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: marshal request: %w", err)
	}

	url := e.baseURL + "/v1/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MiB limit
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vectoradapter/embedding: HTTP %d: %s", resp.StatusCode, truncateStr(string(respBody), 200))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: parse response: %w", err)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("vectoradapter/embedding: empty embedding in response")
	}

	vec := result.Data[0].Embedding

	// Auto-detect dimension on first successful call if not pre-configured.
	e.dimOnce.Do(func() {
		if e.dimension == 0 {
			e.dimension = len(vec)
		}
	})

	return vec, nil
}

// embedGemini calls the Google Gemini /v1beta/models/{model}:embedContent endpoint.
func (e *HTTPEmbeddingProvider) embedGemini(ctx context.Context, text string) ([]float32, error) {
	body := map[string]interface{}{
		"model": "models/" + e.model,
		"content": map[string]interface{}{
			"parts": []map[string]string{{"text": text}},
		},
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: marshal request: %w", err)
	}

	url := e.baseURL + "/v1beta/models/" + e.model + ":embedContent?key=" + e.apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vectoradapter/embedding: HTTP %d: %s", resp.StatusCode, truncateStr(string(respBody), 200))
	}

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: parse response: %w", err)
	}
	if len(result.Embedding.Values) == 0 {
		return nil, fmt.Errorf("vectoradapter/embedding: empty embedding in response")
	}

	vec := result.Embedding.Values

	e.dimOnce.Do(func() {
		if e.dimension == 0 {
			e.dimension = len(vec)
		}
	})

	return vec, nil
}

// embedOllama calls the Ollama /api/embeddings endpoint.
func (e *HTTPEmbeddingProvider) embedOllama(ctx context.Context, text string) ([]float32, error) {
	body := map[string]interface{}{
		"model":  e.model,
		"prompt": text,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: marshal request: %w", err)
	}

	url := e.baseURL + "/api/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vectoradapter/embedding: HTTP %d: %s", resp.StatusCode, truncateStr(string(respBody), 200))
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("vectoradapter/embedding: parse response: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("vectoradapter/embedding: empty embedding in response")
	}

	vec := result.Embedding

	e.dimOnce.Do(func() {
		if e.dimension == 0 {
			e.dimension = len(vec)
		}
	})

	return vec, nil
}

// Probe sends a short test embedding request to verify connectivity.
// Returns nil on success, or an error describing the failure.
func (e *HTTPEmbeddingProvider) Probe(ctx context.Context) error {
	_, err := e.doEmbed(ctx, "probe")
	if err != nil {
		return fmt.Errorf("vectoradapter/probe[%s]: %w", e.provider, err)
	}
	return nil
}

// Provider returns the provider name.
func (e *HTTPEmbeddingProvider) Provider() string { return e.provider }

// Model returns the model name.
func (e *HTTPEmbeddingProvider) Model() string { return e.model }

// inferDimension returns the known embedding dimension for common provider+model pairs.
func inferDimension(provider, model string) int {
	known := map[string]int{
		"text-embedding-3-small": 1536,
		"text-embedding-3-large": 3072,
		"text-embedding-ada-002": 1536,
		"text-embedding-004":     768,  // Gemini
		"text-embedding-v3":      1024, // Qwen (DashScope)
		"text-embedding-v2":      1536, // Qwen (DashScope)
		"nomic-embed-text":       768,  // Ollama
		"mxbai-embed-large":      1024, // Ollama
		"BAAI/bge-small-zh-v1.5": 512,
		"voyage-3":               1024,
		"voyage-3-lite":          512,
		"voyage-code-3":          1024,
	}
	if dim, ok := known[model]; ok {
		return dim
	}
	switch provider {
	case "openai":
		return 1536
	case "ollama":
		return 768
	case "gemini":
		return 768
	case "qwen":
		return 1024
	case "voyage":
		return 1024
	default:
		return 1536
	}
}

// defaultModelForProvider returns the default embedding model for a given provider.
func defaultModelForProvider(provider string) string {
	switch provider {
	case "openai":
		return "text-embedding-3-small"
	case "ollama":
		return "nomic-embed-text"
	case "gemini":
		return "text-embedding-004"
	case "qwen":
		return "text-embedding-v3"
	case "voyage":
		return "voyage-3"
	default:
		return "text-embedding-3-small"
	}
}

// defaultBaseURLForProvider returns the default API base URL for a given provider.
func defaultBaseURLForProvider(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com"
	case "ollama":
		return "http://localhost:11434"
	case "gemini":
		return "https://generativelanguage.googleapis.com"
	case "qwen":
		return "https://dashscope.aliyuncs.com/compatible-mode"
	case "voyage":
		return "https://api.voyageai.com"
	default:
		return "https://api.openai.com"
	}
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
