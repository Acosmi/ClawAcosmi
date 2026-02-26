package vlm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs.
// This works with OpenAI, Qwen, Claude (via proxy), and any other OpenAI-compatible endpoint.
type OpenAIProvider struct {
	name     string
	endpoint string // base URL, e.g. "https://api.openai.com/v1"
	apiKey   string
	model    string
	client   *http.Client
}

// NewOpenAIProvider creates a new OpenAI-compatible provider.
func NewOpenAIProvider(cfg ProviderConfig) *OpenAIProvider {
	endpoint := strings.TrimRight(cfg.Endpoint, "/")

	// Use a custom transport that bypasses proxy for localhost endpoints.
	// This is critical for Ollama running locally — a system HTTP_PROXY
	// would otherwise intercept and fail the request.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if isLocalEndpoint(endpoint) {
		transport.Proxy = nil // bypass proxy for local connections
	}

	return &OpenAIProvider{
		name:     cfg.Name,
		endpoint: endpoint,
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		client: &http.Client{
			Timeout:   120 * time.Second,
			Transport: transport,
		},
	}
}

// isLocalEndpoint checks if the URL points to a local address.
func isLocalEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "localhost") ||
		strings.Contains(endpoint, "127.0.0.1") ||
		strings.Contains(endpoint, "[::1]")
}

func (p *OpenAIProvider) Name() string { return p.name }

func (p *OpenAIProvider) Close() error { return nil }

// ChatCompletion sends a non-streaming chat completion request.
func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Use provider's default model if not specified
	if req.Model == "" {
		req.Model = p.model
	}
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := p.endpoint + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d from %s: %s", resp.StatusCode, url, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &chatResp, nil
}

// ChatCompletionStream sends a streaming chat completion request.
func (p *OpenAIProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	if req.Model == "" {
		req.Model = p.model
	}
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := p.endpoint + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	// Use a client without timeout for streaming, but keep the transport
	// (which may bypass proxy for localhost endpoints).
	streamClient := &http.Client{Transport: p.client.Transport}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending stream request to %s: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d from %s: %s", resp.StatusCode, url, string(respBody))
	}

	ch := make(chan StreamChunk, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		p.parseSSEStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// parseSSEStream reads an SSE stream and sends parsed chunks to the channel.
func (p *OpenAIProvider) parseSSEStream(ctx context.Context, body io.Reader, ch chan<- StreamChunk) {
	scanner := bufio.NewScanner(body)
	// SSE lines can be long (especially with base64 images in responses)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- StreamChunk{Error: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Parse "data: ..." lines
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// End of stream
		if data == "[DONE]" {
			return
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("[OpenAI] Failed to parse stream chunk: %v", err)
			continue
		}

		ch <- chunk
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("reading SSE stream: %w", err)}
	}
}
