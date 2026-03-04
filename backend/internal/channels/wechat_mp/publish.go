package wechat_mp

// ============================================================================
// wechat_mp/publish.go — 微信公众号发布流程
// 实现草稿创建、发布提交和状态查询完整链路。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-2
// ============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/media"
)

// Publisher 公众号发布器。
type Publisher struct {
	client *WeChatMPClient
}

// NewPublisher 创建发布器。
func NewPublisher(client *WeChatMPClient) *Publisher {
	return &Publisher{client: client}
}

// ---------- 草稿创建 ----------

// CreateDraft 上传图文草稿到公众号。
// 返回 draft_media_id 供后续发布使用。
func (p *Publisher) CreateDraft(
	ctx context.Context,
	draft *media.ContentDraft,
) (string, error) {
	if draft == nil {
		return "", fmt.Errorf("draft is nil")
	}

	// 上传首图（如有）。
	thumbMediaID := ""
	if len(draft.Images) > 0 {
		url, err := p.client.UploadImage(ctx, draft.Images[0])
		if err != nil {
			slog.Warn("wechat_mp: upload thumb failed, proceeding without",
				"error", err)
		} else {
			thumbMediaID = url
		}
	}

	// 构建图文消息。
	article := map[string]any{
		"title":   draft.Title,
		"content": formatHTMLContent(draft.Body),
	}
	if thumbMediaID != "" {
		article["thumb_media_id"] = thumbMediaID
	}

	payload := map[string]any{
		"articles": []any{article},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal draft payload: %w", err)
	}

	respBody, err := p.client.DoRequest(ctx, "POST", "/cgi-bin/draft/add", body)
	if err != nil {
		return "", fmt.Errorf("create draft API: %w", err)
	}

	var result struct {
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode draft response: %w", err)
	}
	if result.MediaID == "" {
		return "", fmt.Errorf("empty media_id in draft response")
	}

	slog.Info("wechat_mp draft created", "media_id", result.MediaID,
		"title", draft.Title)
	return result.MediaID, nil
}

// ---------- 发布提交 ----------

// SubmitPublish 提交草稿发布。
// 返回 publish_id 供状态查询使用。
func (p *Publisher) SubmitPublish(
	ctx context.Context,
	draftMediaID string,
) (string, error) {
	payload := map[string]any{
		"media_id": draftMediaID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal publish payload: %w", err)
	}

	respBody, err := p.client.DoRequest(ctx, "POST",
		"/cgi-bin/freepublish/submit", body)
	if err != nil {
		return "", fmt.Errorf("submit publish API: %w", err)
	}

	var result struct {
		PublishID string `json:"publish_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode publish response: %w", err)
	}
	if result.PublishID == "" {
		return "", fmt.Errorf("empty publish_id in response")
	}

	slog.Info("wechat_mp publish submitted", "publish_id", result.PublishID)
	return result.PublishID, nil
}

// ---------- 状态查询 ----------

// publishStatusResp 发布状态 API 响应。
type publishStatusResp struct {
	PublishID     string `json:"publish_id"`
	PublishStatus int    `json:"publish_status"` // 0=成功 1=发布中 2+=失败
	ArticleID     string `json:"article_id,omitempty"`
	ArticleURL    string `json:"article_url,omitempty"`
	FailIdx       []int  `json:"fail_idx,omitempty"`
}

// GetPublishStatus 查询发布状态。
func (p *Publisher) GetPublishStatus(
	ctx context.Context,
	publishID string,
) (*media.PublishResult, error) {
	payload := map[string]any{
		"publish_id": publishID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal status payload: %w", err)
	}

	respBody, err := p.client.DoRequest(ctx, "POST",
		"/cgi-bin/freepublish/get", body)
	if err != nil {
		return nil, fmt.Errorf("get publish status API: %w", err)
	}

	var resp publishStatusResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("decode status response: %w", err)
	}

	result := &media.PublishResult{
		Platform: media.PlatformWeChat,
		PostID:   resp.ArticleID,
		URL:      resp.ArticleURL,
	}

	switch resp.PublishStatus {
	case 0:
		result.Status = "published"
		result.PublishedAt = time.Now().UTC()
	case 1:
		result.Status = "publishing"
	default:
		result.Status = "failed"
		result.Error = fmt.Sprintf("publish failed, status=%d", resp.PublishStatus)
	}

	return result, nil
}

// ---------- 完整发布链路 ----------

// Publish 执行完整发布链路：创建草稿 → 提交发布 → 返回发布结果。
// 实现 media.MediaPublisher 接口。
// 注意: 不做轮询等待，返回 publishing 状态后调用方可自行轮询。
func (p *Publisher) Publish(
	ctx context.Context,
	draft *media.ContentDraft,
) (*media.PublishResult, error) {
	// Step 1: 创建草稿。
	draftMediaID, err := p.CreateDraft(ctx, draft)
	if err != nil {
		return nil, fmt.Errorf("create draft: %w", err)
	}

	// Step 2: 提交发布。
	publishID, err := p.SubmitPublish(ctx, draftMediaID)
	if err != nil {
		return nil, fmt.Errorf("submit publish: %w", err)
	}

	// Step 3: 查询初始状态。
	result, err := p.GetPublishStatus(ctx, publishID)
	if err != nil {
		// 发布已提交但状态查询失败 — 返回 publishing 状态。
		return &media.PublishResult{
			Platform: media.PlatformWeChat,
			PostID:   publishID,
			Status:   "publishing",
		}, nil
	}

	return result, nil
}

// ---------- 辅助函数 ----------

// formatHTMLContent 将纯文本转换为简单 HTML 段落。
func formatHTMLContent(body string) string {
	if body == "" {
		return ""
	}
	return "<p>" + body + "</p>"
}
