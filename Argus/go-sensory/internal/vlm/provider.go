// Package vlm provides VLM (Vision-Language Model) proxy and routing capabilities.
// It defines a common Provider interface and implements backends for OpenAI-compatible
// and Gemini APIs, enabling go-sensory to serve as a unified VLM gateway.
package vlm

import (
	"context"
	"encoding/json"
	"time"
)

// --- Provider Interface ---

// Provider is the unified interface for VLM backends.
// All backends (OpenAI, Gemini, Ollama, etc.) implement this interface.
type Provider interface {
	// ChatCompletion sends a non-streaming chat completion request.
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// ChatCompletionStream sends a streaming chat completion request.
	// Returns a channel that yields chunks and closes when done.
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)

	// Name returns the provider's display name.
	Name() string

	// Close releases any resources held by the provider.
	Close() error
}

// --- Request Types (OpenAI-compatible) ---

// ChatRequest represents an OpenAI-compatible chat completion request.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

// Message represents a chat message with multimodal content support.
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []ContentPart
}

// ContentPart represents one part of a multimodal message.
type ContentPart struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // for type="text"
	ImageURL *ImageURL `json:"image_url,omitempty"` // for type="image_url"
}

// ImageURL contains the URL for an image content part.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// --- Response Types ---

// ChatResponse represents an OpenAI-compatible chat completion response.
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice represents one completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage contains token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents one chunk in a streaming response.
type StreamChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []StreamDelta `json:"choices"`
	Error   error         `json:"-"` // internal, not serialized
}

// StreamDelta represents the delta content in a stream chunk.
type StreamDelta struct {
	Index        int    `json:"index"`
	Delta        Delta  `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// Delta is the incremental content in a streaming chunk.
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// --- Helper functions ---

// NewChatResponse creates a ChatResponse with a single text choice.
func NewChatResponse(id, model, content, finishReason string) *ChatResponse {
	return &ChatResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{{
			Index:        0,
			Message:      Message{Role: "assistant", Content: content},
			FinishReason: finishReason,
		}},
	}
}

// GetTextContent extracts plain text content from a Message.
// If the content is a string, returns it directly.
// If the content is []ContentPart, concatenates all text parts.
func (m *Message) GetTextContent() string {
	switch c := m.Content.(type) {
	case string:
		return c
	case []any:
		var text string
		for _, part := range c {
			if pm, ok := part.(map[string]any); ok {
				if pm["type"] == "text" {
					if t, ok := pm["text"].(string); ok {
						text += t
					}
				}
			}
		}
		return text
	}
	return ""
}

// MarshalJSON implements custom JSON marshaling for StreamChunk
// to exclude the internal Error field.
func (sc StreamChunk) MarshalJSON() ([]byte, error) {
	type Alias StreamChunk
	return json.Marshal(&struct {
		Alias
		Error any `json:"error,omitempty"`
	}{
		Alias: (Alias)(sc),
	})
}
