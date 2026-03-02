package xiaohongshu

// ============================================================================
// xiaohongshu/browser_adapter.go — 浏览器驱动接口 + CDP 适配器
// 定义 BrowserDriver 抽象接口，屏蔽底层浏览器控制差异。
// CDPBrowserAdapter 通过项目已有的 browser.PlaywrightTools 实现。
//
// Design doc: Phase 6A — XHS 浏览器驱动适配
// ============================================================================

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// BrowserDriver 浏览器自动化驱动接口。
// 所有 RPA 操作通过此接口与浏览器交互，便于测试时 mock。
type BrowserDriver interface {
	// Navigate 导航到指定 URL。
	Navigate(ctx context.Context, url string) error
	// SetCookies 设置浏览器 Cookie。
	SetCookies(ctx context.Context, cookies []CookieEntry) error
	// FillBySelector 通过 CSS 选择器填写输入框。
	FillBySelector(ctx context.Context, selector, value string) error
	// ClickBySelector 通过 CSS 选择器点击元素。
	ClickBySelector(ctx context.Context, selector string) error
	// UploadFile 通过 file input 选择器上传文件。
	UploadFile(ctx context.Context, inputSelector, filePath string) error
	// WaitForElement 等待元素出现（超时 ms）。
	WaitForElement(ctx context.Context, selector string, timeoutMs int) error
	// GetPageText 获取页面文本内容。
	GetPageText(ctx context.Context) (string, error)
	// Screenshot 截屏返回 PNG 数据。
	Screenshot(ctx context.Context) ([]byte, error)
	// EvaluateJS 在页面上下文执行 JS 表达式，返回结果字符串。
	EvaluateJS(ctx context.Context, expr string) (string, error)
}

// ---------- CDP 适配器 ----------

// BrowserController 项目已有的浏览器控制器接口（browser 包导出）。
// 此处定义所需最小接口，避免硬依赖 browser 包。
type BrowserController interface {
	NavigateToURL(ctx context.Context, url string) error
	ExecuteJS(ctx context.Context, expr string) (string, error)
	ClickElement(ctx context.Context, selector string) error
	FillInput(ctx context.Context, selector, value string) error
	WaitForSelector(ctx context.Context, selector string, timeoutMs int) error
	UploadFileToInput(ctx context.Context, selector, filePath string) error
	GetPageContent(ctx context.Context) (string, error)
	TakeScreenshot(ctx context.Context) ([]byte, error)
	SetCookies(ctx context.Context, cookies []map[string]interface{}) error
}

// CDPBrowserAdapter 将 BrowserController 适配为 BrowserDriver。
type CDPBrowserAdapter struct {
	controller BrowserController
	errDir     string // 错误截图保存目录
}

// NewCDPBrowserAdapter 创建 CDP 适配器。
func NewCDPBrowserAdapter(controller BrowserController, errDir string) *CDPBrowserAdapter {
	if errDir == "" {
		errDir = "_media/xhs/errors"
	}
	return &CDPBrowserAdapter{
		controller: controller,
		errDir:     errDir,
	}
}

func (a *CDPBrowserAdapter) Navigate(ctx context.Context, url string) error {
	if err := a.controller.NavigateToURL(ctx, url); err != nil {
		a.captureError(ctx, "navigate")
		return fmt.Errorf("navigate to %s: %w", url, err)
	}
	return nil
}

func (a *CDPBrowserAdapter) SetCookies(ctx context.Context, cookies []CookieEntry) error {
	cookieMaps := make([]map[string]interface{}, 0, len(cookies))
	for _, c := range cookies {
		m := map[string]interface{}{
			"name":   c.Name,
			"value":  c.Value,
			"domain": c.Domain,
		}
		if c.Path != "" {
			m["path"] = c.Path
		}
		if c.Expires > 0 {
			m["expires"] = c.Expires
		}
		cookieMaps = append(cookieMaps, m)
	}
	if err := a.controller.SetCookies(ctx, cookieMaps); err != nil {
		return fmt.Errorf("set cookies: %w", err)
	}
	return nil
}

func (a *CDPBrowserAdapter) FillBySelector(ctx context.Context, selector, value string) error {
	if err := a.controller.FillInput(ctx, selector, value); err != nil {
		a.captureError(ctx, "fill-"+selector)
		return fmt.Errorf("fill %s: %w", selector, err)
	}
	return nil
}

func (a *CDPBrowserAdapter) ClickBySelector(ctx context.Context, selector string) error {
	if err := a.controller.ClickElement(ctx, selector); err != nil {
		a.captureError(ctx, "click-"+selector)
		return fmt.Errorf("click %s: %w", selector, err)
	}
	return nil
}

func (a *CDPBrowserAdapter) UploadFile(ctx context.Context, inputSelector, filePath string) error {
	if err := a.controller.UploadFileToInput(ctx, inputSelector, filePath); err != nil {
		a.captureError(ctx, "upload")
		return fmt.Errorf("upload to %s: %w", inputSelector, err)
	}
	return nil
}

func (a *CDPBrowserAdapter) WaitForElement(ctx context.Context, selector string, timeoutMs int) error {
	if err := a.controller.WaitForSelector(ctx, selector, timeoutMs); err != nil {
		return fmt.Errorf("wait for %s (timeout %dms): %w", selector, timeoutMs, err)
	}
	return nil
}

func (a *CDPBrowserAdapter) GetPageText(ctx context.Context) (string, error) {
	text, err := a.controller.GetPageContent(ctx)
	if err != nil {
		return "", fmt.Errorf("get page text: %w", err)
	}
	return text, nil
}

func (a *CDPBrowserAdapter) Screenshot(ctx context.Context) ([]byte, error) {
	data, err := a.controller.TakeScreenshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	return data, nil
}

func (a *CDPBrowserAdapter) EvaluateJS(ctx context.Context, expr string) (string, error) {
	result, err := a.controller.ExecuteJS(ctx, expr)
	if err != nil {
		return "", fmt.Errorf("evaluate JS: %w", err)
	}
	return result, nil
}

// captureError 错误时自动截屏保存，便于调试。
func (a *CDPBrowserAdapter) captureError(ctx context.Context, label string) {
	data, err := a.controller.TakeScreenshot(ctx)
	if err != nil {
		slog.Warn("xhs: error screenshot failed", "error", err)
		return
	}
	if err := os.MkdirAll(a.errDir, 0o755); err != nil {
		slog.Warn("xhs: create error dir failed", "error", err)
		return
	}
	name := fmt.Sprintf("xhs-error-%s-%d.png", label, time.Now().UnixMilli())
	path := filepath.Join(a.errDir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		slog.Warn("xhs: save error screenshot failed", "error", err)
		return
	}
	slog.Info("xhs: error screenshot saved", "path", path)
}
