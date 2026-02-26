package autoreply

import "strings"

// TS 对照: auto-reply/thinking.ts

// ---------- 类型定义 ----------

// ThinkLevel 思考级别。
type ThinkLevel string

const (
	ThinkOff     ThinkLevel = "off"
	ThinkMinimal ThinkLevel = "minimal"
	ThinkLow     ThinkLevel = "low"
	ThinkMedium  ThinkLevel = "medium"
	ThinkHigh    ThinkLevel = "high"
	ThinkXHigh   ThinkLevel = "xhigh"
)

// VerboseLevel 详细输出级别。
type VerboseLevel string

const (
	VerboseOff  VerboseLevel = "off"
	VerboseOn   VerboseLevel = "on"
	VerboseFull VerboseLevel = "full"
)

// NoticeLevel 系统通知级别。
type NoticeLevel string

const (
	NoticeOff  NoticeLevel = "off"
	NoticeOn   NoticeLevel = "on"
	NoticeFull NoticeLevel = "full"
)

// ElevatedLevel 提权级别。
type ElevatedLevel string

const (
	ElevatedOff  ElevatedLevel = "off"
	ElevatedOn   ElevatedLevel = "on"
	ElevatedAsk  ElevatedLevel = "ask"
	ElevatedFull ElevatedLevel = "full"
)

// ElevatedMode 提权模式（解析后）。
type ElevatedMode string

const (
	ElevatedModeOff  ElevatedMode = "off"
	ElevatedModeAsk  ElevatedMode = "ask"
	ElevatedModeFull ElevatedMode = "full"
)

// ReasoningLevel 推理可见性级别。
type ReasoningLevel string

const (
	ReasoningOff    ReasoningLevel = "off"
	ReasoningOn     ReasoningLevel = "on"
	ReasoningStream ReasoningLevel = "stream"
)

// UsageDisplayLevel 用量显示级别。
type UsageDisplayLevel string

const (
	UsageOff    UsageDisplayLevel = "off"
	UsageTokens UsageDisplayLevel = "tokens"
	UsageFull   UsageDisplayLevel = "full"
)

// ---------- xhigh 模型引用 ----------

// XHighModelRefs 支持 xhigh 思考级别的模型引用列表。
// TS 对照: thinking.ts L24-31
var XHighModelRefs = []string{
	"openai/gpt-5.2",
	"openai-codex/gpt-5.3-codex",
	"openai-codex/gpt-5.2-codex",
	"openai-codex/gpt-5.1-codex",
	"github-copilot/gpt-5.2-codex",
	"github-copilot/gpt-5.2",
}

var xhighModelSet map[string]struct{}
var xhighModelIDs map[string]struct{}

func init() {
	xhighModelSet = make(map[string]struct{}, len(XHighModelRefs))
	xhighModelIDs = make(map[string]struct{}, len(XHighModelRefs))
	for _, ref := range XHighModelRefs {
		xhighModelSet[strings.ToLower(ref)] = struct{}{}
		parts := strings.SplitN(ref, "/", 2)
		if len(parts) == 2 {
			xhighModelIDs[strings.ToLower(parts[1])] = struct{}{}
		}
	}
}

// ---------- Provider 辅助 ----------

// normalizeProviderId 规范化 Provider 标识。
// TS 对照: thinking.ts L9-18
func normalizeProviderId(provider string) string {
	if provider == "" {
		return ""
	}
	normalized := strings.ToLower(strings.TrimSpace(provider))
	if normalized == "z.ai" || normalized == "z-ai" {
		return "zai"
	}
	return normalized
}

// IsBinaryThinkingProvider 判断是否为二元思考 Provider。
// TS 对照: thinking.ts L20-22
func IsBinaryThinkingProvider(provider string) bool {
	return normalizeProviderId(provider) == "zai"
}

// ---------- 规范化函数 ----------

// NormalizeThinkLevel 规范化用户输入的思考级别。
// TS 对照: thinking.ts L41-74
func NormalizeThinkLevel(raw string) (ThinkLevel, bool) {
	if raw == "" {
		return "", false
	}
	key := strings.ToLower(strings.TrimSpace(raw))
	collapsed := strings.NewReplacer(" ", "", "_", "", "-", "").Replace(key)

	if collapsed == "xhigh" || collapsed == "extrahigh" {
		return ThinkXHigh, true
	}
	switch key {
	case "off":
		return ThinkOff, true
	case "on", "enable", "enabled":
		return ThinkLow, true
	case "min", "minimal":
		return ThinkMinimal, true
	case "think":
		return ThinkMinimal, true
	}
	switch key {
	case "low", "thinkhard", "think-hard", "think_hard":
		return ThinkLow, true
	}
	switch key {
	case "mid", "med", "medium", "thinkharder", "think-harder", "harder":
		return ThinkMedium, true
	}
	switch key {
	case "high", "ultra", "ultrathink", "thinkhardest", "highest", "max":
		return ThinkHigh, true
	}
	return "", false
}

// SupportsXHighThinking 判断模型是否支持 xhigh 思考级别。
// TS 对照: thinking.ts L76-86
func SupportsXHighThinking(provider, model string) bool {
	modelKey := strings.ToLower(strings.TrimSpace(model))
	if modelKey == "" {
		return false
	}
	providerKey := strings.ToLower(strings.TrimSpace(provider))
	if providerKey != "" {
		_, ok := xhighModelSet[providerKey+"/"+modelKey]
		return ok
	}
	_, ok := xhighModelIDs[modelKey]
	return ok
}

// ListThinkingLevels 列出可用思考级别。
// TS 对照: thinking.ts L88-94
func ListThinkingLevels(provider, model string) []ThinkLevel {
	levels := []ThinkLevel{ThinkOff, ThinkMinimal, ThinkLow, ThinkMedium, ThinkHigh}
	if SupportsXHighThinking(provider, model) {
		levels = append(levels, ThinkXHigh)
	}
	return levels
}

// ListThinkingLevelLabels 列出思考级别标签（二元 Provider 简化）。
// TS 对照: thinking.ts L96-101
func ListThinkingLevelLabels(provider, model string) []string {
	if IsBinaryThinkingProvider(provider) {
		return []string{"off", "on"}
	}
	levels := ListThinkingLevels(provider, model)
	labels := make([]string, len(levels))
	for i, l := range levels {
		labels[i] = string(l)
	}
	return labels
}

// FormatThinkingLevels 格式化思考级别列表。
// TS 对照: thinking.ts L103-109
func FormatThinkingLevels(provider, model, separator string) string {
	if separator == "" {
		separator = ", "
	}
	return strings.Join(ListThinkingLevelLabels(provider, model), separator)
}

// FormatXHighModelHint 格式化 xhigh 模型提示。
// TS 对照: thinking.ts L111-123
func FormatXHighModelHint() string {
	refs := make([]string, len(XHighModelRefs))
	copy(refs, XHighModelRefs)
	if len(refs) == 0 {
		return "unknown model"
	}
	if len(refs) == 1 {
		return refs[0]
	}
	if len(refs) == 2 {
		return refs[0] + " or " + refs[1]
	}
	return strings.Join(refs[:len(refs)-1], ", ") + " or " + refs[len(refs)-1]
}

// NormalizeVerboseLevel 规范化详细输出级别。
// TS 对照: thinking.ts L126-141
func NormalizeVerboseLevel(raw string) (VerboseLevel, bool) {
	if raw == "" {
		return "", false
	}
	key := strings.ToLower(raw)
	switch key {
	case "off", "false", "no", "0":
		return VerboseOff, true
	case "full", "all", "everything":
		return VerboseFull, true
	case "on", "minimal", "true", "yes", "1":
		return VerboseOn, true
	}
	return "", false
}

// NormalizeNoticeLevel 规范化通知级别。
// TS 对照: thinking.ts L144-159
func NormalizeNoticeLevel(raw string) (NoticeLevel, bool) {
	if raw == "" {
		return "", false
	}
	key := strings.ToLower(raw)
	switch key {
	case "off", "false", "no", "0":
		return NoticeOff, true
	case "full", "all", "everything":
		return NoticeFull, true
	case "on", "minimal", "true", "yes", "1":
		return NoticeOn, true
	}
	return "", false
}

// NormalizeUsageDisplay 规范化用量显示级别。
// TS 对照: thinking.ts L162-180
func NormalizeUsageDisplay(raw string) (UsageDisplayLevel, bool) {
	if raw == "" {
		return "", false
	}
	key := strings.ToLower(raw)
	switch key {
	case "off", "false", "no", "0", "disable", "disabled":
		return UsageOff, true
	case "on", "true", "yes", "1", "enable", "enabled":
		return UsageTokens, true
	case "tokens", "token", "tok", "minimal", "min":
		return UsageTokens, true
	case "full", "session":
		return UsageFull, true
	}
	return "", false
}

// ResolveResponseUsageMode 解析用量显示模式。
// TS 对照: thinking.ts L182-184
func ResolveResponseUsageMode(raw string) UsageDisplayLevel {
	level, ok := NormalizeUsageDisplay(raw)
	if !ok {
		return UsageOff
	}
	return level
}

// NormalizeElevatedLevel 规范化提权级别。
// TS 对照: thinking.ts L187-205
func NormalizeElevatedLevel(raw string) (ElevatedLevel, bool) {
	if raw == "" {
		return "", false
	}
	key := strings.ToLower(raw)
	switch key {
	case "off", "false", "no", "0":
		return ElevatedOff, true
	case "full", "auto", "auto-approve", "autoapprove":
		return ElevatedFull, true
	case "ask", "prompt", "approval", "approve":
		return ElevatedAsk, true
	case "on", "true", "yes", "1":
		return ElevatedOn, true
	}
	return "", false
}

// ResolveElevatedMode 解析提权模式。
// TS 对照: thinking.ts L207-215
func ResolveElevatedMode(level ElevatedLevel) ElevatedMode {
	switch level {
	case "", ElevatedOff:
		return ElevatedModeOff
	case ElevatedFull:
		return ElevatedModeFull
	default:
		return ElevatedModeAsk
	}
}

// NormalizeReasoningLevel 规范化推理可见性级别。
// TS 对照: thinking.ts L218-233
func NormalizeReasoningLevel(raw string) (ReasoningLevel, bool) {
	if raw == "" {
		return "", false
	}
	key := strings.ToLower(raw)
	switch key {
	case "off", "false", "no", "0", "hide", "hidden", "disable", "disabled":
		return ReasoningOff, true
	case "on", "true", "yes", "1", "show", "visible", "enable", "enabled":
		return ReasoningOn, true
	case "stream", "streaming", "draft", "live":
		return ReasoningStream, true
	}
	return "", false
}
