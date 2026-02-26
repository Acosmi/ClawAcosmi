package memory

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultGeminiEmbeddingModel is the default model for Gemini embeddings.
	DefaultGeminiEmbeddingModel = "gemini-embedding-001"
	defaultGeminiBaseURL        = "https://generativelanguage.googleapis.com/v1beta"
)

// NormalizeGeminiModel strips prefixes and defaults empty to gemini-embedding-001.
func NormalizeGeminiModel(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return DefaultGeminiEmbeddingModel
	}
	without := strings.TrimPrefix(trimmed, "models/")
	if strings.HasPrefix(without, "gemini/") {
		return strings.TrimPrefix(without, "gemini/")
	}
	if strings.HasPrefix(without, "google/") {
		return strings.TrimPrefix(without, "google/")
	}
	return without
}

func normalizeGeminiBaseURL(raw string) string {
	trimmed := strings.TrimRight(raw, "/")
	if idx := strings.Index(trimmed, "/openai"); idx > -1 {
		return trimmed[:idx]
	}
	return trimmed
}

func buildGeminiModelPath(model string) string {
	if strings.HasPrefix(model, "models/") {
		return model
	}
	return "models/" + model
}

func resolveGeminiRemoteAPIKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if trimmed == "GOOGLE_API_KEY" || trimmed == "GEMINI_API_KEY" {
		return strings.TrimSpace(os.Getenv(trimmed))
	}
	return trimmed
}

func createGeminiProvider(ctx context.Context, opts EmbeddingProviderOptions) (*EmbeddingProvider, error) {
	// Resolve API key — prefer remote override, then resolver, then env.
	var apiKey string
	if opts.Remote != nil {
		apiKey = resolveGeminiRemoteAPIKey(opts.Remote.APIKey)
	}
	if apiKey == "" {
		var err error
		apiKey, err = resolveAPIKey(opts, "google")
		if err != nil {
			return nil, err
		}
	}

	baseURL := defaultGeminiBaseURL
	var headerOverrides map[string]string

	if pc := opts.ProviderConfig; pc != nil {
		if cfg := pc("google"); cfg != nil {
			if cfg.BaseURL != "" {
				baseURL = cfg.BaseURL
			}
			headerOverrides = cfg.Headers
		}
	}
	if opts.Remote != nil && opts.Remote.BaseURL != "" {
		baseURL = opts.Remote.BaseURL
	}
	baseURL = normalizeGeminiBaseURL(baseURL)

	headers := mergeHeaders(map[string]string{
		"Content-Type":   "application/json",
		"x-goog-api-key": apiKey,
	}, headerOverrides)
	if opts.Remote != nil {
		headers = mergeHeaders(headers, opts.Remote.Headers)
	}

	model := NormalizeGeminiModel(opts.Model)
	modelPath := buildGeminiModelPath(model)
	embedURL := baseURL + "/" + modelPath + ":embedContent"
	batchURL := baseURL + "/" + modelPath + ":batchEmbedContents"

	embedQuery := func(ctx context.Context, text string) ([]float64, error) {
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		type part struct {
			Text string `json:"text"`
		}
		type content struct {
			Parts []part `json:"parts"`
		}
		type reqBody struct {
			Content  content `json:"content"`
			TaskType string  `json:"taskType"`
		}
		type embedding struct {
			Values []float64 `json:"values"`
		}
		type respBody struct {
			Embedding *embedding `json:"embedding"`
		}

		var resp respBody
		if err := embeddingHTTPPost(ctx, embedURL, headers, reqBody{
			Content:  content{Parts: []part{{Text: text}}},
			TaskType: "RETRIEVAL_QUERY",
		}, &resp); err != nil {
			return nil, fmt.Errorf("gemini embeddings failed: %w", err)
		}
		if resp.Embedding == nil {
			return nil, nil
		}
		return resp.Embedding.Values, nil
	}

	embedBatch := func(ctx context.Context, texts []string) ([][]float64, error) {
		if len(texts) == 0 {
			return nil, nil
		}
		type part struct {
			Text string `json:"text"`
		}
		type content struct {
			Parts []part `json:"parts"`
		}
		type batchReq struct {
			Model    string  `json:"model"`
			Content  content `json:"content"`
			TaskType string  `json:"taskType"`
		}
		type reqBody struct {
			Requests []batchReq `json:"requests"`
		}
		type embedding struct {
			Values []float64 `json:"values"`
		}
		type respBody struct {
			Embeddings []embedding `json:"embeddings"`
		}

		reqs := make([]batchReq, len(texts))
		for i, t := range texts {
			reqs[i] = batchReq{
				Model:    modelPath,
				Content:  content{Parts: []part{{Text: t}}},
				TaskType: "RETRIEVAL_DOCUMENT",
			}
		}

		var resp respBody
		if err := embeddingHTTPPost(ctx, batchURL, headers, reqBody{Requests: reqs}, &resp); err != nil {
			return nil, fmt.Errorf("gemini embeddings failed: %w", err)
		}

		vecs := make([][]float64, len(texts))
		for i := range texts {
			if i < len(resp.Embeddings) {
				vecs[i] = resp.Embeddings[i].Values
			}
		}
		return vecs, nil
	}

	return &EmbeddingProvider{
		ID:         "gemini",
		Model:      model,
		EmbedQuery: embedQuery,
		EmbedBatch: embedBatch,
	}, nil
}
