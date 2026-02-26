package vlm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OllamaProvider implements the Provider interface using Ollama's native
// /api/chat endpoint. This provides first-class support for:
//   - Vision/multimodal via native `images` field (no base64 data-URI wrapping)
//   - NDJSON streaming (simpler and more reliable than SSE)
//   - No external dependency (lightweight HTTP client)
type OllamaProvider struct {
	name     string
	endpoint string // base URL, e.g. "http://localhost:11434"
	model    string
	client   *http.Client
}

// NewOllamaProvider creates a new Ollama-native provider.
func NewOllamaProvider(cfg ProviderConfig) *OllamaProvider {
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	// Strip /v1 suffix if present (user may pass OpenAI-style endpoint)
	endpoint = strings.TrimSuffix(endpoint, "/v1")

	// Bypass proxy for localhost — Ollama typically runs locally
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if isLocalEndpoint(endpoint) {
		transport.Proxy = nil
	}

	return &OllamaProvider{
		name:     cfg.Name,
		endpoint: endpoint,
		model:    cfg.Model,
		client: &http.Client{
			Timeout:   300 * time.Second, // vision models can be slow
			Transport: transport,
		},
	}
}

func (p *OllamaProvider) Name() string { return p.name }
func (p *OllamaProvider) Close() error { return nil }

// --- Ollama native request/response types ---

// ollamaChatRequest is Ollama's native /api/chat request format.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

// ollamaMessage is Ollama's native message format with images support.
type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // base64-encoded image data (raw, no data-URI prefix)
}

// ollamaChatResponse is Ollama's native /api/chat response (NDJSON lines).
type ollamaChatResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	DoneReason      string        `json:"done_reason,omitempty"`
	TotalDuration   int64         `json:"total_duration,omitempty"`
	LoadDuration    int64         `json:"load_duration,omitempty"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
}

// ChatCompletion sends a non-streaming chat completion request via Ollama's native API.
func (p *OllamaProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	ollamaReq := ollamaChatRequest{
		Model:    model,
		Messages: convertToOllamaMessages(req.Messages),
		Stream:   false,
		Options:  buildOllamaOptions(req),
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling Ollama request: %w", err)
	}

	url := p.endpoint + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request to Ollama %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decoding Ollama response: %w", err)
	}

	// Convert to OpenAI-compatible response
	return &ChatResponse{
		ID:      fmt.Sprintf("ollama-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   ollamaResp.Model,
		Choices: []Choice{{
			Index: 0,
			Message: Message{
				Role:    ollamaResp.Message.Role,
				Content: ollamaResp.Message.Content,
			},
			FinishReason: mapDoneReason(ollamaResp.DoneReason),
		}},
		Usage: &Usage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}, nil
}

// ChatCompletionStream sends a streaming chat completion request via Ollama's native NDJSON API.
func (p *OllamaProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	ollamaReq := ollamaChatRequest{
		Model:    model,
		Messages: convertToOllamaMessages(req.Messages),
		Stream:   true,
		Options:  buildOllamaOptions(req),
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling Ollama request: %w", err)
	}

	url := p.endpoint + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Use a client without timeout for streaming
	streamClient := &http.Client{Transport: p.client.Transport}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending stream request to Ollama %s: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Ollama API error %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		p.parseNDJSONStream(ctx, resp.Body, ch, model)
	}()

	return ch, nil
}

// parseNDJSONStream reads Ollama's NDJSON streaming response.
func (p *OllamaProvider) parseNDJSONStream(ctx context.Context, body io.Reader, ch chan<- StreamChunk, model string) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	chunkID := fmt.Sprintf("ollama-stream-%d", time.Now().UnixNano())

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- StreamChunk{Error: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var ollamaResp ollamaChatResponse
		if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
			log.Printf("[Ollama] Failed to parse stream chunk: %v", err)
			continue
		}

		finishReason := ""
		if ollamaResp.Done {
			finishReason = mapDoneReason(ollamaResp.DoneReason)
		}

		ch <- StreamChunk{
			ID:      chunkID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []StreamDelta{{
				Index: 0,
				Delta: Delta{
					Content: ollamaResp.Message.Content,
				},
				FinishReason: finishReason,
			}},
		}

		if ollamaResp.Done {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("reading Ollama NDJSON stream: %w", err)}
	}
}

// --- Conversion helpers ---

// convertToOllamaMessages converts OpenAI-format messages to Ollama's native format.
// Handles both plain text content and multimodal content with images.
func convertToOllamaMessages(messages []Message) []ollamaMessage {
	result := make([]ollamaMessage, 0, len(messages))

	for _, msg := range messages {
		om := ollamaMessage{Role: msg.Role}

		switch c := msg.Content.(type) {
		case string:
			om.Content = c
		case []any:
			// Multimodal content: extract text and images
			var texts []string
			for _, part := range c {
				pm, ok := part.(map[string]any)
				if !ok {
					continue
				}

				switch pm["type"] {
				case "text":
					if t, ok := pm["text"].(string); ok {
						texts = append(texts, t)
					}
				case "image_url":
					if imgURL, ok := pm["image_url"].(map[string]any); ok {
						if urlStr, ok := imgURL["url"].(string); ok {
							// Extract base64 data from data-URI format
							if b64 := extractBase64FromDataURI(urlStr); b64 != "" {
								om.Images = append(om.Images, b64)
							}
						}
					}
				}
			}
			om.Content = strings.Join(texts, "\n")
		case []ContentPart:
			// Typed multimodal content
			var texts []string
			for _, part := range c {
				switch part.Type {
				case "text":
					texts = append(texts, part.Text)
				case "image_url":
					if part.ImageURL != nil {
						if b64 := extractBase64FromDataURI(part.ImageURL.URL); b64 != "" {
							om.Images = append(om.Images, b64)
						}
					}
				}
			}
			om.Content = strings.Join(texts, "\n")
		}

		result = append(result, om)
	}

	return result
}

// extractBase64FromDataURI extracts raw base64 data from a data URI.
// Input:  "data:image/jpeg;base64,/9j/4AAQ..."
// Output: "/9j/4AAQ..."
// If not a data URI, tries to interpret as raw base64.
func extractBase64FromDataURI(uri string) string {
	if strings.HasPrefix(uri, "data:") {
		if idx := strings.Index(uri, ","); idx != -1 {
			return uri[idx+1:]
		}
	}
	// Check if it's already raw base64
	if _, err := base64.StdEncoding.DecodeString(uri[:min(len(uri), 100)]); err == nil && len(uri) > 20 {
		return uri
	}
	return ""
}

// buildOllamaOptions converts ChatRequest parameters to Ollama options map.
func buildOllamaOptions(req ChatRequest) map[string]any {
	opts := make(map[string]any)

	// Default context window — keeps VRAM usage reasonable.
	// Can be overridden by the caller via ChatRequest parameters.
	opts["num_ctx"] = 16384

	if req.Temperature != nil {
		opts["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		opts["top_p"] = *req.TopP
	}
	if req.MaxTokens > 0 {
		opts["num_predict"] = req.MaxTokens
	}
	if len(req.Stop) > 0 {
		opts["stop"] = req.Stop
	}
	return opts
}

// mapDoneReason converts Ollama's done_reason to OpenAI's finish_reason.
func mapDoneReason(reason string) string {
	switch reason {
	case "stop":
		return "stop"
	case "length":
		return "length"
	case "":
		return "stop" // default
	default:
		return reason
	}
}
