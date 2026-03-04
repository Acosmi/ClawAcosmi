// agents/chutes_oauth.go — Chutes OAuth 流程
// 对应 TS 文件: src/agents/chutes-oauth.ts
package agents

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Chutes OAuth 端点常量
const (
	ChutesOAuthIssuer       = "https://api.chutes.ai"
	ChutesAuthorizeEndpoint = ChutesOAuthIssuer + "/idp/authorize"
	ChutesTokenEndpoint     = ChutesOAuthIssuer + "/idp/token"
	ChutesUserinfoEndpoint  = ChutesOAuthIssuer + "/idp/userinfo"
	DefaultExpiresBufferMs  = 5 * 60 * 1000
)

// ChutesPkce PKCE 验证器和挑战码对。
type ChutesPkce struct {
	Verifier  string
	Challenge string
}

// ChutesUserInfo Chutes 用户信息。
type ChutesUserInfo struct {
	Sub       string `json:"sub,omitempty"`
	Username  string `json:"username,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ChutesOAuthAppConfig Chutes OAuth 应用配置。
type ChutesOAuthAppConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUri  string
	Scopes       []string
}

// ChutesStoredOAuth Chutes 存储的 OAuth 凭证。
type ChutesStoredOAuth struct {
	Access    string `json:"access"`
	Refresh   string `json:"refresh"`
	Expires   int64  `json:"expires"`
	Email     string `json:"email,omitempty"`
	AccountId string `json:"accountId,omitempty"`
	ClientId  string `json:"clientId,omitempty"`
}

// GenerateChutesPkce 生成 PKCE 验证器和挑战码。
// 对应 TS: generateChutesPkce()
func GenerateChutesPkce() (ChutesPkce, error) {
	randomData := make([]byte, 32)
	if _, err := rand.Read(randomData); err != nil {
		return ChutesPkce{}, err
	}
	verifier := hex.EncodeToString(randomData)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	return ChutesPkce{Verifier: verifier, Challenge: challenge}, nil
}

// ParseOAuthCallbackInput 解析 OAuth 回调输入。
// 对应 TS: parseOAuthCallbackInput()
func ParseOAuthCallbackInput(input, expectedState string) (code, state string, errMsg string) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", "No input provided"
	}

	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" {
		// 尝试查询字符串格式
		if !strings.Contains(trimmed, "=") {
			return "", "", "Paste the full redirect URL (must include code + state)."
		}
		qs := trimmed
		if !strings.HasPrefix(qs, "?") {
			qs = "?" + qs
		}
		u, err = url.Parse("http://localhost/" + qs)
		if err != nil {
			return "", "", "Paste the full redirect URL (must include code + state)."
		}
	}

	code = strings.TrimSpace(u.Query().Get("code"))
	state = strings.TrimSpace(u.Query().Get("state"))
	if code == "" {
		return "", "", "Missing 'code' parameter in URL"
	}
	if state == "" {
		return "", "", "Missing 'state' parameter. Paste the full redirect URL."
	}
	if state != expectedState {
		return "", "", "OAuth state mismatch - possible CSRF attack. Please retry login."
	}
	return code, state, ""
}

// coerceExpiresAt 转换过期时间。
func coerceExpiresAt(expiresInSeconds int64, now int64) int64 {
	if expiresInSeconds < 0 {
		expiresInSeconds = 0
	}
	value := now + expiresInSeconds*1000 - int64(DefaultExpiresBufferMs)
	minValue := now + 30000
	if value < minValue {
		return minValue
	}
	return value
}

// FetchChutesUserInfo 获取 Chutes 用户信息。
// 对应 TS: fetchChutesUserInfo()
func FetchChutesUserInfo(accessToken string) (*ChutesUserInfo, error) {
	req, err := http.NewRequest("GET", ChutesUserinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var info ChutesUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, nil
	}
	return &info, nil
}

// ExchangeChutesCodeForTokens 用授权码交换令牌。
// 对应 TS: exchangeChutesCodeForTokens()
func ExchangeChutesCodeForTokens(app ChutesOAuthAppConfig, code, codeVerifier string) (*ChutesStoredOAuth, error) {
	now := time.Now().UnixMilli()
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {app.ClientId},
		"code":          {code},
		"redirect_uri":  {app.RedirectUri},
		"code_verifier": {codeVerifier},
	}
	if app.ClientSecret != "" {
		form.Set("client_secret", app.ClientSecret)
	}

	resp, err := http.PostForm(ChutesTokenEndpoint, form)
	if err != nil {
		return nil, fmt.Errorf("Chutes token exchange failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Chutes token exchange failed: %s", string(body))
	}

	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("Chutes token exchange: failed to parse response: %v", err)
	}

	access := strings.TrimSpace(data.AccessToken)
	refresh := strings.TrimSpace(data.RefreshToken)
	if access == "" {
		return nil, fmt.Errorf("Chutes token exchange returned no access_token")
	}
	if refresh == "" {
		return nil, fmt.Errorf("Chutes token exchange returned no refresh_token")
	}

	info, _ := FetchChutesUserInfo(access)
	result := &ChutesStoredOAuth{
		Access:   access,
		Refresh:  refresh,
		Expires:  coerceExpiresAt(data.ExpiresIn, now),
		ClientId: app.ClientId,
	}
	if info != nil {
		result.Email = info.Username
		result.AccountId = info.Sub
	}
	return result, nil
}

// RefreshChutesTokens 刷新 Chutes OAuth 令牌。
// 对应 TS: refreshChutesTokens()
func RefreshChutesTokens(credential *ChutesStoredOAuth) (*ChutesStoredOAuth, error) {
	now := time.Now().UnixMilli()
	refreshToken := strings.TrimSpace(credential.Refresh)
	if refreshToken == "" {
		return nil, fmt.Errorf("Chutes OAuth credential is missing refresh token")
	}

	clientId := strings.TrimSpace(credential.ClientId)
	if clientId == "" {
		clientId = strings.TrimSpace(os.Getenv("CHUTES_CLIENT_ID"))
	}
	if clientId == "" {
		return nil, fmt.Errorf("Missing CHUTES_CLIENT_ID for Chutes OAuth refresh (set env var or re-auth).")
	}
	clientSecret := strings.TrimSpace(os.Getenv("CHUTES_CLIENT_SECRET"))

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientId},
		"refresh_token": {refreshToken},
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}

	resp, err := http.PostForm(ChutesTokenEndpoint, form)
	if err != nil {
		return nil, fmt.Errorf("Chutes token refresh failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Chutes token refresh failed: %s", string(body))
	}

	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("Chutes token refresh: failed to parse response: %v", err)
	}

	access := strings.TrimSpace(data.AccessToken)
	if access == "" {
		return nil, fmt.Errorf("Chutes token refresh returned no access_token")
	}

	newRefresh := strings.TrimSpace(data.RefreshToken)
	if newRefresh == "" {
		newRefresh = refreshToken
	}

	return &ChutesStoredOAuth{
		Access:    access,
		Refresh:   newRefresh,
		Expires:   coerceExpiresAt(data.ExpiresIn, now),
		Email:     credential.Email,
		AccountId: credential.AccountId,
		ClientId:  clientId,
	}, nil
}
