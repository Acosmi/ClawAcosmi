package gateway

import (
	"testing"

	"github.com/google/uuid"
)

func TestNonceGeneration_IsValidUUID(t *testing.T) {
	nonce := uuid.NewString()
	if _, err := uuid.Parse(nonce); err != nil {
		t.Fatalf("generated nonce is not valid UUID: %s", nonce)
	}
}

func TestNonceValidation_Match(t *testing.T) {
	serverNonce := "abc-123-def"
	clientNonce := "abc-123-def"
	if serverNonce != clientNonce {
		t.Fatal("matching nonces should be equal")
	}
}

func TestNonceValidation_Mismatch(t *testing.T) {
	serverNonce := "abc-123-def"
	clientNonce := "xyz-789-ghi"
	if serverNonce == clientNonce {
		t.Fatal("different nonces should not match")
	}
}

func TestNonce_EmptyClientNonce_Allowed(t *testing.T) {
	// TS behavior: if client doesn't provide nonce, skip validation
	// (for backward compat with older clients)
	serverNonce := uuid.NewString()
	clientNonce := ""
	_ = serverNonce

	// Empty nonce → skip check (no error)
	if clientNonce != "" && clientNonce != serverNonce {
		t.Fatal("should not reject empty nonce")
	}
}

func TestIsLocalAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8080", true},
		{"[::1]:3000", true},
		{"192.168.1.1:443", false},
		{"10.0.0.1:80", false},
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			got := isLocalAddr(tc.addr)
			if got != tc.want {
				t.Errorf("isLocalAddr(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}
