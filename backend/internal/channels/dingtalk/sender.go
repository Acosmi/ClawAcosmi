package dingtalk

// sender.go — 钉钉消息发送
// 使用 dingtalk-stream-sdk-go 的 reply 接口 + 直接 HTTP API 发送

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
	// DingTalkOAuthURL 获取 access_token
	DingTalkOAuthURL = "https://api.dingtalk.com/v1.0/oauth2/accessToken"
	// DingTalkRobotSendURL 机器人消息发送
	DingTalkRobotSendURL = "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend"
	// DingTalkGroupSendURL 群消息发送
	DingTalkGroupSendURL = "https://api.dingtalk.com/v1.0/robot/groupMessages/send"
)

// DingTalkSender 钉钉消息发送器
type DingTalkSender struct {
	appKey    string
	appSecret string
	robotCode string
	client    *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// NewDingTalkSender 创建消息发送器
func NewDingTalkSender(appKey, appSecret, robotCode string) *DingTalkSender {
	return &DingTalkSender{
		appKey:    appKey,
		appSecret: appSecret,
		robotCode: robotCode,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

// getAccessToken 获取 access_token（带缓存）
func (s *DingTalkSender) getAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedToken != "" && time.Now().Before(s.tokenExpiry) {
		return s.cachedToken, nil
	}

	body, _ := json.Marshal(map[string]string{
		"appKey":    s.appKey,
		"appSecret": s.appSecret,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", DingTalkOAuthURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request access_token: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("dingtalk token API error: %s", string(respBody))
	}

	s.cachedToken = result.AccessToken
	expire := result.ExpireIn
	if expire <= 0 {
		expire = 7200
	}
	s.tokenExpiry = time.Now().Add(time.Duration(expire-300) * time.Second)
	return s.cachedToken, nil
}

// SendOToMessage 发送单聊消息
func (s *DingTalkSender) SendOToMessage(ctx context.Context, userIDs []string, text string) error {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]interface{}{
		"robotCode": s.robotCode,
		"userIds":   userIDs,
		"msgKey":    "sampleText",
		"msgParam":  fmt.Sprintf(`{"content":"%s"}`, escapeJSON(text)),
	})

	req, err := http.NewRequestWithContext(ctx, "POST", DingTalkRobotSendURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create send request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", token)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send dingtalk message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dingtalk send HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Debug("dingtalk O2O message sent", "user_ids", userIDs)
	return nil
}

// SendGroupMessage 发送群聊消息
func (s *DingTalkSender) SendGroupMessage(ctx context.Context, openConversationID, text string) error {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]interface{}{
		"robotCode":          s.robotCode,
		"openConversationId": openConversationID,
		"msgKey":             "sampleText",
		"msgParam":           fmt.Sprintf(`{"content":"%s"}`, escapeJSON(text)),
	})

	req, err := http.NewRequestWithContext(ctx, "POST", DingTalkGroupSendURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create group send request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", token)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send dingtalk group message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dingtalk group send HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Debug("dingtalk group message sent", "conversation_id", openConversationID)
	return nil
}

// escapeJSON 转义 JSON 字符串
func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}
