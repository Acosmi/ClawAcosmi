package uhms

import (
	"context"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/llmclient"
)

// LLMClientAdapter bridges llmclient.StreamChat to uhms.LLMProvider.
// It performs non-streaming completions by collecting the full stream output.
type LLMClientAdapter struct {
	Provider string // "anthropic" | "openai" | "ollama" | ...
	Model    string // model ID
	APIKey   string
	BaseURL  string // optional custom base URL
}

var _ LLMProvider = (*LLMClientAdapter)(nil)

// Complete sends a prompt to the LLM and returns the completion text.
func (a *LLMClientAdapter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var sb strings.Builder
	result, err := llmclient.StreamChat(ctx, llmclient.ChatRequest{
		Provider:     a.Provider,
		Model:        a.Model,
		SystemPrompt: systemPrompt,
		Messages: []llmclient.ChatMessage{
			llmclient.TextMessage("user", userPrompt),
		},
		MaxTokens: 4096,
		APIKey:    a.APIKey,
		BaseURL:   a.BaseURL,
	}, func(evt llmclient.StreamEvent) {
		if evt.Type == llmclient.EventText {
			sb.WriteString(evt.Text)
		}
	})
	if err != nil {
		return "", err
	}

	// 优先使用流式收集的文本，回退到 result 中的文本块
	text := sb.String()
	if text == "" && result != nil {
		for _, block := range result.AssistantMessage.Content {
			if block.Type == "text" && block.Text != "" {
				text = block.Text
				break
			}
		}
	}
	return text, nil
}

// EstimateTokens returns an approximate token count for a string.
// Uses a simple heuristic: ~4 chars per token for English, ~2 chars for CJK.
func (a *LLMClientAdapter) EstimateTokens(text string) int {
	runes := []rune(text)
	return len(runes)*2/3 + 1
}
