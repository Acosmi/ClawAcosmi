package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeriveCopilotApiBaseUrlFromToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{"empty token", "", ""},
		{"whitespace token", "   ", ""},
		{"no proxy-ep", "tid=abc;exp=123", ""},
		{"with proxy-ep", "tid=abc;proxy-ep=proxy.individual.githubcopilot.com;exp=123", "https://api.individual.githubcopilot.com"},
		{"proxy-ep at start", "proxy-ep=proxy.business.githubcopilot.com;tid=abc", "https://api.business.githubcopilot.com"},
		{"proxy-ep with https", "tid=abc;proxy-ep=https://proxy.individual.githubcopilot.com;exp=123", "https://api.individual.githubcopilot.com"},
		{"proxy-ep no proxy prefix", "tid=abc;proxy-ep=api.custom.example.com;exp=123", "https://api.custom.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveCopilotApiBaseUrlFromToken(tt.token)
			if got != tt.want {
				t.Errorf("DeriveCopilotApiBaseUrlFromToken(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestIsTokenUsable(t *testing.T) {
	now := int64(1_700_000_000_000) // 2023

	tests := []struct {
		name  string
		cache *CachedCopilotToken
		want  bool
	}{
		{"nil cache", nil, false},
		{"empty token", &CachedCopilotToken{Token: "", ExpiresAt: now + 600_000}, false},
		{"expired", &CachedCopilotToken{Token: "abc", ExpiresAt: now + 100_000}, false},
		{"near expiry", &CachedCopilotToken{Token: "abc", ExpiresAt: now + 200_000}, false},
		{"valid", &CachedCopilotToken{Token: "abc", ExpiresAt: now + 600_000}, true},
		{"well in future", &CachedCopilotToken{Token: "abc", ExpiresAt: now + 3_600_000}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTokenUsable(tt.cache, now)
			if got != tt.want {
				t.Errorf("isTokenUsable(%v, %d) = %v, want %v", tt.cache, now, got, tt.want)
			}
		})
	}
}

func TestParseCopilotTokenResponse(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantToken string
		wantMs    int64
		wantErr   bool
	}{
		{
			name:      "valid unix seconds",
			json:      `{"token":"tok-abc","expires_at":1700000000}`,
			wantToken: "tok-abc",
			wantMs:    1_700_000_000_000,
		},
		{
			name:      "valid unix ms",
			json:      `{"token":"tok-abc","expires_at":1700000000000}`,
			wantToken: "tok-abc",
			wantMs:    1_700_000_000_000,
		},
		{
			name:      "string expires_at",
			json:      `{"token":"tok-abc","expires_at":"1700000000"}`,
			wantToken: "tok-abc",
			wantMs:    1_700_000_000_000,
		},
		{
			name:    "missing token",
			json:    `{"expires_at":1700000000}`,
			wantErr: true,
		},
		{
			name:    "missing expires_at",
			json:    `{"token":"abc"}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			json:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, expiresAt, err := parseCopilotTokenResponse([]byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
			if expiresAt != tt.wantMs {
				t.Errorf("expiresAt = %d, want %d", expiresAt, tt.wantMs)
			}
		})
	}
}

func TestRefreshQwenPortalCredentials(t *testing.T) {
	// 模拟 Qwen OAuth token 端点
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", r.Form.Get("grant_type"))
		}
		if r.Form.Get("client_id") != QwenOAuthClientID {
			t.Errorf("client_id = %q, want %q", r.Form.Get("client_id"), QwenOAuthClientID)
		}
		if r.Form.Get("refresh_token") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in": 3600
		}`))
	}))
	defer server.Close()

	// 注意：由于 QwenOAuthTokenEndpoint 是常量，无法在测试中替换。
	// 此测试验证函数签名和类型正确性，完整 E2E 测试需运行时环境。
	t.Run("nil credentials", func(t *testing.T) {
		_, err := RefreshQwenPortalCredentials(t.Context(), nil, nil)
		if err == nil {
			t.Error("expected error for nil credentials")
		}
	})

	t.Run("empty refresh token", func(t *testing.T) {
		_, err := RefreshQwenPortalCredentials(t.Context(), nil, &OAuthCredentials{
			Refresh: "",
		})
		if err == nil {
			t.Error("expected error for empty refresh token")
		}
	})
}

func TestStoreCopilotAuthProfile(t *testing.T) {
	dir := t.TempDir()
	store := NewAuthStore(dir + "/auth.json")

	if err := StoreCopilotAuthProfile(store, "github-copilot:test", "ghu_test123"); err != nil {
		t.Fatalf("StoreCopilotAuthProfile: %v", err)
	}

	profile := store.GetProfile("github-copilot:test")
	if profile == nil {
		t.Fatal("profile not found")
	}
	if profile.Type != CredentialToken {
		t.Errorf("Type = %q, want %q", profile.Type, CredentialToken)
	}
	if profile.Provider != CopilotProfilePrefix {
		t.Errorf("Provider = %q, want %q", profile.Provider, CopilotProfilePrefix)
	}
	if profile.Token != "ghu_test123" {
		t.Errorf("Token = %q, want %q", profile.Token, "ghu_test123")
	}
}
