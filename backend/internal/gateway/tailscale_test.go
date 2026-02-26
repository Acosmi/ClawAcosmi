package gateway

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TS 对照: src/infra/tailscale.test.ts (192L)

func TestParsePossiblyNoisyJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean JSON", `{"Self":{"DNSName":"host.ts.net."}}`, `{"Self":{"DNSName":"host.ts.net."}}`},
		{"noisy prefix", `some noise {"key":"value"} trailing`, `{"key":"value"}`},
		{"empty", "", ""},
		{"no braces", "no json here", "no json here"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := parsePossiblyNoisyJSON([]byte(tt.input))
			if string(got) != tt.want {
				t.Errorf("parsePossiblyNoisyJSON(%q) = %q, want %q", tt.input, string(got), tt.want)
			}
		})
	}
}

func TestParseWhoisIdentity(t *testing.T) {
	tests := []struct {
		name      string
		payload   map[string]interface{}
		wantLogin string
		wantName  string
		wantNil   bool
	}{
		{
			name: "UserProfile with LoginName",
			payload: map[string]interface{}{
				"UserProfile": map[string]interface{}{
					"LoginName":   "alice@example.com",
					"DisplayName": "Alice",
				},
			},
			wantLogin: "alice@example.com",
			wantName:  "Alice",
		},
		{
			name: "lowercase userProfile",
			payload: map[string]interface{}{
				"userProfile": map[string]interface{}{
					"Login": "bob@example.com",
					"Name":  "Bob",
				},
			},
			wantLogin: "bob@example.com",
			wantName:  "Bob",
		},
		{
			name: "flat login field",
			payload: map[string]interface{}{
				"login": "charlie@example.com",
			},
			wantLogin: "charlie@example.com",
		},
		{
			name:    "empty payload",
			payload: map[string]interface{}{},
			wantNil: true,
		},
		{
			name: "no login field",
			payload: map[string]interface{}{
				"UserProfile": map[string]interface{}{
					"DisplayName": "NoLogin",
				},
			},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWhoisIdentity(tt.payload)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil identity")
			}
			if got.Login != tt.wantLogin {
				t.Errorf("Login = %q, want %q", got.Login, tt.wantLogin)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

func TestIsPermissionDeniedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"permission denied", fmt.Errorf("permission denied"), true},
		{"access denied", fmt.Errorf("access denied"), true},
		{"operation not permitted", fmt.Errorf("operation not permitted"), true},
		{"requires root", fmt.Errorf("requires root"), true},
		{"random error", fmt.Errorf("connection refused"), false},
		{"mixed case", fmt.Errorf("Permission Denied"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPermissionDeniedError(tt.err)
			if got != tt.want {
				t.Errorf("isPermissionDeniedError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestTailscaleStatusParsing(t *testing.T) {
	// DNS 名优先
	t.Run("DNS name preferred", func(t *testing.T) {
		statusJSON := `{"Self":{"DNSName":"host.tailnet.ts.net.","TailscaleIPs":["100.1.1.1"]}}`
		var status tailscaleStatusJSON
		if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if status.Self == nil {
			t.Fatal("Self should not be nil")
		}
		dns := status.Self.DNSName
		if dns != "host.tailnet.ts.net." {
			t.Errorf("DNSName = %q, want %q", dns, "host.tailnet.ts.net.")
		}
	})

	// IP 回退
	t.Run("IP fallback", func(t *testing.T) {
		statusJSON := `{"Self":{"TailscaleIPs":["100.2.2.2"]}}`
		var status tailscaleStatusJSON
		if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if status.Self == nil {
			t.Fatal("Self should not be nil")
		}
		if status.Self.DNSName != "" {
			t.Error("DNSName should be empty")
		}
		if len(status.Self.TailscaleIPs) == 0 || status.Self.TailscaleIPs[0] != "100.2.2.2" {
			t.Error("expected IP fallback 100.2.2.2")
		}
	})
}

func TestWhoisCacheTTL(t *testing.T) {
	// 清理缓存
	whoisCacheMu.Lock()
	whoisCache = make(map[string]whoisCacheEntry)
	whoisCacheMu.Unlock()

	// 手动写入缓存
	whoisCacheMu.Lock()
	whoisCache["100.1.1.1"] = whoisCacheEntry{
		value:     &TailscaleWhoisIdentity{Login: "cached@example.com", Name: "Cached"},
		expiresAt: time.Now().Add(10 * time.Second),
	}
	whoisCacheMu.Unlock()

	// 缓存命中
	identity := ReadTailscaleWhoisIdentity("100.1.1.1", 60*time.Second, 5*time.Second)
	if identity == nil || identity.Login != "cached@example.com" {
		t.Errorf("expected cached identity, got %+v", identity)
	}

	// 过期后缓存未命中（写入 nil 因为无真实 tailscale）
	whoisCacheMu.Lock()
	whoisCache["100.1.1.2"] = whoisCacheEntry{
		value:     &TailscaleWhoisIdentity{Login: "expired@example.com"},
		expiresAt: time.Now().Add(-1 * time.Second),
	}
	whoisCacheMu.Unlock()

	identity = ReadTailscaleWhoisIdentity("100.1.1.2", 60*time.Second, 5*time.Second)
	// 过期后会重新查询，由于没有真实 tailscale，返回 nil
	if identity != nil && identity.Login == "expired@example.com" {
		t.Error("expired entry should not be returned")
	}
}

func TestWhoisEmptyIP(t *testing.T) {
	result := ReadTailscaleWhoisIdentity("", 60*time.Second, 5*time.Second)
	if result != nil {
		t.Errorf("empty IP should return nil, got %+v", result)
	}
	result = ReadTailscaleWhoisIdentity("  ", 60*time.Second, 5*time.Second)
	if result != nil {
		t.Errorf("whitespace IP should return nil, got %+v", result)
	}
}

func TestGetStringFromMaps(t *testing.T) {
	primary := map[string]interface{}{
		"LoginName": "alice@example.com",
	}
	fallback := map[string]interface{}{
		"login": "bob@example.com",
	}

	// 从 primary 找到
	got := getStringFromMaps(primary, fallback, "LoginName", "login")
	if got != "alice@example.com" {
		t.Errorf("expected alice, got %q", got)
	}

	// primary 没有，从 fallback 找
	got = getStringFromMaps(nil, fallback, "LoginName", "login")
	if got != "bob@example.com" {
		t.Errorf("expected bob, got %q", got)
	}

	// 都没有
	got = getStringFromMaps(primary, fallback, "missing")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
