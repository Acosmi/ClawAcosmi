// Package services — Web 搜索服务。
// 为 L4 想象记忆提供真实外部搜索能力，替代 LLM 模拟。
// 支持 DeepSeek / Gemini 双引擎，搜索不可用时自动降级为 LLM 推理。
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/uhms/go-api/internal/config"
)

// ============================================================================
// WebSearchProvider 接口
// ============================================================================

// WebSearchProvider 定义外部搜索接口。
type WebSearchProvider interface {
	// Search 执行搜索查询，返回摘要、参考链接和使用的引擎。
	Search(ctx context.Context, query string) (*WebSearchResult, error)
}

// WebSearchResult 搜索结果。
type WebSearchResult struct {
	Summary    string   `json:"summary"`     // 搜索结果摘要
	SourceURLs []string `json:"source_urls"` // 参考链接证据链
	Provider   string   `json:"provider"`    // 实际使用的搜索引擎标识
}

// ============================================================================
// DeepSeek 搜索实现（通过 Chat API + 联网搜索能力）
// ============================================================================

// deepSeekSearcher 通过 DeepSeek Chat API 实现搜索。
// DeepSeek 模型自带联网搜索能力，使用特定 system prompt 引导产出结构化搜索结果。
type deepSeekSearcher struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

const deepSeekSearchSystemPrompt = `你是一个专业的信息检索助手，拥有联网搜索能力。请针对用户的问题进行深入搜索和分析。

要求：
1. 提供该领域 2-3 个最新发展动态
2. 给出主流观点或趋势判断
3. 指出潜在的机遇或风险

以简洁的要点形式输出，每点不超过两句话。在回答末尾，用 [来源] 标注你参考的网址（如有）。`

func newDeepSeekSearcher(cfg *config.Config) *deepSeekSearcher {
	baseURL := cfg.DeepSeekBaseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	apiKey := cfg.WebSearchAPIKey
	if apiKey == "" {
		apiKey = cfg.DeepSeekAPIKey
	}
	model := cfg.DeepSeekModel
	if model == "" {
		model = "deepseek-chat"
	}
	return &deepSeekSearcher{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (d *deepSeekSearcher) Search(ctx context.Context, query string) (*WebSearchResult, error) {
	if d.apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API key not configured")
	}

	body, err := json.Marshal(map[string]any{
		"model": d.model,
		"messages": []map[string]string{
			{"role": "system", "content": deepSeekSearchSystemPrompt},
			{"role": "user", "content": query},
		},
		"temperature": 0.3,
		"max_tokens":  1200,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", d.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DeepSeek API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty DeepSeek search response")
	}

	content := result.Choices[0].Message.Content
	summary, urls := extractSourceURLs(content)

	return &WebSearchResult{
		Summary:    summary,
		SourceURLs: urls,
		Provider:   "deepseek_search",
	}, nil
}

// ============================================================================
// Gemini 搜索实现（通过 Gemini API + Grounding with Google Search）
// ============================================================================

// geminiSearcher 通过 Google Gemini API 实现搜索。
// 利用 Gemini 的 Grounding with Google Search 功能获取真实搜索结果。
type geminiSearcher struct {
	apiKey string
	model  string
	client *http.Client
}

func newGeminiSearcher(cfg *config.Config) *geminiSearcher {
	apiKey := cfg.GeminiAPIKey
	model := cfg.GeminiModel
	if model == "" {
		model = "gemini-1.5-pro"
	}
	return &geminiSearcher{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// geminiRequest / geminiResponse — Gemini REST API 类型。
type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiTool struct {
	GoogleSearch map[string]any `json:"google_search,omitempty"`
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
	Tools    []geminiTool    `json:"tools,omitempty"`
}

type geminiCandidate struct {
	Content       geminiContent    `json:"content"`
	GroundingMeta *geminiGrounding `json:"groundingMetadata,omitempty"`
}

type geminiGrounding struct {
	WebSearchQueries  []string              `json:"webSearchQueries,omitempty"`
	GroundingChunks   []geminiGroundChunk   `json:"groundingChunks,omitempty"`
	GroundingSupports []geminiGroundSupport `json:"groundingSupports,omitempty"`
}

type geminiGroundChunk struct {
	Web *geminiWebChunk `json:"web,omitempty"`
}

type geminiWebChunk struct {
	URI   string `json:"uri"`
	Title string `json:"title"`
}

type geminiGroundSupport struct {
	GroundingChunkIndices []int     `json:"groundingChunkIndices,omitempty"`
	ConfidenceScores      []float64 `json:"confidenceScores,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

func (g *geminiSearcher) Search(ctx context.Context, query string) (*WebSearchResult, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("Gemini API key not configured")
	}

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{{Text: query}},
				Role:  "user",
			},
		},
		Tools: []geminiTool{
			{GoogleSearch: map[string]any{}},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.model, g.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("empty Gemini search response")
	}

	candidate := result.Candidates[0]

	// 提取文本摘要
	var summaryParts []string
	for _, p := range candidate.Content.Parts {
		if p.Text != "" {
			summaryParts = append(summaryParts, p.Text)
		}
	}

	// 提取 Grounding 来源链接
	var urls []string
	if candidate.GroundingMeta != nil {
		for _, chunk := range candidate.GroundingMeta.GroundingChunks {
			if chunk.Web != nil && chunk.Web.URI != "" {
				urls = append(urls, chunk.Web.URI)
			}
		}
	}

	return &WebSearchResult{
		Summary:    strings.Join(summaryParts, "\n"),
		SourceURLs: urls,
		Provider:   "gemini_search",
	}, nil
}

// ============================================================================
// FallbackSearcher — LLM 降级搜索
// ============================================================================

// fallbackSearcher 当搜索引擎不可用时，使用 LLM Generate 模拟搜索（MVP 行为）。
type fallbackSearcher struct {
	llm LLMProvider
}

func (f *fallbackSearcher) Search(ctx context.Context, query string) (*WebSearchResult, error) {
	prompt := fmt.Sprintf(externalContextPrompt, query)
	result, err := f.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM fallback search: %w", err)
	}
	return &WebSearchResult{
		Summary:    strings.TrimSpace(result),
		SourceURLs: []string{},
		Provider:   "llm_inference",
	}, nil
}

// ============================================================================
// AutoSearcher — 自动选择最佳搜索引擎
// ============================================================================

// autoSearcher 按优先级尝试搜索引擎，失败时自动降级。
type autoSearcher struct {
	primary   WebSearchProvider // DeepSeek 或 Gemini
	secondary WebSearchProvider // 另一个引擎
	fallback  WebSearchProvider // LLM 降级
}

func (a *autoSearcher) Search(ctx context.Context, query string) (*WebSearchResult, error) {
	// 尝试主引擎
	if a.primary != nil {
		result, err := a.primary.Search(ctx, query)
		if err == nil {
			return result, nil
		}
		slog.Warn("主搜索引擎失败，尝试备用", "error", err)
	}

	// 尝试备用引擎
	if a.secondary != nil {
		result, err := a.secondary.Search(ctx, query)
		if err == nil {
			return result, nil
		}
		slog.Warn("备用搜索引擎失败，降级为 LLM", "error", err)
	}

	// 降级为 LLM
	return a.fallback.Search(ctx, query)
}

// ============================================================================
// Helper: 从文本中提取 URL
// ============================================================================

// extractSourceURLs 从搜索结果文本中提取 [来源] 标注的 URL。
func extractSourceURLs(text string) (summary string, urls []string) {
	lines := strings.Split(text, "\n")
	var summaryLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 匹配 http/https 链接
		if strings.Contains(trimmed, "http://") || strings.Contains(trimmed, "https://") {
			// 提取所有 URL
			words := strings.Fields(trimmed)
			for _, w := range words {
				w = strings.Trim(w, "()[]{}<>,;:\"'")
				if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
					urls = append(urls, w)
				}
			}
		}
		summaryLines = append(summaryLines, line)
	}
	summary = strings.TrimSpace(strings.Join(summaryLines, "\n"))
	return
}

// ============================================================================
// 工厂函数与 Singleton
// ============================================================================

// NewWebSearchProvider 根据配置创建搜索提供者。
func NewWebSearchProvider(cfg *config.Config, llm LLMProvider) WebSearchProvider {
	fb := &fallbackSearcher{llm: llm}

	switch cfg.WebSearchProvider {
	case "deepseek":
		ds := newDeepSeekSearcher(cfg)
		if ds.apiKey == "" {
			slog.Warn("DeepSeek 搜索 API key 未配置，使用 LLM 降级")
			return fb
		}
		return &autoSearcher{primary: ds, fallback: fb}

	case "gemini":
		gm := newGeminiSearcher(cfg)
		if gm.apiKey == "" {
			slog.Warn("Gemini 搜索 API key 未配置，使用 LLM 降级")
			return fb
		}
		return &autoSearcher{primary: gm, fallback: fb}

	case "auto":
		ds := newDeepSeekSearcher(cfg)
		gm := newGeminiSearcher(cfg)
		var primary, secondary WebSearchProvider
		if ds.apiKey != "" {
			primary = ds
		}
		if gm.apiKey != "" {
			if primary == nil {
				primary = gm
			} else {
				secondary = gm
			}
		}
		if primary == nil {
			slog.Warn("无可用搜索引擎 API key，使用 LLM 降级")
			return fb
		}
		return &autoSearcher{primary: primary, secondary: secondary, fallback: fb}

	default:
		// 未配置或空值，使用 LLM 降级
		return fb
	}
}

// --- Singleton ---

var (
	webSearchMu  sync.RWMutex
	webSearchSvc WebSearchProvider
)

// GetWebSearchProvider 返回 WebSearchProvider 单例。
func GetWebSearchProvider() WebSearchProvider {
	webSearchMu.RLock()
	if s := webSearchSvc; s != nil {
		webSearchMu.RUnlock()
		return s
	}
	webSearchMu.RUnlock()

	webSearchMu.Lock()
	defer webSearchMu.Unlock()
	if webSearchSvc == nil {
		webSearchSvc = NewWebSearchProvider(config.Get(), GetLLMProvider())
	}
	return webSearchSvc
}
