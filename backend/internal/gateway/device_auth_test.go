package gateway

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"
)

// generateTestKeyPair 生成测试用 Ed25519 密钥对。
func generateTestKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}
	derBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to marshal PKIX: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: derBytes})
	return pub, priv, string(pemBlock)
}

func TestBuildDeviceAuthPayload_V1(t *testing.T) {
	payload := BuildDeviceAuthPayload(DeviceAuthPayloadParams{
		DeviceID:   "dev123",
		ClientID:   "client1",
		ClientMode: "normal",
		Role:       "operator",
		Scopes:     []string{"chat", "admin"},
		SignedAtMs: 1700000000000,
		Token:      "tok",
		Version:    "v1",
	})
	expected := "v1|dev123|client1|normal|operator|chat,admin|1700000000000|tok"
	if payload != expected {
		t.Errorf("v1 payload mismatch:\ngot:  %s\nwant: %s", payload, expected)
	}
}

func TestBuildDeviceAuthPayload_V2(t *testing.T) {
	payload := BuildDeviceAuthPayload(DeviceAuthPayloadParams{
		DeviceID:   "dev123",
		ClientID:   "client1",
		ClientMode: "normal",
		Role:       "operator",
		Scopes:     []string{"chat"},
		SignedAtMs: 1700000000000,
		Nonce:      "nonce-abc",
	})
	expected := "v2|dev123|client1|normal|operator|chat|1700000000000||nonce-abc"
	if payload != expected {
		t.Errorf("v2 payload mismatch:\ngot:  %s\nwant: %s", payload, expected)
	}
}

func TestBuildDeviceAuthPayload_AutoVersion(t *testing.T) {
	// With nonce → auto v2
	p1 := BuildDeviceAuthPayload(DeviceAuthPayloadParams{
		DeviceID: "d", ClientID: "c", ClientMode: "m", Role: "r",
		Scopes: nil, SignedAtMs: 100, Nonce: "n",
	})
	if p1[:2] != "v2" {
		t.Errorf("expected auto v2, got: %s", p1[:2])
	}

	// Without nonce → auto v1
	p2 := BuildDeviceAuthPayload(DeviceAuthPayloadParams{
		DeviceID: "d", ClientID: "c", ClientMode: "m", Role: "r",
		Scopes: nil, SignedAtMs: 100,
	})
	if p2[:2] != "v1" {
		t.Errorf("expected auto v1, got: %s", p2[:2])
	}
}

func TestDeriveDeviceIdFromPublicKey_PEM(t *testing.T) {
	_, _, pemStr := generateTestKeyPair(t)
	id, err := DeriveDeviceIdFromPublicKey(pemStr)
	if err != nil {
		t.Fatalf("deriveDeviceId failed: %v", err)
	}
	if len(id) != 64 { // SHA256 hex = 64 chars
		t.Errorf("expected 64 char hex, got %d chars: %s", len(id), id)
	}

	// Deterministic: same key → same ID
	id2, _ := DeriveDeviceIdFromPublicKey(pemStr)
	if id != id2 {
		t.Error("same key should produce same device ID")
	}
}

func TestDeriveDeviceIdFromPublicKey_Base64Url(t *testing.T) {
	pub, _, pemStr := generateTestKeyPair(t)

	// Get ID from PEM
	idPEM, _ := DeriveDeviceIdFromPublicKey(pemStr)

	// Get ID from base64url raw key
	b64 := base64UrlEncode([]byte(pub))
	idB64, err := DeriveDeviceIdFromPublicKey(b64)
	if err != nil {
		t.Fatalf("deriveDeviceId from base64url failed: %v", err)
	}

	if idPEM != idB64 {
		t.Errorf("PEM and base64url should produce same ID:\nPEM:    %s\nBase64: %s", idPEM, idB64)
	}
}

func TestVerifyDeviceSignature_Valid(t *testing.T) {
	_, priv, pemStr := generateTestKeyPair(t)
	payload := "v2|dev|c|m|r|s|100||nonce"
	sig := ed25519.Sign(priv, []byte(payload))
	sigB64 := base64UrlEncode(sig)

	if !VerifyDeviceSignature(pemStr, payload, sigB64) {
		t.Error("valid signature should verify")
	}
}

func TestVerifyDeviceSignature_Invalid(t *testing.T) {
	_, _, pemStr := generateTestKeyPair(t)
	if VerifyDeviceSignature(pemStr, "payload", "bad-sig") {
		t.Error("invalid signature should not verify")
	}
}

func TestVerifyDeviceSignature_WrongKey(t *testing.T) {
	_, priv, _ := generateTestKeyPair(t)
	_, _, otherPem := generateTestKeyPair(t)

	payload := "test-payload"
	sig := ed25519.Sign(priv, []byte(payload))
	sigB64 := base64UrlEncode(sig)

	if VerifyDeviceSignature(otherPem, payload, sigB64) {
		t.Error("signature with wrong key should not verify")
	}
}

func TestIsSignedAtValid(t *testing.T) {
	nowMs := time.Now().UnixMilli()
	if !IsSignedAtValid(nowMs) {
		t.Error("current time should be valid")
	}
	if !IsSignedAtValid(nowMs - 5*60*1000) { // 5 min ago
		t.Error("5 min ago should be valid")
	}
	if IsSignedAtValid(nowMs - 15*60*1000) { // 15 min ago
		t.Error("15 min ago should be invalid")
	}
}

func TestValidateDeviceAuth_FullFlow(t *testing.T) {
	pub, priv, pemStr := generateTestKeyPair(t)

	deviceID, _ := DeriveDeviceIdFromPublicKey(pemStr)
	signedAt := time.Now().UnixMilli()
	nonce := "test-nonce-123"

	payload := BuildDeviceAuthPayload(DeviceAuthPayloadParams{
		DeviceID:   deviceID,
		ClientID:   "control-ui",
		ClientMode: "normal",
		Role:       "operator",
		Scopes:     []string{"chat"},
		SignedAtMs: signedAt,
		Nonce:      nonce,
	})
	sig := ed25519.Sign(priv, []byte(payload))
	sigB64 := base64UrlEncode(sig)

	device := &ConnectDeviceAuth{
		ID:        deviceID,
		PublicKey: base64UrlEncode([]byte(pub)),
		Signature: sigB64,
		SignedAt:  signedAt,
		Nonce:     nonce,
	}

	result := ValidateDeviceAuth(device, "control-ui", "normal", "operator", []string{"chat"}, "", false)
	if !result.OK {
		t.Fatalf("full flow validation failed: %s", result.Reason)
	}
	if result.DeviceID != deviceID {
		t.Errorf("deviceID mismatch: got %s, want %s", result.DeviceID, deviceID)
	}
}

func TestValidateDeviceAuth_NilDevice(t *testing.T) {
	result := ValidateDeviceAuth(nil, "c", "m", "r", nil, "", false)
	if !result.OK {
		t.Error("nil device should be OK (skip)")
	}
}

func TestValidateDeviceAuth_BadPublicKey(t *testing.T) {
	device := &ConnectDeviceAuth{
		ID:        "fake",
		PublicKey: "not-a-key",
		Signature: "sig",
		SignedAt:  time.Now().UnixMilli(),
	}
	result := ValidateDeviceAuth(device, "c", "m", "r", nil, "", false)
	if result.OK {
		t.Error("bad public key should fail")
	}
}

func TestValidateDeviceAuth_IDMismatch(t *testing.T) {
	pub, _, _ := generateTestKeyPair(t)
	device := &ConnectDeviceAuth{
		ID:        "wrong-id",
		PublicKey: base64UrlEncode([]byte(pub)),
		Signature: "sig",
		SignedAt:  time.Now().UnixMilli(),
	}
	result := ValidateDeviceAuth(device, "c", "m", "r", nil, "", false)
	if result.OK {
		t.Error("ID mismatch should fail")
	}
	if result.Reason != "device ID does not match public key" {
		t.Errorf("unexpected reason: %s", result.Reason)
	}
}

func TestValidateDeviceAuth_ExpiredSignature(t *testing.T) {
	pub, _, pemStr := generateTestKeyPair(t)
	deviceID, _ := DeriveDeviceIdFromPublicKey(pemStr)
	device := &ConnectDeviceAuth{
		ID:        deviceID,
		PublicKey: base64UrlEncode([]byte(pub)),
		Signature: "sig",
		SignedAt:  time.Now().UnixMilli() - 20*60*1000, // 20 min ago
	}
	result := ValidateDeviceAuth(device, "c", "m", "r", nil, "", false)
	if result.OK {
		t.Error("expired signature should fail")
	}
	fmt.Println("reason:", result.Reason) // for debugging
}
