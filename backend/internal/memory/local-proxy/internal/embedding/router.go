package embedding

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/uhms/local-proxy/internal/cloud"
	"github.com/uhms/local-proxy/internal/config"
)

// Router selects the best available embedding engine at call time.
// Engines are tried in priority order: Ollama → (ONNX, Phase 6B) → Cloud.
// If no engine is available, Embed() returns an error — the caller should fall back to TF-IDF.
type Router struct {
	engines []EmbedEngine
	logger  *slog.Logger
}

// log returns a non-nil logger, falling back to slog.Default() if logger is nil.
// This allows Router to be constructed directly in tests without NewRouter().
func (r *Router) log() *slog.Logger {
	if r.logger != nil {
		return r.logger
	}
	return slog.Default()
}

// NewRouter creates an embedding router based on configuration.
//
// Engine priority order:
//  1. Ollama (if EmbedMode is "auto" or "ollama")
//  2. Cloud  (if EmbedMode is "auto" or "cloud")
//
// EmbedMode "auto" registers all engines; specific modes register only one.
func NewRouter(cfg *config.Config, cloudClient *cloud.Client, monitor *cloud.Monitor) *Router {
	r := &Router{
		logger: slog.Default().With("component", "embed-router"),
	}

	mode := strings.ToLower(cfg.EmbedMode)
	if mode == "" {
		mode = "auto"
	}

	switch mode {
	case "auto":
		// Register all engines in priority order: Ollama → ONNX → Cloud
		r.engines = append(r.engines, NewOllamaEngine(cfg.OllamaURL, cfg.OllamaModel))
		if onnx := NewONNXEngineFromConfig(cfg.ONNXModelPath); onnx != nil {
			r.engines = append(r.engines, onnx)
		}
		r.engines = append(r.engines, NewCloudEngine(cloudClient, monitor))

	case "ollama":
		r.engines = append(r.engines, NewOllamaEngine(cfg.OllamaURL, cfg.OllamaModel))

	case "cloud":
		r.engines = append(r.engines, NewCloudEngine(cloudClient, monitor))

	default:
		r.log().Warn("Unknown embed mode, defaulting to auto", "mode", cfg.EmbedMode)
		r.engines = append(r.engines, NewOllamaEngine(cfg.OllamaURL, cfg.OllamaModel))
		if onnx := NewONNXEngineFromConfig(cfg.ONNXModelPath); onnx != nil {
			r.engines = append(r.engines, onnx)
		}
		r.engines = append(r.engines, NewCloudEngine(cloudClient, monitor))
	}

	names := make([]string, len(r.engines))
	for i, e := range r.engines {
		names[i] = e.Name()
	}
	r.log().Info("Embedding router initialized", "mode", mode, "engines", names)

	return r
}

// ErrNoEngine is returned when no embedding engine is available.
var ErrNoEngine = errors.New("no embedding engine available")

// Embed generates embeddings using the highest-priority available engine.
// Returns ErrNoEngine if all engines are unavailable — caller should fall back to TF-IDF.
func (r *Router) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	for _, engine := range r.engines {
		if !engine.Available() {
			r.log().Debug("Engine not available, trying next", "engine", engine.Name())
			continue
		}

		vectors, err := engine.Embed(ctx, texts)
		if err != nil {
			r.log().Warn("Engine embed failed, trying next",
				"engine", engine.Name(), "error", err)
			continue
		}

		r.log().Debug("Embed succeeded", "engine", engine.Name(), "count", len(texts))
		return vectors, nil
	}

	return nil, fmt.Errorf("%w: tried %d engine(s)", ErrNoEngine, len(r.engines))
}

// ActiveEngine returns the name of the first available engine, or "none".
func (r *Router) ActiveEngine() string {
	for _, engine := range r.engines {
		if engine.Available() {
			return engine.Name()
		}
	}
	return "none"
}

// Close releases resources for all registered engines.
func (r *Router) Close() error {
	var errs []error
	for _, engine := range r.engines {
		if err := engine.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", engine.Name(), err))
		}
	}
	return errors.Join(errs...)
}
