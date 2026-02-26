package extensions

// context_pruning.go — 上下文剪枝扩展
// 对应 TS: agents/pi-extensions/context-pruning/ (5 files, ~500L)
//
// 提供 cache-ttl 模式的上下文消息剪枝能力，
// 包括设置解析、软修剪、硬清除。

import (
	"encoding/json"
	"math"
	"strings"
	"time"
)

// ---------- 类型 ----------

// ContextPruningToolMatch 工具匹配规则。
type ContextPruningToolMatch struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

// ContextPruningMode 剪枝模式。
type ContextPruningMode string

const (
	PruningModeOff      ContextPruningMode = "off"
	PruningModeCacheTTL ContextPruningMode = "cache-ttl"
)

// SoftTrimSettings 软修剪设置。
type SoftTrimSettings struct {
	MaxChars  int `json:"maxChars"`
	HeadChars int `json:"headChars"`
	TailChars int `json:"tailChars"`
}

// HardClearSettings 硬清除设置。
type HardClearSettings struct {
	Enabled     bool   `json:"enabled"`
	Placeholder string `json:"placeholder"`
}

// EffectiveContextPruningSettings 生效的剪枝设置。
type EffectiveContextPruningSettings struct {
	Mode                 ContextPruningMode      `json:"mode"`
	TTLMs                int64                   `json:"ttlMs"`
	KeepLastAssistants   int                     `json:"keepLastAssistants"`
	SoftTrimRatio        float64                 `json:"softTrimRatio"`
	HardClearRatio       float64                 `json:"hardClearRatio"`
	MinPrunableToolChars int                     `json:"minPrunableToolChars"`
	Tools                ContextPruningToolMatch `json:"tools"`
	SoftTrim             SoftTrimSettings        `json:"softTrim"`
	HardClear            HardClearSettings       `json:"hardClear"`
}

// ContextPruningConfig 用户配置。
type ContextPruningConfig struct {
	Mode                 *string                  `json:"mode,omitempty"`
	TTL                  *string                  `json:"ttl,omitempty"`
	KeepLastAssistants   *int                     `json:"keepLastAssistants,omitempty"`
	SoftTrimRatio        *float64                 `json:"softTrimRatio,omitempty"`
	HardClearRatio       *float64                 `json:"hardClearRatio,omitempty"`
	MinPrunableToolChars *int                     `json:"minPrunableToolChars,omitempty"`
	Tools                *ContextPruningToolMatch `json:"tools,omitempty"`
	SoftTrim             *SoftTrimSettings        `json:"softTrim,omitempty"`
	HardClear            *HardClearSettings       `json:"hardClear,omitempty"`
}

// 图像字符估算常量
const ImageCharEstimate = 8_000

// ---------- 默认值 ----------

// DefaultContextPruningSettings 默认剪枝设置。
var DefaultContextPruningSettings = EffectiveContextPruningSettings{
	Mode:                 PruningModeCacheTTL,
	TTLMs:                5 * 60 * 1000, // 5分钟
	KeepLastAssistants:   3,
	SoftTrimRatio:        0.3,
	HardClearRatio:       0.5,
	MinPrunableToolChars: 50_000,
	Tools:                ContextPruningToolMatch{},
	SoftTrim: SoftTrimSettings{
		MaxChars:  4_000,
		HeadChars: 1_500,
		TailChars: 1_500,
	},
	HardClear: HardClearSettings{
		Enabled:     true,
		Placeholder: "[Old tool result content cleared]",
	},
}

// ---------- 设置解析 ----------

// ComputeEffectiveSettings 解析有效剪枝设置。
// 对应 TS: computeEffectiveSettings
func ComputeEffectiveSettings(raw json.RawMessage) *EffectiveContextPruningSettings {
	if len(raw) == 0 {
		return nil
	}
	var cfg ContextPruningConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}

	// 必须是 cache-ttl 模式
	if cfg.Mode == nil || *cfg.Mode != string(PruningModeCacheTTL) {
		return nil
	}

	s := DefaultContextPruningSettings // 值拷贝
	s.Mode = PruningModeCacheTTL

	if cfg.TTL != nil {
		if ms := ParseDurationMs(*cfg.TTL); ms > 0 {
			s.TTLMs = ms
		}
	}
	if cfg.KeepLastAssistants != nil {
		s.KeepLastAssistants = max(0, *cfg.KeepLastAssistants)
	}
	if cfg.SoftTrimRatio != nil {
		s.SoftTrimRatio = math.Min(1, math.Max(0, *cfg.SoftTrimRatio))
	}
	if cfg.HardClearRatio != nil {
		s.HardClearRatio = math.Min(1, math.Max(0, *cfg.HardClearRatio))
	}
	if cfg.MinPrunableToolChars != nil {
		s.MinPrunableToolChars = max(0, *cfg.MinPrunableToolChars)
	}
	if cfg.Tools != nil {
		s.Tools = *cfg.Tools
	}
	if cfg.SoftTrim != nil {
		if cfg.SoftTrim.MaxChars > 0 {
			s.SoftTrim.MaxChars = cfg.SoftTrim.MaxChars
		}
		if cfg.SoftTrim.HeadChars > 0 {
			s.SoftTrim.HeadChars = cfg.SoftTrim.HeadChars
		}
		if cfg.SoftTrim.TailChars > 0 {
			s.SoftTrim.TailChars = cfg.SoftTrim.TailChars
		}
	}
	if cfg.HardClear != nil {
		s.HardClear.Enabled = cfg.HardClear.Enabled
		if cfg.HardClear.Placeholder != "" {
			s.HardClear.Placeholder = strings.TrimSpace(cfg.HardClear.Placeholder)
		}
	}

	return &s
}

// ---------- 剪枝逻辑 ----------

// IsToolPrunable 检查工具是否可剪枝。
func IsToolPrunable(toolName string, tools ContextPruningToolMatch) bool {
	if len(tools.Deny) > 0 {
		for _, d := range tools.Deny {
			if d == toolName {
				return false
			}
		}
	}
	if len(tools.Allow) > 0 {
		for _, a := range tools.Allow {
			if a == toolName {
				return true
			}
		}
		return false
	}
	return true
}

// EstimateMessageChars 估算消息字符数。
// 对应 TS: estimateMessageChars
// - image 块使用 IMAGE_CHAR_ESTIMATE (8000) 估算
// - tool_use / toolCall 块使用 JSON 序列化长度
// - text / thinking 块使用实际字符数
func EstimateMessageChars(msg AgentMessage) int {
	if len(msg.Content) == 0 {
		return len(msg.ToolName) + 20
	}

	// 尝试解析为内容块数组
	var blocks []struct {
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		Thinking  string          `json:"thinking"`
		Arguments json.RawMessage `json:"arguments"`
		Input     json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		// 无法解析为块数组时退回到原始长度
		return len(msg.Content) + len(msg.ToolName) + 20
	}

	chars := 0
	for _, b := range blocks {
		switch b.Type {
		case "image", "image_url":
			chars += ImageCharEstimate
		case "text":
			chars += len(b.Text)
		case "thinking":
			chars += len(b.Thinking)
		case "tool_use", "toolCall":
			// 使用 Input 或 Arguments 字段的 JSON 序列化长度
			if len(b.Input) > 0 {
				chars += len(b.Input)
			} else if len(b.Arguments) > 0 {
				chars += len(b.Arguments)
			} else {
				chars += 128 // 默认估算
			}
		default:
			chars += 64
		}
	}
	return chars
}

// FindAssistantCutoffIndex 查找 assistant 消息截断索引。
func FindAssistantCutoffIndex(messages []AgentMessage, keepLastAssistants int) int {
	if keepLastAssistants <= 0 || len(messages) == 0 {
		return 0
	}

	count := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			count++
			if count >= keepLastAssistants {
				return i
			}
		}
	}
	return 0
}

// contentHasImageBlocks 检查内容 JSON 中是否包含图片块。
// 对应 TS: hasImageBlocks
func contentHasImageBlocks(content json.RawMessage) bool {
	if len(content) == 0 {
		return false
	}
	var blocks []struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(content, &blocks); err != nil {
		return false
	}
	for _, b := range blocks {
		if b.Type == "image" || b.Type == "image_url" {
			return true
		}
	}
	return false
}

// SoftTrimToolResult 软修剪工具结果。
// 对应 TS: softTrimToolResultMessage
// 如果内容包含图片块，跳过软修剪（返回 nil 表示跳过）。
func SoftTrimToolResult(content json.RawMessage, settings EffectiveContextPruningSettings) json.RawMessage {
	// E-4: 图片块保护——包含图片的工具结果跳过软修剪
	if contentHasImageBlocks(content) {
		return content
	}

	// 提取文本段进行修剪
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &blocks); err != nil {
		// 无法解析时按原始字符串处理
		text := string(content)
		if len(text) <= settings.SoftTrim.MaxChars {
			return content
		}
		head := text
		if settings.SoftTrim.HeadChars < len(text) {
			head = text[:settings.SoftTrim.HeadChars]
		}
		tail := ""
		if settings.SoftTrim.TailChars > 0 && settings.SoftTrim.TailChars < len(text) {
			tail = text[len(text)-settings.SoftTrim.TailChars:]
		}
		trimmed := head + "\n...\n" + tail
		note := "\n\n[Tool result trimmed: kept first " + itoa(settings.SoftTrim.HeadChars) + " chars and last " + itoa(settings.SoftTrim.TailChars) + " chars of " + itoa(len(text)) + " chars.]"
		data, _ := json.Marshal([]map[string]string{{"type": "text", "text": trimmed + note}})
		return json.RawMessage(data)
	}

	// 收集文本段
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}

	// 计算文本总长度（含换行分隔符）
	rawLen := 0
	for i, p := range parts {
		rawLen += len(p)
		if i < len(parts)-1 {
			rawLen++ // "\n" separator
		}
	}

	if rawLen <= settings.SoftTrim.MaxChars {
		return content
	}

	headChars := settings.SoftTrim.HeadChars
	tailChars := settings.SoftTrim.TailChars
	if headChars < 0 {
		headChars = 0
	}
	if tailChars < 0 {
		tailChars = 0
	}
	if headChars+tailChars >= rawLen {
		return content
	}

	head := takeHead(parts, headChars)
	tail := takeTail(parts, tailChars)
	trimmed := head + "\n...\n" + tail
	note := "\n\n[Tool result trimmed: kept first " + itoa(headChars) + " chars and last " + itoa(tailChars) + " chars of " + itoa(rawLen) + " chars.]"

	data, _ := json.Marshal([]map[string]string{{"type": "text", "text": trimmed + note}})
	return json.RawMessage(data)
}

// takeHead 从文本段列表中取前 maxChars 个字符。
func takeHead(parts []string, maxChars int) string {
	if maxChars <= 0 || len(parts) == 0 {
		return ""
	}
	remaining := maxChars
	out := ""
	for i, p := range parts {
		if i > 0 {
			if remaining <= 0 {
				break
			}
			out += "\n"
			remaining--
		}
		if remaining <= 0 {
			break
		}
		if len(p) <= remaining {
			out += p
			remaining -= len(p)
		} else {
			out += p[:remaining]
			remaining = 0
		}
	}
	return out
}

// takeTail 从文本段列表中取后 maxChars 个字符。
func takeTail(parts []string, maxChars int) string {
	if maxChars <= 0 || len(parts) == 0 {
		return ""
	}
	remaining := maxChars
	out := make([]string, 0)
	for i := len(parts) - 1; i >= 0 && remaining > 0; i-- {
		p := parts[i]
		if len(p) <= remaining {
			out = append(out, p)
			remaining -= len(p)
		} else {
			out = append(out, p[len(p)-remaining:])
			remaining = 0
			break
		}
		if remaining > 0 && i > 0 {
			out = append(out, "\n")
			remaining--
		}
	}
	// reverse
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	result := ""
	for _, s := range out {
		result += s
	}
	return result
}

// itoa 整数转字符串（避免导入 strconv）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for l, r := 0, len(buf)-1; l < r; l, r = l+1, r-1 {
		buf[l], buf[r] = buf[r], buf[l]
	}
	return string(buf)
}

// PruneContextMessages 剪枝上下文消息。
// 对应 TS: pruneContextMessages
func PruneContextMessages(
	messages []AgentMessage,
	settings EffectiveContextPruningSettings,
	contextWindowTokens int,
) []AgentMessage {
	if len(messages) == 0 || settings.Mode == PruningModeOff {
		return messages
	}

	if contextWindowTokens <= 0 {
		return messages
	}

	charWindow := contextWindowTokens * 4
	if charWindow <= 0 {
		return messages
	}

	// E-1: 查找 assistant 消息截断索引
	cutoffIndex := FindAssistantCutoffIndex(messages, settings.KeepLastAssistants)

	// E-1: 查找第一条用户消息的索引，保护系统初始化消息（SOUL.md/USER.md 等）
	// 对应 TS: findFirstUserIndex / pruneStartIndex
	firstUserIdx := -1
	for i, msg := range messages {
		if msg.Role == "user" {
			firstUserIdx = i
			break
		}
	}
	// 剪枝起始索引：从第一条用户消息开始，之前的消息不剪枝
	pruneStartIndex := len(messages) // 若无用户消息，不剪枝
	if firstUserIdx >= 0 {
		pruneStartIndex = firstUserIdx
	}

	// 计算总字符数，判断是否需要剪枝
	totalChars := 0
	for _, msg := range messages {
		totalChars += EstimateMessageChars(msg)
	}
	ratio := float64(totalChars) / float64(charWindow)
	if ratio < settings.SoftTrimRatio {
		return messages
	}

	// E-2: TTL 时间戳
	ttlDuration := time.Duration(settings.TTLMs) * time.Millisecond

	// 第一步：软修剪
	prunableIndexes := make([]int, 0)
	next := make([]AgentMessage, len(messages))
	copy(next, messages)

	for i := pruneStartIndex; i < cutoffIndex; i++ {
		msg := next[i]
		if msg.Role != "toolResult" {
			continue
		}
		if !IsToolPrunable(msg.ToolName, settings.Tools) {
			continue
		}
		// E-4: 图片块保护——包含图片的工具结果跳过软修剪
		if contentHasImageBlocks(msg.Content) {
			continue
		}
		prunableIndexes = append(prunableIndexes, i)

		trimmed := SoftTrimToolResult(msg.Content, settings)
		if string(trimmed) != string(msg.Content) {
			beforeChars := EstimateMessageChars(msg)
			updated := msg
			updated.Content = trimmed
			next[i] = updated
			afterChars := EstimateMessageChars(updated)
			totalChars += afterChars - beforeChars
		}
	}

	ratio = float64(totalChars) / float64(charWindow)
	if ratio < settings.HardClearRatio {
		return next
	}
	if !settings.HardClear.Enabled {
		return next
	}

	// 检查可剪枝工具内容总量
	prunableToolChars := 0
	for _, i := range prunableIndexes {
		prunableToolChars += EstimateMessageChars(next[i])
	}
	if prunableToolChars < settings.MinPrunableToolChars {
		return next
	}

	// 第二步：硬清除（基于 TTL 时间戳）
	placeholder := settings.HardClear.Placeholder
	for _, i := range prunableIndexes {
		if ratio < settings.HardClearRatio {
			break
		}
		msg := next[i]
		if msg.Role != "toolResult" {
			continue
		}

		// E-2: 真实 TTL 检查：若消息有 CachedAt，则只有超过 TTL 的才硬清除
		if msg.CachedAt != nil && !msg.CachedAt.IsZero() {
			if ttlDuration > 0 && time.Since(*msg.CachedAt) <= ttlDuration {
				// 尚未过期，跳过硬清除
				continue
			}
		}

		beforeChars := EstimateMessageChars(msg)
		cleared := msg
		cleared.Content = json.RawMessage(`[{"type":"text","text":"` + placeholder + `"}]`)
		next[i] = cleared
		afterChars := EstimateMessageChars(cleared)
		totalChars += afterChars - beforeChars
		ratio = float64(totalChars) / float64(charWindow)
	}

	return next
}

// ---------- 辅助 ----------

// ParseDurationMs 解析持续时间字符串为毫秒。
func ParseDurationMs(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// 尝试标准 Go duration
	if d, err := time.ParseDuration(s); err == nil {
		return d.Milliseconds()
	}
	// 默认单位：分钟
	var value float64
	unit := "m"
	n, _ := parseFloat(s)
	if n > 0 {
		value = n
	}
	switch unit {
	case "s":
		return int64(value * 1000)
	case "m":
		return int64(value * 60_000)
	case "h":
		return int64(value * 3_600_000)
	default:
		return int64(value * 60_000)
	}
}

func parseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	var result float64
	var dotSeen bool
	var decimals float64 = 1

	for _, c := range s {
		if c >= '0' && c <= '9' {
			if dotSeen {
				decimals *= 10
				result += float64(c-'0') / decimals
			} else {
				result = result*10 + float64(c-'0')
			}
		} else if c == '.' && !dotSeen {
			dotSeen = true
		} else {
			break
		}
	}
	return result, result > 0
}
