package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================================
// Mock Engine for Router tests
// ============================================================================

type mockEngine struct {
	name      string
	available bool
	vectors   [][]float32
	embedErr  error
	dim       int
}

func (m *mockEngine) Name() string    { return m.name }
func (m *mockEngine) Available() bool { return m.available }
func (m *mockEngine) Dimension() int  { return m.dim }
func (m *mockEngine) Close() error    { return nil }
func (m *mockEngine) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if m.embedErr != nil {
		return nil, m.embedErr
	}
	return m.vectors, nil
}

// ============================================================================
// Router Tests
// ============================================================================

func TestRouterFallback(t *testing.T) {
	primary := &mockEngine{name: "primary", available: false}
	secondary := &mockEngine{
		name:      "secondary",
		available: true,
		vectors:   [][]float32{{0.1, 0.2, 0.3}},
	}

	r := &Router{engines: []EmbedEngine{primary, secondary}}

	vecs, err := r.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(vecs) != 1 || len(vecs[0]) != 3 {
		t.Fatalf("expected 1 vector of dim 3, got %d vectors", len(vecs))
	}
	if vecs[0][0] != 0.1 {
		t.Errorf("expected vecs[0][0]=0.1, got %f", vecs[0][0])
	}
}

func TestRouterNoEngine(t *testing.T) {
	e1 := &mockEngine{name: "a", available: false}
	e2 := &mockEngine{name: "b", available: false}

	r := &Router{engines: []EmbedEngine{e1, e2}}

	_, err := r.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error when no engine available")
	}
}

func TestRouterEmptyInput(t *testing.T) {
	r := &Router{engines: []EmbedEngine{}}
	vecs, err := r.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatalf("expected no error for empty input, got: %v", err)
	}
	if vecs != nil {
		t.Fatalf("expected nil for empty input, got: %v", vecs)
	}
}

func TestRouterPriority(t *testing.T) {
	first := &mockEngine{name: "first", available: true, vectors: [][]float32{{1.0}}}
	second := &mockEngine{name: "second", available: true, vectors: [][]float32{{2.0}}}

	r := &Router{engines: []EmbedEngine{first, second}}

	vecs, err := r.Embed(context.Background(), []string{"test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vecs[0][0] != 1.0 {
		t.Errorf("expected first engine result (1.0), got %f — priority order broken", vecs[0][0])
	}
}

func TestActiveEngine(t *testing.T) {
	e1 := &mockEngine{name: "ollama", available: false}
	e2 := &mockEngine{name: "cloud", available: true}

	r := &Router{engines: []EmbedEngine{e1, e2}}

	if name := r.ActiveEngine(); name != "cloud" {
		t.Errorf("expected active engine 'cloud', got '%s'", name)
	}

	e1.available = true
	if name := r.ActiveEngine(); name != "ollama" {
		t.Errorf("expected active engine 'ollama', got '%s'", name)
	}
}

// ============================================================================
// Ollama Tests (using httptest mock server)
// ============================================================================

func TestOllamaAvailable(t *testing.T) {
	// Mock Ollama server — responds 200 OK on root
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ollama is running"))
		}
	}))
	defer srv.Close()

	engine := NewOllamaEngine(srv.URL, "test-model")
	if !engine.Available() {
		t.Error("expected Ollama to be available with mock server")
	}

	// Test with unreachable server
	engine2 := NewOllamaEngine("http://127.0.0.1:1", "test-model")
	if engine2.Available() {
		t.Error("expected Ollama to be unavailable with unreachable server")
	}
}

func TestOllamaEmbed(t *testing.T) {
	// Mock Ollama /api/embed endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/embed":
			var req ollamaEmbedRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Model != "test-model" {
				t.Errorf("expected model 'test-model', got '%s'", req.Model)
			}

			resp := ollamaEmbedResponse{
				Model:      req.Model,
				Embeddings: make([][]float32, len(req.Input)),
			}
			for i := range req.Input {
				resp.Embeddings[i] = []float32{0.1, 0.2, 0.3, 0.4}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	engine := NewOllamaEngine(srv.URL, "test-model")

	vecs, err := engine.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	if len(vecs[0]) != 4 {
		t.Fatalf("expected dim 4, got %d", len(vecs[0]))
	}
	if engine.Dimension() != 4 {
		t.Errorf("expected cached dimension 4, got %d", engine.Dimension())
	}
}

func TestOllamaEmbedServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/embed" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("model not found"))
		}
	}))
	defer srv.Close()

	engine := NewOllamaEngine(srv.URL, "nonexistent")

	_, err := engine.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error for server 500 response")
	}
}

func TestOllamaEmbedEmpty(t *testing.T) {
	engine := NewOllamaEngine("http://localhost:11434", "test-model")

	vecs, err := engine.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatalf("expected no error for empty input, got: %v", err)
	}
	if vecs != nil {
		t.Fatalf("expected nil for empty input, got: %v", vecs)
	}
}
