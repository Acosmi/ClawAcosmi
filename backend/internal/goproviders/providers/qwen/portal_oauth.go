// providers/qwen/portal_oauth.go — 通义千问 Portal OAuth Token 刷新
// 对应 TS 文件: src/providers/qwen-portal-oauth.ts
// 包含 Qwen OAuth Token 刷新逻辑。
package qwen

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// QwenOAuthBaseURL Qwen OAuth 基础 URL。
const QwenOAuthBaseURL = "https://chat.qwen.ai"

// QwenOAuthTokenEndpoint Qwen OAuth Token 端点。
const QwenOAuthTokenEndpoint = QwenOAuthBaseURL + "/api/v1/oauth2/token"

// QwenOAuthClientID Qwen OAuth 客户端 ID。
const QwenOAuthClientID = "f0304373b74a44d2b584a3fb70ca9e56"

// qwenTokenResponse Qwen Token 刷新响应。
type qwenTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    *int   `json:"expires_in"`
}

// RefreshQwenPortalCredentials 刷新通义千问 Portal OAuth 凭证。
// 使用 refresh_token 获取新的 access_token。
// 遵循 RFC 6749 第 6 节：新 refresh_token 为可选，若存在则替换旧的。
// 对应 TS: refreshQwenPortalCredentials()
func RefreshQwenPortalCredentials(credentials types.OAuthCredentials, httpClient *http.Client) (types.OAuthCredentials, error) {
	refreshToken := strings.TrimSpace(credentials.Refresh)
	if refreshToken == "" {
		return types.OAuthCredentials{}, fmt.Errorf("Qwen OAuth 刷新令牌缺失，请重新认证")
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	body := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {QwenOAuthClientID},
	}

	req, err := http.NewRequest("POST", QwenOAuthTokenEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return types.OAuthCredentials{}, fmt.Errorf("创建 Qwen OAuth 刷新请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return types.OAuthCredentials{}, fmt.Errorf("Qwen OAuth 刷新请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusBadRequest {
			return types.OAuthCredentials{}, fmt.Errorf(
				"Qwen OAuth 刷新令牌已过期或无效。请使用 `openclaw models auth login --provider qwen-portal` 重新认证",
			)
		}
		text := strings.TrimSpace(string(respBody))
		if text == "" {
			text = resp.Status
		}
		return types.OAuthCredentials{}, fmt.Errorf("Qwen OAuth 刷新失败: %s", text)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.OAuthCredentials{}, fmt.Errorf("读取 Qwen OAuth 响应失败: %w", err)
	}

	var payload qwenTokenResponse
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return types.OAuthCredentials{}, fmt.Errorf("解析 Qwen OAuth 响应失败: %w", err)
	}

	accessToken := strings.TrimSpace(payload.AccessToken)
	if accessToken == "" {
		return types.OAuthCredentials{}, fmt.Errorf("Qwen OAuth 刷新响应缺少 access_token")
	}

	if payload.ExpiresIn == nil || *payload.ExpiresIn <= 0 {
		return types.OAuthCredentials{}, fmt.Errorf("Qwen OAuth 刷新响应缺少或无效的 expires_in")
	}

	// RFC 6749 第 6 节：新 refresh_token 为可选；若存在则替换旧的
	newRefresh := strings.TrimSpace(payload.RefreshToken)
	if newRefresh == "" {
		newRefresh = refreshToken
	}

	return types.OAuthCredentials{
		Access:  accessToken,
		Refresh: newRefresh,
		Expires: time.Now().UnixMilli() + int64(*payload.ExpiresIn)*1000,
	}, nil
}
