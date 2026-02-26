package memory

import (
	"context"
	"fmt"
	"strings"
)

const (
	// DefaultOpenAIEmbeddingModel is the default model for OpenAI embeddings.
	DefaultOpenAIEmbeddingModel = "text-embedding-3-small"
	defaultOpenAIBaseURL        = "https://api.openai.com/v1"
)

// NormalizeOpenAIModel strips the openai/ prefix and defaults empty to text-embedding-3-small.
func NormalizeOpenAIModel(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return DefaultOpenAIEmbeddingModel
	}
	if strings.HasPrefix(trimmed, "openai/") {
		return strings.TrimPrefix(trimmed, "openai/")
	}
	return trimmed
}

func createOpenAIProvider(ctx context.Context, opts EmbeddingProviderOptions) (*EmbeddingProvider, error) {
	apiKey, err := resolveAPIKey(opts, "openai")
	if err != nil {
		return nil, err
	}

	baseURL := defaultOpenAIBaseURL
	var headerOverrides map[string]string

	if pc := opts.ProviderConfig; pc != nil {
		if cfg := pc("openai"); cfg != nil {
			if cfg.BaseURL != "" {
				baseURL = cfg.BaseURL
			}
			headerOverrides = cfg.Headers
		}
	}
	if opts.Remote != nil && opts.Remote.BaseURL != "" {
		baseURL = opts.Remote.BaseURL
	}

	headers := mergeHeaders(map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + apiKey,
	}, headerOverrides)
	if opts.Remote != nil {
		headers = mergeHeaders(headers, opts.Remote.Headers)
	}

	model := NormalizeOpenAIModel(opts.Model)
	url := strings.TrimRight(baseURL, "/") + "/embeddings"

	embed := func(ctx context.Context, input []string) ([][]float64, error) {
		if len(input) == 0 {
			return nil, nil
		}
		type reqBody struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		type respEntry struct {
			Embedding []float64 `json:"embedding"`
		}
		type respBody struct {
			Data []respEntry `json:"data"`
		}
		var resp respBody
		if err := embeddingHTTPPost(ctx, url, headers, reqBody{Model: model, Input: input}, &resp); err != nil {
			return nil, fmt.Errorf("openai embeddings failed: %w", err)
		}
		vecs := make([][]float64, len(input))
		for i := range input {
			if i < len(resp.Data) {
				vecs[i] = resp.Data[i].Embedding
			}
		}
		return vecs, nil
	}

	return &EmbeddingProvider{
		ID:    "openai",
		Model: model,
		EmbedQuery: func(ctx context.Context, text string) ([]float64, error) {
			vecs, err := embed(ctx, []string{text})
			if err != nil {
				return nil, err
			}
			if len(vecs) == 0 {
				return nil, nil
			}
			return vecs[0], nil
		},
		EmbedBatch: embed,
	}, nil
}
