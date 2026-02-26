// Package services — Rerank service with multi-provider support.
// Mirrors Python services/rerank/ — HTTP-based rerank for Cohere, SiliconFlow, Aliyun, Volcengine.
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

// RerankResult represents a single reranked document with index and score.
type RerankResult struct {
	Index int     `json:"index"`
	Score float64 `json:"relevance_score"`
}

// --- Provider Configs ---

type rerankProviderConfig struct {
	BaseURL    string
	Endpoint   string
	AuthHeader string
	AuthPrefix string
}

var rerankProviders = map[string]rerankProviderConfig{
	"cohere": {
		BaseURL: "https://api.cohere.ai/v1", Endpoint: "/rerank",
		AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	},
	"siliconflow": {
		BaseURL: "https://api.siliconflow.cn/v1", Endpoint: "/rerank",
		AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	},
	"aliyun": {
		BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Endpoint: "/rerank",
		AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	},
	"volcengine": {
		BaseURL: "https://ark.cn-beijing.volces.com/api/v3", Endpoint: "/rerank",
		AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	},
}

// --- Rerank Service ---

// RerankService provides document reranking via cloud APIs.
type RerankService struct {
	apiKey    string
	provider  string
	modelName string
	topN      int
	url       string
	config    rerankProviderConfig
	client    *http.Client
}

// NewRerankService creates a rerank service from config.
func NewRerankService(cfg *config.Config) (*RerankService, error) {
	if !cfg.RerankEnabled() {
		return nil, nil // Reranking disabled
	}

	provider := cfg.RerankProvider
	pc, ok := rerankProviders[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported rerank provider: %s", provider)
	}

	if cfg.RerankAPIKey == "" {
		return nil, fmt.Errorf("rerank API key is required (set RERANK_API_KEY)")
	}

	svc := &RerankService{
		apiKey:    cfg.RerankAPIKey,
		provider:  provider,
		modelName: cfg.RerankModelName(),
		topN:      cfg.RerankTopN,
		url:       pc.BaseURL + pc.Endpoint,
		config:    pc,
		client:    &http.Client{Timeout: 30 * time.Second},
	}

	slog.Info("RerankService initialized", "provider", provider, "model", cfg.RerankModelName())
	return svc, nil
}

// rerankRequest is the request body for the rerank API.
type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
}

// rerankResponse is the response from the rerank API.
type rerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Score          float64 `json:"score"` // Some providers use this field
	} `json:"results"`
}

// Rerank reorders documents by relevance to a query.
func (r *RerankService) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	if topN <= 0 {
		topN = r.topN
	}
	if topN > len(documents) {
		topN = len(documents)
	}

	body, err := json.Marshal(rerankRequest{
		Model:     r.modelName,
		Query:     query,
		Documents: documents,
		TopN:      topN,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(r.config.AuthHeader, r.config.AuthPrefix+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rerank API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result rerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]RerankResult, len(result.Results))
	for i, item := range result.Results {
		score := item.RelevanceScore
		if score == 0 {
			score = item.Score // Fallback field
		}
		results[i] = RerankResult{Index: item.Index, Score: score}
	}
	return results, nil
}

func (r *RerankService) Close() error { return nil }

// --- Singleton ---

var (
	rerankOnce    sync.Once
	rerankService *RerankService
)

// GetRerankService returns the singleton RerankService (may be nil if disabled).
func GetRerankService() *RerankService {
	rerankOnce.Do(func() {
		svc, err := NewRerankService(config.Get())
		if err != nil {
			slog.Warn("Rerank service creation failed", "error", err)
			return
		}
		rerankService = svc
	})
	return rerankService
}
