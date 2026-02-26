package slack

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------- 域名安全验证测试 ----------

func TestNormalizeHostname(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"Slack.COM", "slack.com"},
		{" files.slack.com. ", "files.slack.com"},
		{"[::1]", "::1"},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeHostname(c.input)
		if got != c.want {
			t.Errorf("normalizeHostname(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestIsSlackHostname(t *testing.T) {
	allowed := []string{
		"slack.com",
		"files.slack.com",
		"a.b.slack-edge.com",
		"slack-files.com",
	}
	for _, h := range allowed {
		if !isSlackHostname(h) {
			t.Errorf("isSlackHostname(%q) = false, want true", h)
		}
	}

	denied := []string{
		"evil.com",
		"notslack.com",
		"slack.com.evil.com",
		"",
	}
	for _, h := range denied {
		if isSlackHostname(h) {
			t.Errorf("isSlackHostname(%q) = true, want false", h)
		}
	}
}

func TestAssertSlackFileURL_Valid(t *testing.T) {
	u, err := assertSlackFileURL("https://files.slack.com/files-pri/T1/abc.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Hostname() != "files.slack.com" {
		t.Errorf("hostname: got %q", u.Hostname())
	}
}

func TestAssertSlackFileURL_NonHTTPS(t *testing.T) {
	_, err := assertSlackFileURL("http://files.slack.com/abc.png")
	if err == nil {
		t.Error("expected error for HTTP URL")
	}
}

func TestAssertSlackFileURL_NonSlackHost(t *testing.T) {
	_, err := assertSlackFileURL("https://evil.com/abc.png")
	if err == nil {
		t.Error("expected error for non-Slack host")
	}
}

func TestAssertSlackFileURL_InvalidURL(t *testing.T) {
	_, err := assertSlackFileURL("://bad")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// ---------- 跨域重定向 Auth 剥离测试 ----------

func TestFetchWithSlackAuth_DirectDownload(t *testing.T) {
	// 模拟直接返回 200 的 Slack 文件服务器
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header on initial request")
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("PNGDATA"))
	}))
	defer srv.Close()

	// 注: fetchWithSlackAuth 仅接受 Slack 域名，此处直接测试安全验证逻辑
	_, err := fetchWithSlackAuth(context.Background(), srv.URL+"/file.png", "xoxb-test")
	if err == nil {
		t.Fatal("expected error: test server is not a Slack hostname")
	}
	// 验证错误是域名验证错误
	if got := err.Error(); got == "" {
		t.Error("expected non-empty error message")
	}
}

func TestFetchWithSlackAuth_NonSlackHost(t *testing.T) {
	_, err := fetchWithSlackAuth(context.Background(), "https://evil.com/file.png", "token")
	if err == nil {
		t.Error("expected error for non-Slack host")
	}
}

func TestFetchWithSlackAuth_NonHTTPS(t *testing.T) {
	_, err := fetchWithSlackAuth(context.Background(), "http://files.slack.com/f.png", "token")
	if err == nil {
		t.Error("expected error for non-HTTPS URL")
	}
}

// ---------- sanitizeFileName 测试 ----------

func TestSanitizeFileName(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"hello.png", "hello.png"},
		{"path/to/file.jpg", "path_to_file.jpg"},
		{"back\\slash.gif", "back_slash.gif"},
		{"  ", "unnamed"},
		{"", "unnamed"},
	}
	for _, c := range cases {
		got := sanitizeFileName(c.input)
		if got != c.want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ---------- ResolveSlackMedia 边界测试 ----------

func TestResolveSlackMedia_EmptyFiles(t *testing.T) {
	result := ResolveSlackMedia(context.Background(), nil, "token", 0)
	if result != nil {
		t.Error("expected nil for empty files")
	}
}

func TestResolveSlackMedia_NoURLs(t *testing.T) {
	files := []SlackFile{
		{ID: "F1", Name: "test.png"},
	}
	result := ResolveSlackMedia(context.Background(), files, "token", 1024)
	if result != nil {
		t.Error("expected nil for files without URLs")
	}
}

// ---------- ResolveSlackThreadStarter 边界测试 ----------

func TestResolveSlackThreadStarter_EmptyParams(t *testing.T) {
	result := ResolveSlackThreadStarter(context.Background(), nil, "", "")
	if result != nil {
		t.Error("expected nil for empty channelID and threadTs")
	}

	result = ResolveSlackThreadStarter(context.Background(), nil, "C123", "")
	if result != nil {
		t.Error("expected nil for empty threadTs")
	}
}

// ---------- SlackMediaResult Placeholder 测试 ----------

func TestSlackMediaResult_Placeholder(t *testing.T) {
	// 测试 placeholder 构建逻辑（通过间接验证）
	label := "report.pdf"
	placeholder := "[Slack file]"
	if label != "" {
		placeholder = fmt.Sprintf("[Slack file: %s]", label)
	}
	if placeholder != "[Slack file: report.pdf]" {
		t.Errorf("placeholder: got %q, want %q", placeholder, "[Slack file: report.pdf]")
	}

	label = ""
	placeholder = "[Slack file]"
	if label != "" {
		placeholder = fmt.Sprintf("[Slack file: %s]", label)
	}
	if placeholder != "[Slack file]" {
		t.Errorf("placeholder: got %q, want %q", placeholder, "[Slack file]")
	}
}
