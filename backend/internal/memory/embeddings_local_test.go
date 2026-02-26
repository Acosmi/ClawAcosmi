package memory

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// ---------- resolveOllamaEmbedConfig ----------

func TestResolveOllamaEmbedConfig_Defaults(t *testing.T) {
	cfg := resolveOllamaEmbedConfig(EmbeddingProviderOptions{})
	if cfg.BaseURL != DefaultOllamaBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, DefaultOllamaBaseURL)
	}
	if cfg.Model != DefaultLocalEmbeddingModel {
		t.Errorf("Model = %q, want %q", cfg.Model, DefaultLocalEmbeddingModel)
	}
}

func TestResolveOllamaEmbedConfig_EnvOverride(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://myhost:11434")
	cfg := resolveOllamaEmbedConfig(EmbeddingProviderOptions{})
	if cfg.BaseURL != "http://myhost:11434" {
		t.Errorf("BaseURL = %q, want http://myhost:11434", cfg.BaseURL)
	}
}

func TestResolveOllamaEmbedConfig_ModelOverride(t *testing.T) {
	cfg := resolveOllamaEmbedConfig(EmbeddingProviderOptions{
		Model: "mxbai-embed-large",
	})
	if cfg.Model != "mxbai-embed-large" {
		t.Errorf("Model = %q, want mxbai-embed-large", cfg.Model)
	}
}

func TestResolveOllamaEmbedConfig_LocalModelPath(t *testing.T) {
	cfg := resolveOllamaEmbedConfig(EmbeddingProviderOptions{
		Local: &EmbeddingLocalConfig{
			ModelPath: "all-minilm",
		},
	})
	if cfg.Model != "all-minilm" {
		t.Errorf("Model = %q, want all-minilm", cfg.Model)
	}
}

// ---------- detectOllamaAvailability ----------

func TestDetectOllamaAvailability_Running(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := detectOllamaAvailability(context.Background(), srv.URL)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDetectOllamaAvailability_NotRunning(t *testing.T) {
	err := detectOllamaAvailability(context.Background(), "http://localhost:59999")
	if err == nil {
		t.Error("expected error for non-running server")
	}
}

func TestDetectOllamaAvailability_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := detectOllamaAvailability(context.Background(), srv.URL)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

// ---------- ollamaEmbed ----------

func TestOllamaEmbed_SingleText(t *testing.T) {
	expectedEmbed := []float64{0.1, 0.2, 0.3, 0.4}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path != "/api/embed" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req ollamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Model != "nomic-embed-text" {
			t.Errorf("model = %q, want nomic-embed-text", req.Model)
		}
		resp := ollamaEmbedResponse{
			Model:      req.Model,
			Embeddings: [][]float64{expectedEmbed},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := OllamaEmbedConfig{BaseURL: srv.URL, Model: "nomic-embed-text"}
	embeddings, err := ollamaEmbed(context.Background(), cfg, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if len(embeddings) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(embeddings))
	}
	if len(embeddings[0]) != 4 {
		t.Errorf("embedding dim = %d, want 4", len(embeddings[0]))
	}
}

func TestOllamaEmbed_BatchText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		resp := ollamaEmbedResponse{
			Model: "nomic-embed-text",
			Embeddings: [][]float64{
				{0.1, 0.2, 0.3},
				{0.4, 0.5, 0.6},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := OllamaEmbedConfig{BaseURL: srv.URL, Model: "nomic-embed-text"}
	texts := []string{"hello", "world"}
	embeddings, err := ollamaEmbed(context.Background(), cfg, texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embeddings))
	}
}

func TestOllamaEmbed_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer srv.Close()

	cfg := OllamaEmbedConfig{BaseURL: srv.URL, Model: "nonexistent"}
	_, err := ollamaEmbed(context.Background(), cfg, "test")
	if err == nil {
		t.Error("expected error for 400 response")
	}
}

// ---------- createOllamaEmbeddingProvider ----------

func TestCreateOllamaEmbeddingProvider_Integration(t *testing.T) {
	embedVec := []float64{0.5, 0.3, 0.1, 0.7}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		resp := ollamaEmbedResponse{
			Model:      "nomic-embed-text",
			Embeddings: [][]float64{embedVec},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	os.Setenv("OLLAMA_HOST", srv.URL)
	defer os.Unsetenv("OLLAMA_HOST")

	provider, err := createOllamaEmbeddingProvider(context.Background(), EmbeddingProviderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if provider.ID != "local" {
		t.Errorf("ID = %q, want local", provider.ID)
	}

	// Test embedQuery
	vec, err := provider.EmbedQuery(context.Background(), "test query")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 4 {
		t.Errorf("vector dim = %d, want 4", len(vec))
	}
}

// ---------- formatOllamaSetupGuide ----------

func TestFormatOllamaSetupGuide(t *testing.T) {
	guide := formatOllamaSetupGuide()
	if guide == "" {
		t.Error("expected non-empty guide")
	}
	if !containsSubstr(guide, "ollama") {
		t.Error("guide should mention ollama")
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && len(s) >= len(sub) && (s == sub || findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------- SanitizeAndNormalizeEmbedding ----------

func TestSanitizeAndNormalizeEmbedding(t *testing.T) {
	vec := []float64{3, 4}
	result := SanitizeAndNormalizeEmbedding(vec)
	// Expected: 3/5=0.6, 4/5=0.8
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if abs(result[0]-0.6) > 1e-10 {
		t.Errorf("result[0] = %f, want 0.6", result[0])
	}
	if abs(result[1]-0.8) > 1e-10 {
		t.Errorf("result[1] = %f, want 0.8", result[1])
	}
}

func TestSanitizeAndNormalizeEmbedding_WithNaN(t *testing.T) {
	nan := math.NaN()
	vec := []float64{nan, 3, 4}
	result := SanitizeAndNormalizeEmbedding(vec)
	if result[0] != 0 {
		t.Errorf("NaN should be replaced with 0, got %f", result[0])
	}
}

func TestSanitizeAndNormalizeEmbedding_ZeroVector(t *testing.T) {
	vec := []float64{0, 0, 0}
	result := SanitizeAndNormalizeEmbedding(vec)
	for i, v := range result {
		if v != 0 {
			t.Errorf("result[%d] = %f, want 0", i, v)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
