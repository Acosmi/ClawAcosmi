package website

// ============================================================================
// website/rest_client.go — 通用 REST 网站发布器
// 实现 media.MediaPublisher 接口，支持多种认证方式和重试机制。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P4-1
// ============================================================================

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/media"
)

// ---------- WebsiteClient ----------

// WebsiteClient 通用 REST 网站发布客户端。
type WebsiteClient struct {
	cfg        *WebsiteConfig
	httpClient *http.Client
}

// NewWebsiteClient 创建网站发布客户端。
func NewWebsiteClient(cfg *WebsiteConfig) *WebsiteClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &WebsiteClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ---------- 发布请求/响应 ----------

// publishPayload REST API 发布请求体。
type publishPayload struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// publishResponse REST API 发布响应。
type publishResponse struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Error string `json:"error,omitempty"`
}

// ---------- MediaPublisher 接口实现 ----------

// Publish 发布内容到自有网站。
// 实现 media.MediaPublisher 接口。
func (c *WebsiteClient) Publish(
	ctx context.Context,
	draft *media.ContentDraft,
) (*media.PublishResult, error) {
	if draft == nil {
		return nil, fmt.Errorf("website: draft is nil")
	}

	images := draft.Images
	if c.cfg.ImageUploadURL != "" && len(draft.Images) > 0 {
		uploaded, err := c.uploadImages(ctx, draft.Images)
		if err != nil {
			slog.Warn("website: image upload failed, using original URLs",
				"error", err)
		} else {
			images = uploaded
		}
	}

	payload := publishPayload{
		Title:   draft.Title,
		Content: draft.Body,
		Images:  images,
		Tags:    draft.Tags,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("website: marshal payload: %w", err)
	}

	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.cfg.APIURL, body)
	if err != nil {
		return nil, fmt.Errorf("website: publish: %w", err)
	}

	var resp publishResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("website: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("website: API error: %s", resp.Error)
	}

	return &media.PublishResult{
		Platform:    media.PlatformWebsite,
		PostID:      resp.ID,
		URL:         resp.URL,
		Status:      "published",
		PublishedAt: time.Now(),
	}, nil
}

// ---------- 图片上传 ----------

// uploadImages 上传图片到配置的图片上传 URL。
func (c *WebsiteClient) uploadImages(
	ctx context.Context,
	imageURLs []string,
) ([]string, error) {
	uploaded := make([]string, 0, len(imageURLs))
	for _, imgURL := range imageURLs {
		payload, _ := json.Marshal(map[string]string{"url": imgURL})
		respBody, err := c.doRequestWithRetry(
			ctx, http.MethodPost, c.cfg.ImageUploadURL, payload)
		if err != nil {
			return nil, fmt.Errorf("upload image %s: %w", imgURL, err)
		}
		var resp struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("parse upload response: %w", err)
		}
		uploaded = append(uploaded, resp.URL)
	}
	return uploaded, nil
}

// ---------- HTTP 封装 ----------

// doRequestWithRetry 带重试的 HTTP 请求。
func (c *WebsiteClient) doRequestWithRetry(
	ctx context.Context,
	method, url string,
	body []byte,
) ([]byte, error) {
	maxRetries := c.cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			slog.Info("website: retrying request",
				"attempt", attempt+1, "backoff", backoff)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		respBody, err := c.doRequest(ctx, method, url, body)
		if err == nil {
			return respBody, nil
		}
		lastErr = err
		slog.Warn("website: request failed",
			"attempt", attempt+1, "error", err)
	}
	return nil, fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

// doRequest 执行单次 HTTP 请求。
func (c *WebsiteClient) doRequest(
	ctx context.Context,
	method, url string,
	body []byte,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(
		ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf(
			"HTTP %d: %s", resp.StatusCode, truncateBody(respBody, 200))
	}
	return respBody, nil
}

// applyAuth 在请求上应用认证头。
func (c *WebsiteClient) applyAuth(req *http.Request) {
	switch c.cfg.AuthType_ {
	case AuthBearer:
		req.Header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
	case AuthAPIKey:
		req.Header.Set("X-API-Key", c.cfg.AuthToken)
	case AuthBasic:
		parts := strings.SplitN(c.cfg.AuthToken, ":", 2)
		if len(parts) == 2 {
			req.SetBasicAuth(parts[0], parts[1])
		}
	}
}

// truncateBody 截断响应体用于错误消息。
func truncateBody(body []byte, maxLen int) string {
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "..."
}
