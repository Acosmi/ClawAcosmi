package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/channels/ratelimit"
)

// Slack Web Client — 继承自 src/slack/client.ts (20L)
// 原版使用 @slack/web-api 的 WebClient。Go 端使用原生 HTTP 封装。

// slackAPIBaseURL Slack API 基础 URL（var 以便测试注入）
var slackAPIBaseURL = "https://slack.com/api/"

// setSlackAPIBaseURL 设置 Slack API 基础 URL（仅测试用）。
func setSlackAPIBaseURL(url string) {
	slackAPIBaseURL = url
}

// SlackRetryOptions 重试策略配置
type SlackRetryOptions struct {
	Retries    int
	Factor     float64
	MinTimeout time.Duration
	MaxTimeout time.Duration
	Randomize  bool
}

// DefaultSlackRetryOptions 默认重试参数（等同 TS 端 SLACK_DEFAULT_RETRY_OPTIONS）
var DefaultSlackRetryOptions = SlackRetryOptions{
	Retries:    2,
	Factor:     2,
	MinTimeout: 500 * time.Millisecond,
	MaxTimeout: 3 * time.Second,
	Randomize:  true,
}

// SlackWebClient 封装 Slack Web API 调用，内置重试逻辑。
type SlackWebClient struct {
	token      string
	httpClient *http.Client
	retry      SlackRetryOptions
	limiter    *ratelimit.ChannelLimiter // W3-D1: 令牌桶限速
}

// NewSlackWebClient 创建 Slack Web API 客户端。
func NewSlackWebClient(token string) *SlackWebClient {
	return &SlackWebClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retry:   DefaultSlackRetryOptions,
		limiter: ratelimit.GetSlackLimiter(token), // W3-D1: 按 token 分隔限速
	}
}

// NewSlackWebClientWithOptions 创建带自定义选项的 Slack Web API 客户端。
func NewSlackWebClientWithOptions(token string, retry SlackRetryOptions) *SlackWebClient {
	return &SlackWebClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retry:   retry,
		limiter: ratelimit.GetSlackLimiter(token), // W3-D1: 按 token 分隔限速
	}
}

// SlackAPIResponse Slack API 通用响应
type SlackAPIResponse struct {
	OK       bool            `json:"ok"`
	Error    string          `json:"error,omitempty"`
	Warning  string          `json:"warning,omitempty"`
	RawJSON  json.RawMessage `json:"-"`
	Metadata json.RawMessage `json:"response_metadata,omitempty"`
}

// APICall 调用 Slack Web API 方法并返回原始 JSON 响应。
func (c *SlackWebClient) APICall(ctx context.Context, method string, params map[string]interface{}) (json.RawMessage, error) {
	// W3-D1: 速率限制
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("slack rate limit wait: %w", err)
		}
	}

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("slack api marshal params: %w", err)
	}

	url := slackAPIBaseURL + method

	var lastErr error
	for attempt := 0; attempt <= c.retry.Retries; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("slack api create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Authorization", "Bearer "+c.token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("slack api %s: %w", method, err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("slack api %s read body: %w", method, err)
			continue
		}

		// 429 Too Many Requests → 重试
		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("slack api %s: rate limited (429)", method)
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("slack api %s: server error %d", method, resp.StatusCode)
			continue
		}

		var apiResp SlackAPIResponse
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			return nil, fmt.Errorf("slack api %s unmarshal: %w", method, err)
		}
		if !apiResp.OK {
			return nil, fmt.Errorf("slack api %s: %s", method, apiResp.Error)
		}

		return respBody, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("slack api %s: unknown error", method)
}

// APICallResult 调用 Slack Web API 并反序列化到目标结构体。
func (c *SlackWebClient) APICallResult(ctx context.Context, method string, params map[string]interface{}, result interface{}) error {
	raw, err := c.APICall(ctx, method, params)
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(raw, result)
	}
	return nil
}

// AuthTestResponse auth.test 响应
type AuthTestResponse struct {
	OK       bool   `json:"ok"`
	UserID   string `json:"user_id,omitempty"`
	TeamID   string `json:"team_id,omitempty"`
	APIAppID string `json:"api_app_id,omitempty"`
	BotID    string `json:"bot_id,omitempty"`
}

// AuthTest 调用 auth.test 获取 bot 信息。
func (c *SlackWebClient) AuthTest(ctx context.Context) (*AuthTestResponse, error) {
	var resp AuthTestResponse
	if err := c.APICallResult(ctx, "auth.test", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PostMessageParams chat.postMessage 参数
type PostMessageParams struct {
	Channel  string `json:"channel"`
	Text     string `json:"text,omitempty"`
	ThreadTs string `json:"thread_ts,omitempty"`
	Blocks   []any  `json:"blocks,omitempty"`
}

// PostMessageResponse chat.postMessage 响应
type PostMessageResponse struct {
	OK      bool   `json:"ok"`
	Channel string `json:"channel,omitempty"`
	Ts      string `json:"ts,omitempty"`
}

// PostMessage 发送消息。
func (c *SlackWebClient) PostMessage(ctx context.Context, params PostMessageParams) (*PostMessageResponse, error) {
	p := map[string]interface{}{
		"channel": params.Channel,
	}
	if params.Text != "" {
		p["text"] = params.Text
	}
	if params.ThreadTs != "" {
		p["thread_ts"] = params.ThreadTs
	}
	if len(params.Blocks) > 0 {
		p["blocks"] = params.Blocks
	}
	var resp PostMessageResponse
	if err := c.APICallResult(ctx, "chat.postMessage", p, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UploadFileParams files.uploadV2 参数
type UploadFileParams struct {
	Channel        string
	ThreadTs       string
	Filename       string
	Content        io.Reader
	InitialComment string
}

// uploadURLResponse files.getUploadURLExternal 响应
type uploadURLResponse struct {
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	UploadURL string `json:"upload_url"`
	FileID    string `json:"file_id"`
}

// UploadFileV2 上传文件到 Slack（files.uploadV2 三阶段 API）。
//
// 流程:
//  1. files.getUploadURLExternal — 获取预签名上传 URL + file_id
//  2. PUT raw bytes 到上传 URL
//  3. files.completeUploadExternal — 完成上传并分享到频道
func (c *SlackWebClient) UploadFileV2(ctx context.Context, params UploadFileParams) error {
	// 预读 Content 到 buffer 以获取字节长度（getUploadURLExternal 需要 length 参数）
	data, err := io.ReadAll(params.Content)
	if err != nil {
		return fmt.Errorf("slack uploadV2 read content: %w", err)
	}

	filename := params.Filename
	if filename == "" {
		filename = "file"
	}

	// --- Step 1: files.getUploadURLExternal ---
	step1Params := map[string]interface{}{
		"filename": filename,
		"length":   len(data),
	}
	raw, err := c.APICall(ctx, "files.getUploadURLExternal", step1Params)
	if err != nil {
		return fmt.Errorf("slack uploadV2 getUploadURL: %w", err)
	}
	var urlResp uploadURLResponse
	if err := json.Unmarshal(raw, &urlResp); err != nil {
		return fmt.Errorf("slack uploadV2 getUploadURL unmarshal: %w", err)
	}
	if urlResp.UploadURL == "" || urlResp.FileID == "" {
		return fmt.Errorf("slack uploadV2 getUploadURL: empty upload_url or file_id")
	}

	// --- Step 2: PUT raw bytes to upload URL ---
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPost, urlResp.UploadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("slack uploadV2 put request: %w", err)
	}
	putReq.Header.Set("Content-Type", "application/octet-stream")

	putResp, err := c.httpClient.Do(putReq)
	if err != nil {
		return fmt.Errorf("slack uploadV2 put: %w", err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack uploadV2 put: unexpected status %d", putResp.StatusCode)
	}

	// --- Step 3: files.completeUploadExternal ---
	fileEntry := map[string]interface{}{
		"id":    urlResp.FileID,
		"title": filename,
	}
	step3Params := map[string]interface{}{
		"files": []interface{}{fileEntry},
	}
	if params.Channel != "" {
		step3Params["channel_id"] = params.Channel
	}
	if params.ThreadTs != "" {
		step3Params["thread_ts"] = params.ThreadTs
	}
	if params.InitialComment != "" {
		step3Params["initial_comment"] = params.InitialComment
	}

	if _, err := c.APICall(ctx, "files.completeUploadExternal", step3Params); err != nil {
		return fmt.Errorf("slack uploadV2 completeUpload: %w", err)
	}

	return nil
}

// retryDelay 计算指数退避延迟。
func (c *SlackWebClient) retryDelay(attempt int) time.Duration {
	delay := float64(c.retry.MinTimeout) * math.Pow(c.retry.Factor, float64(attempt-1))
	if delay > float64(c.retry.MaxTimeout) {
		delay = float64(c.retry.MaxTimeout)
	}
	if c.retry.Randomize {
		delay = delay * (0.5 + rand.Float64()*0.5) //nolint:gosec // 重试抖动无需密码学安全
	}
	return time.Duration(delay)
}

// Token 返回当前客户端使用的 token。
func (c *SlackWebClient) Token() string {
	return c.token
}

// ResolveSlackWebClientOptions 解析客户端选项（兼容 TS 端接口）。
func ResolveSlackWebClientOptions() SlackRetryOptions {
	return DefaultSlackRetryOptions
}

// CreateSlackWebClient 创建 Slack Web API 客户端（便捷函数）。
func CreateSlackWebClient(token string) *SlackWebClient {
	return NewSlackWebClient(token)
}

// stripPrefix 去除字符串前缀（大小写不敏感）。
func stripPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return s, false
}
