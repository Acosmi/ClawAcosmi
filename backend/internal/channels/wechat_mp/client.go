package wechat_mp

// ============================================================================
// wechat_mp/client.go — 微信公众号 API 客户端
// 封装 access_token 获取（带缓存）、图片上传和通用请求方法。
// 参照 wecom/client.go 的 token 缓存 + DoAPIRequest 模式。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-1
// ============================================================================

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// WeChatMPBaseURL 微信公众号 API 基础 URL。
	WeChatMPBaseURL = "https://api.weixin.qq.com"

	// tokenRefreshMargin token 提前刷新裕量（秒）。
	tokenRefreshMargin = 300

	// defaultTokenExpiry 默认 token 有效期（秒）。
	defaultTokenExpiry = 7200

	// maxImageSize 上传图片最大尺寸（1MB）。
	maxImageSize = 1 << 20

	// rateLimitInterval 最小请求间隔（50ms ≈ 20次/秒）。
	rateLimitInterval = 50 * time.Millisecond
)

// apiError 微信 API 通用错误响应。
type apiError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// WeChatMPClient 微信公众号 API 客户端。
type WeChatMPClient struct {
	AppID     string
	AppSecret string
	BaseURL   string // 允许测试覆盖，默认 WeChatMPBaseURL
	Client    *http.Client

	mu          sync.Mutex // 保护 token 缓存
	accessToken string
	tokenExpiry time.Time

	rateMu      sync.Mutex // 保护 rate limiter（独立锁，避免与 token 锁竞争）
	lastRequest time.Time
}

// NewWeChatMPClient 创建公众号客户端。
func NewWeChatMPClient(cfg *WeChatMPConfig) *WeChatMPClient {
	baseURL := WeChatMPBaseURL
	return &WeChatMPClient{
		AppID:     cfg.AppID,
		AppSecret: cfg.AppSecret,
		BaseURL:   baseURL,
		Client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// ---------- Access Token ----------

// GetAccessToken 获取 access_token（带缓存，提前 5 分钟刷新）。
func (c *WeChatMPClient) GetAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	url := fmt.Sprintf(
		"%s/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		c.BaseURL, c.AppID, c.AppSecret,
	)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request access_token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		apiError
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("wechat_mp token error: code=%d, msg=%s",
			result.ErrCode, result.ErrMsg)
	}

	c.accessToken = result.AccessToken
	expire := result.ExpiresIn
	if expire <= 0 {
		expire = defaultTokenExpiry
	}
	c.tokenExpiry = time.Now().Add(time.Duration(expire-tokenRefreshMargin) * time.Second)

	slog.Debug("wechat_mp access_token refreshed",
		"expires_in", expire, "app_id", c.AppID)
	return c.accessToken, nil
}

// ---------- 通用请求 ----------

// DoRequest 执行带 access_token 的 API 请求。
func (c *WeChatMPClient) DoRequest(
	ctx context.Context,
	method, path string,
	body []byte,
) ([]byte, error) {
	token, err := c.GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	c.rateLimit()

	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	url := c.BaseURL + path + sep + "access_token=" + token

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wechat_mp API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("wechat_mp API HTTP %d: %s",
			resp.StatusCode, string(respBody))
	}

	// 检查微信业务错误码。
	var apiErr apiError
	if json.Unmarshal(respBody, &apiErr) == nil && apiErr.ErrCode != 0 {
		return respBody, fmt.Errorf("wechat_mp API error: code=%d, msg=%s",
			apiErr.ErrCode, apiErr.ErrMsg)
	}
	return respBody, nil
}

// ---------- 图片上传 ----------

// allowedImageExts 允许上传的图片格式。
var allowedImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// UploadImage 上传永久图片素材。
// 返回 media_id 供图文消息引用。
func (c *WeChatMPClient) UploadImage(
	ctx context.Context,
	filePath string,
) (string, error) {
	// 校验文件扩展名。
	ext := strings.ToLower(filepath.Ext(filePath))
	if !allowedImageExts[ext] {
		return "", fmt.Errorf("unsupported image format %q (only jpg/png)", ext)
	}

	// 校验文件大小。
	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("stat image file: %w", err)
	}
	if info.Size() > maxImageSize {
		return "", fmt.Errorf("image too large: %d bytes (max %d)",
			info.Size(), maxImageSize)
	}

	// 构建 multipart 请求体。
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open image file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("media", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("copy image data: %w", err)
	}
	writer.Close()

	// 获取 token 并发送请求。
	token, err := c.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}

	c.rateLimit()

	url := fmt.Sprintf("%s/cgi-bin/media/uploadimg?access_token=%s",
		c.BaseURL, token)

	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload image request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		apiError
		URL string `json:"url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("upload image error: code=%d, msg=%s",
			result.ErrCode, result.ErrMsg)
	}

	return result.URL, nil
}

// ---------- rate limiter ----------

// rateLimit 简易速率限制，确保请求间隔不小于 rateLimitInterval。
func (c *WeChatMPClient) rateLimit() {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	now := time.Now()
	elapsed := now.Sub(c.lastRequest)
	if elapsed < rateLimitInterval {
		time.Sleep(rateLimitInterval - elapsed)
	}
	c.lastRequest = time.Now()
}
