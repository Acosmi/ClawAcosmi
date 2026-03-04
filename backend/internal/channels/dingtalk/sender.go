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
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
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
	// DingTalkMediaUploadURL 上传媒体文件
	DingTalkMediaUploadURL = "https://oapi.dingtalk.com/media/upload"
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
	return s.sendOToTemplate(ctx, userIDs, "sampleText", map[string]string{"content": text})
}

// SendOToImage 发送单聊图片消息（支持公网 URL 或 media_id）。
func (s *DingTalkSender) SendOToImage(ctx context.Context, userIDs []string, photoURL string) error {
	return s.sendOToTemplate(ctx, userIDs, "sampleImageMsg", buildDingTalkImageMsgParam(photoURL))
}

// SendOToFile 发送单聊文件消息。
func (s *DingTalkSender) SendOToFile(ctx context.Context, userIDs []string, mediaID, fileName, fileType string) error {
	return s.sendOToTemplate(ctx, userIDs, "sampleFile", map[string]string{
		"mediaId":  mediaID,
		"fileName": fileName,
		"fileType": fileType,
	})
}

func (s *DingTalkSender) sendOToTemplate(ctx context.Context, userIDs []string, msgKey string, msgParam map[string]string) error {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]interface{}{
		"robotCode": s.robotCode,
		"userIds":   userIDs,
		"msgKey":    msgKey,
		"msgParam":  marshalMsgParam(msgParam),
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

	slog.Debug("dingtalk O2O message sent", "user_ids", userIDs, "msg_key", msgKey)
	return nil
}

// SendGroupMessage 发送群聊消息
func (s *DingTalkSender) SendGroupMessage(ctx context.Context, openConversationID, text string) error {
	return s.sendGroupTemplate(ctx, openConversationID, "sampleText", map[string]string{"content": text})
}

// SendGroupImage 发送群聊图片消息（支持公网 URL 或 media_id）。
func (s *DingTalkSender) SendGroupImage(ctx context.Context, openConversationID, photoURL string) error {
	return s.sendGroupTemplate(ctx, openConversationID, "sampleImageMsg", buildDingTalkImageMsgParam(photoURL))
}

// SendGroupFile 发送群聊文件消息。
func (s *DingTalkSender) SendGroupFile(ctx context.Context, openConversationID, mediaID, fileName, fileType string) error {
	return s.sendGroupTemplate(ctx, openConversationID, "sampleFile", map[string]string{
		"mediaId":  mediaID,
		"fileName": fileName,
		"fileType": fileType,
	})
}

func (s *DingTalkSender) sendGroupTemplate(ctx context.Context, openConversationID, msgKey string, msgParam map[string]string) error {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]interface{}{
		"robotCode":          s.robotCode,
		"openConversationId": openConversationID,
		"msgKey":             msgKey,
		"msgParam":           marshalMsgParam(msgParam),
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

	slog.Debug("dingtalk group message sent", "conversation_id", openConversationID, "msg_key", msgKey)
	return nil
}

// UploadMedia 上传媒体文件，返回 media_id。
// mediaType 支持 image|voice|file；这里主要用于 image/file。
func (s *DingTalkSender) UploadMedia(ctx context.Context, mediaType, fileName string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("dingtalk upload media: empty payload")
	}
	uploadType := strings.ToLower(strings.TrimSpace(mediaType))
	if uploadType == "" {
		uploadType = "file"
	}
	if fileName == "" {
		fileName = "upload.bin"
	}
	if filepath.Ext(fileName) == "" {
		fileName += ".bin"
	}

	token, err := s.getAccessToken(ctx)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", filepath.Base(fileName))
	if err != nil {
		return "", fmt.Errorf("dingtalk upload media: create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("dingtalk upload media: write payload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("dingtalk upload media: close form: %w", err)
	}

	uploadURL := fmt.Sprintf("%s?access_token=%s&type=%s",
		DingTalkMediaUploadURL, url.QueryEscape(token), url.QueryEscape(uploadType))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", fmt.Errorf("dingtalk upload media: create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("dingtalk upload media: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("dingtalk upload media HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("dingtalk upload media: decode response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("dingtalk upload media error: code=%d msg=%s", result.ErrCode, result.ErrMsg)
	}
	if strings.TrimSpace(result.MediaID) == "" {
		return "", fmt.Errorf("dingtalk upload media: empty media_id")
	}
	return strings.TrimSpace(result.MediaID), nil
}

// escapeJSON 转义 JSON 字符串
func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}

func marshalMsgParam(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// buildDingTalkImageMsgParam 构建钉钉图片模板参数。
// 兼容两种输入：
//   - URL：photoURL
//   - 上传返回的 media_id：mediaId
func buildDingTalkImageMsgParam(value string) map[string]string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return map[string]string{"photoURL": ""}
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return map[string]string{"photoURL": trimmed}
	}
	return map[string]string{"mediaId": trimmed}
}
