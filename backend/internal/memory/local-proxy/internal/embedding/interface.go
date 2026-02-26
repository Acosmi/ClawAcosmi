// Package embedding — pluggable embedding engine with automatic fallback.
// Provides the EmbedEngine interface and a Router that selects the best available
// engine at call time: Ollama → Cloud → error (TF-IDF handled by caller).
//
// Part of the OpenAethel local-proxy hybrid architecture.
package embedding

import "context"

// EmbedEngine is the unified interface for all embedding backends.
// Every engine (Ollama, ONNX, Cloud) implements this interface so the Router
// can transparently switch between them based on availability.
type EmbedEngine interface {
	// Name returns a human-readable identifier for this engine (e.g. "ollama", "cloud").
	Name() string

	// Available reports whether this engine is ready to serve requests right now.
	// Implementations should be fast (cached or ≤2s timeout) — called on every Embed().
	Available() bool

	// Embed generates vector embeddings for the given texts.
	// Returns one []float32 per input text. Implementations MUST preserve input order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the output vector dimension (e.g. 384 for all-minilm).
	// Returns 0 if dimension is unknown or variable.
	Dimension() int

	// Close releases any resources held by this engine (connections, model files, etc.).
	Close() error
}
