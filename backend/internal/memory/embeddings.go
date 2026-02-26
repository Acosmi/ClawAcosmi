package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strings"
)

// EmbeddingProvider is the abstraction for generating text embeddings.
type EmbeddingProvider struct {
	ID         string
	Model      string
	EmbedQuery func(ctx context.Context, text string) ([]float64, error)
	EmbedBatch func(ctx context.Context, texts []string) ([][]float64, error)
}

// EmbeddingProviderResult holds from creating an embedding provider.
type EmbeddingProviderResult struct {
	Provider          *EmbeddingProvider
	RequestedProvider string // "openai" | "local" | "gemini" | "voyage" | "auto"
	FallbackFrom      string
	FallbackReason    string
}

// EmbeddingProviderOptions configures which provider to create.
type EmbeddingProviderOptions struct {
	Provider       string // "openai" | "local" | "gemini" | "voyage" | "auto"
	Model          string
	Fallback       string // "openai" | "gemini" | "local" | "voyage" | "none"
	Remote         *EmbeddingRemoteConfig
	Local          *EmbeddingLocalConfig
	APIKeyResolver func(provider string) (string, error)
	ProviderConfig func(provider string) *ProviderEndpointConfig
	Logger         *slog.Logger
}

// EmbeddingRemoteConfig holds remote API overrides.
type EmbeddingRemoteConfig struct {
	BaseURL string
	APIKey  string
	Headers map[string]string
}

// EmbeddingLocalConfig holds local embedding model settings.
type EmbeddingLocalConfig struct {
	ModelPath     string
	ModelCacheDir string
}

// ProviderEndpointConfig holds provider-level base URL and headers from config.
type ProviderEndpointConfig struct {
	BaseURL string
	Headers map[string]string
}

// SanitizeAndNormalizeEmbedding normalizes an embedding vector to unit length.
func SanitizeAndNormalizeEmbedding(vec []float64) []float64 {
	sanitized := make([]float64, len(vec))
	for i, v := range vec {
		if v != v { // NaN check
			sanitized[i] = 0
		} else {
			sanitized[i] = v
		}
	}
	var magnitude float64
	for _, v := range sanitized {
		magnitude += v * v
	}
	if magnitude < 1e-10 {
		return sanitized
	}
	magnitude = sqrtFloat64(magnitude)
	for i := range sanitized {
		sanitized[i] /= magnitude
	}
	return sanitized
}

// sqrtFloat64 wraps math.Sqrt.
func sqrtFloat64(x float64) float64 {
	return math.Sqrt(x)
}

// CreateEmbeddingProvider creates an embedding provider using the auto/explicit
// selection logic with optional fallback.
func CreateEmbeddingProvider(ctx context.Context, opts EmbeddingProviderOptions) (*EmbeddingProviderResult, error) {
	requested := opts.Provider
	fallback := opts.Fallback

	if requested == "auto" {
		return createAutoProvider(ctx, opts)
	}

	provider, err := createNamedProvider(ctx, opts, requested)
	if err == nil {
		return &EmbeddingProviderResult{
			Provider:          provider,
			RequestedProvider: requested,
		}, nil
	}

	reason := err.Error()
	if fallback != "" && fallback != "none" && fallback != requested {
		fb, fbErr := createNamedProvider(ctx, opts, fallback)
		if fbErr == nil {
			return &EmbeddingProviderResult{
				Provider:          fb,
				RequestedProvider: requested,
				FallbackFrom:      requested,
				FallbackReason:    reason,
			}, nil
		}
		return nil, fmt.Errorf("%s\n\nFallback to %s failed: %v", reason, fallback, fbErr)
	}
	return nil, err
}

func createAutoProvider(ctx context.Context, opts EmbeddingProviderOptions) (*EmbeddingProviderResult, error) {
	// Try local first if a model path is configured and file exists.
	if opts.Local != nil && opts.Local.ModelPath != "" {
		if _, err := os.Stat(opts.Local.ModelPath); err == nil {
			p, localErr := createNamedProvider(ctx, opts, "local")
			if localErr == nil {
				return &EmbeddingProviderResult{
					Provider:          p,
					RequestedProvider: "auto",
				}, nil
			}
			// local failed, fall through to remote providers
		}
	}
	var lastErr error
	for _, name := range []string{"openai", "gemini", "voyage"} {
		p, err := createNamedProvider(ctx, opts, name)
		if err != nil {
			if isMissingKeyError(err) {
				lastErr = err
				continue
			}
			return nil, err
		}
		return &EmbeddingProviderResult{
			Provider:          p,
			RequestedProvider: "auto",
		}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no embeddings provider available")
}

func createNamedProvider(ctx context.Context, opts EmbeddingProviderOptions, name string) (*EmbeddingProvider, error) {
	switch name {
	case "openai":
		return createOpenAIProvider(ctx, opts)
	case "gemini":
		return createGeminiProvider(ctx, opts)
	case "voyage":
		return createVoyageProvider(ctx, opts)
	case "local":
		return createOllamaEmbeddingProvider(ctx, opts)
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", name)
	}
}

func isMissingKeyError(err error) bool {
	return strings.Contains(err.Error(), "No API key found for provider")
}

// requireAPIKey returns the API key or an error if empty.
func requireAPIKey(key, provider string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("No API key found for provider %q", provider)
	}
	return key, nil
}

// resolveAPIKey resolves the API key for a provider from remote config,
// environment, or resolver function.
func resolveAPIKey(opts EmbeddingProviderOptions, provider string) (string, error) {
	if opts.Remote != nil && strings.TrimSpace(opts.Remote.APIKey) != "" {
		return strings.TrimSpace(opts.Remote.APIKey), nil
	}
	if opts.APIKeyResolver != nil {
		key, err := opts.APIKeyResolver(provider)
		if err != nil {
			return "", err
		}
		return requireAPIKey(key, provider)
	}
	// Fallback: try env vars.
	envNames := map[string][]string{
		"openai": {"OPENAI_API_KEY"},
		"google": {"GOOGLE_API_KEY", "GEMINI_API_KEY"},
		"voyage": {"VOYAGE_API_KEY"},
	}
	for _, name := range envNames[provider] {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("No API key found for provider %q", provider)
}

// mergeHeaders merges base headers with overrides.
func mergeHeaders(base, overrides map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overrides {
		result[k] = v
	}
	return result
}

// embeddingHTTPPost posts a JSON body and decodes the response.
func embeddingHTTPPost(ctx context.Context, url string, headers map[string]string, body any, result any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		text, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(text))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}
