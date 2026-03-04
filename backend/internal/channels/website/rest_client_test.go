package website

// ============================================================================
// website/rest_client_test.go — 通用 REST 发布器单元测试
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P4-1
// ============================================================================

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/media"
)

// ---------- helpers ----------

func newTestConfig(serverURL string) *WebsiteConfig {
	return &WebsiteConfig{
		Enabled:        true,
		APIURL:         serverURL + "/api/posts",
		AuthType_:      AuthBearer,
		AuthToken:      "test-token-123",
		TimeoutSeconds: 5,
		MaxRetries:     1,
	}
}

func newTestDraft() *media.ContentDraft {
	return &media.ContentDraft{
		ID:       "draft-001",
		Title:    "Test Article",
		Body:     "This is a test article body.",
		Images:   []string{"https://example.com/img1.jpg"},
		Tags:     []string{"test", "golang"},
		Platform: media.PlatformWebsite,
	}
}

// ---------- tests ----------

func TestPublish_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token-123" {
			t.Errorf("auth: got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type: got %q", r.Header.Get("Content-Type"))
		}

		var payload publishPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Title != "Test Article" {
			t.Errorf("title: got %q", payload.Title)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(publishResponse{
			ID:  "post-123",
			URL: "https://blog.example.com/post-123",
		})
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client := NewWebsiteClient(cfg)

	result, err := client.Publish(context.Background(), newTestDraft())
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.PostID != "post-123" {
		t.Errorf("PostID: got %q, want %q", result.PostID, "post-123")
	}
	if result.Platform != media.PlatformWebsite {
		t.Errorf("Platform: got %q", result.Platform)
	}
	if result.Status != "published" {
		t.Errorf("Status: got %q", result.Status)
	}
}

func TestPublish_APIKeyAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "my-api-key" {
			t.Errorf("X-API-Key: got %q", r.Header.Get("X-API-Key"))
		}
		json.NewEncoder(w).Encode(publishResponse{ID: "p1", URL: "https://x.com/p1"})
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.AuthType_ = AuthAPIKey
	cfg.AuthToken = "my-api-key"

	client := NewWebsiteClient(cfg)
	result, err := client.Publish(context.Background(), newTestDraft())
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.PostID != "p1" {
		t.Errorf("PostID: got %q", result.PostID)
	}
}

func TestPublish_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("basic auth: ok=%v user=%q pass=%q", ok, user, pass)
		}
		json.NewEncoder(w).Encode(publishResponse{ID: "p2", URL: "https://x.com/p2"})
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.AuthType_ = AuthBasic
	cfg.AuthToken = "admin:secret"

	client := NewWebsiteClient(cfg)
	result, err := client.Publish(context.Background(), newTestDraft())
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.PostID != "p2" {
		t.Errorf("PostID: got %q", result.PostID)
	}
}

func TestPublish_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client := NewWebsiteClient(cfg)

	_, err := client.Publish(context.Background(), newTestDraft())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPublish_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(publishResponse{Error: "invalid content"})
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client := NewWebsiteClient(cfg)

	_, err := client.Publish(context.Background(), newTestDraft())
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "invalid content") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPublish_NilDraft(t *testing.T) {
	cfg := &WebsiteConfig{
		APIURL:         "https://example.com/api",
		AuthType_:      AuthBearer,
		AuthToken:      "tok",
		TimeoutSeconds: 5,
		MaxRetries:     1,
	}
	client := NewWebsiteClient(cfg)
	_, err := client.Publish(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil draft")
	}
}

func TestPublish_ImageUpload(t *testing.T) {
	uploadCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "upload") {
			uploadCalled = true
			json.NewEncoder(w).Encode(map[string]string{
				"url": "https://cdn.example.com/uploaded.jpg",
			})
			return
		}
		// Main publish endpoint.
		var payload publishPayload
		json.NewDecoder(r.Body).Decode(&payload)
		if len(payload.Images) != 1 || payload.Images[0] != "https://cdn.example.com/uploaded.jpg" {
			t.Errorf("images: got %v", payload.Images)
		}
		json.NewEncoder(w).Encode(publishResponse{ID: "p3", URL: "https://x.com/p3"})
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.ImageUploadURL = server.URL + "/upload"

	client := NewWebsiteClient(cfg)
	result, err := client.Publish(context.Background(), newTestDraft())
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if !uploadCalled {
		t.Error("image upload endpoint not called")
	}
	if result.PostID != "p3" {
		t.Errorf("PostID: got %q", result.PostID)
	}
}

func TestPublish_RetryOnFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(publishResponse{ID: "p4", URL: "https://x.com/p4"})
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.MaxRetries = 3

	client := NewWebsiteClient(cfg)
	result, err := client.Publish(context.Background(), newTestDraft())
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.PostID != "p4" {
		t.Errorf("PostID: got %q", result.PostID)
	}
	if attempts != 2 {
		t.Errorf("attempts: got %d, want 2", attempts)
	}
}
