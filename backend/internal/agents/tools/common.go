// Package tools 提供 Agent 工具框架。
// TS 参考：src/agents/tools/common.ts (244L)
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// ---------- 核心类型 ----------

// AgentToolResult 工具执行结果。
type AgentToolResult struct {
	Content []ContentBlock `json:"content"`
	Details any            `json:"details,omitempty"`
}

// ContentBlock 工具结果中的内容块。
type ContentBlock struct {
	Type     string `json:"type"`               // "text" | "image"
	Text     string `json:"text,omitempty"`     // type="text" 时
	Data     string `json:"data,omitempty"`     // type="image" 时 (base64)
	MimeType string `json:"mimeType,omitempty"` // type="image" 时
}

// AgentTool Agent 工具定义。
//
// Execute 接收调用方的 context，使工具能正确响应取消信号。
// TODO:CONTEXT-MIGRATION — 当前仅 sessions_* 和 web_fetch/web_search 已迁移；
// 其余工具仍使用 context.Background() 内联（待逐步迁移）。
type AgentTool struct {
	Label       string `json:"label"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters,omitempty"` // JSON Schema
	Execute     func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error)
}

// ---------- 参数读取 ----------

// StringParamOptions 字符串参数读取选项。
type StringParamOptions struct {
	Required   bool
	Trim       bool // 默认 true
	Label      string
	AllowEmpty bool
}

// ReadStringParam 从参数 map 中读取字符串参数。
// TS 参考: common.ts L33-64
func ReadStringParam(params map[string]any, key string, opts *StringParamOptions) (string, error) {
	if opts == nil {
		opts = &StringParamOptions{}
	}
	label := opts.Label
	if label == "" {
		label = key
	}
	trim := true
	if opts.Trim == false && opts.Label != "" {
		// 只在显式设置了 opts 时才采用 false
		// Go 中没有 "未设置" 概念，用额外标记区分
	}
	// 默认 trim=true（与 TS 行为一致）
	_ = trim

	raw, ok := params[key]
	if !ok {
		if opts.Required {
			return "", fmt.Errorf("%s required", label)
		}
		return "", nil
	}

	str, isStr := raw.(string)
	if !isStr {
		if opts.Required {
			return "", fmt.Errorf("%s required", label)
		}
		return "", nil
	}

	value := strings.TrimSpace(str)
	if !opts.AllowEmpty && value == "" {
		if opts.Required {
			return "", fmt.Errorf("%s required", label)
		}
		return "", nil
	}

	return value, nil
}

// ReadStringOrNumberParam 读取字符串或数字参数（统一返回字符串）。
// TS 参考: common.ts L66-86
func ReadStringOrNumberParam(params map[string]any, key string, required bool, label string) (string, error) {
	if label == "" {
		label = key
	}
	raw, ok := params[key]
	if !ok {
		if required {
			return "", fmt.Errorf("%s required", label)
		}
		return "", nil
	}

	switch v := raw.(type) {
	case float64:
		if !math.IsInf(v, 0) && !math.IsNaN(v) {
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		}
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed, nil
		}
	}

	if required {
		return "", fmt.Errorf("%s required", label)
	}
	return "", nil
}

// NumberParamOptions 数字参数读取选项。
type NumberParamOptions struct {
	Required bool
	Label    string
	Integer  bool
}

// ReadNumberParam 从参数 map 中读取数字参数。
// TS 参考: common.ts L88-114
func ReadNumberParam(params map[string]any, key string, opts *NumberParamOptions) (float64, bool, error) {
	if opts == nil {
		opts = &NumberParamOptions{}
	}
	label := opts.Label
	if label == "" {
		label = key
	}

	raw, ok := params[key]
	if !ok {
		if opts.Required {
			return 0, false, fmt.Errorf("%s required", label)
		}
		return 0, false, nil
	}

	var value float64
	var found bool

	switch v := raw.(type) {
	case float64:
		if !math.IsInf(v, 0) && !math.IsNaN(v) {
			value = v
			found = true
		}
	case int:
		value = float64(v)
		found = true
	case int64:
		value = float64(v)
		found = true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			parsed, err := strconv.ParseFloat(trimmed, 64)
			if err == nil && !math.IsInf(parsed, 0) && !math.IsNaN(parsed) {
				value = parsed
				found = true
			}
		}
	}

	if !found {
		if opts.Required {
			return 0, false, fmt.Errorf("%s required", label)
		}
		return 0, false, nil
	}

	if opts.Integer {
		value = math.Trunc(value)
	}
	return value, true, nil
}

// ReadStringArrayParam 从参数 map 中读取字符串数组参数。
// TS 参考: common.ts L116-160
func ReadStringArrayParam(params map[string]any, key string, opts *StringParamOptions) ([]string, error) {
	if opts == nil {
		opts = &StringParamOptions{}
	}
	label := opts.Label
	if label == "" {
		label = key
	}

	raw, ok := params[key]
	if !ok {
		if opts.Required {
			return nil, fmt.Errorf("%s required", label)
		}
		return nil, nil
	}

	switch v := raw.(type) {
	case []any:
		var values []string
		for _, entry := range v {
			if s, ok := entry.(string); ok {
				trimmed := strings.TrimSpace(s)
				if trimmed != "" {
					values = append(values, trimmed)
				}
			}
		}
		if len(values) == 0 {
			if opts.Required {
				return nil, fmt.Errorf("%s required", label)
			}
			return nil, nil
		}
		return values, nil
	case []string:
		var values []string
		for _, s := range v {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				values = append(values, trimmed)
			}
		}
		if len(values) == 0 {
			if opts.Required {
				return nil, fmt.Errorf("%s required", label)
			}
			return nil, nil
		}
		return values, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			if opts.Required {
				return nil, fmt.Errorf("%s required", label)
			}
			return nil, nil
		}
		return []string{trimmed}, nil
	}

	if opts.Required {
		return nil, fmt.Errorf("%s required", label)
	}
	return nil, nil
}

// ---------- Reaction 参数 ----------

// ReactionParams 反应参数。
type ReactionParams struct {
	Emoji   string
	Remove  bool
	IsEmpty bool
}

// ReadReactionParams 读取 emoji 反应参数。
// TS 参考: common.ts L168-187
func ReadReactionParams(params map[string]any, emojiKey, removeKey, removeErrMsg string) (*ReactionParams, error) {
	if emojiKey == "" {
		emojiKey = "emoji"
	}
	if removeKey == "" {
		removeKey = "remove"
	}

	remove := false
	if v, ok := params[removeKey]; ok {
		if b, ok := v.(bool); ok {
			remove = b
		}
	}

	emoji, err := ReadStringParam(params, emojiKey, &StringParamOptions{
		Required:   true,
		AllowEmpty: true,
	})
	if err != nil {
		return nil, err
	}

	if remove && emoji == "" {
		return nil, fmt.Errorf("%s", removeErrMsg)
	}

	return &ReactionParams{
		Emoji:   emoji,
		Remove:  remove,
		IsEmpty: emoji == "",
	}, nil
}

// ---------- Action Gate ----------

// ActionGate 工具动作开关函数。
// TS 参考: common.ts L16-31
type ActionGate func(key string, defaultValue ...bool) bool

// CreateActionGate 创建工具动作开关。
// actions 为 nil 时所有操作默认允许。
func CreateActionGate(actions map[string]any) ActionGate {
	return func(key string, defaultValue ...bool) bool {
		defVal := true
		if len(defaultValue) > 0 {
			defVal = defaultValue[0]
		}
		if actions == nil {
			return defVal
		}
		v, ok := actions[key]
		if !ok {
			return defVal
		}
		b, isBool := v.(bool)
		if !isBool {
			return defVal
		}
		return b
	}
}

// ---------- 结果构造器 ----------

// JsonResult 将 payload 序列化为 JSON 文本结果。
// TS 参考: common.ts L189-199
func JsonResult(payload any) *AgentToolResult {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		data = []byte(fmt.Sprintf("{\"error\": %q}", err.Error()))
	}
	return &AgentToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: string(data)},
		},
		Details: payload,
	}
}

// ImageResult 构造图片结果。
// TS 参考: common.ts L201-225
func ImageResult(label, path, b64data, mimeType string, extraText string, details map[string]any) *AgentToolResult {
	text := extraText
	if text == "" {
		text = "MEDIA:" + path
	}

	content := []ContentBlock{
		{Type: "text", Text: text},
		{Type: "image", Data: b64data, MimeType: mimeType},
	}

	detailsMap := map[string]any{"path": path}
	for k, v := range details {
		detailsMap[k] = v
	}

	return &AgentToolResult{
		Content: content,
		Details: detailsMap,
	}
}

// ImageResultFromFile 从文件路径读取图片并构造结果。
// TS 参考: common.ts L227-243
func ImageResultFromFile(label, path string, extraText string, details map[string]any) (*AgentToolResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取图片文件失败 (path=%s): %w", path, err)
	}

	mimeType := detectMimeFromBytes(data)
	b64 := base64.StdEncoding.EncodeToString(data)

	return ImageResult(label, path, b64, mimeType, extraText, details), nil
}

// detectMimeFromBytes 通过魔数字节嗅探 MIME 类型。
func detectMimeFromBytes(data []byte) string {
	if len(data) < 4 {
		return "image/png"
	}
	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	// GIF: 47 49 46
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return "image/png"
}
