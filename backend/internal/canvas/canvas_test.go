package canvas

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// =========== host_url_test ===========

func TestResolveCanvasHostURL_Basic(t *testing.T) {
	tests := []struct {
		name   string
		params CanvasHostURLParams
		want   string
	}{
		{
			name:   "zero port",
			params: CanvasHostURLParams{CanvasPort: 0},
			want:   "",
		},
		{
			name:   "override host",
			params: CanvasHostURLParams{CanvasPort: 18793, HostOverride: "myhost.local"},
			want:   "http://myhost.local:18793",
		},
		{
			name:   "request host",
			params: CanvasHostURLParams{CanvasPort: 18793, RequestHost: "example.com:8080"},
			want:   "http://example.com:18793",
		},
		{
			name:   "https via forwarded proto",
			params: CanvasHostURLParams{CanvasPort: 443, HostOverride: "secure.io", ForwardedProto: "https"},
			want:   "https://secure.io:443",
		},
		{
			name:   "explicit scheme overrides forwarded",
			params: CanvasHostURLParams{CanvasPort: 8080, HostOverride: "a.com", ForwardedProto: "https", Scheme: "http"},
			want:   "http://a.com:8080",
		},
		{
			name:   "local address fallback",
			params: CanvasHostURLParams{CanvasPort: 18793, LocalAddress: "192.168.1.100"},
			want:   "http://192.168.1.100:18793",
		},
		{
			name:   "no host → empty",
			params: CanvasHostURLParams{CanvasPort: 18793},
			want:   "",
		},
		{
			name:   "IPv6 host brackets",
			params: CanvasHostURLParams{CanvasPort: 18793, HostOverride: "fe80::1"},
			want:   "http://[fe80::1]:18793",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveCanvasHostURL(tt.params)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveCanvasHostURL_LoopbackReject(t *testing.T) {
	// override 不应被 reject，requestHost / localAddress 在 override 存在时应被 reject
	got := ResolveCanvasHostURL(CanvasHostURLParams{
		CanvasPort:   18793,
		HostOverride: "myhost.local",
		RequestHost:  "127.0.0.1:8080",
	})
	if got != "http://myhost.local:18793" {
		t.Errorf("got %q, want override used", got)
	}

	// 仅 requestHost=loopback，无 override → 应使用 requestHost
	got2 := ResolveCanvasHostURL(CanvasHostURLParams{
		CanvasPort:  18793,
		RequestHost: "127.0.0.1:8080",
	})
	if got2 != "http://127.0.0.1:18793" {
		t.Errorf("got %q, want loopback used when no override", got2)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for _, h := range []string{"localhost", "127.0.0.1", "127.0.0.2", "::1", "0.0.0.0", "::"} {
		if !isLoopbackHost(h) {
			t.Errorf("%q should be loopback", h)
		}
	}
	for _, h := range []string{"192.168.1.1", "example.com", ""} {
		if isLoopbackHost(h) {
			t.Errorf("%q should NOT be loopback", h)
		}
	}
}

func TestParseHostHeader(t *testing.T) {
	tests := []struct{ input, want string }{
		{"example.com:8080", "example.com"},
		{"example.com", "example.com"},
		{"[::1]:8080", "::1"},
		{"", ""},
	}
	for _, tt := range tests {
		got := parseHostHeader(tt.input)
		if got != tt.want {
			t.Errorf("parseHostHeader(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// =========== a2ui_test ===========

func TestInjectCanvasLiveReload_WithBody(t *testing.T) {
	html := `<html><body><p>Hello</p></body></html>`
	result := InjectCanvasLiveReload(html)
	if !strings.Contains(result, "openacosmiSendUserAction") {
		t.Error("should contain bridge script")
	}
	if !strings.Contains(result, CanvasWSPath) {
		t.Errorf("should contain WS path %s", CanvasWSPath)
	}
	// 应在 </body> 之前注入
	idx := strings.Index(result, "openacosmiSendUserAction")
	bodyIdx := strings.LastIndex(result, "</body>")
	if idx > bodyIdx {
		t.Error("script should be injected before </body>")
	}
}

func TestInjectCanvasLiveReload_NoBody(t *testing.T) {
	html := `<html><div>No body tag</div></html>`
	result := InjectCanvasLiveReload(html)
	if !strings.Contains(result, "openacosmiSendUserAction") {
		t.Error("should still inject script even without </body>")
	}
}

func TestNormalizeURLPath(t *testing.T) {
	tests := []struct{ input, want string }{
		{"", "/"},
		{"/foo/bar", "/foo/bar"},
		{"/foo/../bar", "/bar"},
		{"/foo/./bar", "/foo/bar"},
		{"foo", "/foo"},
	}
	for _, tt := range tests {
		got := normalizeURLPath(tt.input)
		if got != tt.want {
			t.Errorf("normalizeURLPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// =========== handler_test ===========

func TestResolveCanvasFilePath_Security(t *testing.T) {
	dir := t.TempDir()
	// 创建测试文件
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>Test</h1>"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "page.html"), []byte("<h1>Sub</h1>"), 0644)

	rootReal, _ := filepath.EvalSymlinks(dir)

	// 正常文件
	if got := resolveCanvasFilePath(rootReal, "/index.html"); got == "" {
		t.Error("should resolve index.html")
	}
	// 子目录
	if got := resolveCanvasFilePath(rootReal, "/sub/page.html"); got == "" {
		t.Error("should resolve sub/page.html")
	}
	// 目录 → index.html
	if got := resolveCanvasFilePath(rootReal, "/"); got == "" {
		t.Error("should resolve / to index.html")
	}
	// 路径遍历
	if got := resolveCanvasFilePath(rootReal, "/../../../etc/passwd"); got != "" {
		t.Errorf("should reject path traversal, got %q", got)
	}
	// 不存在的文件
	if got := resolveCanvasFilePath(rootReal, "/nonexistent"); got != "" {
		t.Errorf("should return empty for nonexistent, got %q", got)
	}
}

func TestCanvasHandler_HTTP(t *testing.T) {
	dir := t.TempDir()
	html := `<!doctype html><body><p>Hello Canvas</p></body>`
	os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0644)
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{}"), 0644)

	liveReload := false
	h, err := NewCanvasHandler(CanvasHandlerOpts{
		RootDir:    dir,
		LiveReload: &liveReload,
	})
	if err != nil {
		t.Fatalf("NewCanvasHandler: %v", err)
	}
	defer h.Close()

	// HTML 请求
	req := httptest.NewRequest(http.MethodGet, CanvasHostPath+"/", nil)
	w := httptest.NewRecorder()
	handled := h.HandleHTTP(w, req)
	if !handled {
		t.Fatal("should handle canvas path")
	}
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Hello Canvas") {
		t.Error("body should contain HTML content")
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		t.Errorf("want text/html, got %s", resp.Header.Get("Content-Type"))
	}

	// CSS 请求
	req2 := httptest.NewRequest(http.MethodGet, CanvasHostPath+"/style.css", nil)
	w2 := httptest.NewRecorder()
	h.HandleHTTP(w2, req2)
	resp2 := w2.Result()
	if resp2.StatusCode != 200 {
		t.Errorf("CSS: want 200, got %d", resp2.StatusCode)
	}

	// 404
	req3 := httptest.NewRequest(http.MethodGet, CanvasHostPath+"/nofile", nil)
	w3 := httptest.NewRecorder()
	h.HandleHTTP(w3, req3)
	if w3.Result().StatusCode != 404 {
		t.Errorf("want 404, got %d", w3.Result().StatusCode)
	}

	// 不匹配的路径
	req4 := httptest.NewRequest(http.MethodGet, "/other/path", nil)
	w4 := httptest.NewRecorder()
	if h.HandleHTTP(w4, req4) {
		t.Error("should not handle non-canvas path")
	}

	// POST → 405
	req5 := httptest.NewRequest(http.MethodPost, CanvasHostPath+"/", nil)
	w5 := httptest.NewRecorder()
	h.HandleHTTP(w5, req5)
	if w5.Result().StatusCode != 405 {
		t.Errorf("POST: want 405, got %d", w5.Result().StatusCode)
	}
}

func TestCanvasHandler_LiveReloadInject(t *testing.T) {
	dir := t.TempDir()
	html := `<!doctype html><body><p>Live</p></body>`
	os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0644)

	liveReload := true
	h, err := NewCanvasHandler(CanvasHandlerOpts{
		RootDir:    dir,
		LiveReload: &liveReload,
	})
	if err != nil {
		t.Fatalf("NewCanvasHandler: %v", err)
	}
	defer h.Close()

	req := httptest.NewRequest(http.MethodGet, CanvasHostPath+"/", nil)
	w := httptest.NewRecorder()
	h.HandleHTTP(w, req)
	body, _ := io.ReadAll(w.Result().Body)
	if !strings.Contains(string(body), "openacosmiSendUserAction") {
		t.Error("live-reload should inject bridge script")
	}
}

func TestDefaultIndexHTML(t *testing.T) {
	html := defaultIndexHTML()
	required := []string{
		"OpenAcosmi Canvas",
		"btn-hello",
		"openacosmiSendUserAction",
		"auto-reload",
	}
	for _, s := range required {
		if !strings.Contains(html, s) {
			t.Errorf("defaultIndexHTML should contain %q", s)
		}
	}
}

func TestStartCanvasHost(t *testing.T) {
	dir := t.TempDir()
	liveReload := false
	srv, err := StartCanvasHost(CanvasHostServerOpts{
		CanvasHandlerOpts: CanvasHandlerOpts{
			RootDir:    dir,
			LiveReload: &liveReload,
		},
		Port: 0, // random port
	})
	if err != nil {
		t.Fatalf("StartCanvasHost: %v", err)
	}
	defer srv.Close()

	if srv.Port == 0 {
		t.Error("port should be bound")
	}

	// default index.html 应该被创建
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(srv.Port) + CanvasHostPath + "/")
	if err != nil {
		t.Fatalf("HTTP GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "OpenAcosmi Canvas") {
		t.Error("should serve default index.html")
	}
}
