// providers/copilot/auth.go — GitHub Copilot 认证流程（Device Code Flow）
// 对应 TS 文件: src/providers/github-copilot-auth.ts
// 包含 GitHub OAuth Device Code 授权流程的核心 HTTP 逻辑。
// 注意：CLI 交互入口 GithubCopilotLoginCommand 依赖 CLI/UI 层（窗口4+），
// 此处仅实现纯 HTTP 逻辑部分。
package copilot

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClientID GitHub OAuth 应用客户端 ID。
const ClientID = "Iv1.b507a08c87ecfe98"

// DeviceCodeURL GitHub Device Code 请求端点。
const DeviceCodeURL = "https://github.com/login/device/code"

// AccessTokenURL GitHub OAuth Access Token 请求端点。
const AccessTokenURL = "https://github.com/login/oauth/access_token"

// DeviceCodeResponse GitHub Device Code 响应。
// 对应 TS: DeviceCodeResponse
type DeviceCodeResponse struct {
	// DeviceCode 设备码
	DeviceCode string `json:"device_code"`
	// UserCode 用户码（需要在浏览器中输入）
	UserCode string `json:"user_code"`
	// VerificationURI 验证 URL（用户需访问此 URL）
	VerificationURI string `json:"verification_uri"`
	// ExpiresIn 设备码过期时间（秒）
	ExpiresIn int `json:"expires_in"`
	// Interval 轮询间隔（秒）
	Interval int `json:"interval"`
}

// deviceTokenSuccessResponse Device Token 成功响应。
type deviceTokenSuccessResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope,omitempty"`
}

// deviceTokenErrorResponse Device Token 错误响应。
type deviceTokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

// RequestDeviceCode 向 GitHub 请求设备码。
// 对应 TS: requestDeviceCode()
func RequestDeviceCode(scope string, httpClient *http.Client) (*DeviceCodeResponse, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	body := url.Values{
		"client_id": {ClientID},
		"scope":     {scope},
	}

	req, err := http.NewRequest("POST", DeviceCodeURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建 GitHub Device Code 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub Device Code 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub Device Code 请求失败: HTTP %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 GitHub Device Code 响应失败: %w", err)
	}

	var result DeviceCodeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("GitHub Device Code 响应解析失败: %w", err)
	}

	if result.DeviceCode == "" || result.UserCode == "" || result.VerificationURI == "" {
		return nil, fmt.Errorf("GitHub Device Code 响应缺少必要字段")
	}

	return &result, nil
}

// PollForAccessToken 轮询 GitHub 获取 Access Token。
// 在设备码过期前持续轮询。
// 对应 TS: pollForAccessToken()
func PollForAccessToken(deviceCode string, intervalMs int64, expiresAt int64, httpClient *http.Client) (string, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	body := url.Values{
		"client_id":   {ClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	for time.Now().UnixMilli() < expiresAt {
		req, err := http.NewRequest("POST", AccessTokenURL, strings.NewReader(body.Encode()))
		if err != nil {
			return "", fmt.Errorf("创建 GitHub Token 请求失败: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("GitHub Token 请求失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", fmt.Errorf("GitHub Device Token 请求失败: HTTP %d", resp.StatusCode)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("读取 GitHub Token 响应失败: %w", err)
		}

		// 尝试解析为成功响应
		var success deviceTokenSuccessResponse
		if err := json.Unmarshal(respBody, &success); err == nil && success.AccessToken != "" {
			return success.AccessToken, nil
		}

		// 解析为错误响应
		var errResp deviceTokenErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err != nil {
			return "", fmt.Errorf("GitHub Token 响应解析失败: %w", err)
		}

		errCode := errResp.Error
		if errCode == "" {
			errCode = "unknown"
		}

		switch errCode {
		case "authorization_pending":
			time.Sleep(time.Duration(intervalMs) * time.Millisecond)
			continue
		case "slow_down":
			time.Sleep(time.Duration(intervalMs+2000) * time.Millisecond)
			continue
		case "expired_token":
			return "", fmt.Errorf("GitHub 设备码已过期，请重新运行登录")
		case "access_denied":
			return "", fmt.Errorf("GitHub 登录已取消")
		default:
			return "", fmt.Errorf("GitHub Device Flow 错误: %s", errCode)
		}
	}

	return "", fmt.Errorf("GitHub 设备码已过期，请重新运行登录")
}

// TODO: GithubCopilotLoginCommand — CLI 交互入口
// 依赖 CLI/UI 层（@clack/prompts、ensureAuthProfileStore、updateConfig、
// applyAuthProfileConfig、logConfigUpdated、RuntimeEnv、stylePromptTitle），
// 这些将在窗口4+实现。
// 对应 TS: githubCopilotLoginCommand()
