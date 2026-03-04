// oauth/gemini/oauth.go — Google Gemini CLI OAuth 完整流程
// 对应 TS 文件: extensions/google-gemini-cli-auth/oauth.ts (734 行)
// 本文件实现 Gemini CLI 的 OAuth 2.0 PKCE 认证流程，包括：
// - 本地回调服务器模式（localhost:8085）
// - 远程/WSL2 手动粘贴模式
// - 从已安装的 Gemini CLI 提取 OAuth 凭证
// - Google Cloud 项目发现（loadCodeAssist + onboardUser）
package gemini

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/oauth"
)

// ──────────────────── 常量定义 ────────────────────

// clientIDKeys 客户端 ID 环境变量键列表。
var clientIDKeys = []string{
	"OPENCLAW_GEMINI_OAUTH_CLIENT_ID",
	"GEMINI_CLI_OAUTH_CLIENT_ID",
}

// clientSecretKeys 客户端 Secret 环境变量键列表。
var clientSecretKeys = []string{
	"OPENCLAW_GEMINI_OAUTH_CLIENT_SECRET",
	"GEMINI_CLI_OAUTH_CLIENT_SECRET",
}

const (
	redirectURI                = "http://localhost:8085/oauth2callback"
	authURL                    = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL                   = "https://oauth2.googleapis.com/token"
	userinfoURL                = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"
	codeAssistEndpointProd     = "https://cloudcode-pa.googleapis.com"
	codeAssistEndpointDaily    = "https://daily-cloudcode-pa.sandbox.googleapis.com"
	codeAssistEndpointAutopush = "https://autopush-cloudcode-pa.sandbox.googleapis.com"
	defaultFetchTimeoutMs      = 10000
)

// loadCodeAssistEndpoints Code Assist 端点列表（按优先级）。
var loadCodeAssistEndpoints = []string{
	codeAssistEndpointProd,
	codeAssistEndpointDaily,
	codeAssistEndpointAutopush,
}

// scopes OAuth 作用域列表。
var scopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

const (
	tierFree     = "free-tier"
	tierLegacy   = "legacy-tier"
	tierStandard = "standard-tier"
)

// ──────────────────── 类型定义 ────────────────────

// GeminiCliOAuthCredentials Gemini CLI OAuth 凭证。
// 对应 TS: GeminiCliOAuthCredentials
type GeminiCliOAuthCredentials struct {
	// Access 访问令牌
	Access string `json:"access"`
	// Refresh 刷新令牌
	Refresh string `json:"refresh"`
	// Expires 过期时间戳（毫秒级 Unix 时间）
	Expires int64 `json:"expires"`
	// Email 用户邮箱（可选）
	Email string `json:"email,omitempty"`
	// ProjectID Google Cloud 项目 ID
	ProjectID string `json:"projectId"`
}

// GeminiCliOAuthContext Gemini CLI OAuth 上下文。
// 对应 TS: GeminiCliOAuthContext
type GeminiCliOAuthContext struct {
	// IsRemote 是否运行在远程环境
	IsRemote bool
	// OpenURL 在浏览器中打开 URL
	OpenURL func(url string) error
	// Log 输出日志信息
	Log func(msg string)
	// Note 显示通知消息
	Note func(message string, title string) error
	// Prompt 提示用户输入并返回用户输入
	Prompt func(message string) (string, error)
	// Progress 进度报告器
	Progress oauth.ProgressReporter
}

// oauthClientConfig OAuth 客户端配置。
type oauthClientConfig struct {
	ClientID     string
	ClientSecret string
}

// ──────────────────── 凭证缓存 ────────────────────

var (
	cachedCredentials *oauthClientConfig
	cacheMu           sync.Mutex
)

// ClearCredentialsCache 清除凭证缓存。
// 对应 TS: clearCredentialsCache()
func ClearCredentialsCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachedCredentials = nil
}

// ──────────────────── 环境变量解析 ────────────────────

// resolveEnv 从环境变量键列表中解析第一个有效值。
// 对应 TS: resolveEnv()
func resolveEnv(keys []string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

// ──────────────────── Gemini CLI 凭证提取 ────────────────────

// clientIDRegexp 匹配 Google OAuth 客户端 ID 的正则。
var clientIDRegexp = regexp.MustCompile(`(\d+-[a-z0-9]+\.apps\.googleusercontent\.com)`)

// clientSecretRegexp 匹配 Google OAuth 客户端 Secret 的正则。
var clientSecretRegexp = regexp.MustCompile(`(GOCSPX-[A-Za-z0-9_-]+)`)

// ExtractGeminiCliCredentials 从已安装的 Gemini CLI 中提取 OAuth 凭证。
// 对应 TS: extractGeminiCliCredentials()
func ExtractGeminiCliCredentials() *oauthClientConfig {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cachedCredentials != nil {
		return cachedCredentials
	}

	geminiPath := findInPath("gemini")
	if geminiPath == "" {
		return nil
	}

	resolvedPath, err := filepath.EvalSymlinks(geminiPath)
	if err != nil {
		return nil
	}

	geminiCliDirs := resolveGeminiCliDirs(geminiPath, resolvedPath)

	var content string
	for _, dir := range geminiCliDirs {
		searchPaths := []string{
			filepath.Join(dir, "node_modules", "@google", "gemini-cli-core", "dist", "src", "code_assist", "oauth2.js"),
			filepath.Join(dir, "node_modules", "@google", "gemini-cli-core", "dist", "code_assist", "oauth2.js"),
		}
		for _, p := range searchPaths {
			data, err := os.ReadFile(p)
			if err == nil {
				content = string(data)
				break
			}
		}
		if content != "" {
			break
		}
		// 递归搜索 oauth2.js
		found := findFile(dir, "oauth2.js", 10)
		if found != "" {
			data, err := os.ReadFile(found)
			if err == nil {
				content = string(data)
				break
			}
		}
	}

	if content == "" {
		return nil
	}

	idMatch := clientIDRegexp.FindString(content)
	secretMatch := clientSecretRegexp.FindString(content)
	if idMatch != "" && secretMatch != "" {
		cachedCredentials = &oauthClientConfig{ClientID: idMatch, ClientSecret: secretMatch}
		return cachedCredentials
	}
	return nil
}

// ──────────────────── 路径发现 ────────────────────

// resolveGeminiCliDirs 解析 Gemini CLI 可能的安装目录列表。
// 对应 TS: resolveGeminiCliDirs()
func resolveGeminiCliDirs(geminiPath, resolvedPath string) []string {
	binDir := filepath.Dir(geminiPath)
	candidates := []string{
		filepath.Dir(filepath.Dir(resolvedPath)),
		filepath.Join(filepath.Dir(resolvedPath), "node_modules", "@google", "gemini-cli"),
		filepath.Join(binDir, "node_modules", "@google", "gemini-cli"),
		filepath.Join(filepath.Dir(binDir), "node_modules", "@google", "gemini-cli"),
		filepath.Join(filepath.Dir(binDir), "lib", "node_modules", "@google", "gemini-cli"),
	}

	seen := make(map[string]bool)
	var deduped []string
	for _, c := range candidates {
		key := c
		if runtime.GOOS == "windows" {
			key = strings.ToLower(strings.ReplaceAll(c, "\\", "/"))
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, c)
	}
	return deduped
}

// findInPath 在 PATH 环境变量中查找可执行文件。
// 对应 TS: findInPath()
func findInPath(name string) string {
	exts := []string{""}
	if runtime.GOOS == "windows" {
		exts = []string{".cmd", ".bat", ".exe", ""}
	}
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return ""
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		for _, ext := range exts {
			p := filepath.Join(dir, name+ext)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// findFile 在目录中递归查找指定名称的文件。
// 对应 TS: findFile()
func findFile(dir, name string, depth int) string {
	if depth <= 0 {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if !e.IsDir() && e.Name() == name {
			return p
		}
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			found := findFile(p, name, depth-1)
			if found != "" {
				return found
			}
		}
	}
	return ""
}

// ──────────────────── OAuth 配置解析 ────────────────────

// resolveOAuthClientConfig 解析 OAuth 客户端配置。
// 优先级: 1. 环境变量  2. 从已安装的 Gemini CLI 提取  3. 返回错误
// 对应 TS: resolveOAuthClientConfig()
func resolveOAuthClientConfig() (*oauthClientConfig, error) {
	// 1. 检查环境变量
	envClientID := resolveEnv(clientIDKeys)
	envClientSecret := resolveEnv(clientSecretKeys)
	if envClientID != "" {
		return &oauthClientConfig{ClientID: envClientID, ClientSecret: envClientSecret}, nil
	}

	// 2. 尝试从已安装的 Gemini CLI 提取
	extracted := ExtractGeminiCliCredentials()
	if extracted != nil {
		return extracted, nil
	}

	// 3. 无可用凭证
	return nil, fmt.Errorf("Gemini CLI not found. Install it first: brew install gemini-cli (or npm install -g @google/gemini-cli), or set GEMINI_CLI_OAUTH_CLIENT_ID")
}

// shouldUseManualOAuthFlow 判断是否需要手动 OAuth 流程。
// 对应 TS: shouldUseManualOAuthFlow()
func shouldUseManualOAuthFlow(isRemote bool) bool {
	return isRemote || oauth.IsWSL2()
}

// ──────────────────── PKCE 生成 ────────────────────

// generatePkce 生成 PKCE 验证器和挑战码。
// 对应 TS: generatePkce()
func generatePkce() (verifier string, challenge string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier = fmt.Sprintf("%x", b)
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	return
}

// ──────────────────── 授权 URL 构建 ────────────────────

// buildAuthURL 构建 Google OAuth 授权 URL。
// 对应 TS: buildAuthUrl()
func buildAuthURL(challenge, verifier string) (string, error) {
	config, err := resolveOAuthClientConfig()
	if err != nil {
		return "", err
	}
	params := url.Values{
		"client_id":             {config.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {strings.Join(scopes, " ")},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {verifier},
		"access_type":           {"offline"},
		"prompt":                {"consent"},
	}
	return authURL + "?" + params.Encode(), nil
}

// ──────────────────── 回调解析 ────────────────────

// callbackResult 回调解析结果。
type callbackResult struct {
	Code  string
	State string
	Error string
}

// parseCallbackInput 解析回调输入（URL 或纯 code）。
// 对应 TS: parseCallbackInput()
func parseCallbackInput(input, expectedState string) callbackResult {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return callbackResult{Error: "No input provided"}
	}

	u, err := url.Parse(trimmed)
	if err == nil && u.Scheme != "" {
		code := u.Query().Get("code")
		state := u.Query().Get("state")
		if state == "" {
			state = expectedState
		}
		if code == "" {
			return callbackResult{Error: "Missing 'code' parameter in URL"}
		}
		if state == "" {
			return callbackResult{Error: "Missing 'state' parameter. Paste the full URL."}
		}
		return callbackResult{Code: code, State: state}
	}

	if expectedState == "" {
		return callbackResult{Error: "Paste the full redirect URL, not just the code."}
	}
	return callbackResult{Code: trimmed, State: expectedState}
}

// ──────────────────── 本地回调服务器 ────────────────────

// waitForLocalCallback 启动本地 HTTP 服务器等待 OAuth 回调。
// 对应 TS: waitForLocalCallback()
func waitForLocalCallback(expectedState string, timeoutMs int, onProgress func(string)) (code string, state string, err error) {
	const port = "8085"
	const hostname = "localhost"
	const expectedPath = "/oauth2callback"

	type result struct {
		code  string
		state string
		err   error
	}
	ch := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(expectedPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		errParam := q.Get("error")
		codeParam := strings.TrimSpace(q.Get("code"))
		stateParam := strings.TrimSpace(q.Get("state"))

		if errParam != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "Authentication failed: %s", errParam)
			ch <- result{err: fmt.Errorf("OAuth error: %s", errParam)}
			return
		}

		if codeParam == "" || stateParam == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "Missing code or state")
			ch <- result{err: fmt.Errorf("缺少 OAuth code 或 state 参数")}
			return
		}

		if stateParam != expectedState {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "Invalid state")
			ch <- result{err: fmt.Errorf("OAuth state 不匹配")}
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w,
			"<!doctype html><html><head><meta charset='utf-8'/></head>"+
				"<body><h2>Gemini CLI OAuth complete</h2>"+
				"<p>You can close this window and return to OpenClaw.</p></body></html>")
		ch <- result{code: codeParam, state: stateParam}
	})

	listener, listenErr := net.Listen("tcp", hostname+":"+port)
	if listenErr != nil {
		return "", "", fmt.Errorf("启动 OAuth 回调服务器失败: %w", listenErr)
	}

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()

	if onProgress != nil {
		onProgress(fmt.Sprintf("Waiting for OAuth callback on %s…", redirectURI))
	}

	timer := time.NewTimer(time.Duration(timeoutMs) * time.Millisecond)
	defer timer.Stop()

	select {
	case r := <-ch:
		_ = server.Close()
		return r.code, r.state, r.err
	case <-timer.C:
		_ = server.Close()
		return "", "", fmt.Errorf("OAuth 回调超时")
	}
}

// ──────────────────── Token 交换 ────────────────────

// exchangeCodeForTokens 用授权码交换 Token。
// 对应 TS: exchangeCodeForTokens()
func exchangeCodeForTokens(code, verifier string) (*GeminiCliOAuthCredentials, error) {
	config, err := resolveOAuthClientConfig()
	if err != nil {
		return nil, err
	}

	data := url.Values{
		"client_id":     {config.ClientID},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	req, _ := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "google-api-nodejs-client/9.15.1")

	client := oauth.NewHTTPClientWithTimeout(defaultFetchTimeoutMs)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Token 交换请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Token 交换失败: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 Token 响应失败: %w", err)
	}

	if tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("No refresh token received. Please try again.")
	}

	email := getUserEmail(tokenResp.AccessToken)
	projectID, err := discoverProject(tokenResp.AccessToken)
	if err != nil {
		return nil, err
	}
	// 过期时间 = 当前时间 + expires_in秒 - 5分钟缓冲
	expiresAt := time.Now().UnixMilli() + tokenResp.ExpiresIn*1000 - 5*60*1000

	return &GeminiCliOAuthCredentials{
		Refresh:   tokenResp.RefreshToken,
		Access:    tokenResp.AccessToken,
		Expires:   expiresAt,
		ProjectID: projectID,
		Email:     email,
	}, nil
}

// ──────────────────── 用户信息获取 ────────────────────

// getUserEmail 获取 OAuth 用户的邮箱地址。
// 对应 TS: getUserEmail()
func getUserEmail(accessToken string) string {
	req, _ := http.NewRequest("GET", userinfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := oauth.NewHTTPClientWithTimeout(defaultFetchTimeoutMs)
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var info struct {
		Email string `json:"email"`
	}
	body, _ := io.ReadAll(resp.Body)
	if json.Unmarshal(body, &info) == nil {
		return info.Email
	}
	return ""
}

// ──────────────────── 项目发现 ────────────────────

// discoverProject 发现或创建 Google Cloud 项目。
// 优先级: 环境变量 > loadCodeAssist API > onboardUser API
// 对应 TS: discoverProject()
func discoverProject(accessToken string) (string, error) {
	envProject := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if envProject == "" {
		envProject = os.Getenv("GOOGLE_CLOUD_PROJECT_ID")
	}
	platform := oauth.ResolvePlatform()
	metadata := map[string]interface{}{
		"ideType":    "ANTIGRAVITY",
		"platform":   platform,
		"pluginType": "GEMINI",
	}
	metadataJSON, _ := json.Marshal(metadata)
	headers := map[string]string{
		"Authorization":     "Bearer " + accessToken,
		"Content-Type":      "application/json",
		"User-Agent":        "google-api-nodejs-client/9.15.1",
		"X-Goog-Api-Client": fmt.Sprintf("gl-go/%s", runtime.Version()),
		"Client-Metadata":   string(metadataJSON),
	}

	// 构建 loadCodeAssist 请求体
	loadBody := map[string]interface{}{
		"metadata": metadata,
	}
	if envProject != "" {
		loadBody["cloudaicompanionProject"] = envProject
		loadBody["metadata"].(map[string]interface{})["duetProject"] = envProject
	}
	loadBodyJSON, _ := json.Marshal(loadBody)

	// 尝试 loadCodeAssist 端点
	var loadData map[string]interface{}
	var activeEndpoint = codeAssistEndpointProd
	var loadErr error

	client := oauth.NewHTTPClientWithTimeout(defaultFetchTimeoutMs)
	for _, endpoint := range loadCodeAssistEndpoints {
		req, _ := http.NewRequest("POST", endpoint+"/v1internal:loadCodeAssist", strings.NewReader(string(loadBodyJSON)))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			loadErr = fmt.Errorf("loadCodeAssist 失败: %w", err)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errorPayload interface{}
			_ = json.Unmarshal(respBody, &errorPayload)
			if isVpcScAffected(errorPayload) {
				loadData = map[string]interface{}{
					"currentTier": map[string]interface{}{"id": tierStandard},
				}
				activeEndpoint = endpoint
				loadErr = nil
				break
			}
			loadErr = fmt.Errorf("loadCodeAssist 失败: %d %s", resp.StatusCode, resp.Status)
			continue
		}

		_ = json.Unmarshal(respBody, &loadData)
		activeEndpoint = endpoint
		loadErr = nil
		break
	}

	// 检查 loadCodeAssist 是否返回有效数据
	hasData := loadData["currentTier"] != nil || loadData["cloudaicompanionProject"] != nil
	if !hasData {
		if tiers, ok := loadData["allowedTiers"].([]interface{}); ok && len(tiers) > 0 {
			hasData = true
		}
	}
	if !hasData && loadErr != nil {
		if envProject != "" {
			return envProject, nil
		}
		return "", loadErr
	}

	// 已有 currentTier，直接提取项目 ID
	if loadData["currentTier"] != nil {
		project := loadData["cloudaicompanionProject"]
		if s, ok := project.(string); ok && s != "" {
			return s, nil
		}
		if m, ok := project.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok && id != "" {
				return id, nil
			}
		}
		if envProject != "" {
			return envProject, nil
		}
		return "", fmt.Errorf("This account requires GOOGLE_CLOUD_PROJECT or GOOGLE_CLOUD_PROJECT_ID to be set.")
	}

	// 需要 onboard — 获取默认 tier
	tier := getDefaultTier(loadData)
	tierId := tierFree
	if tier != "" {
		tierId = tier
	}
	if tierId != tierFree && envProject == "" {
		return "", fmt.Errorf("This account requires GOOGLE_CLOUD_PROJECT or GOOGLE_CLOUD_PROJECT_ID to be set.")
	}

	// 构建 onboardUser 请求体
	onboardBody := map[string]interface{}{
		"tierId":   tierId,
		"metadata": metadata,
	}
	if tierId != tierFree && envProject != "" {
		onboardBody["cloudaicompanionProject"] = envProject
		onboardBody["metadata"].(map[string]interface{})["duetProject"] = envProject
	}
	onboardBodyJSON, _ := json.Marshal(onboardBody)

	req, _ := http.NewRequest("POST", activeEndpoint+"/v1internal:onboardUser", strings.NewReader(string(onboardBodyJSON)))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("onboardUser 请求失败: %w", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("onboardUser 失败: %d %s", resp.StatusCode, resp.Status)
	}

	var lro map[string]interface{}
	_ = json.Unmarshal(respBody, &lro)

	// 检查 LRO 是否完成
	if done, _ := lro["done"].(bool); !done {
		if name, ok := lro["name"].(string); ok && name != "" {
			lro, err = pollOperation(activeEndpoint, name, headers, client)
			if err != nil {
				if envProject != "" {
					return envProject, nil
				}
				return "", err
			}
		}
	}

	// 提取项目 ID
	if response, ok := lro["response"].(map[string]interface{}); ok {
		if project, ok := response["cloudaicompanionProject"].(map[string]interface{}); ok {
			if id, ok := project["id"].(string); ok && id != "" {
				return id, nil
			}
		}
	}
	if envProject != "" {
		return envProject, nil
	}
	return "", fmt.Errorf("Could not discover or provision a Google Cloud project. Set GOOGLE_CLOUD_PROJECT or GOOGLE_CLOUD_PROJECT_ID.")
}

// isVpcScAffected 检查错误响应是否由 VPC Service Controls 导致。
// 对应 TS: isVpcScAffected()
func isVpcScAffected(payload interface{}) bool {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return false
	}
	errorObj, ok := m["error"].(map[string]interface{})
	if !ok {
		return false
	}
	details, ok := errorObj["details"].([]interface{})
	if !ok {
		return false
	}
	for _, item := range details {
		if detail, ok := item.(map[string]interface{}); ok {
			if reason, _ := detail["reason"].(string); reason == "SECURITY_POLICY_VIOLATED" {
				return true
			}
		}
	}
	return false
}

// getDefaultTier 获取默认 tier ID。
// 对应 TS: getDefaultTier()
func getDefaultTier(data map[string]interface{}) string {
	tiers, ok := data["allowedTiers"].([]interface{})
	if !ok || len(tiers) == 0 {
		return tierLegacy
	}
	for _, t := range tiers {
		tier, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		if isDefault, _ := tier["isDefault"].(bool); isDefault {
			if id, ok := tier["id"].(string); ok {
				return id
			}
		}
	}
	return tierLegacy
}

// pollOperation 轮询 Google Cloud LRO 操作直到完成。
// 对应 TS: pollOperation()
func pollOperation(endpoint, operationName string, headers map[string]string, client *http.Client) (map[string]interface{}, error) {
	for attempt := 0; attempt < 24; attempt++ {
		time.Sleep(5 * time.Second)
		req, _ := http.NewRequest("GET", endpoint+"/v1internal/"+operationName, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var data map[string]interface{}
		if json.Unmarshal(body, &data) != nil {
			continue
		}
		if done, _ := data["done"].(bool); done {
			return data, nil
		}
	}
	return nil, fmt.Errorf("Operation polling timeout")
}

// ──────────────────── 主入口 ────────────────────

// LoginGeminiCliOAuth 执行 Gemini CLI OAuth 认证完整流程。
// 对应 TS: loginGeminiCliOAuth()
func LoginGeminiCliOAuth(ctx *GeminiCliOAuthContext) (*GeminiCliOAuthCredentials, error) {
	needsManual := shouldUseManualOAuthFlow(ctx.IsRemote)

	if needsManual {
		_ = ctx.Note(
			strings.Join([]string{
				"You are running in a remote/VPS environment.",
				"A URL will be shown for you to open in your LOCAL browser.",
				"After signing in, copy the redirect URL and paste it back here.",
			}, "\n"),
			"Gemini CLI OAuth",
		)
	} else {
		_ = ctx.Note(
			strings.Join([]string{
				"Browser will open for Google authentication.",
				"Sign in with your Google account for Gemini CLI access.",
				"The callback will be captured automatically on localhost:8085.",
			}, "\n"),
			"Gemini CLI OAuth",
		)
	}

	verifier, challenge := generatePkce()
	authURLStr, err := buildAuthURL(challenge, verifier)
	if err != nil {
		return nil, err
	}

	if needsManual {
		ctx.Progress.Update("OAuth URL ready")
		ctx.Log(fmt.Sprintf("\nOpen this URL in your LOCAL browser:\n\n%s\n", authURLStr))
		ctx.Progress.Update("Waiting for you to paste the callback URL...")
		callbackInput, err := ctx.Prompt("Paste the redirect URL here: ")
		if err != nil {
			return nil, fmt.Errorf("读取用户输入失败: %w", err)
		}
		parsed := parseCallbackInput(callbackInput, verifier)
		if parsed.Error != "" {
			return nil, fmt.Errorf("%s", parsed.Error)
		}
		if parsed.State != verifier {
			return nil, fmt.Errorf("OAuth state mismatch - please try again")
		}
		ctx.Progress.Update("Exchanging authorization code for tokens...")
		return exchangeCodeForTokens(parsed.Code, verifier)
	}

	// 本地模式：打开浏览器 + 启动回调服务器
	ctx.Progress.Update("Complete sign-in in browser...")
	if err := ctx.OpenURL(authURLStr); err != nil {
		ctx.Log(fmt.Sprintf("\nOpen this URL in your browser:\n\n%s\n", authURLStr))
	}

	code, _, cbErr := waitForLocalCallback(
		verifier,
		5*60*1000, // 5分钟超时
		func(msg string) { ctx.Progress.Update(msg) },
	)

	if cbErr != nil {
		// 如果是端口占用等本地服务器错误，切换到手动模式
		errMsg := cbErr.Error()
		if strings.Contains(errMsg, "EADDRINUSE") ||
			strings.Contains(errMsg, "address already in use") ||
			strings.Contains(errMsg, "bind") ||
			strings.Contains(errMsg, "listen") {
			ctx.Progress.Update("Local callback server failed. Switching to manual mode...")
			ctx.Log(fmt.Sprintf("\nOpen this URL in your LOCAL browser:\n\n%s\n", authURLStr))
			callbackInput, err := ctx.Prompt("Paste the redirect URL here: ")
			if err != nil {
				return nil, fmt.Errorf("读取用户输入失败: %w", err)
			}
			parsed := parseCallbackInput(callbackInput, verifier)
			if parsed.Error != "" {
				return nil, fmt.Errorf("%s", parsed.Error)
			}
			if parsed.State != verifier {
				return nil, fmt.Errorf("OAuth state mismatch - please try again")
			}
			ctx.Progress.Update("Exchanging authorization code for tokens...")
			return exchangeCodeForTokens(parsed.Code, verifier)
		}
		return nil, cbErr
	}

	ctx.Progress.Update("Exchanging authorization code for tokens...")
	return exchangeCodeForTokens(code, verifier)
}
