// Package services — Multi-provider LLM client with adapter pattern.
// Mirrors Python services/llm_client.py — supports OpenAI-compatible providers.
// RUST_CANDIDATE: tokenizer — 分词器后续迁移 Rust
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/uhms/go-api/internal/config"
)

// LLMProvider defines the unified interface for all LLM operations.
// Implementations: LLMClient (OpenAI-compatible), future Rust-backed providers.
type LLMProvider interface {
	Generate(ctx context.Context, prompt string) (string, error)
	ExtractEntities(ctx context.Context, text string) (*ExtractionResult, error)
	ScoreImportance(ctx context.Context, text string) (*ImportanceScore, error)
	GenerateReflection(ctx context.Context, memories []string, coreMemoryContext string) (string, error)
}

// --- Prompts ---

const entityExtractionPrompt = `You are an entity extraction system. Extract entities and their relationships from the given text.

Text: %s

Return a JSON object with this exact structure:
{
    "entities": [
        {"name": "entity name", "entity_type": "type (Person/Technology/Concept/Organization/Location/Event/Other)", "description": "brief description"}
    ],
    "relations": [
        {"source": "entity1 name", "target": "entity2 name", "relation_type": "relationship type (e.g., USES, KNOWS, CREATED_BY, PART_OF, RELATED_TO)"}
    ]
}

Only include entities that are explicitly mentioned in the text. Be concise.`

const importanceScoringPrompt = `Rate the importance of this memory on a scale from 0.0 to 1.0.

Memory: %s

Consider:
- Is this factual information that might be referenced later? (higher score)
- Is this a preference or opinion? (medium score)
- Is this routine/trivial information? (lower score)
- Is this a significant event or decision? (higher score)

Return a JSON object:
{"score": 0.X, "reasoning": "brief explanation"}`

const reflectionPrompt = `Based on these recent memories, generate a higher-level insight or observation.
Also, evaluate whether the user's core memory (persona, preferences, instructions) should be updated based on the patterns you observe.

Memories:
%s

Current Core Memory:
%s

Generate a JSON response with this exact structure:
{
  "reflection": "Your reflection text here - synthesize patterns, draw conclusions about preferences/habits/trends. Be concise but informative.",
  "core_memory_edits": [
    {"section": "persona|preferences|instructions", "mode": "replace|append", "content": "new content"}
  ]
}

Rules for core_memory_edits:
1. Only include edits if the memories clearly indicate a change in user identity, preferences, or instructions
2. "section" must be one of: "persona", "preferences", "instructions"
3. Use "append" to add new information, "replace" to correct outdated information
4. If no core memory update is needed, use an empty array: "core_memory_edits": []
5. Be conservative - only edit when there is strong evidence from the memories

Return ONLY valid JSON, no markdown formatting.`

// --- Types ---

// ExtractedEntity represents an entity extracted by LLM.
type ExtractedEntity struct {
	Name        string `json:"name"`
	EntityType  string `json:"entity_type"`
	Description string `json:"description,omitempty"`
}

// ExtractedRelation represents a relation extracted by LLM.
type ExtractedRelation struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	RelationType string `json:"relation_type"`
}

// ExtractionResult holds extracted entities and relations.
type ExtractionResult struct {
	Entities  []ExtractedEntity   `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

// ImportanceScore holds the LLM-scored importance and reasoning.
type ImportanceScore struct {
	Score     float64 `json:"score"`
	Reasoning string  `json:"reasoning,omitempty"`
}

// --- OpenAI Chat API types ---

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// --- LLM Client ---

// LLMClient provides a unified interface for LLM operations.
// Uses OpenAI-compatible chat completions API (works with most providers).
type LLMClient struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewLLMClient creates an LLM client from config.
func NewLLMClient(cfg *config.Config) *LLMClient {
	baseURL := cfg.LLMBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	slog.Info("LLMClient initialized",
		"provider", cfg.LLMProvider,
		"model", cfg.LLMModel(),
		"base_url", baseURL,
	)

	return &LLMClient{
		apiKey:  cfg.LLMAPIKey(),
		baseURL: baseURL,
		model:   cfg.LLMModel(),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Generate sends a prompt to the LLM and returns the response text.
func (l *LLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: l.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.7,
		MaxTokens:   1000,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty LLM response")
	}
	return result.Choices[0].Message.Content, nil
}

// ExtractEntities uses LLM to extract entities and relations from text.
func (l *LLMClient) ExtractEntities(ctx context.Context, text string) (*ExtractionResult, error) {
	prompt := fmt.Sprintf(entityExtractionPrompt, text)

	response, err := l.Generate(ctx, prompt)
	if err != nil {
		slog.Error("Entity extraction failed", "error", err)
		return &ExtractionResult{}, nil
	}

	data, err := ParseLLMJSON(response)
	if err != nil {
		slog.Error("Failed to parse extraction response", "error", err)
		return &ExtractionResult{}, nil
	}

	var result ExtractionResult
	if entitiesRaw, ok := data["entities"]; ok {
		b, _ := json.Marshal(entitiesRaw)
		if err := json.Unmarshal(b, &result.Entities); err != nil {
			slog.Warn("Failed to unmarshal entities from LLM response", "error", err)
		}
	}
	if relationsRaw, ok := data["relations"]; ok {
		b, _ := json.Marshal(relationsRaw)
		if err := json.Unmarshal(b, &result.Relations); err != nil {
			slog.Warn("Failed to unmarshal relations from LLM response", "error", err)
		}
	}

	return &result, nil
}

// ScoreImportance uses LLM to score the importance of a memory.
func (l *LLMClient) ScoreImportance(ctx context.Context, text string) (*ImportanceScore, error) {
	prompt := fmt.Sprintf(importanceScoringPrompt, text)

	response, err := l.Generate(ctx, prompt)
	if err != nil {
		slog.Error("Importance scoring failed", "error", err)
		return &ImportanceScore{Score: 0.5, Reasoning: "Default score due to error"}, nil
	}

	data, err := ParseLLMJSON(response)
	if err != nil {
		return &ImportanceScore{Score: 0.5, Reasoning: "Default score due to parse error"}, nil
	}

	score := 0.5
	if s, ok := data["score"].(float64); ok {
		score = s
	}
	reasoning := ""
	if r, ok := data["reasoning"].(string); ok {
		reasoning = r
	}
	return &ImportanceScore{Score: score, Reasoning: reasoning}, nil
}

// GenerateReflection generates a reflection from multiple memories.
// coreMemoryContext 为当前用户核心记忆的文本快照，注入 Prompt 以启用自编辑检测。
func (l *LLMClient) GenerateReflection(ctx context.Context, memories []string, coreMemoryContext string) (string, error) {
	memoriesText := ""
	for _, m := range memories {
		memoriesText += "- " + m + "\n"
	}
	if coreMemoryContext == "" {
		coreMemoryContext = "(未设置核心记忆)"
	}
	prompt := fmt.Sprintf(reflectionPrompt, memoriesText, coreMemoryContext)

	response, err := l.Generate(ctx, prompt)
	if err != nil {
		slog.Error("Reflection generation failed", "error", err)
		return "", nil
	}
	return response, nil
}

// --- Robust JSON Parser ---

var (
	codeBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?\\s*```")
	jsonObjRe   = regexp.MustCompile(`(?s)\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
)

// ParseLLMJSON robustly parses JSON from LLM responses.
// Handles: markdown code blocks, raw JSON, JSON embedded in text.
func ParseLLMJSON(text string) (map[string]any, error) {
	text = strings.TrimSpace(text)

	// Strategy 1: Extract from markdown code blocks
	if matches := codeBlockRe.FindStringSubmatch(text); len(matches) > 1 {
		var result map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(matches[1])), &result); err == nil {
			return result, nil
		}
	}

	// Strategy 2: Direct parse
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err == nil {
		return result, nil
	}

	// Strategy 3: Find first JSON object in text
	if match := jsonObjRe.FindString(text); match != "" {
		if err := json.Unmarshal([]byte(match), &result); err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("could not extract valid JSON from LLM response: %.200s", text)
}

// --- Retry Wrapper ---

// retryLLMProvider wraps an LLMProvider with exponential-backoff retry.
type retryLLMProvider struct {
	inner      LLMProvider
	maxRetries int
}

func (r *retryLLMProvider) Generate(ctx context.Context, prompt string) (string, error) {
	var lastErr error
	for i := 0; i <= r.maxRetries; i++ {
		result, err := r.inner.Generate(ctx, prompt)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if i < r.maxRetries {
			time.Sleep(time.Duration(math.Pow(2, float64(i))) * 500 * time.Millisecond)
		}
	}
	return "", fmt.Errorf("LLM Generate failed after %d retries: %w", r.maxRetries+1, lastErr)
}

func (r *retryLLMProvider) ExtractEntities(ctx context.Context, text string) (*ExtractionResult, error) {
	return r.inner.ExtractEntities(ctx, text)
}

func (r *retryLLMProvider) ScoreImportance(ctx context.Context, text string) (*ImportanceScore, error) {
	return r.inner.ScoreImportance(ctx, text)
}

func (r *retryLLMProvider) GenerateReflection(ctx context.Context, memories []string, coreMemoryContext string) (string, error) {
	return r.inner.GenerateReflection(ctx, memories, coreMemoryContext)
}

// NewLLMProvider creates an LLMProvider with retry from config.
func NewLLMProvider(cfg *config.Config) LLMProvider {
	client := NewLLMClient(cfg)
	return &retryLLMProvider{inner: client, maxRetries: 2}
}

// --- Singleton (BUG-V7-02 修复: 使用 RWMutex 替代 sync.Once 重置竞态) ---

var (
	llmMu          sync.RWMutex
	llmClient      *LLMClient
	llmProviderSvc LLMProvider
)

// GetLLMClient returns the singleton LLMClient (concrete, for backward compat).
func GetLLMClient() *LLMClient {
	llmMu.RLock()
	if c := llmClient; c != nil {
		llmMu.RUnlock()
		return c
	}
	llmMu.RUnlock()

	llmMu.Lock()
	defer llmMu.Unlock()
	if llmClient == nil {
		llmClient = NewLLMClient(config.Get())
	}
	return llmClient
}

// GetLLMProvider returns the singleton LLMProvider (with retry).
func GetLLMProvider() LLMProvider {
	llmMu.RLock()
	if p := llmProviderSvc; p != nil {
		llmMu.RUnlock()
		return p
	}
	llmMu.RUnlock()

	llmMu.Lock()
	defer llmMu.Unlock()
	if llmProviderSvc == nil {
		llmProviderSvc = NewLLMProvider(config.Get())
	}
	return llmProviderSvc
}

// ReloadLLMClient forces recreation of LLM client and provider singletons.
func ReloadLLMClient() {
	llmMu.Lock()
	defer llmMu.Unlock()
	llmClient = nil
	llmProviderSvc = nil
	slog.Info("LLM client singletons reset for hot-reload")
}
