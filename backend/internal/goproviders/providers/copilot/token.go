// providers/copilot/token.go — GitHub Copilot Token 管理
// 对应 TS 文件: src/providers/github-copilot-token.ts
// 包含 Copilot Token 的缓存、交换和 API 基础 URL 解析逻辑。
package copilot

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CopilotTokenURL Copilot Token 交换端点。
const CopilotTokenURL = "https://api.github.com/copilot_internal/v2/token"

// DefaultCopilotAPIBaseURL 默认 Copilot API 基础 URL。
const DefaultCopilotAPIBaseURL = "https://api.individual.githubcopilot.com"

// CachedCopilotToken 缓存的 Copilot Token。
// 对应 TS: CachedCopilotToken
type CachedCopilotToken struct {
	// Token Copilot API 令牌
	Token string `json:"token"`
	// ExpiresAt 过期时间（毫秒级 Unix 时间戳）
	ExpiresAt int64 `json:"expiresAt"`
	// UpdatedAt 更新时间（毫秒级 Unix 时间戳）
	UpdatedAt int64 `json:"updatedAt"`
}

// proxyEpRE 提取 token 中 proxy-ep 值的正则表达式。
var proxyEpRE = regexp.MustCompile(`(?:^|;)\s*proxy-ep=([^;\s]+)`)

// isTokenUsable 检查缓存 token 是否仍可用。
// 保留 5 分钟安全余量。
// 对应 TS: isTokenUsable()
func isTokenUsable(cache *CachedCopilotToken, now int64) bool {
	return cache.ExpiresAt-now > 5*60*1000
}

// copilotTokenAPIResponse Copilot Token API 原始响应结构。
type copilotTokenAPIResponse struct {
	Token     string      `json:"token"`
	ExpiresAt interface{} `json:"expires_at"`
}

// parseCopilotTokenResponse 解析 Copilot Token API 响应。
// GitHub 返回 unix 秒级时间戳，但防御性地也接受毫秒级。
// 对应 TS: parseCopilotTokenResponse()
func parseCopilotTokenResponse(data []byte) (token string, expiresAt int64, err error) {
	var resp copilotTokenAPIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", 0, fmt.Errorf("Copilot Token 响应解析失败: %w", err)
	}

	token = strings.TrimSpace(resp.Token)
	if token == "" {
		return "", 0, fmt.Errorf("Copilot Token 响应缺少 token 字段")
	}

	// 解析 expires_at，支持数字和字符串两种格式
	switch v := resp.ExpiresAt.(type) {
	case float64:
		ts := int64(v)
		if ts > 10_000_000_000 {
			expiresAt = ts
		} else {
			expiresAt = ts * 1000
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return "", 0, fmt.Errorf("Copilot Token 响应缺少 expires_at 字段")
		}
		var parsed int64
		if _, err := fmt.Sscanf(trimmed, "%d", &parsed); err != nil {
			return "", 0, fmt.Errorf("Copilot Token 响应 expires_at 格式无效")
		}
		if parsed > 10_000_000_000 {
			expiresAt = parsed
		} else {
			expiresAt = parsed * 1000
		}
	default:
		return "", 0, fmt.Errorf("Copilot Token 响应缺少 expires_at 字段")
	}

	return token, expiresAt, nil
}

// DeriveCopilotAPIBaseURLFromToken 从 Copilot Token 中提取 API 基础 URL。
// Token 是分号分隔的键值对，其中 proxy-ep=... 指定代理端点。
// 将 proxy.* 替换为 api.*（参考上游 getGitHubCopilotBaseUrl）。
// 对应 TS: deriveCopilotApiBaseUrlFromToken()
func DeriveCopilotAPIBaseURLFromToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}

	matches := proxyEpRE.FindStringSubmatch(trimmed)
	if matches == nil || len(matches) < 2 {
		return ""
	}

	proxyEp := strings.TrimSpace(matches[1])
	if proxyEp == "" {
		return ""
	}

	// 移除协议前缀，将 proxy. 替换为 api.
	host := proxyEp
	// 移除 http:// 或 https:// 前缀
	if idx := strings.Index(host, "://"); idx >= 0 {
		host = host[idx+3:]
	}
	// 将 proxy. 前缀替换为 api.
	if strings.HasPrefix(strings.ToLower(host), "proxy.") {
		host = "api." + host[6:]
	}

	if host == "" {
		return ""
	}

	return "https://" + host
}

// CopilotAPITokenResult Copilot API Token 解析结果。
type CopilotAPITokenResult struct {
	// Token Copilot API 令牌
	Token string
	// ExpiresAt 过期时间（毫秒级 Unix 时间戳）
	ExpiresAt int64
	// Source 令牌来源描述
	Source string
	// BaseURL API 基础 URL
	BaseURL string
}

// ResolveCopilotAPITokenParams Copilot API Token 解析参数。
type ResolveCopilotAPITokenParams struct {
	// GitHubToken GitHub 访问令牌（必填）
	GitHubToken string
	// CachePath 缓存文件路径（可选，默认自动推导）
	CachePath string
	// HTTPClient 自定义 HTTP 客户端（可选，默认 http.DefaultClient）
	HTTPClient *http.Client
}

// ResolveCopilotAPIToken 解析 Copilot API Token。
// 先检查缓存文件，若缓存有效则直接返回；否则使用 GitHub Token 向 Copilot 端点交换新 Token。
// 对应 TS: resolveCopilotApiToken()
func ResolveCopilotAPIToken(params ResolveCopilotAPITokenParams) (*CopilotAPITokenResult, error) {
	if params.GitHubToken == "" {
		return nil, fmt.Errorf("GitHub Token 不能为空")
	}

	cachePath := strings.TrimSpace(params.CachePath)
	if cachePath == "" {
		stateDir := os.Getenv("OPENCLAW_STATE_DIR")
		if stateDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("无法获取用户目录: %w", err)
			}
			stateDir = filepath.Join(homeDir, ".openclaw")
		}
		cachePath = filepath.Join(stateDir, "credentials", "github-copilot.token.json")
	}

	client := params.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	nowMs := time.Now().UnixMilli()

	// 尝试从缓存加载
	cached, err := loadCachedToken(cachePath)
	if err == nil && cached != nil && cached.Token != "" && isTokenUsable(cached, nowMs) {
		baseURL := DeriveCopilotAPIBaseURLFromToken(cached.Token)
		if baseURL == "" {
			baseURL = DefaultCopilotAPIBaseURL
		}
		return &CopilotAPITokenResult{
			Token:     cached.Token,
			ExpiresAt: cached.ExpiresAt,
			Source:    fmt.Sprintf("cache:%s", cachePath),
			BaseURL:   baseURL,
		}, nil
	}

	// 向 Copilot 端点交换新 Token
	req, err := http.NewRequest("GET", CopilotTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Copilot Token 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+params.GitHubToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Copilot Token 交换请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Copilot Token 交换失败: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Copilot Token 响应失败: %w", err)
	}

	token, expiresAt, err := parseCopilotTokenResponse(body)
	if err != nil {
		return nil, err
	}

	// 保存到缓存
	payload := &CachedCopilotToken{
		Token:     token,
		ExpiresAt: expiresAt,
		UpdatedAt: nowMs,
	}
	_ = saveCachedToken(cachePath, payload)

	baseURL := DeriveCopilotAPIBaseURLFromToken(token)
	if baseURL == "" {
		baseURL = DefaultCopilotAPIBaseURL
	}

	return &CopilotAPITokenResult{
		Token:     payload.Token,
		ExpiresAt: payload.ExpiresAt,
		Source:    fmt.Sprintf("fetched:%s", CopilotTokenURL),
		BaseURL:   baseURL,
	}, nil
}

// loadCachedToken 从文件加载缓存的 Copilot Token。
func loadCachedToken(path string) (*CachedCopilotToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cached CachedCopilotToken
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, err
	}
	return &cached, nil
}

// saveCachedToken 将 Copilot Token 缓存到文件。
func saveCachedToken(path string, token *CachedCopilotToken) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
