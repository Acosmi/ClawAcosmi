package media

// ============================================================================
// media/media_tool.go — 媒体工具本地类型定义
// 定义 MediaTool / MediaToolResult 避免循环导入 tools 包。
// 结构与 tools.AgentTool / tools.AgentToolResult 完全对齐，
// 集成时由注册层零成本转换。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P1
// ============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// ---------- 工具类型 ----------

// MediaToolResult 工具执行结果（镜像 tools.AgentToolResult）。
type MediaToolResult struct {
	Content []MediaContentBlock `json:"content"`
	Details any                 `json:"details,omitempty"`
}

// MediaContentBlock 结果内容块（镜像 tools.ContentBlock）。
type MediaContentBlock struct {
	Type string `json:"type"`           // "text"
	Text string `json:"text,omitempty"` // type="text" 时
}

// MediaTool 媒体工具定义（镜像 tools.AgentTool）。
type MediaTool struct {
	ToolName    string `json:"name"`
	ToolLabel   string `json:"label"`
	ToolDesc    string `json:"description"`
	ToolParams  any    `json:"parameters,omitempty"` // JSON Schema
	ToolExecute func(ctx context.Context, toolCallID string, args map[string]any) (*MediaToolResult, error)
}

// ---------- 结果构造 ----------

// jsonMediaResult 序列化 payload 为 JSON 文本结果。
func jsonMediaResult(payload any) *MediaToolResult {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		data = []byte(fmt.Sprintf(`{"error": %q}`, err.Error()))
	}
	return &MediaToolResult{
		Content: []MediaContentBlock{
			{Type: "text", Text: string(data)},
		},
		Details: payload,
	}
}

// ---------- 参数读取 ----------

// readStringArg 从 args 中读取字符串参数。
func readStringArg(args map[string]any, key string, required bool) (string, error) {
	raw, ok := args[key]
	if !ok {
		if required {
			return "", fmt.Errorf("%s required", key)
		}
		return "", nil
	}
	str, isStr := raw.(string)
	if !isStr {
		if required {
			return "", fmt.Errorf("%s must be a string", key)
		}
		return "", nil
	}
	trimmed := strings.TrimSpace(str)
	if trimmed == "" && required {
		return "", fmt.Errorf("%s required", key)
	}
	return trimmed, nil
}

// readIntArg 从 args 中读取整数参数。
func readIntArg(args map[string]any, key string) (int, bool) {
	raw, ok := args[key]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		if !math.IsInf(v, 0) && !math.IsNaN(v) {
			return int(v), true
		}
	case int:
		return v, true
	case int64:
		return int(v), true
	}
	return 0, false
}

// readStringArrayArg 从 args 中读取字符串数组参数。
func readStringArrayArg(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		var out []string
		for _, entry := range v {
			if s, ok := entry.(string); ok {
				trimmed := strings.TrimSpace(s)
				if trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	case []string:
		var out []string
		for _, s := range v {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return nil
}

// ---------- ComposeAction 常量 ----------

// ComposeAction 内容生成工具操作类型。
type ComposeAction string

const (
	ComposeActionDraft   ComposeAction = "draft"
	ComposeActionPreview ComposeAction = "preview"
	ComposeActionRevise  ComposeAction = "revise"
	ComposeActionList    ComposeAction = "list"
)

// ---------- 平台内容约束 ----------

// platformConstraint 各平台的内容限制。
type platformConstraint struct {
	MaxTitleRunes int // 0 = no limit
	MaxBodyRunes  int // 0 = no limit
}

var platformConstraints = map[Platform]platformConstraint{
	PlatformWeChat:      {MaxTitleRunes: 64, MaxBodyRunes: 0},
	PlatformXiaohongshu: {MaxTitleRunes: 20, MaxBodyRunes: 1000},
	PlatformWebsite:     {MaxTitleRunes: 0, MaxBodyRunes: 0},
}

// validatePlatformContent 校验内容是否满足平台约束。
func validatePlatformContent(platform Platform, title, body string) error {
	c, ok := platformConstraints[platform]
	if !ok {
		return nil // unknown platform → no constraints
	}
	if c.MaxTitleRunes > 0 {
		n := utf8.RuneCountInString(title)
		if n > c.MaxTitleRunes {
			return fmt.Errorf(
				"title too long for %s: %d characters (max %d)",
				platform, n, c.MaxTitleRunes,
			)
		}
	}
	if c.MaxBodyRunes > 0 {
		n := utf8.RuneCountInString(body)
		if n > c.MaxBodyRunes {
			return fmt.Errorf(
				"body too long for %s: %d characters (max %d)",
				platform, n, c.MaxBodyRunes,
			)
		}
	}
	return nil
}
