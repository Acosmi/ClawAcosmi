package uhms

// claude_integration.go — Claude/Anthropic API 特化压缩策略
//
// 功能:
//   1. Anthropic Compaction API (Beta): 服务端摘要压缩，无额外推理成本
//   2. Prompt Caching 标注: cache_control ephemeral 支持 (需 llmclient 扩展)
//   3. Provider 感知的压缩策略选择

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ============================================================================
// Provider-Aware Compression Strategy
// ============================================================================

// CompressionStrategy determines which compression method to use.
type CompressionStrategy string

const (
	// StrategyAnthropicCompaction uses Anthropic's server-side compaction API (beta).
	// No additional inference cost; supported for Anthropic provider only.
	StrategyAnthropicCompaction CompressionStrategy = "anthropic_compaction"

	// StrategyLocalLLM uses local LLM summarization via the configured provider.
	// Works with any provider (Anthropic/OpenAI/Ollama/etc).
	StrategyLocalLLM CompressionStrategy = "local_llm"

	// StrategyTruncate falls back to simple truncation when no LLM is available.
	StrategyTruncate CompressionStrategy = "truncate"
)

// SelectCompressionStrategy picks the best compression strategy based on provider.
func SelectCompressionStrategy(provider string, hasLLM bool) CompressionStrategy {
	if strings.ToLower(provider) == "anthropic" {
		return StrategyAnthropicCompaction
	}
	if hasLLM {
		return StrategyLocalLLM
	}
	return StrategyTruncate
}

// ============================================================================
// Anthropic Compaction API Client (Beta)
// ============================================================================

// AnthropicCompactionClient calls the Anthropic Messages Compaction API.
//
// API: POST /v1/messages/compaction (beta)
// Header: anthropic-beta: prompt-caching-2024-07-31
// Docs: https://platform.claude.com/docs/en/build-with-claude/compaction
type AnthropicCompactionClient struct {
	APIKey  string
	BaseURL string // default: https://api.anthropic.com
}

const (
	defaultCompactionBaseURL = "https://api.anthropic.com"
	anthropicCompactVersion  = "2023-06-01"
	compactionBeta           = "prompt-caching-2024-07-31"
	compactionTimeout        = 60 * time.Second
)

// compactionRequest is the Anthropic compaction API request body.
type compactionRequest struct {
	Model        string              `json:"model"`
	SystemPrompt string              `json:"system,omitempty"`
	Messages     []compactionMessage `json:"messages"`
	ContextLimit int                 `json:"context_limit,omitempty"` // target token budget
}

type compactionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// compactionResponse is the Anthropic compaction API response.
type compactionResponse struct {
	Summary string `json:"summary"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Compact sends a conversation to the Anthropic compaction API and returns a summary.
//
// The compaction API summarizes the conversation server-side, preserving key context.
// It's designed specifically for context window management in long-running agents.
func (c *AnthropicCompactionClient) Compact(ctx context.Context, systemPrompt string, messages []Message, contextLimit int) (string, error) {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = defaultCompactionBaseURL
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/v1/messages/compaction"

	// Convert messages
	compactMsgs := make([]compactionMessage, len(messages))
	for i, m := range messages {
		compactMsgs[i] = compactionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	reqBody := compactionRequest{
		Model:        "claude-sonnet-4-5-20250514", // 使用高性价比模型做压缩
		SystemPrompt: systemPrompt,
		Messages:     compactMsgs,
		ContextLimit: contextLimit,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("uhms/compaction: marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, compactionTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("uhms/compaction: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicCompactVersion)
	httpReq.Header.Set("anthropic-beta", compactionBeta)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("uhms/compaction: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("uhms/compaction: API error %d: %s", resp.StatusCode, string(bodyData))
	}

	var compactResp compactionResponse
	// 限制响应体大小 (10 MB) 防止异常大响应
	limitedBody := io.LimitReader(resp.Body, 10*1024*1024)
	if err := json.NewDecoder(limitedBody).Decode(&compactResp); err != nil {
		return "", fmt.Errorf("uhms/compaction: decode response: %w", err)
	}

	slog.Debug("uhms/compaction: success",
		"inputTokens", compactResp.Usage.InputTokens,
		"outputTokens", compactResp.Usage.OutputTokens,
	)
	return compactResp.Summary, nil
}

// ============================================================================
// Prompt Caching Helper
// ============================================================================

// PromptCacheBlock represents a system prompt content block with cache_control.
// Anthropic Prompt Caching (2024-07-31 beta) caches system prompt + tool definitions
// to reduce latency by ~85% and cost.
//
// Usage: set cache_control: {"type": "ephemeral"} on system message content blocks.
// The API then caches the prompt for 5min (auto-renewed on hit).
//
// NOTE: This requires llmclient to support content-block format for system prompts
// (currently llmclient sends system as a plain string). When llmclient is extended
// to support cache_control, use BuildCacheableSystemBlocks() to construct the blocks.
type PromptCacheBlock struct {
	Type         string            `json:"type"`
	Text         string            `json:"text,omitempty"`
	CacheControl *CacheControlSpec `json:"cache_control,omitempty"`
}

// CacheControlSpec specifies the caching strategy.
type CacheControlSpec struct {
	Type string `json:"type"` // "ephemeral"
}

// BuildCacheableSystemBlocks constructs system prompt content blocks with cache_control.
//
// Strategy: split system prompt into stable prefix (cached) + dynamic suffix (not cached).
// - Block 1: base system instructions + tool definitions → cache_control: ephemeral
// - Block 2: skills snapshot + memory context → no cache (changes per turn)
//
// This is a preparation helper. To activate, llmclient.ChatRequest needs to accept
// []PromptCacheBlock instead of string for SystemPrompt.
func BuildCacheableSystemBlocks(stablePrefix, dynamicSuffix string) []PromptCacheBlock {
	blocks := []PromptCacheBlock{
		{
			Type: "text",
			Text: stablePrefix,
			CacheControl: &CacheControlSpec{
				Type: "ephemeral",
			},
		},
	}
	if dynamicSuffix != "" {
		blocks = append(blocks, PromptCacheBlock{
			Type: "text",
			Text: dynamicSuffix,
		})
	}
	return blocks
}

// ============================================================================
// Integrated Compression with Provider Awareness
// ============================================================================

// CompressWithStrategy applies the appropriate compression strategy.
//
// For Anthropic provider: tries Compaction API first, falls back to local LLM.
// For other providers: uses local LLM summarization.
// Without LLM: simple truncation.
func CompressWithStrategy(
	ctx context.Context,
	strategy CompressionStrategy,
	systemPrompt string,
	messages []Message,
	tokenBudget int,
	compactionClient *AnthropicCompactionClient,
	localLLM LLMProvider,
) (string, error) {
	switch strategy {
	case StrategyAnthropicCompaction:
		if compactionClient != nil {
			summary, err := compactionClient.Compact(ctx, systemPrompt, messages, tokenBudget)
			if err != nil {
				slog.Warn("uhms: Anthropic compaction failed, falling back to local LLM",
					"error", err)
				// Fall through to local LLM
			} else {
				return summary, nil
			}
		}
		// Fallback to local LLM
		if localLLM != nil {
			return summarizeWithLocalLLM(ctx, localLLM, messages)
		}
		return truncateMessages(messages, tokenBudget), nil

	case StrategyLocalLLM:
		if localLLM != nil {
			return summarizeWithLocalLLM(ctx, localLLM, messages)
		}
		return truncateMessages(messages, tokenBudget), nil

	case StrategyTruncate:
		return truncateMessages(messages, tokenBudget), nil

	default:
		return truncateMessages(messages, tokenBudget), nil
	}
}

// summarizeWithLocalLLM uses any LLM provider to generate a conversation summary.
func summarizeWithLocalLLM(ctx context.Context, llm LLMProvider, messages []Message) (string, error) {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", msg.Role, msg.Content))
	}

	prompt := fmt.Sprintf(SummarizeNewPromptFmt, sb.String(), StructuredSummaryTemplate)

	return llm.Complete(ctx, SummarizeSystemPrompt, prompt)
}

// truncateMessages creates a simple text summary by truncating messages.
func truncateMessages(messages []Message, _ int) string {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", msg.Role, truncate(msg.Content, 200)))
	}
	return sb.String()
}
