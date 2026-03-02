package media

// ============================================================================
// media/content_compose_tool.go — 内容生成 LLM 工具
// 封装 DraftStore，为子智能体提供 content_compose 工具接口。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P1-2
//
// NOTE: 不导入 tools 包以避免循环依赖（tools → channels → media）。
// 使用 media 包内定义的 MediaTool / MediaToolResult 类型。
// ============================================================================

import (
	"context"
	"fmt"
)

// ---------- 工具构造器 ----------

// CreateContentComposeTool 创建内容生成工具。
// store 为 nil 时工具仍可构造，但执行时返回错误。
func CreateContentComposeTool(store DraftStore) *MediaTool {
	return &MediaTool{
		ToolName:  ToolContentCompose,
		ToolLabel: "Content Compose",
		ToolDesc: "Draft, preview, revise, and list content for specific platforms. " +
			"Supports WeChat (公众号), Xiaohongshu (小红书), and Website.",
		ToolParams: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []any{"draft", "preview", "revise", "list"},
					"description": "The compose action to perform",
				},
				"platform": map[string]any{
					"type":        "string",
					"enum":        []any{"wechat", "xiaohongshu", "website"},
					"description": "Target platform for the content",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Content title",
				},
				"body": map[string]any{
					"type":        "string",
					"description": "Content body text",
				},
				"tags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Content tags/hashtags",
				},
				"style": map[string]any{
					"type":        "string",
					"enum":        []any{"informative", "casual", "professional"},
					"description": "Content writing style (default: informative)",
				},
				"draft_id": map[string]any{
					"type":        "string",
					"description": "Draft ID for preview/revise actions",
				},
				"revise_notes": map[string]any{
					"type":        "string",
					"description": "Revision instructions for revise action",
				},
			},
			"required": []any{"action"},
		},
		ToolExecute: func(ctx context.Context, toolCallID string, args map[string]any) (*MediaToolResult, error) {
			action, err := readStringArg(args, "action", true)
			if err != nil {
				return nil, err
			}

			switch ComposeAction(action) {
			case ComposeActionDraft:
				return executeComposeDraft(store, args)
			case ComposeActionPreview:
				return executeComposePreview(store, args)
			case ComposeActionRevise:
				return executeComposeRevise(store, args)
			case ComposeActionList:
				return executeComposeList(store, args)
			default:
				return nil, fmt.Errorf("unknown compose action: %s", action)
			}
		},
	}
}

// ---------- Action 实现 ----------

// executeComposeDraft 创建新草稿并保存到 DraftStore。
func executeComposeDraft(store DraftStore, args map[string]any) (*MediaToolResult, error) {
	if store == nil {
		return nil, fmt.Errorf("draft store not configured")
	}

	title, err := readStringArg(args, "title", true)
	if err != nil {
		return nil, err
	}
	body, err := readStringArg(args, "body", true)
	if err != nil {
		return nil, err
	}
	platformStr, _ := readStringArg(args, "platform", false)
	if platformStr == "" {
		platformStr = "website"
	}
	platform := Platform(platformStr)
	if !isValidPlatform(platform) {
		return nil, fmt.Errorf("unknown platform %q (use wechat/xiaohongshu/website)", platformStr)
	}

	// Platform content constraints.
	if err := validatePlatformContent(platform, title, body); err != nil {
		return nil, err
	}

	styleStr, _ := readStringArg(args, "style", false)
	if styleStr == "" {
		styleStr = "informative"
	}
	style := ContentStyle(styleStr)
	if !isValidStyle(style) {
		return nil, fmt.Errorf("unknown style %q (use informative/casual/professional)", styleStr)
	}
	tags := readStringArrayArg(args, "tags")

	draft := &ContentDraft{
		Title:    title,
		Body:     body,
		Tags:     tags,
		Platform: platform,
		Style:    style,
	}

	if err := store.Save(draft); err != nil {
		return nil, fmt.Errorf("save draft: %w", err)
	}

	return jsonMediaResult(map[string]any{
		"status":   "created",
		"draft_id": draft.ID,
		"platform": string(draft.Platform),
		"title":    draft.Title,
	}), nil
}

// executeComposePreview 预览已有草稿。
func executeComposePreview(store DraftStore, args map[string]any) (*MediaToolResult, error) {
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
		"draft_id":   draft.ID,
		"title":      draft.Title,
		"body":       draft.Body,
		"platform":   string(draft.Platform),
		"style":      string(draft.Style),
		"tags":       draft.Tags,
		"images":     draft.Images,
		"status":     string(draft.Status),
		"created_at": draft.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at": draft.UpdatedAt.Format("2006-01-02 15:04:05"),
	}), nil
}

// executeComposeRevise 修改已有草稿。
func executeComposeRevise(store DraftStore, args map[string]any) (*MediaToolResult, error) {
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

	// Apply field updates if provided.
	if title, _ := readStringArg(args, "title", false); title != "" {
		draft.Title = title
	}
	if body, _ := readStringArg(args, "body", false); body != "" {
		draft.Body = body
	}
	if tags := readStringArrayArg(args, "tags"); tags != nil {
		draft.Tags = tags
	}
	if styleStr, _ := readStringArg(args, "style", false); styleStr != "" {
		draft.Style = ContentStyle(styleStr)
	}

	// Validate against platform constraints after revision.
	if err := validatePlatformContent(draft.Platform, draft.Title, draft.Body); err != nil {
		return nil, err
	}

	// Reset status to draft on revision.
	draft.Status = DraftStatusDraft

	if err := store.Save(draft); err != nil {
		return nil, fmt.Errorf("save revised draft: %w", err)
	}

	return jsonMediaResult(map[string]any{
		"status":   "revised",
		"draft_id": draft.ID,
		"title":    draft.Title,
		"platform": string(draft.Platform),
	}), nil
}

// executeComposeList 列出草稿。
func executeComposeList(store DraftStore, args map[string]any) (*MediaToolResult, error) {
	if store == nil {
		return nil, fmt.Errorf("draft store not configured")
	}

	platformStr, _ := readStringArg(args, "platform", false)

	drafts, err := store.List(platformStr)
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}

	// Build compact summaries.
	items := make([]map[string]any, 0, len(drafts))
	for _, d := range drafts {
		items = append(items, map[string]any{
			"draft_id": d.ID,
			"title":    d.Title,
			"platform": string(d.Platform),
			"status":   string(d.Status),
			"style":    string(d.Style),
		})
	}

	return jsonMediaResult(map[string]any{
		"count":  len(items),
		"drafts": items,
	}), nil
}
