// Package services — Embedding service with factory pattern and hot reload.
// Mirrors Python services/embeddings/ — base interface + OpenAI-compatible adapter + factory.
// RUST_CANDIDATE: vector_ops — 向量计算后续迁移 Rust (nexus-vector)
package services

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

	"github.com/uhms/go-api/internal/config"
)

// EmbeddingService defines the interface for embedding providers.
type EmbeddingService interface {
	// EmbedDocuments generates embeddings for a batch of texts.
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
	// EmbedQuery generates an embedding for a single query.
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	// Dimension returns the embedding vector dimension.
	Dimension() int
	// Close releases resources.
	Close() error
}

// --- OpenAI-Compatible Embedding ---

// openAIEmbeddingRequest is the request body for /v1/embeddings.
type openAIEmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

// openAIEmbeddingResponse is the response from /v1/embeddings.
type openAIEmbeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// OpenAICompatibleEmbedding is a universal adapter for OpenAI-compatible embedding APIs.
// Supports: OpenAI, Aliyun DashScope, Volcengine Doubao, Tencent, DeepSeek, Azure.
type OpenAICompatibleEmbedding struct {
	apiKey    string
	baseURL   string
	modelName string
	dim       int
	client    *http.Client
}

// NewOpenAICompatibleEmbedding creates a new embedding service.
func NewOpenAICompatibleEmbedding(apiKey, baseURL, modelName string, dimension int) (*OpenAICompatibleEmbedding, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("embedding API key is required (set EMBEDDING_API_KEY)")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	slog.Info("OpenAICompatibleEmbedding initialized",
		"base_url", baseURL,
		"model", modelName,
		"dimension", dimension,
	)
	return &OpenAICompatibleEmbedding{
		apiKey:    apiKey,
		baseURL:   baseURL,
		modelName: modelName,
		dim:       dimension,
		client:    &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (e *OpenAICompatibleEmbedding) Dimension() int { return e.dim }

// EmbedDocuments generates embeddings for multiple texts via POST /v1/embeddings.
func (e *OpenAICompatibleEmbedding) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(openAIEmbeddingRequest{Input: texts, Model: e.modelName})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Sort by index to match input order
	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}
	return embeddings, nil
}

// EmbedQuery generates an embedding for a single query text.
func (e *OpenAICompatibleEmbedding) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("query text cannot be empty")
	}
	results, err := e.EmbedDocuments(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	return results[0], nil
}

func (e *OpenAICompatibleEmbedding) Close() error { return nil }

// --- Factory ---

// CreateEmbeddingService creates an embedding service based on config.
func CreateEmbeddingService(cfg *config.Config) (EmbeddingService, error) {
	provider := cfg.EmbeddingProvider
	slog.Info("Creating embedding service", "provider", provider)

	switch provider {
	case "local":
		// Local embedding requires a separate service (e.g., sentence-transformers server)
		// For now, use OpenAI-compatible with local endpoint
		return NewOpenAICompatibleEmbedding(
			"local",
			"http://localhost:8080/v1",
			cfg.EmbeddingModelName,
			cfg.EmbeddingDimension,
		)
	case "openai", "aliyun", "volcengine", "tencent", "deepseek", "azure":
		return NewOpenAICompatibleEmbedding(
			cfg.EmbeddingAPIKey,
			cfg.EmbeddingBaseURL,
			cfg.EmbeddingModelName,
			cfg.EmbeddingDimension,
		)
	case "google":
		// Google uses a different endpoint format
		return NewOpenAICompatibleEmbedding(
			cfg.EmbeddingAPIKey,
			"https://generativelanguage.googleapis.com/v1beta",
			cfg.EmbeddingModelName,
			cfg.EmbeddingDimension,
		)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", provider)
	}
}

// --- Singleton with Hot Reload ---

var (
	embeddingMu      sync.RWMutex
	embeddingService EmbeddingService
	embeddingHash    string
)

func embeddingConfigHash(cfg *config.Config) string {
	return fmt.Sprintf("%s:%s:%s", cfg.EmbeddingProvider, cfg.EmbeddingModelName, cfg.EmbeddingBaseURL)
}

// GetEmbeddingService returns the singleton, recreating if config changed.
func GetEmbeddingService() EmbeddingService {
	embeddingMu.RLock()
	svc := embeddingService
	embeddingMu.RUnlock()
	if svc != nil {
		return svc
	}
	return ReloadEmbeddingService()
}

// ReloadEmbeddingService forces recreation of the embedding service.
func ReloadEmbeddingService() EmbeddingService {
	embeddingMu.Lock()
	defer embeddingMu.Unlock()

	cfg := config.Get()
	newHash := embeddingConfigHash(cfg)

	if embeddingService != nil && newHash == embeddingHash {
		return embeddingService
	}

	if embeddingService != nil {
		embeddingService.Close()
		slog.Info("Embedding service config changed, recreating")
	}

	svc, err := CreateEmbeddingService(cfg)
	if err != nil {
		slog.Error("Failed to create embedding service", "error", err)
		return nil
	}

	embeddingService = svc
	embeddingHash = newHash
	return embeddingService
}
