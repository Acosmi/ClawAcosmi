package llmclient

import (
	"context"
	"fmt"
	"strings"
)

// ---------- 统一分发入口 ----------

// StreamChat 根据 provider 路由到对应 API 客户端（流式）。
//
// onEvent 在每个流式事件到达时被调用。可传 nil 忽略流事件。
// 返回最终完整结果。
func StreamChat(ctx context.Context, req ChatRequest, onEvent func(StreamEvent)) (*ChatResult, error) {
	if onEvent == nil {
		onEvent = func(StreamEvent) {} // noop
	}

	provider := strings.ToLower(req.Provider)

	switch provider {
	case "anthropic":
		return anthropicStreamChat(ctx, req, onEvent)
	case "openai":
		return openaiStreamChat(ctx, req, onEvent)
	case "deepseek", "deepseek-reasoner":
		return openaiStreamChat(ctx, req, onEvent)
	case "ollama":
		return ollamaStreamChat(ctx, req, onEvent)
	case "gemini", "google", "google-gemini", "google-gemini-cli",
		"google-generative-ai", "google-antigravity":
		return geminiStreamChat(ctx, req, onEvent)
	default:
		// 尝试检测 API 兼容类型
		if isAnthropicCompatible(req.BaseURL) {
			return anthropicStreamChat(ctx, req, onEvent)
		}
		if isGeminiCompatible(req.BaseURL) {
			return geminiStreamChat(ctx, req, onEvent)
		}
		if isOpenAICompatible(req.BaseURL) {
			return openaiStreamChat(ctx, req, onEvent)
		}
		return nil, fmt.Errorf("llmclient: unsupported provider %q", req.Provider)
	}
}

// Chat 简单调用（内部汇聚流，不暴露增量事件）。
func Chat(ctx context.Context, req ChatRequest) (*ChatResult, error) {
	return StreamChat(ctx, req, nil)
}

// ---------- 兼容检测 ----------

// isAnthropicCompatible 检查 BaseURL 是否为 Anthropic 兼容 API。
func isAnthropicCompatible(baseURL string) bool {
	if baseURL == "" {
		return false
	}
	lower := strings.ToLower(baseURL)
	return strings.Contains(lower, "anthropic") ||
		strings.Contains(lower, "/v1/messages")
}

// isOpenAICompatible 检查 BaseURL 是否为 OpenAI 兼容 API。
func isOpenAICompatible(baseURL string) bool {
	if baseURL == "" {
		return false
	}
	lower := strings.ToLower(baseURL)
	return strings.Contains(lower, "openai") ||
		strings.Contains(lower, "/v1/chat")
}
