package wechat_mp

// ============================================================================
// wechat_mp/client_test.go — 微信公众号客户端单元测试
// 使用 httptest.Server mock 微信 API 进行测试。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-1
// ============================================================================

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------- helpers ----------

// newTestServer creates a mock WeChat API server.
func newTestServer(handler http.HandlerFunc) (*httptest.Server, *WeChatMPClient) {
	srv := httptest.NewServer(handler)
	client := &WeChatMPClient{
		AppID:     "test_app_id",
		AppSecret: "test_secret",
		BaseURL:   srv.URL,
		Client:    srv.Client(),
	}
	return srv, client
}

// ---------- GetAccessToken tests ----------

func TestGetAccessToken_Success(t *testing.T) {
	srv, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/cgi-bin/token") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("appid") != "test_app_id" {
			t.Errorf("missing appid param")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "mock_token_123",
			"expires_in":   7200,
		})
	})
	defer srv.Close()

	token, err := client.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if token != "mock_token_123" {
		t.Errorf("token: got %q, want %q", token, "mock_token_123")
	}
}

func TestGetAccessToken_CacheHit(t *testing.T) {
	callCount := 0
	srv, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "cached_token",
			"expires_in":   7200,
		})
	})
	defer srv.Close()

	// First call — should hit server.
	_, err := client.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Second call — should use cache.
	token, err := client.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if token != "cached_token" {
		t.Errorf("token: got %q, want %q", token, "cached_token")
	}
	if callCount != 1 {
		t.Errorf("server called %d times, expected 1 (cache miss only)", callCount)
	}
}

func TestGetAccessToken_Expired(t *testing.T) {
	callCount := 0
	srv, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new_token",
			"expires_in":   7200,
		})
	})
	defer srv.Close()

	// Pre-set an expired token.
	client.accessToken = "old_token"
	client.tokenExpiry = time.Now().Add(-1 * time.Minute)

	token, err := client.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if token != "new_token" {
		t.Errorf("expected new token after expiry, got %q", token)
	}
	if callCount != 1 {
		t.Errorf("expected 1 server call for refresh, got %d", callCount)
	}
}

func TestGetAccessToken_APIError(t *testing.T) {
	srv, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"errcode": 40013,
			"errmsg":  "invalid appid",
		})
	})
	defer srv.Close()

	_, err := client.GetAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid appid")
	}
	if !strings.Contains(err.Error(), "40013") {
		t.Errorf("error should contain error code: %v", err)
	}
}

// ---------- DoRequest tests ----------

func TestDoRequest_Success(t *testing.T) {
	srv, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/cgi-bin/token") {
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "req_token",
				"expires_in":   7200,
			})
			return
		}
		// Verify access_token is passed.
		if r.URL.Query().Get("access_token") != "req_token" {
			t.Errorf("missing access_token in request")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"errcode": 0,
			"errmsg":  "ok",
			"result":  "success",
		})
	})
	defer srv.Close()

	body, err := client.DoRequest(context.Background(), "POST",
		"/cgi-bin/draft/add", []byte(`{"test":"data"}`))
	if err != nil {
		t.Fatalf("DoRequest: %v", err)
	}
	if !strings.Contains(string(body), "success") {
		t.Errorf("unexpected response: %s", string(body))
	}
}

func TestDoRequest_APIError(t *testing.T) {
	srv, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/cgi-bin/token") {
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "err_token",
				"expires_in":   7200,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"errcode": 45009,
			"errmsg":  "reach max api daily quota limit",
		})
	})
	defer srv.Close()

	_, err := client.DoRequest(context.Background(), "POST",
		"/cgi-bin/freepublish/submit", nil)
	if err == nil {
		t.Fatal("expected error for API quota exceeded")
	}
	if !strings.Contains(err.Error(), "45009") {
		t.Errorf("error should contain error code: %v", err)
	}
}

// ---------- UploadImage tests ----------

func TestUploadImage_BadExtension(t *testing.T) {
	_, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {})

	_, err := client.UploadImage(context.Background(), "/tmp/test.gif")
	if err == nil {
		t.Fatal("expected error for .gif extension")
	}
	if !strings.Contains(err.Error(), "unsupported image format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUploadImage_TooLarge(t *testing.T) {
	// Create an oversized temp file.
	dir := t.TempDir()
	path := filepath.Join(dir, "large.jpg")
	data := make([]byte, maxImageSize+1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	_, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {})

	_, err := client.UploadImage(context.Background(), path)
	if err == nil {
		t.Fatal("expected error for oversized image")
	}
	if !strings.Contains(err.Error(), "image too large") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUploadImage_Success(t *testing.T) {
	srv, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/cgi-bin/token") {
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "upload_token",
				"expires_in":   7200,
			})
			return
		}
		if !strings.Contains(r.URL.Path, "/cgi-bin/media/uploadimg") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"url": "https://mmbiz.qpic.cn/test_image.jpg",
		})
	})
	defer srv.Close()

	// Create a small temp jpg.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	if err := os.WriteFile(path, []byte("fake-jpg-data"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	url, err := client.UploadImage(context.Background(), path)
	if err != nil {
		t.Fatalf("UploadImage: %v", err)
	}
	if !strings.Contains(url, "test_image.jpg") {
		t.Errorf("unexpected URL: %s", url)
	}
}

// ---------- Config validation tests ----------

func TestWeChatMPConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *WeChatMPConfig
		wantErr bool
	}{
		{"valid", &WeChatMPConfig{AppID: "id", AppSecret: "secret"}, false},
		{"nil config", nil, true},
		{"missing app_id", &WeChatMPConfig{AppSecret: "secret"}, true},
		{"missing app_secret", &WeChatMPConfig{AppID: "id"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
