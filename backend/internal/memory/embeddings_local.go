package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// TS 对照: memory/embeddings.ts createLocalEmbeddingProvider()
// Go 实现: 通过 Ollama /api/embed HTTP API 替代 node-llama-cpp 直接加载 GGUF。
// RUST_CANDIDATE: P3 — Phase 13+ 可选升级为 Rust FFI 直接加载 GGUF 模型。

const (
	// DefaultOllamaBaseURL is the default Ollama server address.
	DefaultOllamaBaseURL = "http://localhost:11434"

	// DefaultLocalEmbeddingModel is the default Ollama embedding model.
	// TS 原版默认: embeddinggemma-300M-Q8_0.gguf (node-llama-cpp)
	// Go 替代: nomic-embed-text (Ollama 生态最常用嵌入模型)
	DefaultLocalEmbeddingModel = "nomic-embed-text"

	// ollamaEmbedTimeout is the default timeout for Ollama embedding requests.
	ollamaEmbedTimeout = 60 * time.Second
)

// OllamaEmbedConfig holds configuration for the Ollama embedding provider.
type OllamaEmbedConfig struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

// ---------- Ollama /api/embed 请求/响应 ----------

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"` // string 或 []string
}

type ollamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
}

// ---------- Ollama 嵌入 provider ----------

// createOllamaEmbeddingProvider creates a local embedding provider using Ollama.
// TS 对照: embeddings.ts createLocalEmbeddingProvider()
func createOllamaEmbeddingProvider(ctx context.Context, opts EmbeddingProviderOptions) (*EmbeddingProvider, error) {
	cfg := resolveOllamaEmbedConfig(opts)

	// 检测 Ollama 是否可用
	if err := detectOllamaAvailability(ctx, cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("%s\n\n%s", err.Error(), formatOllamaSetupGuide())
	}

	return &EmbeddingProvider{
		ID:    "local",
		Model: cfg.Model,
		EmbedQuery: func(ctx context.Context, text string) ([]float64, error) {
			embeddings, err := ollamaEmbed(ctx, cfg, text)
			if err != nil {
				return nil, err
			}
			if len(embeddings) == 0 {
				return nil, fmt.Errorf("ollama returned empty embeddings")
			}
			return SanitizeAndNormalizeEmbedding(embeddings[0]), nil
		},
		EmbedBatch: func(ctx context.Context, texts []string) ([][]float64, error) {
			embeddings, err := ollamaEmbed(ctx, cfg, texts)
			if err != nil {
				return nil, err
			}
			if len(embeddings) != len(texts) {
				return nil, fmt.Errorf("ollama returned %d embeddings for %d inputs", len(embeddings), len(texts))
			}
			result := make([][]float64, len(embeddings))
			for i, vec := range embeddings {
				result[i] = SanitizeAndNormalizeEmbedding(vec)
			}
			return result, nil
		},
	}, nil
}

// ollamaEmbed calls Ollama /api/embed endpoint.
func ollamaEmbed(ctx context.Context, cfg OllamaEmbedConfig, input any) ([][]float64, error) {
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/api/embed"

	body := ollamaEmbedRequest{
		Model: cfg.Model,
		Input: input,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama embed request: %w", err)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = ollamaEmbedTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("create ollama embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		text, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ollama embed HTTP %d: %s", resp.StatusCode, string(text))
	}

	var result ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode ollama embed response: %w", err)
	}

	return result.Embeddings, nil
}

// ---------- Ollama 可用性检测 ----------

// detectOllamaAvailability checks if Ollama is running and reachable.
func detectOllamaAvailability(ctx context.Context, baseURL string) error {
	endpoint := strings.TrimRight(baseURL, "/") + "/"

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return fmt.Errorf("local embeddings unavailable: cannot create probe request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("local embeddings unavailable: Ollama is not running at %s", baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("local embeddings unavailable: Ollama returned %d", resp.StatusCode)
	}

	return nil
}

// ---------- 配置解析 ----------

// resolveOllamaEmbedConfig resolves Ollama embedding config from options.
func resolveOllamaEmbedConfig(opts EmbeddingProviderOptions) OllamaEmbedConfig {
	cfg := OllamaEmbedConfig{
		BaseURL: DefaultOllamaBaseURL,
		Model:   DefaultLocalEmbeddingModel,
		Timeout: ollamaEmbedTimeout,
	}

	// 环境变量覆盖
	if envHost := strings.TrimSpace(os.Getenv("OLLAMA_HOST")); envHost != "" {
		cfg.BaseURL = envHost
	}

	// Local 配置覆盖
	if opts.Local != nil {
		if opts.Local.ModelPath != "" {
			// ModelPath 在 Ollama 模式下作为模型名使用
			cfg.Model = opts.Local.ModelPath
		}
	}

	// 显式模型名覆盖
	if opts.Model != "" && opts.Model != "auto" {
		cfg.Model = opts.Model
	}

	return cfg
}

// ---------- 错误指引 ----------

// formatOllamaSetupGuide returns user-facing setup instructions.
// TS 对照: embeddings.ts formatLocalSetupError()
func formatOllamaSetupGuide() string {
	return strings.Join([]string{
		"To enable local embeddings:",
		"1) Install Ollama: https://ollama.com/download",
		"2) Start Ollama: ollama serve",
		"3) Pull an embedding model: ollama pull nomic-embed-text",
		"",
		`Or set agents.defaults.memorySearch.provider = "openai" (remote).`,
		`Or set agents.defaults.memorySearch.provider = "voyage" (remote).`,
	}, "\n")
}
