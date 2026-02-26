package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractHookToken_Bearer(t *testing.T) {
	r := httptest.NewRequest("POST", "/hooks/test", nil)
	r.Header.Set("Authorization", "Bearer secret123")
	token := ExtractHookToken(r)
	if token != "secret123" {
		t.Errorf("got %q, want secret123", token)
	}
}

func TestExtractHookToken_HeaderToken(t *testing.T) {
	r := httptest.NewRequest("POST", "/hooks/test", nil)
	r.Header.Set("X-OpenAcosmi-Token", "abc456")
	token := ExtractHookToken(r)
	if token != "abc456" {
		t.Errorf("got %q, want abc456", token)
	}
}

func TestExtractHookToken_Empty(t *testing.T) {
	r := httptest.NewRequest("POST", "/hooks/test", nil)
	token := ExtractHookToken(r)
	if token != "" {
		t.Errorf("got %q, want empty", token)
	}
}

func TestResolveHooksConfig_Disabled(t *testing.T) {
	cfg, err := ResolveHooksConfig(nil)
	if err != nil || cfg != nil {
		t.Errorf("nil raw should return nil, got %v, %v", cfg, err)
	}
}

func TestResolveHooksConfig_Enabled(t *testing.T) {
	enabled := true
	raw := &HooksRawConfig{
		Enabled: &enabled,
		Token:   "mytoken",
	}
	cfg, err := ResolveHooksConfig(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if cfg.BasePath != "/hooks" {
		t.Errorf("basePath = %q", cfg.BasePath)
	}
	if cfg.Token != "mytoken" {
		t.Errorf("token = %q", cfg.Token)
	}
	if cfg.MaxBodyBytes != DefaultHooksMaxBodyBytes {
		t.Errorf("maxBodyBytes = %d", cfg.MaxBodyBytes)
	}
}

func TestResolveHooksConfig_NoToken(t *testing.T) {
	enabled := true
	raw := &HooksRawConfig{
		Enabled: &enabled,
	}
	_, err := ResolveHooksConfig(raw)
	if err == nil {
		t.Error("expected error for missing token")
	}
}

func TestResolveHooksConfig_CustomPath(t *testing.T) {
	enabled := true
	raw := &HooksRawConfig{
		Enabled: &enabled,
		Token:   "tok",
		Path:    "webhooks/",
	}
	cfg, err := ResolveHooksConfig(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if cfg.BasePath != "/webhooks" {
		t.Errorf("basePath = %q", cfg.BasePath)
	}
}

func TestNormalizeHookHeaders(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("X-GitHub-Event", "push")
	r.Header.Set("Content-Type", "application/json")
	h := NormalizeHookHeaders(r)
	if h["x-github-event"] != "push" {
		t.Errorf("github event = %q", h["x-github-event"])
	}
	if h["content-type"] != "application/json" {
		t.Errorf("content-type = %q", h["content-type"])
	}
}

func TestNormalizeWakePayload(t *testing.T) {
	payload := map[string]interface{}{"text": "wake up"}
	p, err := NormalizeWakePayload(payload)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.Text != "wake up" || p.Mode != "now" {
		t.Errorf("got %+v", p)
	}
}

func TestNormalizeWakePayload_MissingText(t *testing.T) {
	payload := map[string]interface{}{}
	_, err := NormalizeWakePayload(payload)
	if err == nil {
		t.Error("expected error for missing text")
	}
}

func TestNormalizeAgentPayload(t *testing.T) {
	payload := map[string]interface{}{
		"message": "hello agent",
		"channel": "new",
	}
	p, err := NormalizeAgentPayload(payload)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.Message != "hello agent" {
		t.Errorf("message = %q", p.Message)
	}
	if p.Name != "Hook" {
		t.Errorf("name = %q", p.Name)
	}
	if p.Channel != "new" {
		t.Errorf("channel = %q", string(p.Channel))
	}
	if !p.Deliver {
		t.Error("deliver should default to true")
	}
}

func TestNormalizeAgentPayload_MissingMessage(t *testing.T) {
	payload := map[string]interface{}{}
	_, err := NormalizeAgentPayload(payload)
	if err == nil {
		t.Error("expected error for missing message")
	}
}

func TestHooksHTTPHandler_NoHooksConfig(t *testing.T) {
	handler := createHooksHTTPHandler(GatewayHTTPHandlerConfig{
		GetHooksConfig: func() *HooksConfig { return nil },
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/hooks/test", nil)
	handler(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHooksHTTPHandler_WrongMethod(t *testing.T) {
	handler := createHooksHTTPHandler(GatewayHTTPHandlerConfig{
		GetHooksConfig: func() *HooksConfig {
			return &HooksConfig{Token: "secret", BasePath: "/hooks"}
		},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/hooks/test", nil)
	handler(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHooksHTTPHandler_BadToken(t *testing.T) {
	handler := createHooksHTTPHandler(GatewayHTTPHandlerConfig{
		GetHooksConfig: func() *HooksConfig {
			return &HooksConfig{Token: "secret", BasePath: "/hooks", MaxBodyBytes: 1024}
		},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/hooks/test", strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer wrong")
	handler(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
