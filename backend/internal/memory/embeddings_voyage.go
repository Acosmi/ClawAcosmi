package memory

import (
	"context"
	"fmt"
	"strings"
)

const (
	// DefaultVoyageEmbeddingModel is the default model for Voyage embeddings.
	DefaultVoyageEmbeddingModel = "voyage-4-large"
	defaultVoyageBaseURL        = "https://api.voyageai.com/v1"
)

// NormalizeVoyageModel strips the voyage/ prefix and defaults empty to voyage-4-large.
func NormalizeVoyageModel(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return DefaultVoyageEmbeddingModel
	}
	if strings.HasPrefix(trimmed, "voyage/") {
		return strings.TrimPrefix(trimmed, "voyage/")
	}
	return trimmed
}

func createVoyageProvider(ctx context.Context, opts EmbeddingProviderOptions) (*EmbeddingProvider, error) {
	apiKey, err := resolveAPIKey(opts, "voyage")
	if err != nil {
		return nil, err
	}

	baseURL := defaultVoyageBaseURL
	var headerOverrides map[string]string

	if pc := opts.ProviderConfig; pc != nil {
		if cfg := pc("voyage"); cfg != nil {
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

	model := NormalizeVoyageModel(opts.Model)
	url := strings.TrimRight(baseURL, "/") + "/embeddings"

	embed := func(ctx context.Context, input []string, inputType string) ([][]float64, error) {
		if len(input) == 0 {
			return nil, nil
		}
		type reqBody struct {
			Model     string   `json:"model"`
			Input     []string `json:"input"`
			InputType string   `json:"input_type,omitempty"`
		}
		type respEntry struct {
			Embedding []float64 `json:"embedding"`
		}
		type respBody struct {
			Data []respEntry `json:"data"`
		}
		body := reqBody{Model: model, Input: input}
		if inputType != "" {
			body.InputType = inputType
		}
		var resp respBody
		if err := embeddingHTTPPost(ctx, url, headers, body, &resp); err != nil {
			return nil, fmt.Errorf("voyage embeddings failed: %w", err)
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
		ID:    "voyage",
		Model: model,
		EmbedQuery: func(ctx context.Context, text string) ([]float64, error) {
			vecs, err := embed(ctx, []string{text}, "query")
			if err != nil {
				return nil, err
			}
			if len(vecs) == 0 {
				return nil, nil
			}
			return vecs[0], nil
		},
		EmbedBatch: func(ctx context.Context, texts []string) ([][]float64, error) {
			return embed(ctx, texts, "document")
		},
	}, nil
}
