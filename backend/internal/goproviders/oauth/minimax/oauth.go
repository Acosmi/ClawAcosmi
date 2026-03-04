// oauth/minimax/oauth.go — MiniMax Portal OAuth 设备码流程
// 对应 TS 文件: extensions/minimax-portal-auth/oauth.ts (242 行)
// 本文件实现 MiniMax Portal 的 OAuth 设备码认证流程。
package minimax

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

// ──────────────────── 类型和常量 ────────────────────

// MiniMaxRegion MiniMax 区域标识。
// 对应 TS: MiniMaxRegion
type MiniMaxRegion string

const (
	// RegionCN 中国大陆区域
	RegionCN MiniMaxRegion = "cn"
	// RegionGlobal 全球区域
	RegionGlobal MiniMaxRegion = "global"
)

// miniMaxOAuthConfig 区域级 OAuth 配置。
type miniMaxOAuthConfig struct {
	BaseURL  string
	ClientID string
}

// oauthConfigs 各区域的 OAuth 配置。
var oauthConfigs = map[MiniMaxRegion]miniMaxOAuthConfig{
	RegionCN: {
		BaseURL:  "https://api.minimaxi.com",
		ClientID: "78257093-7e40-4613-99e0-527b14b39113",
	},
	RegionGlobal: {
		BaseURL:  "https://api.minimax.io",
		ClientID: "78257093-7e40-4613-99e0-527b14b39113",
	},
}

const (
	minimaxOAuthScope     = "group_id profile model.completion"
	minimaxOAuthGrantType = "urn:ietf:params:oauth:grant-type:user_code"
)

// MiniMaxOAuthAuthorization 设备码授权响应。
// 对应 TS: MiniMaxOAuthAuthorization
type MiniMaxOAuthAuthorization struct {
	// UserCode 用户验证码
	UserCode string `json:"user_code"`
	// VerificationURI 验证 URI
	VerificationURI string `json:"verification_uri"`
	// ExpiredIn 过期时间戳（Unix 毫秒）
	ExpiredIn int64 `json:"expired_in"`
	// Interval 轮询间隔（毫秒）
	Interval int `json:"interval,omitempty"`
	// State 状态参数
	State string `json:"state"`
}

// MiniMaxOAuthToken MiniMax OAuth Token。
// 对应 TS: MiniMaxOAuthToken
type MiniMaxOAuthToken struct {
	// Access 访问令牌
	Access string `json:"access"`
	// Refresh 刷新令牌
	Refresh string `json:"refresh"`
	// Expires 过期时间（Unix 毫秒时间戳）
	Expires int64 `json:"expires"`
	// ResourceURL 资源 URL（可选）
	ResourceURL string `json:"resourceUrl,omitempty"`
	// NotificationMessage 通知消息（可选）
	NotificationMessage string `json:"notification_message,omitempty"`
}

// TokenPollResult Token 轮询结果。
type TokenPollResult struct {
	Status  string // "success", "pending", "error"
	Token   *MiniMaxOAuthToken
	Message string
}

// ──────────────────── OAuth 端点 ────────────────────

type oauthEndpoints struct {
	CodeEndpoint  string
	TokenEndpoint string
	ClientID      string
	BaseURL       string
}

func getOAuthEndpoints(region MiniMaxRegion) oauthEndpoints {
	config := oauthConfigs[region]
	return oauthEndpoints{
		CodeEndpoint:  config.BaseURL + "/oauth/code",
		TokenEndpoint: config.BaseURL + "/oauth/token",
		ClientID:      config.ClientID,
		BaseURL:       config.BaseURL,
	}
}

// ──────────────────── 设备码请求 ────────────────────

// RequestOAuthCode 请求 MiniMax OAuth 设备码。
// 对应 TS: RequestOAuthCode()
func RequestOAuthCode(challenge, state string, region MiniMaxRegion) (*MiniMaxOAuthAuthorization, error) {
	endpoints := getOAuthEndpoints(region)
	body := common.ToFormURLEncoded(map[string]string{
		"response_type":         "code",
		"client_id":             endpoints.ClientID,
		"scope":                 minimaxOAuthScope,
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
		"state":                 state,
	})

	req, _ := http.NewRequest("POST", endpoints.CodeEndpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-request-id", oauth.GenerateUUIDv4())

	resp, err := oauth.DefaultHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MiniMax OAuth 授权请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MiniMax OAuth authorization failed: %s", string(respBody))
	}

	var payload struct {
		MiniMaxOAuthAuthorization
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("解析 MiniMax OAuth 授权响应失败: %w", err)
	}

	if payload.UserCode == "" || payload.VerificationURI == "" {
		errMsg := payload.Error
		if errMsg == "" {
			errMsg = "MiniMax OAuth authorization returned an incomplete payload (missing user_code or verification_uri)."
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	if payload.State != state {
		return nil, fmt.Errorf("MiniMax OAuth state mismatch: possible CSRF attack or session corruption.")
	}
	return &payload.MiniMaxOAuthAuthorization, nil
}

// ──────────────────── Token 轮询 ────────────────────

// PollOAuthToken 轮询 MiniMax OAuth Token。
// 对应 TS: PollOAuthToken()
func PollOAuthToken(userCode, verifier string, region MiniMaxRegion) TokenPollResult {
	endpoints := getOAuthEndpoints(region)
	body := common.ToFormURLEncoded(map[string]string{
		"grant_type":    minimaxOAuthGrantType,
		"client_id":     endpoints.ClientID,
		"user_code":     userCode,
		"code_verifier": verifier,
	})

	req, _ := http.NewRequest("POST", endpoints.TokenEndpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := oauth.DefaultHTTPClient.Do(req)
	if err != nil {
		return TokenPollResult{Status: "error", Message: fmt.Sprintf("MiniMax OAuth 请求失败: %v", err)}
	}
	defer resp.Body.Close()

	text, _ := io.ReadAll(resp.Body)

	var payload struct {
		Status   string `json:"status"`
		BaseResp *struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
		AccessToken         *string `json:"access_token"`
		RefreshToken        *string `json:"refresh_token"`
		ExpiredIn           *int64  `json:"expired_in"`
		TokenType           string  `json:"token_type"`
		ResourceURL         string  `json:"resource_url"`
		NotificationMessage string  `json:"notification_message"`
	}

	if len(text) > 0 {
		_ = json.Unmarshal(text, &payload)
	}

	if resp.StatusCode != http.StatusOK {
		msg := string(text)
		if payload.BaseResp != nil && payload.BaseResp.StatusMsg != "" {
			msg = payload.BaseResp.StatusMsg
		}
		if msg == "" {
			msg = "MiniMax OAuth failed to parse response."
		}
		return TokenPollResult{Status: "error", Message: msg}
	}

	if payload.Status == "error" {
		return TokenPollResult{Status: "error", Message: "An error occurred. Please try again later"}
	}

	if payload.Status != "success" {
		return TokenPollResult{Status: "pending", Message: "current user code is not authorized"}
	}

	if payload.AccessToken == nil || payload.RefreshToken == nil || payload.ExpiredIn == nil ||
		*payload.AccessToken == "" || *payload.RefreshToken == "" || *payload.ExpiredIn == 0 {
		return TokenPollResult{Status: "error", Message: "MiniMax OAuth returned incomplete token payload."}
	}

	return TokenPollResult{
		Status: "success",
		Token: &MiniMaxOAuthToken{
			Access:              *payload.AccessToken,
			Refresh:             *payload.RefreshToken,
			Expires:             *payload.ExpiredIn,
			ResourceURL:         payload.ResourceURL,
			NotificationMessage: payload.NotificationMessage,
		},
	}
}

// ──────────────────── 主入口 ────────────────────

// LoginMiniMaxPortalOAuthParams MiniMax Portal OAuth 登录参数。
type LoginMiniMaxPortalOAuthParams struct {
	// OpenURL 在浏览器中打开 URL
	OpenURL func(url string) error
	// Note 显示通知消息
	Note func(message string, title string) error
	// Progress 进度报告器
	Progress oauth.ProgressReporter
	// Region 区域（可选，默认 "global"）
	Region MiniMaxRegion
}

// LoginMiniMaxPortalOAuth 执行 MiniMax Portal OAuth 设备码认证流程。
// 对应 TS: loginMiniMaxPortalOAuth()
func LoginMiniMaxPortalOAuth(params LoginMiniMaxPortalOAuthParams) (*MiniMaxOAuthToken, error) {
	region := params.Region
	if region == "" {
		region = RegionGlobal
	}

	// 生成 PKCE
	pkce, err := common.GeneratePKCEVerifierChallenge()
	if err != nil {
		return nil, fmt.Errorf("生成 PKCE 失败: %w", err)
	}
	state, err := oauth.GenerateRandomState()
	if err != nil {
		return nil, err
	}

	// 请求设备码
	oauthResp, err := RequestOAuthCode(pkce.Challenge, state, region)
	if err != nil {
		return nil, err
	}

	verificationURL := oauthResp.VerificationURI
	noteLines := fmt.Sprintf(
		"Open %s to approve access.\nIf prompted, enter the code %s.\nInterval: %v, Expires at: %d unix timestamp",
		verificationURL, oauthResp.UserCode,
		func() interface{} {
			if oauthResp.Interval > 0 {
				return oauthResp.Interval
			}
			return "default (2000ms)"
		}(),
		oauthResp.ExpiredIn,
	)
	_ = params.Note(noteLines, "MiniMax OAuth")

	// 尝试打开浏览器
	if params.OpenURL != nil {
		_ = params.OpenURL(verificationURL) // 忽略错误，回退到手动复制粘贴
	}

	// 轮询 Token
	pollIntervalMs := 2000
	if oauthResp.Interval > 0 {
		pollIntervalMs = oauthResp.Interval
	}
	expireTimeMs := oauthResp.ExpiredIn

	for time.Now().UnixMilli() < expireTimeMs {
		params.Progress.Update("Waiting for MiniMax OAuth approval…")
		result := PollOAuthToken(oauthResp.UserCode, pkce.Verifier, region)

		if result.Status == "success" {
			return result.Token, nil
		}
		if result.Status == "error" {
			return nil, fmt.Errorf("MiniMax OAuth failed: %s", result.Message)
		}
		if result.Status == "pending" {
			pollIntervalMs = int(math.Min(float64(pollIntervalMs)*1.5, 10000))
		}
		time.Sleep(time.Duration(pollIntervalMs) * time.Millisecond)
	}

	return nil, fmt.Errorf("MiniMax OAuth timed out waiting for authorization.")
}
