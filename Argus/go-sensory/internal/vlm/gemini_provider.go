package vlm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// GeminiProvider implements the Provider interface for Google Gemini API.
// It translates between OpenAI-format requests/responses and Gemini's native format.
type GeminiProvider struct {
	name     string
	endpoint string // base URL, e.g. "https://generativelanguage.googleapis.com/v1beta"
	apiKey   string
	model    string
	client   *http.Client
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(cfg ProviderConfig) *GeminiProvider {
	return &GeminiProvider{
		name:     cfg.Name,
		endpoint: cfg.Endpoint,
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *GeminiProvider) Name() string { return p.name }

func (p *GeminiProvider) Close() error { return nil }

// --- Gemini-specific request/response types ---

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text       string        `json:"text,omitempty"`
	InlineData *geminiInline `json:"inline_data,omitempty"`
}

type geminiInline struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsage      `json:"usageMetadata,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// ChatCompletion sends a non-streaming request to Gemini.
func (p *GeminiProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	gemReq := p.toGeminiRequest(req)

	body, err := json.Marshal(gemReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling gemini request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.endpoint, model, p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request to Gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error %d: %s", resp.StatusCode, string(respBody))
	}

	var gemResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gemResp); err != nil {
		return nil, fmt.Errorf("decoding Gemini response: %w", err)
	}

	return p.fromGeminiResponse(model, &gemResp), nil
}

// ChatCompletionStream for Gemini: currently falls back to non-streaming
// and emits a single chunk. Gemini streaming support can be added later.
func (p *GeminiProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 1)

	go func() {
		defer close(ch)

		resp, err := p.ChatCompletion(ctx, req)
		if err != nil {
			ch <- StreamChunk{Error: err}
			return
		}

		// Emit as a single stream chunk
		if len(resp.Choices) > 0 {
			content, _ := resp.Choices[0].Message.Content.(string)
			ch <- StreamChunk{
				ID:      resp.ID,
				Object:  "chat.completion.chunk",
				Created: resp.Created,
				Model:   resp.Model,
				Choices: []StreamDelta{{
					Index: 0,
					Delta: Delta{
						Role:    "assistant",
						Content: content,
					},
					FinishReason: "stop",
				}},
			}
		}
	}()

	return ch, nil
}

// toGeminiRequest converts an OpenAI-format request to Gemini format.
func (p *GeminiProvider) toGeminiRequest(req ChatRequest) *geminiRequest {
	gemReq := &geminiRequest{
		GenerationConfig: geminiGenerationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
		},
	}

	for _, msg := range req.Messages {
		gemContent := geminiContent{
			Role: mapRole(msg.Role),
		}

		switch c := msg.Content.(type) {
		case string:
			gemContent.Parts = append(gemContent.Parts, geminiPart{Text: c})
		case []any:
			for _, part := range c {
				pm, ok := part.(map[string]any)
				if !ok {
					continue
				}
				switch pm["type"] {
				case "text":
					if text, ok := pm["text"].(string); ok {
						gemContent.Parts = append(gemContent.Parts, geminiPart{Text: text})
					}
				case "image_url":
					if imgURL, ok := pm["image_url"].(map[string]any); ok {
						if url, ok := imgURL["url"].(string); ok {
							gemContent.Parts = append(gemContent.Parts, p.parseImagePart(url))
						}
					}
				}
			}
		}

		gemReq.Contents = append(gemReq.Contents, gemContent)
	}

	return gemReq
}

// parseImagePart extracts base64 data from a data URL.
func (p *GeminiProvider) parseImagePart(url string) geminiPart {
	// Expected format: "data:image/jpeg;base64,..."
	const prefix = "data:"
	if len(url) > len(prefix) && url[:len(prefix)] == prefix {
		// Find the semicolon and comma
		semiIdx := -1
		commaIdx := -1
		for i, c := range url {
			if c == ';' && semiIdx == -1 {
				semiIdx = i
			}
			if c == ',' && commaIdx == -1 {
				commaIdx = i
				break
			}
		}
		if semiIdx > 0 && commaIdx > 0 {
			mimeType := url[len(prefix):semiIdx]
			data := url[commaIdx+1:]
			return geminiPart{
				InlineData: &geminiInline{
					MimeType: mimeType,
					Data:     data,
				},
			}
		}
	}

	log.Printf("[Gemini] Warning: unsupported image URL format, treating as text")
	return geminiPart{Text: "[image: " + url + "]"}
}

// fromGeminiResponse converts a Gemini response to OpenAI format.
func (p *GeminiProvider) fromGeminiResponse(model string, resp *geminiResponse) *ChatResponse {
	chatResp := &ChatResponse{
		ID:      fmt.Sprintf("gemini-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
	}

	for i, candidate := range resp.Candidates {
		var text string
		for _, part := range candidate.Content.Parts {
			text += part.Text
		}
		chatResp.Choices = append(chatResp.Choices, Choice{
			Index:        i,
			Message:      Message{Role: "assistant", Content: text},
			FinishReason: mapFinishReason(candidate.FinishReason),
		})
	}

	if resp.UsageMetadata != nil {
		chatResp.Usage = &Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return chatResp
}

// mapRole maps OpenAI roles to Gemini roles.
func mapRole(role string) string {
	switch role {
	case "assistant":
		return "model"
	case "system":
		return "user" // Gemini doesn't have system role, treat as user
	default:
		return role
	}
}

// mapFinishReason maps Gemini finish reasons to OpenAI format.
func mapFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	default:
		return reason
	}
}
