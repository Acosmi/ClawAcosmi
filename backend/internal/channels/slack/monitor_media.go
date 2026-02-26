package slack

// Slack 媒体下载 — 继承自 src/slack/monitor/media.ts (209L)
// 完整实现：域名安全验证 + 跨域重定向 Auth 剥离 + 文件大小限制
// + 统一媒体存储 + 多文件循环 + 线程起始消息缓存。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/anthropic/open-acosmi/internal/media"
)

// ---------- P0-安全: Slack 域名白名单验证 ----------
// TS 对照: media.ts L7-45 normalizeHostname + isSlackHostname + assertSlackFileUrl

// slackAllowedHostSuffixes Slack 域名白名单。
// TS 对照: media.ts L23
var slackAllowedHostSuffixes = []string{
	"slack.com",
	"slack-edge.com",
	"slack-files.com",
}

// normalizeHostname 规范化主机名。
// TS 对照: media.ts L7-13
func normalizeHostname(hostname string) string {
	normalized := strings.TrimSpace(strings.ToLower(hostname))
	normalized = strings.TrimSuffix(normalized, ".")
	if strings.HasPrefix(normalized, "[") && strings.HasSuffix(normalized, "]") {
		normalized = normalized[1 : len(normalized)-1]
	}
	return normalized
}

// isSlackHostname 检查是否为 Slack 域名。
// TS 对照: media.ts L15-27
func isSlackHostname(hostname string) bool {
	normalized := normalizeHostname(hostname)
	if normalized == "" {
		return false
	}
	for _, suffix := range slackAllowedHostSuffixes {
		if normalized == suffix || strings.HasSuffix(normalized, "."+suffix) {
			return true
		}
	}
	return false
}

// assertSlackFileURL 验证 URL 是否为安全的 Slack 文件 URL。
// 防止 Bot Token 泄漏到非 Slack 域名。
// TS 对照: media.ts L29-45
func assertSlackFileURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("slack: invalid file URL: %s", rawURL)
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("slack: refusing file URL with non-HTTPS protocol: %s", parsed.Scheme)
	}
	if !isSlackHostname(parsed.Hostname()) {
		return nil, fmt.Errorf("slack: refusing to send token to non-Slack host %q (url: %s)", parsed.Hostname(), rawURL)
	}
	return parsed, nil
}

// ---------- P0-安全: 跨域重定向 Auth Header 剥离 ----------
// TS 对照: media.ts L60-116 createSlackMediaFetch + fetchWithSlackAuth

// newSlackMediaHTTPClient 创建安全的 HTTP 客户端。
// 首次请求手动处理重定向（redirect: manual），跨域重定向时不带 Auth header。
// TS 对照: media.ts L85-116 fetchWithSlackAuth
func fetchWithSlackAuth(ctx context.Context, rawURL, token string) (*http.Response, error) {
	// 1. 验证 URL 安全性
	parsed, err := assertSlackFileURL(rawURL)
	if err != nil {
		return nil, err
	}

	// 2. 首次请求：带 Auth，不自动跟随重定向
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 手动处理重定向
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("slack: create request failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	initialResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack: download failed: %w", err)
	}

	// 3. 非重定向状态码 → 直接返回
	if initialResp.StatusCode < 300 || initialResp.StatusCode >= 400 {
		return initialResp, nil
	}

	// 4. 处理重定向 — CDN URL 是 pre-signed 的，不需要 Auth
	redirectURL := initialResp.Header.Get("Location")
	if redirectURL == "" {
		return initialResp, nil
	}
	initialResp.Body.Close()

	// 解析相对 URL
	resolved, err := url.Parse(redirectURL)
	if err != nil {
		return nil, fmt.Errorf("slack: invalid redirect URL: %s", redirectURL)
	}
	if !resolved.IsAbs() {
		resolved = parsed.ResolveReference(resolved)
	}

	// 只跟随 HTTPS 重定向
	if resolved.Scheme != "https" {
		return nil, fmt.Errorf("slack: refusing non-HTTPS redirect: %s", resolved.Scheme)
	}

	// 5. 跟随重定向 — 不带 Authorization header
	// TS 对照: media.ts L115 "Follow the redirect without the Authorization header"
	redirectReq, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("slack: redirect request failed: %w", err)
	}

	return http.DefaultClient.Do(redirectReq)
}

// ---------- P1: 文件大小限制 + 统一媒体存储 ----------

// SlackMediaResult 媒体下载结果。
// TS 对照: media.ts L122-126 resolveSlackMedia 返回值
type SlackMediaResult struct {
	Path        string
	ContentType string
	Placeholder string
}

// ResolveSlackMedia 解析 Slack 文件附件。
// 遍历多个文件，安全下载+大小限制+统一存储。
// TS 对照: media.ts L118-164 resolveSlackMedia
func ResolveSlackMedia(ctx context.Context, files []SlackFile, token string, maxBytes int64) *SlackMediaResult {
	if maxBytes <= 0 {
		maxBytes = media.MediaMaxBytes
	}
	for _, file := range files {
		fileURL := file.URLPrivateDownload
		if fileURL == "" {
			fileURL = file.URLPrivate
		}
		if fileURL == "" {
			continue
		}

		result, err := downloadAndSaveSlackFile(ctx, fileURL, token, file, maxBytes)
		if err != nil {
			// TS: catch {} — 静默跳过，尝试下一个文件
			log.Printf("[slack] media download failed for %s: %v", file.Name, err)
			continue
		}
		return result
	}
	return nil
}

// downloadAndSaveSlackFile 安全下载单个 Slack 文件并保存到统一媒体存储。
func downloadAndSaveSlackFile(ctx context.Context, fileURL, token string, file SlackFile, maxBytes int64) (*SlackMediaResult, error) {
	// 安全下载（含域名验证 + 重定向 Auth 剥离）
	resp, err := fetchWithSlackAuth(ctx, fileURL, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("slack: download returned %d", resp.StatusCode)
	}

	// P1: 文件大小限制 — 使用 io.LimitReader
	// TS 对照: media.ts L144 "if (fetched.buffer.byteLength > params.maxBytes) continue"
	limitReader := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("slack: read body failed: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("slack: file exceeds %d byte limit", maxBytes)
	}

	// P1: 统一媒体存储
	// TS 对照: media.ts L147-152 saveMediaBuffer(fetched.buffer, ...)
	contentType := file.MimeType
	if contentType == "" {
		contentType = resp.Header.Get("Content-Type")
	}

	saved, err := media.SaveMediaBuffer(data, contentType, "inbound", maxBytes, file.Name)
	if err != nil {
		return nil, fmt.Errorf("slack: save media failed: %w", err)
	}

	// P1: Placeholder 构建
	// TS 对照: media.ts L153-157
	label := file.Name
	placeholder := "[Slack file]"
	if label != "" {
		placeholder = fmt.Sprintf("[Slack file: %s]", label)
	}

	return &SlackMediaResult{
		Path:        saved.Path,
		ContentType: saved.ContentType,
		Placeholder: placeholder,
	}, nil
}

// ---------- 保留旧 API 兼容（已加安全验证） ----------

// DownloadSlackMedia 下载 Slack 私有文件（安全版）。
// 使用 Bot token 授权下载，含域名验证 + 重定向安全 + 大小限制。
func DownloadSlackMedia(ctx context.Context, client *SlackWebClient, file SlackFile, destDir string, maxBytes int64) (string, string, error) {
	if maxBytes <= 0 {
		maxBytes = media.MediaMaxBytes
	}
	result := ResolveSlackMedia(ctx, []SlackFile{file}, client.Token(), maxBytes)
	if result == nil {
		return "", "", fmt.Errorf("slack: download failed for file %s", file.ID)
	}
	return result.Path, result.ContentType, nil
}

// sanitizeFileName 安全化文件名。
func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "unnamed"
	}
	return name
}

// ---------- P2: 线程起始消息解析 + 缓存 ----------
// TS 对照: media.ts L166-208 resolveSlackThreadStarter + THREAD_STARTER_CACHE

// SlackThreadStarter 线程起始消息。
// TS 对照: media.ts L166-171
type SlackThreadStarter struct {
	Text   string
	UserID string
	Ts     string
	Files  []SlackFile
}

// threadStarterCache 线程起始消息缓存。
// TS 对照: media.ts L173 THREAD_STARTER_CACHE = new Map<string, SlackThreadStarter>()
var threadStarterCache = struct {
	mu    sync.RWMutex
	items map[string]*SlackThreadStarter
}{
	items: make(map[string]*SlackThreadStarter),
}

// ResolveSlackThreadStarter 获取线程首条消息。
// 使用 conversations.replies API（limit=1, inclusive=true），结果缓存。
// TS 对照: media.ts L175-208
func ResolveSlackThreadStarter(ctx context.Context, client *SlackWebClient, channelID, threadTs string) *SlackThreadStarter {
	if channelID == "" || threadTs == "" {
		return nil
	}

	cacheKey := channelID + ":" + threadTs

	// 读缓存
	threadStarterCache.mu.RLock()
	cached, ok := threadStarterCache.items[cacheKey]
	threadStarterCache.mu.RUnlock()
	if ok {
		return cached
	}

	// API 调用: conversations.replies
	raw, err := client.APICall(ctx, "conversations.replies", map[string]interface{}{
		"channel":   channelID,
		"ts":        threadTs,
		"limit":     1,
		"inclusive": true,
	})
	if err != nil {
		// TS: catch { return null }
		return nil
	}

	var resp struct {
		Messages []struct {
			Text  string      `json:"text"`
			User  string      `json:"user"`
			Ts    string      `json:"ts"`
			Files []SlackFile `json:"files"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}

	if len(resp.Messages) == 0 {
		return nil
	}

	msg := resp.Messages[0]
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		// TS: if (!message || !text) return null
		return nil
	}

	starter := &SlackThreadStarter{
		Text:   text,
		UserID: msg.User,
		Ts:     msg.Ts,
		Files:  msg.Files,
	}

	// 写缓存
	threadStarterCache.mu.Lock()
	threadStarterCache.items[cacheKey] = starter
	threadStarterCache.mu.Unlock()

	return starter
}
