// oauth/qwen/oauth.go — Qwen Portal OAuth 设备码流程
// 对应 TS 文件: extensions/qwen-portal-auth/oauth.ts (180 行)
// 本文件实现通义千问 Portal 的 RFC 8628 设备码认证流程。
package qwen

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/oauth"
)

// ──────────────────── 常量定义 ────────────────────

const (
	qwenOAuthBaseURL            = "https://chat.qwen.ai"
	qwenOAuthDeviceCodeEndpoint = qwenOAuthBaseURL + "/api/v1/oauth2/device/code"
	qwenOAuthTokenEndpoint      = qwenOAuthBaseURL + "/api/v1/oauth2/token"
	qwenOAuthClientID           = "f0304373b74a44d2b584a3fb70ca9e56"
	qwenOAuthScope              = "openid profile email model.completion"
	qwenOAuthGrantType          = "urn:ietf:params:oauth:grant-type:device_code"
)

// ──────────────────── 类型定义 ────────────────────

// QwenDeviceAuthorization 设备码授权响应。
// 对应 TS: QwenDeviceAuthorization
type QwenDeviceAuthorization struct {
	// DeviceCode 设备码
	DeviceCode string `json:"device_code"`
	// UserCode 用户验证码
	UserCode string `json:"user_code"`
	// VerificationURI 验证 URI
	VerificationURI string `json:"verification_uri"`
	// VerificationURIComplete 完整验证 URI（含 user_code，可选）
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	// ExpiresIn 过期时间（秒）
	ExpiresIn int `json:"expires_in"`
	// Interval 轮询间隔（秒，可选）
	Interval int `json:"interval,omitempty"`
}

// QwenOAuthToken Qwen OAuth Token。
// 对应 TS: QwenOAuthToken
type QwenOAuthToken struct {
	// Access 访问令牌
	Access string `json:"access"`
	// Refresh 刷新令牌
	Refresh string `json:"refresh"`
	// Expires 过期时间（毫秒级 Unix 时间戳）
	Expires int64 `json:"expires"`
	// ResourceURL 资源 URL（可选）
	ResourceURL string `json:"resourceUrl,omitempty"`
}

// DeviceTokenResult 设备码 Token 轮询结果。
type DeviceTokenResult struct {
	Status   string // "success", "pending", "error"
	Token    *QwenOAuthToken
	Message  string
	SlowDown bool
}

// ──────────────────── 设备码请求 ────────────────────

// RequestDeviceCode 请求 Qwen OAuth 设备码。
// 对应 TS: RequestDeviceCode()
func RequestDeviceCode(challenge string) (*QwenDeviceAuthorization, error) {
	body := common.ToFormURLEncoded(map[string]string{
		"client_id":             qwenOAuthClientID,
		"scope":                 qwenOAuthScope,
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	})

	req, _ := http.NewRequest("POST", qwenOAuthDeviceCodeEndpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-request-id", oauth.GenerateUUIDv4())

	resp, err := oauth.DefaultHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Qwen 设备码请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Qwen device authorization failed: %s", string(respBody))
	}

	var payload struct {
		QwenDeviceAuthorization
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("解析 Qwen 设备码响应失败: %w", err)
	}

	if payload.DeviceCode == "" || payload.UserCode == "" || payload.VerificationURI == "" {
		errMsg := payload.Error
		if errMsg == "" {
			errMsg = "Qwen device authorization returned an incomplete payload (missing user_code or verification_uri)."
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	return &payload.QwenDeviceAuthorization, nil
}

// ──────────────────── Token 轮询 ────────────────────

// PollDeviceToken 轮询 Qwen OAuth Token。
// 对应 TS: PollDeviceToken()
func PollDeviceToken(deviceCode, verifier string) DeviceTokenResult {
	body := common.ToFormURLEncoded(map[string]string{
		"grant_type":    qwenOAuthGrantType,
		"client_id":     qwenOAuthClientID,
		"device_code":   deviceCode,
		"code_verifier": verifier,
	})

	req, _ := http.NewRequest("POST", qwenOAuthTokenEndpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := oauth.DefaultHTTPClient.Do(req)
	if err != nil {
		return DeviceTokenResult{Status: "error", Message: fmt.Sprintf("Qwen OAuth 请求失败: %v", err)}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errorPayload struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(respBody, &errorPayload) == nil {
			if errorPayload.Error == "authorization_pending" {
				return DeviceTokenResult{Status: "pending"}
			}
			if errorPayload.Error == "slow_down" {
				return DeviceTokenResult{Status: "pending", SlowDown: true}
			}
			msg := errorPayload.ErrorDescription
			if msg == "" {
				msg = errorPayload.Error
			}
			if msg == "" {
				msg = resp.Status
			}
			return DeviceTokenResult{Status: "error", Message: msg}
		}
		return DeviceTokenResult{Status: "error", Message: string(respBody)}
	}

	var tokenPayload struct {
		AccessToken  *string `json:"access_token"`
		RefreshToken *string `json:"refresh_token"`
		ExpiresIn    *int64  `json:"expires_in"`
		TokenType    string  `json:"token_type"`
		ResourceURL  string  `json:"resource_url"`
	}
	if err := json.Unmarshal(respBody, &tokenPayload); err != nil {
		return DeviceTokenResult{Status: "error", Message: "Qwen OAuth failed to parse token response."}
	}

	if tokenPayload.AccessToken == nil || tokenPayload.RefreshToken == nil || tokenPayload.ExpiresIn == nil ||
		*tokenPayload.AccessToken == "" || *tokenPayload.RefreshToken == "" || *tokenPayload.ExpiresIn == 0 {
		return DeviceTokenResult{Status: "error", Message: "Qwen OAuth returned incomplete token payload."}
	}

	return DeviceTokenResult{
		Status: "success",
		Token: &QwenOAuthToken{
			Access:      *tokenPayload.AccessToken,
			Refresh:     *tokenPayload.RefreshToken,
			Expires:     time.Now().UnixMilli() + *tokenPayload.ExpiresIn*1000,
			ResourceURL: tokenPayload.ResourceURL,
		},
	}
}

// ──────────────────── 主入口 ────────────────────

// LoginQwenPortalOAuthParams Qwen Portal OAuth 登录参数。
type LoginQwenPortalOAuthParams struct {
	// OpenURL 在浏览器中打开 URL
	OpenURL func(url string) error
	// Note 显示通知消息
	Note func(message string, title string) error
	// Progress 进度报告器
	Progress oauth.ProgressReporter
}

// LoginQwenPortalOAuth 执行 Qwen Portal OAuth 设备码认证流程。
// 对应 TS: loginQwenPortalOAuth()
func LoginQwenPortalOAuth(params LoginQwenPortalOAuthParams) (*QwenOAuthToken, error) {
	// 生成 PKCE
	pkce, err := common.GeneratePKCEVerifierChallenge()
	if err != nil {
		return nil, fmt.Errorf("生成 PKCE 失败: %w", err)
	}

	// 请求设备码
	device, err := RequestDeviceCode(pkce.Challenge)
	if err != nil {
		return nil, err
	}

	// 构建验证 URL
	verificationURL := device.VerificationURIComplete
	if verificationURL == "" {
		verificationURL = device.VerificationURI
	}

	_ = params.Note(
		fmt.Sprintf("Open %s to approve access.\nIf prompted, enter the code %s.", verificationURL, device.UserCode),
		"Qwen OAuth",
	)

	// 尝试打开浏览器
	if params.OpenURL != nil {
		_ = params.OpenURL(verificationURL) // 忽略错误，回退到手动复制粘贴
	}

	// 轮询 Token
	start := time.Now()
	pollIntervalMs := 2000
	if device.Interval > 0 {
		pollIntervalMs = device.Interval * 1000
	}
	timeoutMs := int64(device.ExpiresIn) * 1000

	for time.Since(start).Milliseconds() < timeoutMs {
		params.Progress.Update("Waiting for Qwen OAuth approval…")
		result := PollDeviceToken(device.DeviceCode, pkce.Verifier)

		if result.Status == "success" {
			return result.Token, nil
		}
		if result.Status == "error" {
			return nil, fmt.Errorf("Qwen OAuth failed: %s", result.Message)
		}
		if result.Status == "pending" && result.SlowDown {
			pollIntervalMs = int(math.Min(float64(pollIntervalMs)*1.5, 10000))
		}
		time.Sleep(time.Duration(pollIntervalMs) * time.Millisecond)
	}

	return nil, fmt.Errorf("Qwen OAuth timed out waiting for authorization.")
}
