package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/browser"
)

// ---------- mock PlaywrightTools ----------

type mockPlaywrightTools struct {
	browser.StubPlaywrightTools
	navigateURL    string
	clickRef       string
	fillRef        string
	fillValue      string
	evaluateExpr   string
	evaluateResult json.RawMessage
	waitForText    string
	waitTimeoutMs  int
	cookiesSet     []browser.PWCookieSetOpts
	inputFiles     browser.PWSetInputFilesOpts
	screenshotData []byte
}

func (m *mockPlaywrightTools) Navigate(_ context.Context, opts browser.PWNavigateOpts) error {
	m.navigateURL = opts.URL
	return nil
}

func (m *mockPlaywrightTools) Click(_ context.Context, opts browser.PWClickOpts) error {
	m.clickRef = opts.Ref
	return nil
}

func (m *mockPlaywrightTools) Fill(_ context.Context, opts browser.PWFillOpts) error {
	m.fillRef = opts.Ref
	m.fillValue = opts.Value
	return nil
}

func (m *mockPlaywrightTools) Evaluate(_ context.Context, opts browser.PWEvaluateOpts) (json.RawMessage, error) {
	m.evaluateExpr = opts.Expression
	if m.evaluateResult != nil {
		return m.evaluateResult, nil
	}
	return json.RawMessage(`"mock result"`), nil
}

func (m *mockPlaywrightTools) WaitFor(_ context.Context, opts browser.PWWaitForOpts) error {
	m.waitForText = opts.Text
	m.waitTimeoutMs = opts.TimeoutMs
	return nil
}

func (m *mockPlaywrightTools) CookiesSet(_ context.Context, opts browser.PWCookieSetOpts) error {
	m.cookiesSet = append(m.cookiesSet, opts)
	return nil
}

func (m *mockPlaywrightTools) SetInputFiles(_ context.Context, opts browser.PWSetInputFilesOpts) error {
	m.inputFiles = opts
	return nil
}

func (m *mockPlaywrightTools) Screenshot(_ context.Context, _ browser.PWTargetOpts) ([]byte, error) {
	if m.screenshotData != nil {
		return m.screenshotData, nil
	}
	return []byte{0x89, 0x50, 0x4E, 0x47}, nil // PNG magic bytes
}

// ---------- tests ----------

func TestPlaywrightBrowserBridge_NavigateToURL(t *testing.T) {
	mock := &mockPlaywrightTools{}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	if err := bridge.NavigateToURL(context.Background(), "https://example.com"); err != nil {
		t.Fatalf("NavigateToURL: %v", err)
	}
	if mock.navigateURL != "https://example.com" {
		t.Errorf("URL: got %q, want %q", mock.navigateURL, "https://example.com")
	}
}

func TestPlaywrightBrowserBridge_ClickElement(t *testing.T) {
	mock := &mockPlaywrightTools{}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	if err := bridge.ClickElement(context.Background(), "#submit-btn"); err != nil {
		t.Fatalf("ClickElement: %v", err)
	}
	if mock.clickRef != "#submit-btn" {
		t.Errorf("Ref: got %q, want %q", mock.clickRef, "#submit-btn")
	}
}

func TestPlaywrightBrowserBridge_FillInput(t *testing.T) {
	mock := &mockPlaywrightTools{}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	if err := bridge.FillInput(context.Background(), "#title", "Hello World"); err != nil {
		t.Fatalf("FillInput: %v", err)
	}
	if mock.fillRef != "#title" {
		t.Errorf("Ref: got %q, want %q", mock.fillRef, "#title")
	}
	if mock.fillValue != "Hello World" {
		t.Errorf("Value: got %q, want %q", mock.fillValue, "Hello World")
	}
}

func TestPlaywrightBrowserBridge_ExecuteJS(t *testing.T) {
	mock := &mockPlaywrightTools{
		evaluateResult: json.RawMessage(`"test value"`),
	}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	result, err := bridge.ExecuteJS(context.Background(), "document.title")
	if err != nil {
		t.Fatalf("ExecuteJS: %v", err)
	}
	if result != "test value" {
		t.Errorf("result: got %q, want %q", result, "test value")
	}
	if mock.evaluateExpr != "document.title" {
		t.Errorf("expr: got %q, want %q", mock.evaluateExpr, "document.title")
	}
}

func TestPlaywrightBrowserBridge_ExecuteJS_NonString(t *testing.T) {
	mock := &mockPlaywrightTools{
		evaluateResult: json.RawMessage(`42`),
	}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	result, err := bridge.ExecuteJS(context.Background(), "1+1")
	if err != nil {
		t.Fatalf("ExecuteJS: %v", err)
	}
	if result != "42" {
		t.Errorf("result: got %q, want %q", result, "42")
	}
}

func TestPlaywrightBrowserBridge_WaitForSelector(t *testing.T) {
	mock := &mockPlaywrightTools{}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	if err := bridge.WaitForSelector(context.Background(), ".loaded", 5000); err != nil {
		t.Fatalf("WaitForSelector: %v", err)
	}
	if mock.waitForText != ".loaded" {
		t.Errorf("Text: got %q, want %q", mock.waitForText, ".loaded")
	}
	if mock.waitTimeoutMs != 5000 {
		t.Errorf("TimeoutMs: got %d, want %d", mock.waitTimeoutMs, 5000)
	}
}

func TestPlaywrightBrowserBridge_WaitForSelector_DefaultTimeout(t *testing.T) {
	mock := &mockPlaywrightTools{}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	if err := bridge.WaitForSelector(context.Background(), ".loaded", 0); err != nil {
		t.Fatalf("WaitForSelector: %v", err)
	}
	if mock.waitTimeoutMs != 10000 {
		t.Errorf("TimeoutMs: got %d, want %d (default)", mock.waitTimeoutMs, 10000)
	}
}

func TestPlaywrightBrowserBridge_SetCookies(t *testing.T) {
	mock := &mockPlaywrightTools{}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	cookies := []map[string]interface{}{
		{"name": "session", "value": "abc123", "domain": ".example.com", "path": "/"},
		{"name": "token", "value": "xyz789", "domain": ".example.com"},
	}
	if err := bridge.SetCookies(context.Background(), cookies); err != nil {
		t.Fatalf("SetCookies: %v", err)
	}
	if len(mock.cookiesSet) != 2 {
		t.Fatalf("cookies set: got %d, want 2", len(mock.cookiesSet))
	}
	if mock.cookiesSet[0].Name != "session" {
		t.Errorf("cookie[0].Name: got %q, want %q", mock.cookiesSet[0].Name, "session")
	}
	if mock.cookiesSet[0].Path != "/" {
		t.Errorf("cookie[0].Path: got %q, want %q", mock.cookiesSet[0].Path, "/")
	}
	if mock.cookiesSet[1].Name != "token" {
		t.Errorf("cookie[1].Name: got %q, want %q", mock.cookiesSet[1].Name, "token")
	}
}

func TestPlaywrightBrowserBridge_SetCookies_SkipsEmpty(t *testing.T) {
	mock := &mockPlaywrightTools{}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	cookies := []map[string]interface{}{
		{"name": "", "value": "ignored"},
		{"name": "valid", "value": "kept", "domain": ".x.com"},
	}
	if err := bridge.SetCookies(context.Background(), cookies); err != nil {
		t.Fatalf("SetCookies: %v", err)
	}
	if len(mock.cookiesSet) != 1 {
		t.Fatalf("cookies set: got %d, want 1 (empty name skipped)", len(mock.cookiesSet))
	}
}

func TestPlaywrightBrowserBridge_SetCookies_ExpiresTypes(t *testing.T) {
	mock := &mockPlaywrightTools{}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	cookies := []map[string]interface{}{
		{"name": "a", "value": "1", "domain": ".x.com", "expires": float64(1700000000)},
		{"name": "b", "value": "2", "domain": ".x.com", "expires": int64(1700000001)},
	}
	if err := bridge.SetCookies(context.Background(), cookies); err != nil {
		t.Fatalf("SetCookies: %v", err)
	}
	if mock.cookiesSet[0].Expires != 1700000000 {
		t.Errorf("cookie[0].Expires: got %f, want %f", mock.cookiesSet[0].Expires, float64(1700000000))
	}
	if mock.cookiesSet[1].Expires != 1700000001 {
		t.Errorf("cookie[1].Expires: got %f, want %f", mock.cookiesSet[1].Expires, float64(1700000001))
	}
}

func TestPlaywrightBrowserBridge_UploadFileToInput(t *testing.T) {
	mock := &mockPlaywrightTools{}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	if err := bridge.UploadFileToInput(context.Background(), "#file-input", "/tmp/photo.jpg"); err != nil {
		t.Fatalf("UploadFileToInput: %v", err)
	}
	if mock.inputFiles.Element != "#file-input" {
		t.Errorf("Element: got %q, want %q", mock.inputFiles.Element, "#file-input")
	}
	if len(mock.inputFiles.Paths) != 1 || mock.inputFiles.Paths[0] != "/tmp/photo.jpg" {
		t.Errorf("Paths: got %v, want [/tmp/photo.jpg]", mock.inputFiles.Paths)
	}
}

func TestPlaywrightBrowserBridge_GetPageContent(t *testing.T) {
	mock := &mockPlaywrightTools{
		evaluateResult: json.RawMessage(`"Hello page content"`),
	}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	text, err := bridge.GetPageContent(context.Background())
	if err != nil {
		t.Fatalf("GetPageContent: %v", err)
	}
	if text != "Hello page content" {
		t.Errorf("text: got %q, want %q", text, "Hello page content")
	}
	if mock.evaluateExpr != "document.body.innerText" {
		t.Errorf("expr: got %q, want %q", mock.evaluateExpr, "document.body.innerText")
	}
}

func TestPlaywrightBrowserBridge_TakeScreenshot(t *testing.T) {
	mock := &mockPlaywrightTools{
		screenshotData: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A},
	}
	bridge := NewPlaywrightBrowserBridge(mock, "ws://localhost:9222")

	data, err := bridge.TakeScreenshot(context.Background())
	if err != nil {
		t.Fatalf("TakeScreenshot: %v", err)
	}
	if len(data) != 6 {
		t.Errorf("data length: got %d, want 6", len(data))
	}
}

func TestPlaywrightBrowserBridge_InterfaceCompliance(t *testing.T) {
	// 编译期检查已在 playwright_adapter.go 中完成，此处为运行时 double-check
	var _ BrowserController = (*PlaywrightBrowserBridge)(nil)
}

func TestSetBrowserFromPlaywright(t *testing.T) {
	mock := &mockPlaywrightTools{}
	client := NewXHSRPAClient(&XiaohongshuConfig{
		Enabled: true,
	})

	client.SetBrowserFromPlaywright(mock, "ws://localhost:9222", "")

	// 验证 browser 已注入
	client.mu.Lock()
	hasBrowser := client.browser != nil
	client.mu.Unlock()
	if !hasBrowser {
		t.Fatal("browser should be injected after SetBrowserFromPlaywright")
	}
}

func TestLoadCookiesIfAvailable_NoCookiePath(t *testing.T) {
	client := NewXHSRPAClient(&XiaohongshuConfig{
		Enabled: true,
		// CookiePath 为空
	})

	if err := client.LoadCookiesIfAvailable(); err != nil {
		t.Fatalf("LoadCookiesIfAvailable with empty path: %v", err)
	}
}

func TestLoadCookiesIfAvailable_FileNotExist(t *testing.T) {
	client := NewXHSRPAClient(&XiaohongshuConfig{
		Enabled:    true,
		CookiePath: "/nonexistent/cookies.json",
	})

	if err := client.LoadCookiesIfAvailable(); err != nil {
		t.Fatalf("LoadCookiesIfAvailable with missing file: %v", err)
	}
}

func TestLoadCookiesIfAvailable_ValidFile(t *testing.T) {
	// 创建临时 cookie 文件
	dir := t.TempDir()
	cookiePath := fmt.Sprintf("%s/cookies.json", dir)
	data := `[{"name":"test","value":"v1","domain":".example.com"}]`
	if err := writeTestFile(cookiePath, data); err != nil {
		t.Fatalf("write cookie file: %v", err)
	}

	client := NewXHSRPAClient(&XiaohongshuConfig{
		Enabled:    true,
		CookiePath: cookiePath,
	})

	if err := client.LoadCookiesIfAvailable(); err != nil {
		t.Fatalf("LoadCookiesIfAvailable: %v", err)
	}

	client.mu.Lock()
	count := len(client.cookies)
	client.mu.Unlock()
	if count != 1 {
		t.Errorf("cookies: got %d, want 1", count)
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
