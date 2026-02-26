package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// computeTestHMAC 计算测试用 HMAC-SHA256 签名。
func computeTestHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// ── VerifyWhatsAppWebhookSignature ──

func TestVerifySignature_Valid(t *testing.T) {
	secret := "test-secret-123"
	body := []byte(`{"entry":[]}`)
	sig := computeTestHMAC(secret, body)

	if !VerifyWhatsAppWebhookSignature(secret, sig, body) {
		t.Error("valid HMAC signature should pass verification")
	}
}

func TestVerifySignature_InvalidSignature(t *testing.T) {
	secret := "test-secret-123"
	body := []byte(`{"entry":[]}`)
	// 使用错误的 secret 计算签名
	wrongSig := computeTestHMAC("wrong-secret", body)

	if VerifyWhatsAppWebhookSignature(secret, wrongSig, body) {
		t.Error("wrong secret signature should fail verification")
	}
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	secret := "test-secret-123"
	body := []byte(`{"entry":[]}`)
	sig := computeTestHMAC(secret, body)

	tampered := []byte(`{"entry":[{"id":"tampered"}]}`)
	if VerifyWhatsAppWebhookSignature(secret, sig, tampered) {
		t.Error("tampered body should fail verification")
	}
}

func TestVerifySignature_MissingPrefix(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{}`)
	// 缺少 "sha256=" 前缀
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	rawHex := hex.EncodeToString(mac.Sum(nil))

	if VerifyWhatsAppWebhookSignature(secret, rawHex, body) {
		t.Error("signature without sha256= prefix should fail")
	}
}

func TestVerifySignature_EmptySecret(t *testing.T) {
	if VerifyWhatsAppWebhookSignature("", "sha256=aabb", []byte("x")) {
		t.Error("empty secret should fail")
	}
}

func TestVerifySignature_EmptySignature(t *testing.T) {
	if VerifyWhatsAppWebhookSignature("secret", "", []byte("x")) {
		t.Error("empty signature should fail")
	}
}

func TestVerifySignature_InvalidHex(t *testing.T) {
	if VerifyWhatsAppWebhookSignature("secret", "sha256=zzzz", []byte("x")) {
		t.Error("invalid hex should fail")
	}
}

// ── WhatsAppWebhookHandler: GET 验证 ──

func TestHandler_VerifySubscription_Valid(t *testing.T) {
	handler := &WhatsAppWebhookHandler{
		VerifyToken: "my-verify-token",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/webhook?hub.mode=subscribe&hub.verify_token=my-verify-token&hub.challenge=abc123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "abc123" {
		t.Errorf("expected challenge 'abc123', got %q", w.Body.String())
	}
}

func TestHandler_VerifySubscription_WrongToken(t *testing.T) {
	handler := &WhatsAppWebhookHandler{
		VerifyToken: "my-verify-token",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/webhook?hub.mode=subscribe&hub.verify_token=wrong-token&hub.challenge=abc", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestHandler_VerifySubscription_MissingChallenge(t *testing.T) {
	handler := &WhatsAppWebhookHandler{
		VerifyToken: "my-verify-token",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/webhook?hub.mode=subscribe&hub.verify_token=my-verify-token", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for missing challenge, got %d", w.Code)
	}
}

func TestHandler_VerifySubscription_WrongMode(t *testing.T) {
	handler := &WhatsAppWebhookHandler{
		VerifyToken: "my-verify-token",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/webhook?hub.mode=unsubscribe&hub.verify_token=my-verify-token&hub.challenge=abc", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong mode, got %d", w.Code)
	}
}

// ── WhatsAppWebhookHandler: POST 事件 ──

func TestHandler_PostEvent_ValidSignature(t *testing.T) {
	secret := "app-secret-xyz"
	body := `{"entry":[{"id":"12345"}]}`
	sig := computeTestHMAC(secret, []byte(body))

	var gotBody []byte
	handler := &WhatsAppWebhookHandler{
		AppSecret: secret,
		OnEvent: func(b []byte) {
			gotBody = b
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(string(gotBody), "12345") {
		t.Errorf("callback should receive body, got: %s", gotBody)
	}
}

func TestHandler_PostEvent_InvalidSignature(t *testing.T) {
	handler := &WhatsAppWebhookHandler{
		AppSecret: "app-secret-xyz",
	}

	body := `{"entry":[]}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256="+strings.Repeat("aa", 32))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandler_PostEvent_MissingSignature(t *testing.T) {
	handler := &WhatsAppWebhookHandler{
		AppSecret: "app-secret-xyz",
	}

	body := `{"entry":[]}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	// 不设置 X-Hub-Signature-256 header
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandler_PostEvent_NoSecret(t *testing.T) {
	// 不配置 AppSecret 时跳过签名验证
	var called bool
	handler := &WhatsAppWebhookHandler{
		OnEvent: func(b []byte) {
			called = true
		},
	}

	body := `{"entry":[]}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when no secret configured, got %d", w.Code)
	}
	if !called {
		t.Error("OnEvent should be called when no secret configured")
	}
}

func TestHandler_PostEvent_InvalidJSON(t *testing.T) {
	handler := &WhatsAppWebhookHandler{}

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("not json!"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	handler := &WhatsAppWebhookHandler{}

	req := httptest.NewRequest(http.MethodPut, "/webhook", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
