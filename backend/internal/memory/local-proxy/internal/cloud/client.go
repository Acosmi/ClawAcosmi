// Package cloud — HTTP client for the cloud UHMS Algorithm API.
// Calls /api/v1/algo/* endpoints for pure computation (embed, classify, rank, reflect, extract).
// NEVER sends user data for storage — only for algorithm processing.
package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Client communicates with the cloud UHMS algo API.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient creates a new cloud algo API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// IsConfigured returns true if the client has a valid cloud URL.
func (c *Client) IsConfigured() bool {
	return c.baseURL != "" && c.apiKey != ""
}

// ============================================================================
// Request / Response types (mirror cloud algo package)
// ============================================================================

type EmbedRequest struct {
	Texts []string `json:"texts"`
}
type EmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Dimension  int         `json:"dimension"`
}

type ClassifyRequest struct {
	Content string `json:"content"`
}
type ClassifyResponse struct {
	Category        string  `json:"category"`
	ImportanceScore float64 `json:"importance_score"`
	Reasoning       string  `json:"reasoning,omitempty"`
}

type RankRequest struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}
type RankResult struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
}
type RankResponse struct {
	Results []RankResult `json:"results"`
}

type ReflectRequest struct {
	Memories          []string `json:"memories"`
	CoreMemoryContext string   `json:"core_memory_context,omitempty"`
}
type ReflectResponse struct {
	Reflection      string           `json:"reflection"`
	CoreMemoryEdits []CoreMemoryEdit `json:"core_memory_edits,omitempty"`
}
type CoreMemoryEdit struct {
	Section string `json:"section"`
	Content string `json:"content"`
	Mode    string `json:"mode"`
}

type ExtractRequest struct {
	Content string `json:"content"`
}
type ExtractedEntity struct {
	Name        string `json:"name"`
	EntityType  string `json:"entity_type"`
	Description string `json:"description,omitempty"`
}
type ExtractedRelation struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	RelationType string `json:"relation_type"`
}
type ExtractResponse struct {
	Entities  []ExtractedEntity   `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Embedding bool   `json:"embedding"`
	LLM       bool   `json:"llm"`
	Rerank    bool   `json:"rerank"`
}

// ============================================================================
// API Methods
// ============================================================================

// Embed calls POST /algo/embed to generate vector embeddings.
func (c *Client) Embed(ctx context.Context, texts []string) (*EmbedResponse, error) {
	var resp EmbedResponse
	if err := c.post(ctx, "/algo/embed", EmbedRequest{Texts: texts}, &resp); err != nil {
		return nil, fmt.Errorf("cloud.Embed: %w", err)
	}
	return &resp, nil
}

// Classify calls POST /algo/classify for NLP classification + importance scoring.
func (c *Client) Classify(ctx context.Context, content string) (*ClassifyResponse, error) {
	var resp ClassifyResponse
	if err := c.post(ctx, "/algo/classify", ClassifyRequest{Content: content}, &resp); err != nil {
		return nil, fmt.Errorf("cloud.Classify: %w", err)
	}
	return &resp, nil
}

// Rank calls POST /algo/rank for semantic reranking.
func (c *Client) Rank(ctx context.Context, query string, documents []string, topN int) (*RankResponse, error) {
	var resp RankResponse
	if err := c.post(ctx, "/algo/rank", RankRequest{Query: query, Documents: documents, TopN: topN}, &resp); err != nil {
		return nil, fmt.Errorf("cloud.Rank: %w", err)
	}
	return &resp, nil
}

// Reflect calls POST /algo/reflect to generate reflections.
func (c *Client) Reflect(ctx context.Context, memories []string, coreCtx string) (*ReflectResponse, error) {
	var resp ReflectResponse
	if err := c.post(ctx, "/algo/reflect", ReflectRequest{Memories: memories, CoreMemoryContext: coreCtx}, &resp); err != nil {
		return nil, fmt.Errorf("cloud.Reflect: %w", err)
	}
	return &resp, nil
}

// Extract calls POST /algo/extract for entity/relation extraction.
func (c *Client) Extract(ctx context.Context, content string) (*ExtractResponse, error) {
	var resp ExtractResponse
	if err := c.post(ctx, "/algo/extract", ExtractRequest{Content: content}, &resp); err != nil {
		return nil, fmt.Errorf("cloud.Extract: %w", err)
	}
	return &resp, nil
}

// Health checks the cloud algo API health.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var resp HealthResponse
	if err := c.get(ctx, "/algo/health", &resp); err != nil {
		return nil, fmt.Errorf("cloud.Health: %w", err)
	}
	return &resp, nil
}

// ============================================================================
// Internal HTTP helpers
// ============================================================================

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	return c.doRequest(req, out)
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	return c.doRequest(req, out)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("User-Agent", "uhms-local-proxy/1.0")
}

func (c *Client) doRequest(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		slog.Error("Cloud API error", "status", resp.StatusCode, "body", string(respBody))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
