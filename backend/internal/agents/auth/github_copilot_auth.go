package auth

// github_copilot_auth.go — GitHub Copilot 设备码 OAuth 流程
// 对应 TS src/providers/github-copilot-auth.ts (185L)
//
// 实现 GitHub Device Flow (RFC 8628) 登录:
//   1. 请求 device code
//   2. 用户在浏览器中授权
//   3. 轮询获取 access token
//   4. 存入 AuthStore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHub Copilot OAuth 常量。
const (
	CopilotClientID       = "Iv1.b507a08c87ecfe98"
	CopilotDeviceCodeURL  = "https://github.com/login/device/code"
	CopilotAccessTokenURL = "https://github.com/login/oauth/access_token"
	CopilotDefaultScope   = "read:user"
	CopilotProfilePrefix  = "github-copilot"
)

// CopilotDeviceCodeResponse GitHub 设备码响应。
type CopilotDeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// CopilotDeviceTokenResponse GitHub 设备码 token 响应（成功或错误）。
type CopilotDeviceTokenResponse struct {
	// 成功字段
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
	// 错误字段
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// RequestCopilotDeviceCode 请求 GitHub 设备码。
// 对应 TS: requestDeviceCode({ scope })
func RequestCopilotDeviceCode(ctx context.Context, client *http.Client, scope string) (*CopilotDeviceCodeResponse, error) {
	if scope == "" {
		scope = CopilotDefaultScope
	}
	if client == nil {
		client = http.DefaultClient
	}

	body := url.Values{
		"client_id": {CopilotClientID},
		"scope":     {scope},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, CopilotDeviceCodeURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建设备码请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub 设备码失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 设备码请求失败: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取设备码响应失败: %w", err)
	}

	var result CopilotDeviceCodeResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析设备码响应失败: %w", err)
	}

	if result.DeviceCode == "" || result.UserCode == "" || result.VerificationURI == "" {
		return nil, fmt.Errorf("GitHub 设备码响应缺少必要字段")
	}

	return &result, nil
}

// PollForCopilotAccessToken 轮询 GitHub 直到用户授权并获得 access token。
// 对应 TS: pollForAccessToken({ deviceCode, intervalMs, expiresAt })
func PollForCopilotAccessToken(ctx context.Context, client *http.Client, deviceCode string, intervalMs int, expiresAt time.Time) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}

	body := url.Values{
		"client_id":   {CopilotClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	interval := time.Duration(intervalMs) * time.Millisecond
	if interval < time.Second {
		interval = time.Second
	}

	for {
		if time.Now().After(expiresAt) {
			return "", fmt.Errorf("GitHub 设备码已过期，请重新运行登录")
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, CopilotAccessTokenURL, strings.NewReader(body.Encode()))
		if err != nil {
			return "", fmt.Errorf("创建 token 请求失败: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("请求 GitHub access token 失败: %w", err)
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("GitHub device token 请求失败: HTTP %d", resp.StatusCode)
		}

		if err != nil {
			return "", fmt.Errorf("读取 token 响应失败: %w", err)
		}

		var tokenResp CopilotDeviceTokenResponse
		if err := json.Unmarshal(data, &tokenResp); err != nil {
			return "", fmt.Errorf("解析 token 响应失败: %w", err)
		}

		// 成功
		if tokenResp.AccessToken != "" {
			return tokenResp.AccessToken, nil
		}

		// 处理错误状态
		switch tokenResp.Error {
		case "authorization_pending":
			// 继续轮询
			continue
		case "slow_down":
			// 增加间隔
			interval += 2 * time.Second
			slog.Debug("copilot: slow_down, increasing interval", "interval", interval)
			continue
		case "expired_token":
			return "", fmt.Errorf("GitHub 设备码已过期，请重新运行登录")
		case "access_denied":
			return "", fmt.Errorf("GitHub 登录已取消")
		default:
			return "", fmt.Errorf("GitHub device flow 错误: %s", tokenResp.Error)
		}
	}
}

// StoreCopilotAuthProfile 将 GitHub Copilot access token 存入 AuthStore。
// 对应 TS: upsertAuthProfile + applyAuthProfileConfig
func StoreCopilotAuthProfile(store *AuthStore, profileId, accessToken string) error {
	if store == nil {
		return nil
	}
	if profileId == "" {
		profileId = FormatProfileId(CopilotProfilePrefix, "github")
	}

	UpsertAuthProfile(store, profileId, &AuthProfileCredential{
		Type:     CredentialToken,
		Provider: CopilotProfilePrefix,
		Token:    accessToken,
		// GitHub device flow token 不包含可靠的过期时间。
		// 后续通过 Copilot token 交换获取带过期的 token。
	})

	return nil
}
