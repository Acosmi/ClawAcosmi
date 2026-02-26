package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// generateTestEd25519Keypair 生成测试用 Ed25519 密钥对。
func generateTestEd25519Keypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}
	return pub, priv, hex.EncodeToString(pub)
}

// signInteraction 用私钥签名 timestamp+body。
func signInteraction(priv ed25519.PrivateKey, timestamp string, body []byte) string {
	msg := make([]byte, 0, len(timestamp)+len(body))
	msg = append(msg, []byte(timestamp)...)
	msg = append(msg, body...)
	sig := ed25519.Sign(priv, msg)
	return hex.EncodeToString(sig)
}

// ── VerifyDiscordInteractionSignature ──

func TestVerifySignature_Valid(t *testing.T) {
	_, priv, pubHex := generateTestEd25519Keypair(t)

	timestamp := "1234567890"
	body := []byte(`{"type":1}`)
	sigHex := signInteraction(priv, timestamp, body)

	if !VerifyDiscordInteractionSignature(pubHex, sigHex, timestamp, body) {
		t.Error("valid signature should pass verification")
	}
}

func TestVerifySignature_InvalidSignature(t *testing.T) {
	_, _, pubHex := generateTestEd25519Keypair(t)

	timestamp := "1234567890"
	body := []byte(`{"type":1}`)
	// 使用伪造的签名
	fakeSig := strings.Repeat("aa", ed25519.SignatureSize)

	if VerifyDiscordInteractionSignature(pubHex, fakeSig, timestamp, body) {
		t.Error("forged signature should fail verification")
	}
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	_, priv, pubHex := generateTestEd25519Keypair(t)

	timestamp := "1234567890"
	body := []byte(`{"type":1}`)
	sigHex := signInteraction(priv, timestamp, body)

	// 篡改 body
	tampered := []byte(`{"type":2}`)
	if VerifyDiscordInteractionSignature(pubHex, sigHex, timestamp, tampered) {
		t.Error("tampered body should fail verification")
	}
}

func TestVerifySignature_TamperedTimestamp(t *testing.T) {
	_, priv, pubHex := generateTestEd25519Keypair(t)

	timestamp := "1234567890"
	body := []byte(`{"type":1}`)
	sigHex := signInteraction(priv, timestamp, body)

	// 篡改 timestamp
	if VerifyDiscordInteractionSignature(pubHex, sigHex, "9999999999", body) {
		t.Error("tampered timestamp should fail verification")
	}
}

func TestVerifySignature_InvalidPublicKey(t *testing.T) {
	if VerifyDiscordInteractionSignature("notahexkey", "aabb", "123", []byte("x")) {
		t.Error("invalid public key should fail")
	}
}

func TestVerifySignature_InvalidSignatureHex(t *testing.T) {
	_, _, pubHex := generateTestEd25519Keypair(t)
	if VerifyDiscordInteractionSignature(pubHex, "zzzz", "123", []byte("x")) {
		t.Error("invalid hex signature should fail")
	}
}

func TestVerifySignature_WrongKeySize(t *testing.T) {
	// 公钥长度错误
	if VerifyDiscordInteractionSignature("aabbccdd", strings.Repeat("aa", 64), "123", []byte("x")) {
		t.Error("wrong key size should fail")
	}
}

// ── DiscordInteractionHandler HTTP ──

func TestHandler_MissingSignatureHeaders(t *testing.T) {
	_, _, pubHex := generateTestEd25519Keypair(t)
	handler := &DiscordInteractionHandler{PublicKey: pubHex}

	req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(`{"type":1}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandler_InvalidSignature(t *testing.T) {
	_, _, pubHex := generateTestEd25519Keypair(t)
	handler := &DiscordInteractionHandler{PublicKey: pubHex}

	body := `{"type":1}`
	req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(body))
	req.Header.Set("X-Signature-Ed25519", strings.Repeat("aa", ed25519.SignatureSize))
	req.Header.Set("X-Signature-Timestamp", "1234567890")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandler_PingPong(t *testing.T) {
	_, priv, pubHex := generateTestEd25519Keypair(t)
	handler := &DiscordInteractionHandler{PublicKey: pubHex}

	body := `{"type":1}`
	timestamp := "1234567890"
	sigHex := signInteraction(priv, timestamp, []byte(body))

	req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(body))
	req.Header.Set("X-Signature-Ed25519", sigHex)
	req.Header.Set("X-Signature-Timestamp", timestamp)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp interactionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Type != interactionResponsePong {
		t.Errorf("expected PONG type=%d, got %d", interactionResponsePong, resp.Type)
	}
}

func TestHandler_ApplicationCommand(t *testing.T) {
	_, priv, pubHex := generateTestEd25519Keypair(t)

	var gotType int
	var gotBody []byte

	handler := &DiscordInteractionHandler{
		PublicKey: pubHex,
		OnInteraction: func(w http.ResponseWriter, body []byte, interactionType int) {
			gotType = interactionType
			gotBody = body
			w.WriteHeader(http.StatusOK)
		},
	}

	body := `{"type":2,"data":{"name":"test"}}`
	timestamp := "1234567890"
	sigHex := signInteraction(priv, timestamp, []byte(body))

	req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(body))
	req.Header.Set("X-Signature-Ed25519", sigHex)
	req.Header.Set("X-Signature-Timestamp", timestamp)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotType != interactionTypeApplicationCommand {
		t.Errorf("expected interaction type %d, got %d", interactionTypeApplicationCommand, gotType)
	}
	if !strings.Contains(string(gotBody), `"name":"test"`) {
		t.Errorf("body not passed to callback: %s", gotBody)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	handler := &DiscordInteractionHandler{PublicKey: "abc"}

	req := httptest.NewRequest(http.MethodGet, "/interactions", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
