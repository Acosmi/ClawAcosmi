package auth

// github_copilot_token.go — GitHub Copilot API Token 交换与缓存
// 对应 TS src/providers/github-copilot-token.ts (133L)
//
// 用 GitHub access token 交换 Copilot API token，并缓存到磁盘。
// Copilot API token 是调用 Copilot 后端（chat/completions）的真正凭据。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/internal/config"
)

// Copilot Token 端点与默认值。
const (
	CopilotTokenURL            = "https://api.github.com/copilot_internal/v2/token"
	DefaultCopilotApiBaseURL   = "https://api.individual.githubcopilot.com"
	copilotTokenCacheFile      = "github-copilot.token.json"
	copilotTokenSafetyMarginMs = 5 * 60 * 1000 // 5 分钟安全余量
)

// CachedCopilotToken 磁盘缓存的 Copilot API token。
type CachedCopilotToken struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"` // 毫秒时间戳
	UpdatedAt int64  `json:"updatedAt"` // 毫秒时间戳
}

// CopilotApiTokenResult 解析后的 Copilot API token 结果。
type CopilotApiTokenResult struct {
	Token     string
	ExpiresAt int64
	Source    string
	BaseURL   string
}

// resolveCopilotTokenCachePath 返回 Copilot token 缓存文件路径。
// 对应 TS: resolveCopilotTokenCachePath(env)
func resolveCopilotTokenCachePath() string {
	stateDir := config.ResolveStateDir()
	return filepath.Join(stateDir, "credentials", copilotTokenCacheFile)
}

// isTokenUsable 检查缓存 token 是否仍可用（含安全余量）。
// 对应 TS: isTokenUsable(cache, now)
func isTokenUsable(cache *CachedCopilotToken, now int64) bool {
	if cache == nil || cache.Token == "" {
		return false
	}
	return cache.ExpiresAt-now > copilotTokenSafetyMarginMs
}

// proxyEpRegex 匹配 Copilot token 中的 proxy-ep 字段。
var proxyEpRegex = regexp.MustCompile(`(?:^|;)\s*proxy-ep=([^;\s]+)`)

// DeriveCopilotApiBaseUrlFromToken 从 Copilot API token 中解析 API base URL。
// Copilot token 是分号分隔的 key=value 对，其中 proxy-ep 指向代理端点。
// 对应 TS: deriveCopilotApiBaseUrlFromToken(token)
func DeriveCopilotApiBaseUrlFromToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}

	matches := proxyEpRegex.FindStringSubmatch(trimmed)
	if len(matches) < 2 {
		return ""
	}

	proxyEp := strings.TrimSpace(matches[1])
	if proxyEp == "" {
		return ""
	}

	// proxy.* → api.*（对齐 TS 上游 getGitHubCopilotBaseUrl）
	host := proxyEp
	// 移除协议前缀
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	// 替换 proxy. → api.
	host = regexp.MustCompile(`(?i)^proxy\.`).ReplaceAllString(host, "api.")

	if host == "" {
		return ""
	}

	return "https://" + host
}

// parseCopilotTokenResponse 解析 Copilot token 端点响应。
// 对应 TS: parseCopilotTokenResponse(value)
func parseCopilotTokenResponse(data []byte) (token string, expiresAtMs int64, err error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", 0, fmt.Errorf("解析 Copilot token 响应失败: %w", err)
	}

	// token
	tokenVal, ok := raw["token"]
	if !ok {
		return "", 0, fmt.Errorf("Copilot token 响应缺少 token 字段")
	}
	tokenStr, ok := tokenVal.(string)
	if !ok || strings.TrimSpace(tokenStr) == "" {
		return "", 0, fmt.Errorf("Copilot token 响应 token 字段无效")
	}

	// expires_at — GitHub 返回 unix 秒，防御性接受毫秒
	expiresAtVal, ok := raw["expires_at"]
	if !ok {
		return "", 0, fmt.Errorf("Copilot token 响应缺少 expires_at 字段")
	}

	switch v := expiresAtVal.(type) {
	case float64:
		ts := int64(v)
		if ts > 10_000_000_000 {
			expiresAtMs = ts // 已经是毫秒
		} else {
			expiresAtMs = ts * 1000 // 秒 → 毫秒
		}
	case string:
		parsed, parseErr := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if parseErr != nil {
			return "", 0, fmt.Errorf("Copilot token 响应 expires_at 无效: %w", parseErr)
		}
		if parsed > 10_000_000_000 {
			expiresAtMs = parsed
		} else {
			expiresAtMs = parsed * 1000
		}
	default:
		return "", 0, fmt.Errorf("Copilot token 响应 expires_at 类型无效")
	}

	return tokenStr, expiresAtMs, nil
}

// loadCopilotTokenCache 从磁盘加载缓存。
func loadCopilotTokenCache(path string) *CachedCopilotToken {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cache CachedCopilotToken
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	return &cache
}

// saveCopilotTokenCache 写入缓存到磁盘。
func saveCopilotTokenCache(path string, cache *CachedCopilotToken) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		slog.Warn("copilot: 创建缓存目录失败", "error", err)
		return
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		slog.Warn("copilot: 序列化 token 缓存失败", "error", err)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		slog.Warn("copilot: 写入 token 缓存失败", "error", err)
	}
}

// ResolveCopilotApiToken 用 GitHub access token 交换 Copilot API token。
// 优先返回磁盘缓存（如仍可用），否则远程获取并缓存。
// 对应 TS: resolveCopilotApiToken(params)
func ResolveCopilotApiToken(ctx context.Context, client *http.Client, githubToken string) (*CopilotApiTokenResult, error) {
	if client == nil {
		client = http.DefaultClient
	}

	cachePath := resolveCopilotTokenCachePath()
	now := time.Now().UnixMilli()

	// 1. 尝试从缓存读取
	cached := loadCopilotTokenCache(cachePath)
	if cached != nil && isTokenUsable(cached, now) {
		baseURL := DeriveCopilotApiBaseUrlFromToken(cached.Token)
		if baseURL == "" {
			baseURL = DefaultCopilotApiBaseURL
		}
		return &CopilotApiTokenResult{
			Token:     cached.Token,
			ExpiresAt: cached.ExpiresAt,
			Source:    fmt.Sprintf("cache:%s", cachePath),
			BaseURL:   baseURL,
		}, nil
	}

	// 2. 远程获取
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, CopilotTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Copilot token 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+githubToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Copilot API token 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Copilot token 交换失败: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Copilot token 响应失败: %w", err)
	}

	token, expiresAtMs, err := parseCopilotTokenResponse(data)
	if err != nil {
		return nil, err
	}

	// 3. 缓存到磁盘
	saveCopilotTokenCache(cachePath, &CachedCopilotToken{
		Token:     token,
		ExpiresAt: expiresAtMs,
		UpdatedAt: time.Now().UnixMilli(),
	})

	baseURL := DeriveCopilotApiBaseUrlFromToken(token)
	if baseURL == "" {
		baseURL = DefaultCopilotApiBaseURL
	}

	return &CopilotApiTokenResult{
		Token:     token,
		ExpiresAt: expiresAtMs,
		Source:    fmt.Sprintf("fetched:%s", CopilotTokenURL),
		BaseURL:   baseURL,
	}, nil
}
