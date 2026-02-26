package gateway

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------- IP 地址工具测试 ----------

func TestIsLoopbackAddress(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"::1", true},
		{"::ffff:127.0.0.1", true},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsLoopbackAddress(tc.ip); got != tc.want {
			t.Errorf("IsLoopbackAddress(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestNormalizeIPv4Mapped(t *testing.T) {
	cases := []struct{ ip, want string }{
		{"::ffff:192.168.1.1", "192.168.1.1"},
		{"192.168.1.1", "192.168.1.1"},
		{"::1", "::1"},
	}
	for _, tc := range cases {
		if got := NormalizeIPv4Mapped(tc.ip); got != tc.want {
			t.Errorf("NormalizeIPv4Mapped(%q) = %q, want %q", tc.ip, got, tc.want)
		}
	}
}

func TestNormalizeIP(t *testing.T) {
	cases := []struct{ ip, want string }{
		{"  ::FFFF:192.168.1.1  ", "192.168.1.1"},
		{"127.0.0.1", "127.0.0.1"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeIP(tc.ip); got != tc.want {
			t.Errorf("NormalizeIP(%q) = %q, want %q", tc.ip, got, tc.want)
		}
	}
}

func TestStripOptionalPort(t *testing.T) {
	cases := []struct{ ip, want string }{
		{"192.168.1.1:8080", "192.168.1.1"},
		{"192.168.1.1", "192.168.1.1"},
		{"[::1]:443", "::1"},
		{"::1", "::1"},
	}
	for _, tc := range cases {
		if got := StripOptionalPort(tc.ip); got != tc.want {
			t.Errorf("StripOptionalPort(%q) = %q, want %q", tc.ip, got, tc.want)
		}
	}
}

func TestParseForwardedForClientIP(t *testing.T) {
	cases := []struct{ ff, want string }{
		{"10.0.0.1, 10.0.0.2", "10.0.0.1"},
		{"::ffff:10.0.0.1", "10.0.0.1"},
		{"", ""},
		{"  10.0.0.1:9999 , 10.0.0.2", "10.0.0.1"},
	}
	for _, tc := range cases {
		if got := ParseForwardedForClientIP(tc.ff); got != tc.want {
			t.Errorf("ParseForwardedForClientIP(%q) = %q, want %q", tc.ff, got, tc.want)
		}
	}
}

func TestIsTrustedProxyAddress(t *testing.T) {
	proxies := []string{"10.0.0.1", "172.16.0.1"}
	if !IsTrustedProxyAddress("10.0.0.1", proxies) {
		t.Error("expected 10.0.0.1 to be trusted")
	}
	if IsTrustedProxyAddress("192.168.0.1", proxies) {
		t.Error("expected 192.168.0.1 to NOT be trusted")
	}
	if IsTrustedProxyAddress("10.0.0.1", nil) {
		t.Error("expected nil proxies → not trusted")
	}
}

func TestResolveGatewayClientIP(t *testing.T) {
	proxies := []string{"10.0.0.1"}
	// 非代理：使用 remote
	got := ResolveGatewayClientIP("192.168.1.100", "10.0.0.2", "", proxies)
	if got != "192.168.1.100" {
		t.Errorf("expected direct remote, got %q", got)
	}
	// 代理：使用 X-Forwarded-For
	got = ResolveGatewayClientIP("10.0.0.1", "192.168.1.200, 10.0.0.1", "", proxies)
	if got != "192.168.1.200" {
		t.Errorf("expected forwarded IP, got %q", got)
	}
}

func TestIsValidIPv4(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"256.0.0.1", false},
		{"", false},
		{"1.2.3", false},
		{"01.2.3.4", false}, // 前导零
	}
	for _, tc := range cases {
		if got := IsValidIPv4(tc.ip); got != tc.want {
			t.Errorf("IsValidIPv4(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestCanBindToHost(t *testing.T) {
	if !CanBindToHost("127.0.0.1") {
		t.Error("should be able to bind 127.0.0.1")
	}
}

func TestResolveGatewayBindHost(t *testing.T) {
	got := ResolveGatewayBindHost(BindLoopback, "")
	if got != "127.0.0.1" {
		t.Errorf("BindLoopback → %q, want 127.0.0.1", got)
	}
	got = ResolveGatewayBindHost(BindLAN, "")
	if got != "0.0.0.0" {
		t.Errorf("BindLAN → %q, want 0.0.0.0", got)
	}
}

// ---------- HTTP 请求工具测试 ----------

func TestGetBearerToken(t *testing.T) {
	cases := []struct{ auth, want string }{
		{"Bearer abc123", "abc123"},
		{"bearer  xyz ", "xyz"},
		{"Basic dXNlci1wYXNz", ""},
		{"", ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/", nil)
		if tc.auth != "" {
			req.Header.Set("Authorization", tc.auth)
		}
		got := GetBearerToken(req)
		if got != tc.want {
			t.Errorf("GetBearerToken(%q) = %q, want %q", tc.auth, got, tc.want)
		}
	}
}

func TestGetHostName(t *testing.T) {
	cases := []struct{ host, want string }{
		{"example.com:8080", "example.com"},
		{"example.com", "example.com"},
		{"[::1]:443", "::1"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := GetHostName(tc.host); got != tc.want {
			t.Errorf("GetHostName(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}

func TestReadJSONBody(t *testing.T) {
	body := strings.NewReader(`{"name":"test","value":42}`)
	req := httptest.NewRequest("POST", "/", body)
	result, err := ReadJSONBody(req, 1024)
	if err != nil {
		t.Fatalf("ReadJSONBody: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map")
	}
	if m["name"] != "test" {
		t.Errorf("name = %v, want test", m["name"])
	}
}

func TestReadJSONBodyTooLarge(t *testing.T) {
	body := strings.NewReader(`{"data":"` + strings.Repeat("x", 1000) + `"}`)
	req := httptest.NewRequest("POST", "/", body)
	_, err := ReadJSONBody(req, 100)
	if err != ErrPayloadTooLarge {
		t.Errorf("expected ErrPayloadTooLarge, got %v", err)
	}
}

func TestReadJSONBodyEmpty(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(""))
	result, err := ReadJSONBody(req, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok || len(m) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

// ---------- HTTP 响应工具测试 ----------

func TestSendJSON(t *testing.T) {
	w := httptest.NewRecorder()
	SendJSON(w, 200, map[string]string{"status": "ok"})
	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Errorf("body = %s", body)
	}
}

func TestSendUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	SendUnauthorized(w)
	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestSetSSEHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	SetSSEHeaders(w)
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want event-stream", ct)
	}
	cc := w.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
}

func TestWriteSSEDone(t *testing.T) {
	w := httptest.NewRecorder()
	WriteSSEDone(w)
	body := w.Body.String()
	if !strings.Contains(body, "data: [DONE]") {
		t.Errorf("body = %q, missing [DONE]", body)
	}
}
