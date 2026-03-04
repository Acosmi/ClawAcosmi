package media

// ============================================================================
// media/publish_tool.go — 平台发布 LLM 工具
// 为子智能体提供 media_publish 工具接口。
// 放在 media 包避免循环依赖（与 trending_tool.go, content_compose_tool.go 一致）。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-5
// ============================================================================

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

// ---------- Publisher 接口 ----------

// MediaPublisher 各平台的发布能力抽象。
// wechat_mp.Publisher、xiaohongshu RPA 等均应实现此接口。
type MediaPublisher interface {
	Publish(ctx context.Context, draft *ContentDraft) (*PublishResult, error)
}

// ---------- PublishAction 常量 ----------

// PublishAction 发布工具操作类型。
type PublishAction string

const (
	PublishActionPublish PublishAction = "publish"
	PublishActionStatus  PublishAction = "status"
	PublishActionApprove PublishAction = "approve"
)

// ---------- 工具构造器 ----------

// CreateMediaPublishTool 创建平台发布工具。
// store 提供草稿存取，publishers 按 Platform 注册各平台发布器。
// history 可为 nil，非 nil 时发布成功后自动写入历史。
func CreateMediaPublishTool(
	store DraftStore,
	publishers map[Platform]MediaPublisher,
	history PublishHistoryStore,
) *MediaTool {
	historyStore := history
	return &MediaTool{
		ToolName:  ToolMediaPublish,
		ToolLabel: "Media Publish",
		ToolDesc: "Publish approved content to WeChat MP, Xiaohongshu, or Website. " +
			"Supports approve, publish, and status actions.",
		ToolParams: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []any{"publish", "status", "approve"},
					"description": "The publish action to perform",
				},
				"draft_id": map[string]any{
					"type":        "string",
					"description": "Draft ID to publish/approve/check status",
				},
				"platform": map[string]any{
					"type":        "string",
					"enum":        []any{"wechat", "xiaohongshu", "website"},
					"description": "Target platform (required for publish)",
				},
			},
			"required": []any{"action", "draft_id"},
		},
		ToolExecute: func(ctx context.Context, toolCallID string, args map[string]any) (*MediaToolResult, error) {
			action, err := readStringArg(args, "action", true)
			if err != nil {
				return nil, err
			}

			switch PublishAction(action) {
			case PublishActionApprove:
				return executePublishApprove(store, args)
			case PublishActionPublish:
				return executePublishPublish(ctx, store, publishers, historyStore, args)
			case PublishActionStatus:
				return executePublishStatus(store, args)
			default:
				return nil, fmt.Errorf("unknown publish action: %s", action)
			}
		},
	}
}

// ---------- Action 实现 ----------

// executePublishApprove 审批草稿（状态 → approved）。
func executePublishApprove(
	store DraftStore,
	args map[string]any,
) (*MediaToolResult, error) {
	if store == nil {
		return nil, fmt.Errorf("draft store not configured")
	}

	draftID, err := readStringArg(args, "draft_id", true)
	if err != nil {
		return nil, err
	}

	draft, err := store.Get(draftID)
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}

	if draft.Status == DraftStatusPublished {
		return nil, fmt.Errorf("draft %s already published", draftID)
	}

	if err := store.UpdateStatus(draftID, DraftStatusApproved); err != nil {
		return nil, fmt.Errorf("approve draft: %w", err)
	}

	return jsonMediaResult(map[string]any{
		"status":   "approved",
		"draft_id": draftID,
		"title":    draft.Title,
		"platform": string(draft.Platform),
	}), nil
}

// executePublishPublish 发布已审批草稿。
func executePublishPublish(
	ctx context.Context,
	store DraftStore,
	publishers map[Platform]MediaPublisher,
	history PublishHistoryStore,
	args map[string]any,
) (*MediaToolResult, error) {
	if store == nil {
		return nil, fmt.Errorf("draft store not configured")
	}

	draftID, err := readStringArg(args, "draft_id", true)
	if err != nil {
		return nil, err
	}

	draft, err := store.Get(draftID)
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}

	// 审批门控：只有 approved 状态才能发布。
	if draft.Status != DraftStatusApproved {
		return nil, fmt.Errorf(
			"draft %s is not approved (status: %s), approve first",
			draftID, draft.Status)
	}

	// 路由到对应平台发布器。
	publisher, ok := publishers[draft.Platform]
	if !ok {
		registered := registeredPlatforms(publishers)
		return nil, fmt.Errorf(
			"platform %q not registered; registered: %s",
			draft.Platform, registered)
	}

	result, err := publisher.Publish(ctx, draft)
	if err != nil {
		return nil, fmt.Errorf("publish to %s: %w", draft.Platform, err)
	}

	// 更新草稿状态为已发布。
	if err := store.UpdateStatus(draftID, DraftStatusPublished); err != nil {
		slog.Warn("failed to update draft status after publish",
			"draft_id", draftID,
			"error", err,
		)
	}

	// P0-4: 写入发布历史
	if history != nil {
		record := &PublishRecord{
			DraftID:  draftID,
			Title:    draft.Title,
			Platform: result.Platform,
			PostID:   result.PostID,
			URL:      result.URL,
			Status:   result.Status,
		}
		if saveErr := history.Save(record); saveErr != nil {
			slog.Warn("failed to save publish history",
				"draft_id", draftID,
				"error", saveErr,
			)
		}
	}

	return jsonMediaResult(map[string]any{
		"status":   "published",
		"draft_id": draftID,
		"platform": string(result.Platform),
		"post_id":  result.PostID,
		"url":      result.URL,
	}), nil
}

// executePublishStatus 查询草稿发布状态。
func executePublishStatus(
	store DraftStore,
	args map[string]any,
) (*MediaToolResult, error) {
	if store == nil {
		return nil, fmt.Errorf("draft store not configured")
	}

	draftID, err := readStringArg(args, "draft_id", true)
	if err != nil {
		return nil, err
	}

	draft, err := store.Get(draftID)
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}

	return jsonMediaResult(map[string]any{
		"draft_id": draft.ID,
		"title":    draft.Title,
		"platform": string(draft.Platform),
		"status":   string(draft.Status),
	}), nil
}

// registeredPlatforms 返回已注册发布平台的可读列表。
func registeredPlatforms(publishers map[Platform]MediaPublisher) string {
	if len(publishers) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(publishers))
	for p := range publishers {
		names = append(names, string(p))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
