package wecom

// client.go — 企业微信 HTTP API 客户端
// 直接 HTTP API 调用（与 Telegram/Feishu remote_approval 一致的模式）
// 封装 access_token 获取 + 自动缓存刷新

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	// WeComBaseURL 企业微信 API 基础 URL
	WeComBaseURL = "https://qyapi.weixin.qq.com"
)

// WeComClient 企业微信 API 客户端
type WeComClient struct {
	CorpID  string
	Secret  string
	AgentID int
	Token   string // 回调验证 token
	AESKey  string // 消息加密 key
	Client  *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// NewWeComClient 创建企业微信客户端
func NewWeComClient(acct *ResolvedWeComAccount) *WeComClient {
	agentID := 0
	if acct.Config.AgentID != nil {
		agentID = *acct.Config.AgentID
	}

	slog.Info("wecom client created",
		"corp_id", acct.Config.CorpID,
		"agent_id", agentID,
	)

	return &WeComClient{
		CorpID:  acct.Config.CorpID,
		Secret:  acct.Config.Secret,
		AgentID: agentID,
		Token:   acct.Config.Token,
		AESKey:  acct.Config.AESKey,
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// GetAccessToken 获取 access_token（带自动缓存，7200 秒有效，提前 300 秒刷新）
func (c *WeComClient) GetAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	url := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s", WeComBaseURL, c.CorpID, c.Secret)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request access_token: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("wecom token API error: code=%d, msg=%s", result.ErrCode, result.ErrMsg)
	}

	c.cachedToken = result.AccessToken
	expire := result.ExpiresIn
	if expire <= 0 {
		expire = 7200
	}
	c.tokenExpiry = time.Now().Add(time.Duration(expire-300) * time.Second)
	return c.cachedToken, nil
}

// DoAPIRequest 执行带 access_token 的 API 请求
func (c *WeComClient) DoAPIRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	token, err := c.GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	url := WeComBaseURL + path + "?access_token=" + token
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wecom API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("wecom API HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}
