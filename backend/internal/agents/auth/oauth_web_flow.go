package auth

// oauth_web_flow.go — OAuth Web Flow（基于 golang.org/x/oauth2 官方库）
//
// 实现通用 OAuth 2.0 Authorization Code + PKCE 流程：
//   1. 启动本地 HTTP 服务器监听回调
//   2. 用 oauth2.Config 构建授权 URL（含 state + PKCE S256）
//   3. 打开浏览器引导用户授权
//   4. 接收回调中的 authorization code
//   5. 用 oauth2.Config.Exchange() 换取 token（自动处理 PKCE）
//   6. 存入 auth store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// ---------- OAuth Provider 配置 ----------

// OAuthProviderConfig 单个 OAuth 提供商的配置。
type OAuthProviderConfig struct {
	Provider     string   // 提供商标识（如 "google"）
	Label        string   // 显示名称（如 "Google Gemini OAuth"）
	AuthURL      string   // OAuth 授权端点
	TokenURL     string   // OAuth 令牌端点
	ClientID     string   // OAuth Client ID
	ClientSecret string   // OAuth Client Secret（部分 provider 不需要）
	Scopes       []string // OAuth scopes
	UsePKCE      bool     // 是否使用 PKCE (Proof Key for Code Exchange)
}

// toOAuth2Config 转换为 golang.org/x/oauth2 标准配置。
func (c *OAuthProviderConfig) toOAuth2Config(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.AuthURL,
			TokenURL: c.TokenURL,
		},
		Scopes:      c.Scopes,
		RedirectURL: redirectURL,
	}
}

// OAuthProviderRegistry 各 provider OAuth 配置注册表。
var OAuthProviderRegistry = map[string]*OAuthProviderConfig{
	"google": {
		Provider:     "google",
		Label:        "Google Gemini OAuth",
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		ClientID:     GeminiOAuthClientID,     // 来自 gemini-cli（Apache-2.0）
		ClientSecret: GeminiOAuthClientSecret, // Google Desktop OAuth 必须提供（非机密）
		Scopes: []string{
			GeminiOAuthScope, // cloud-platform
			"openid",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		UsePKCE: true,
	},
	"minimax-portal": {
		Provider: "minimax-portal",
		Label:    "MiniMax OAuth",
		AuthURL:  "https://api.minimax.chat/oauth/authorize",
		TokenURL: "https://api.minimax.chat/oauth/token",
		Scopes:   []string{"api"},
		UsePKCE:  true,
	},
	"qwen-portal": {
		Provider: "qwen-portal",
		Label:    "Qwen OAuth",
		AuthURL:  "https://chat.qwen.ai/api/v1/oauth2/authorize",
		TokenURL: QwenOAuthTokenEndpoint,
		Scopes:   []string{"openid"},
		UsePKCE:  true,
	},
}

// ---------- OAuth Web Flow ----------

// oauthCallbackResult OAuth 回调内部结果。
type oauthCallbackResult struct {
	Code  string
	State string
	Error string
}

// randomState 生成安全随机 state 参数。
func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RunOAuthWebFlow 执行完整 OAuth Web Flow（基于 golang.org/x/oauth2）。
//
// 流程：
//  1. 启动本地 HTTP 回调服务器
//  2. 用 oauth2.Config.AuthCodeURL() 构建授权 URL（含 PKCE S256）
//  3. 打开浏览器
//  4. 等待回调接收 authorization code
//  5. 用 oauth2.Config.Exchange() 换取 token
//  6. 存入 auth store
func RunOAuthWebFlow(ctx context.Context, config *OAuthProviderConfig, store *AuthStore) (*oauth2.Token, error) {
	if config.ClientID == "" {
		return nil, fmt.Errorf("%s OAuth requires a Client ID. "+
			"Set it via environment variable (e.g. %s_CLIENT_ID) or config",
			config.Label, strings.ToUpper(strings.ReplaceAll(config.Provider, "-", "_")))
	}

	// 1. 找到可用端口
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// 2. 构建 oauth2.Config
	oauthCfg := config.toOAuth2Config(redirectURI)

	// 3. 生成 state 和 PKCE verifier
	state := randomState()
	var verifier string
	authOpts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	exchangeOpts := []oauth2.AuthCodeOption{}
	if config.UsePKCE {
		verifier = oauth2.GenerateVerifier()
		authOpts = append(authOpts, oauth2.S256ChallengeOption(verifier))
		exchangeOpts = append(exchangeOpts, oauth2.VerifierOption(verifier))
	}

	// 4. 构建授权 URL
	authURL := oauthCfg.AuthCodeURL(state, authOpts...)

	// 5. 设置回调处理
	resultCh := make(chan oauthCallbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		result := oauthCallbackResult{
			Code:  q.Get("code"),
			State: q.Get("state"),
			Error: q.Get("error"),
		}

		if result.Error != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "<html><body><h2>❌ Authorization failed</h2><p>%s</p><p>You can close this window.</p></body></html>", result.Error)
			resultCh <- result
			return
		}
		if result.Code == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<html><body><h2>❌ No authorization code received</h2><p>You can close this window.</p></body></html>")
			result.Error = "no authorization code"
			resultCh <- result
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<html><body><h2>✅ Authorization successful!</h2><p>You can close this window and return to the terminal.</p></body></html>")
		resultCh <- result
	})

	server := &http.Server{Handler: mux}

	// 6. 启动服务器
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("OAuth callback server error", "error", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	// 7. 打开浏览器
	slog.Info("oauth: opening browser for authorization",
		"provider", config.Provider,
		"url", authURL)
	if err := openBrowserForOAuth(authURL); err != nil {
		slog.Warn("Failed to open browser automatically", "error", err)
		// 不是致命错误，用户可以手动复制 URL
	}

	// 8. 等待回调（带超时）
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var result oauthCallbackResult
	select {
	case result = <-resultCh:
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("OAuth authorization timed out (5 minute limit)")
	}

	if result.Error != "" {
		return nil, fmt.Errorf("OAuth authorization error: %s", result.Error)
	}
	if result.State != state {
		return nil, fmt.Errorf("OAuth state mismatch (possible CSRF attack)")
	}

	// 9. 用 oauth2.Config.Exchange() 换 token（自动处理 PKCE verifier + client_secret）
	token, err := oauthCfg.Exchange(ctx, result.Code, exchangeOpts...)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	// 10. 存入 auth store
	if store != nil {
		profileID := FormatProfileId(config.Provider, "default")
		expiresAt := token.Expiry.UnixMilli()
		store.Update(func(s *AuthProfileStore) bool {
			s.Profiles[profileID] = &AuthProfileCredential{
				Type:     CredentialOAuth,
				Provider: config.Provider,
				Token:    token.AccessToken,
				Key:      token.RefreshToken,
				Expires:  &expiresAt,
			}
			return true
		})
	}

	return token, nil
}

// ResolveOAuthClientID 从环境变量获取 OAuth Client ID。
func ResolveOAuthClientID(provider string) string {
	envKey := strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_CLIENT_ID"
	return os.Getenv(envKey)
}

// ResolveOAuthClientSecret 从环境变量获取 OAuth Client Secret。
func ResolveOAuthClientSecret(provider string) string {
	envKey := strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_CLIENT_SECRET"
	return os.Getenv(envKey)
}

// GetOAuthProviderConfig 获取指定 provider 的 OAuth 配置，自动填充 ClientID/Secret。
// 如果 provider 不支持 OAuth 或缺少 ClientID 则返回 nil。
func GetOAuthProviderConfig(provider string) *OAuthProviderConfig {
	cfg, ok := OAuthProviderRegistry[provider]
	if !ok {
		return nil
	}

	// 复制一份，避免修改全局注册表
	resolved := *cfg // shallow copy
	if resolved.ClientID == "" {
		resolved.ClientID = ResolveOAuthClientID(provider)
	}
	if resolved.ClientSecret == "" {
		resolved.ClientSecret = ResolveOAuthClientSecret(provider)
	}

	// Qwen 使用内置 ClientID
	if provider == "qwen-portal" && resolved.ClientID == "" {
		resolved.ClientID = QwenOAuthClientID
	}
	// Google 使用 gemini-cli 内置 ClientID（Apache-2.0）
	if provider == "google" && resolved.ClientID == "" {
		resolved.ClientID = GeminiOAuthClientID
	}

	return &resolved
}

// openBrowserForOAuth 打开系统默认浏览器。
func openBrowserForOAuth(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// CreateTokenSource 创建可自动刷新的 TokenSource。
// 用于从存储的凭证恢复 OAuth 会话，token 过期时自动用 refresh_token 刷新。
func CreateTokenSource(config *OAuthProviderConfig, token *oauth2.Token) oauth2.TokenSource {
	oauthCfg := config.toOAuth2Config("")
	return oauthCfg.TokenSource(context.Background(), token)
}
