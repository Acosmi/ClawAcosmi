package autoreply

import (
	"fmt"
	"sort"
	"strings"
)

// TS 对照: auto-reply/status.ts (679L)

// ---------- DI 接口 ----------

// StatusDeps 状态构建外部依赖。
type StatusDeps interface {
	// ResolveModelRef 解析当前配置的模型引用（provider/model）。
	ResolveModelRef(agentID string) (provider, model string)
	// LookupContextTokens 查询模型最大上下文 token。
	LookupContextTokens(provider, model string) int
	// ResolveSandboxStatus 获取沙箱运行时状态描述。
	ResolveSandboxStatus() string
	// ResolveTtsStatus 获取 TTS 状态描述。
	ResolveTtsStatus() string
	// ResolveModelAuthMode 获取模型认证模式。
	ResolveModelAuthMode() string
	// GetVersion 获取当前版本。
	GetVersion() string
	// ListPluginCommands 列出插件注册的命令。
	ListPluginCommands() []string
	// EstimateUsageCost 估算 token 用量成本（美分）。
	EstimateUsageCost(inputTokens, outputTokens int) float64
}

// ---------- 状态参数 ----------

// QueueStatus 队列状态。
type QueueStatus struct {
	Mode        string
	Depth       int
	DebounceMs  int
	Cap         int
	DropPolicy  string
	ShowDetails bool
}

// MediaDecision 媒体理解决策。
type MediaDecision struct {
	Kind     string // "image", "audio", "video", "file"
	Provider string
	Accepted bool
	Reason   string
}

// StatusArgs 状态消息构建参数。
type StatusArgs struct {
	AgentID           string
	AgentLabel        string
	SessionKey        string
	SessionScope      string
	GroupActivation   string // "mention" | "always"
	ResolvedThink     ThinkLevel
	ResolvedVerbose   VerboseLevel
	ResolvedReasoning ReasoningLevel
	ResolvedElevated  ElevatedLevel
	ModelAuth         string
	UsageLine         string
	TimeLine          string
	Queue             *QueueStatus
	MediaDecisions    []MediaDecision
	SubagentsLine     string
	IncludeTranscript bool
	Now               int64
}

// ---------- 格式化辅助函数（纯函数，无 DI） ----------

// FormatTokenCount 格式化 token 数量。
// TS 对照: status.ts formatTokenCount → 共享版
func FormatTokenCount(count int) string {
	if count < 0 {
		return "0"
	}
	if count < 1000 {
		return fmt.Sprintf("%d", count)
	}
	if count < 10_000 {
		return fmt.Sprintf("%.1fk", float64(count)/1000)
	}
	if count < 1_000_000 {
		return fmt.Sprintf("%dk", count/1000)
	}
	return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
}

// FormatContextUsageShort 格式化上下文用量简短表示。
// 输出: "Context 12k/128k (9%)"
// TS 对照: status.ts formatContextUsageShort
func FormatContextUsageShort(usedTokens, maxTokens int) string {
	if maxTokens <= 0 {
		return fmt.Sprintf("Context %s", FormatTokenCount(usedTokens))
	}
	pct := 0
	if maxTokens > 0 {
		pct = usedTokens * 100 / maxTokens
	}
	return fmt.Sprintf("Context %s/%s (%d%%)",
		FormatTokenCount(usedTokens),
		FormatTokenCount(maxTokens),
		pct)
}

// FormatQueueDetails 格式化队列详情。
// TS 对照: status.ts formatQueueDetails
func FormatQueueDetails(q *QueueStatus) string {
	if q == nil {
		return ""
	}
	var parts []string
	if q.Mode != "" {
		parts = append(parts, "mode: "+q.Mode)
	}
	if q.Depth > 0 {
		parts = append(parts, fmt.Sprintf("depth: %d", q.Depth))
	}
	if q.Cap > 0 {
		parts = append(parts, fmt.Sprintf("cap: %d", q.Cap))
	}
	if q.DebounceMs > 0 {
		parts = append(parts, fmt.Sprintf("debounce: %dms", q.DebounceMs))
	}
	if q.DropPolicy != "" {
		parts = append(parts, "drop: "+q.DropPolicy)
	}
	if len(parts) == 0 {
		return ""
	}
	return "Queue: " + strings.Join(parts, " | ")
}

// FormatMediaDecisionsLine 格式化媒体理解决策。
// TS 对照: status.ts formatMediaUnderstandingLine
func FormatMediaDecisionsLine(decisions []MediaDecision) string {
	if len(decisions) == 0 {
		return ""
	}
	var parts []string
	for _, d := range decisions {
		status := "✅"
		if !d.Accepted {
			status = "❌"
		}
		label := d.Kind
		if d.Provider != "" {
			label += " (" + d.Provider + ")"
		}
		parts = append(parts, status+" "+label)
	}
	return "Media: " + strings.Join(parts, ", ")
}

// FormatUsagePair 格式化用量对（输入/输出）。
// TS 对照: status.ts formatUsagePair
func FormatUsagePair(inputTokens, outputTokens int) string {
	return fmt.Sprintf("↑%s ↓%s",
		FormatTokenCount(inputTokens),
		FormatTokenCount(outputTokens))
}

// ---------- 命令列表构建 ----------

// CommandCategoryOrder 命令类别排序。
var CommandCategoryOrder = []CommandCategory{
	CategorySession,
	CategoryOptions,
	CategoryStatus,
	CategoryManagement,
	CategoryMedia,
	CategoryTools,
	CategoryDocks,
}

// CategoryLabels 类别显示标签。
var CategoryLabels = map[CommandCategory]string{
	CategorySession:    "Session",
	CategoryOptions:    "Options",
	CategoryStatus:     "Status",
	CategoryManagement: "Management",
	CategoryMedia:      "Media",
	CategoryTools:      "Tools",
	CategoryDocks:      "Docks",
}

// CommandListEntry 命令列表条目。
type CommandListEntry struct {
	Name        string
	Description string
	Category    CommandCategory
}

// BuildCommandList 构建命令列表。
// TS 对照: status.ts buildCommandList
func BuildCommandList() []CommandListEntry {
	commands := ListChatCommands()
	entries := make([]CommandListEntry, 0, len(commands))
	for _, cmd := range commands {
		name := cmd.Key
		if cmd.NativeName != "" {
			name = cmd.NativeName
		}
		entries = append(entries, CommandListEntry{
			Name:        "/" + name,
			Description: cmd.Description,
			Category:    cmd.Category,
		})
	}
	return entries
}

// FormatCommandsGrouped 按类别分组格式化命令列表。
// TS 对照: status.ts formatStatusCommands
func FormatCommandsGrouped(entries []CommandListEntry, pluginCmds []string) string {
	// 按类别分组
	byCategory := make(map[CommandCategory][]CommandListEntry)
	for _, e := range entries {
		byCategory[e.Category] = append(byCategory[e.Category], e)
	}

	var sb strings.Builder
	sb.WriteString("📋 **Commands**\n")

	for _, cat := range CommandCategoryOrder {
		group := byCategory[cat]
		if len(group) == 0 {
			continue
		}
		label := CategoryLabels[cat]
		if label == "" {
			label = string(cat)
		}
		sb.WriteString("\n**" + label + "**\n")
		sort.Slice(group, func(i, j int) bool {
			return group[i].Name < group[j].Name
		})
		for _, e := range group {
			sb.WriteString(fmt.Sprintf("  `%s` — %s\n", e.Name, e.Description))
		}
	}

	// 插件命令
	if len(pluginCmds) > 0 {
		sb.WriteString("\n**Plugins**\n")
		for _, cmd := range pluginCmds {
			sb.WriteString(fmt.Sprintf("  `/%s`\n", cmd))
		}
	}

	return sb.String()
}

// ---------- 状态消息构建 ----------

// BuildStatusMessage 构建 /status 命令响应。
// TS 对照: status.ts buildStatusReply
func BuildStatusMessage(args *StatusArgs, deps StatusDeps) string {
	var sb strings.Builder
	sb.WriteString("🤖 **Status**\n\n")

	// 版本
	if deps != nil {
		if v := deps.GetVersion(); v != "" {
			sb.WriteString("Version: " + v + "\n")
		}
	}

	// Agent
	if args.AgentLabel != "" {
		sb.WriteString("Agent: " + args.AgentLabel + "\n")
	}

	// 模型
	if deps != nil && args.AgentID != "" {
		provider, model := deps.ResolveModelRef(args.AgentID)
		if model != "" {
			modelLine := "Model: " + model
			if provider != "" {
				modelLine += " (" + provider + ")"
			}
			sb.WriteString(modelLine + "\n")

			// 上下文
			maxCtx := deps.LookupContextTokens(provider, model)
			if maxCtx > 0 {
				sb.WriteString(fmt.Sprintf("Context: %s\n", FormatTokenCount(maxCtx)))
			}
		}

		// 认证
		if auth := deps.ResolveModelAuthMode(); auth != "" {
			sb.WriteString("Auth: " + auth + "\n")
		}
	}

	// 会话
	if args.SessionKey != "" {
		sb.WriteString("Session: " + args.SessionKey + "\n")
	}
	if args.SessionScope != "" {
		sb.WriteString("Scope: " + args.SessionScope + "\n")
	}

	// 激活模式
	if args.GroupActivation != "" {
		sb.WriteString("Activation: " + args.GroupActivation + "\n")
	}

	// 思考/详细/推理/提权
	if args.ResolvedThink != "" {
		sb.WriteString("Thinking: " + string(args.ResolvedThink) + "\n")
	}
	if args.ResolvedVerbose != "" {
		sb.WriteString("Verbose: " + string(args.ResolvedVerbose) + "\n")
	}
	if args.ResolvedReasoning != "" {
		sb.WriteString("Reasoning: " + string(args.ResolvedReasoning) + "\n")
	}
	if args.ResolvedElevated != "" {
		sb.WriteString("Elevated: " + string(args.ResolvedElevated) + "\n")
	}

	// 用量
	if args.UsageLine != "" {
		sb.WriteString("Usage: " + args.UsageLine + "\n")
	}
	if args.TimeLine != "" {
		sb.WriteString("Time: " + args.TimeLine + "\n")
	}

	// 队列
	if q := FormatQueueDetails(args.Queue); q != "" {
		sb.WriteString(q + "\n")
	}

	// 媒体
	if m := FormatMediaDecisionsLine(args.MediaDecisions); m != "" {
		sb.WriteString(m + "\n")
	}

	// Subagents
	if args.SubagentsLine != "" {
		sb.WriteString("Subagents: " + args.SubagentsLine + "\n")
	}

	// 沙箱 / TTS
	if deps != nil {
		if s := deps.ResolveSandboxStatus(); s != "" {
			sb.WriteString("Sandbox: " + s + "\n")
		}
		if t := deps.ResolveTtsStatus(); t != "" {
			sb.WriteString("TTS: " + t + "\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// BuildHelpMessage 构建 /help 命令响应。
// TS 对照: status.ts buildHelpMessage
func BuildHelpMessage(deps StatusDeps) string {
	var sb strings.Builder
	sb.WriteString("ℹ️ **Help**\n\n")
	sb.WriteString("Use `/commands` to see all available commands.\n")
	sb.WriteString("Use `/status` to see current session status.\n")
	sb.WriteString("Use `/model` to change model.\n")
	sb.WriteString("Use `/thinking` to adjust thinking level.\n")
	sb.WriteString("Use `/new` to start a new session.\n")
	sb.WriteString("Use `/config` to view or change settings.\n")

	if deps != nil {
		if v := deps.GetVersion(); v != "" {
			sb.WriteString("\nVersion: " + v)
		}
	}

	return sb.String()
}

// ---------- 缺失函数补全 ----------

// ResolveRuntimeLabel 解析运行时标签（direct / docker/all 等）。
// TS 对照: status.ts resolveRuntimeLabel (L77-119)
func ResolveRuntimeLabel(deps StatusDeps, sessionKey string) string {
	if deps == nil {
		return "direct"
	}
	sandboxStatus := deps.ResolveSandboxStatus()
	if sandboxStatus == "" || sandboxStatus == "off" {
		return "direct"
	}
	// sandboxStatus 可能是 "sandboxed" / "direct" / "docker/all" 等
	// 如果包含 "/" 则直接返回
	if strings.Contains(sandboxStatus, "/") {
		return sandboxStatus
	}
	if sandboxStatus == "sandboxed" {
		return "docker/auto"
	}
	return "direct"
}

// FormatVoiceModeLine 格式化 TTS 语音模式状态行。
// TS 对照: status.ts formatVoiceModeLine (L286-307)
func FormatVoiceModeLine(deps StatusDeps) string {
	if deps == nil {
		return ""
	}
	ttsStatus := deps.ResolveTtsStatus()
	if ttsStatus == "" || ttsStatus == "off" {
		return ""
	}
	return "🔊 Voice: " + ttsStatus
}

// ReadUsageFromSessionLog 从会话日志读取 token 用量。
// TS 对照: status.ts readUsageFromSessionLog (L165-232)
// 注：完整实现需要 session log 文件 I/O，此处提供 DI 接口桩。
// 实际用量数据通过 StatusArgs.UsageLine 和 StatusDeps 注入。
type SessionUsage struct {
	Input        int
	Output       int
	PromptTokens int
	Total        int
	Model        string
}

// CommandsMessageOptions 命令列表消息选项。
// TS 对照: status.ts CommandsMessageOptions
type CommandsMessageOptions struct {
	Page    int
	Surface string
}

// CommandsMessageResult 命令列表消息结果。
// TS 对照: status.ts CommandsMessageResult
type CommandsMessageResult struct {
	Text        string
	TotalPages  int
	CurrentPage int
	HasNext     bool
	HasPrev     bool
}

const commandsPerPage = 8

// BuildCommandsMessagePaginated 构建分页命令列表消息。
// TS 对照: status.ts buildCommandsMessagePaginated (L635-679)
func BuildCommandsMessagePaginated(deps StatusDeps, opts *CommandsMessageOptions) CommandsMessageResult {
	entries := BuildCommandList()

	var pluginCmds []string
	if deps != nil {
		pluginCmds = deps.ListPluginCommands()
	}

	page := 1
	surface := ""
	if opts != nil {
		if opts.Page > 0 {
			page = opts.Page
		}
		surface = strings.ToLower(opts.Surface)
	}

	isTelegram := surface == "telegram"

	if !isTelegram {
		text := "ℹ️ Slash commands\n\n" + FormatCommandsGrouped(entries, pluginCmds)
		return CommandsMessageResult{
			Text:        strings.TrimSpace(text),
			TotalPages:  1,
			CurrentPage: 1,
		}
	}

	// Telegram: 分页
	totalItems := len(entries) + len(pluginCmds)
	totalPages := (totalItems + commandsPerPage - 1) / commandsPerPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	// 简化分页：基于 entries 切片
	start := (page - 1) * commandsPerPage
	end := start + commandsPerPage
	if end > len(entries) {
		end = len(entries)
	}
	pageEntries := entries
	if start < len(entries) {
		pageEntries = entries[start:end]
	}
	text := fmt.Sprintf("ℹ️ Commands (%d/%d)\n\n%s",
		page, totalPages, FormatCommandsGrouped(pageEntries, nil))
	return CommandsMessageResult{
		Text:        strings.TrimSpace(text),
		TotalPages:  totalPages,
		CurrentPage: page,
		HasNext:     page < totalPages,
		HasPrev:     page > 1,
	}
}

// BuildCommandsMessage 构建命令列表消息（非分页版本）。
// TS 对照: status.ts buildCommandsMessage (L626-633)
func BuildCommandsMessage(deps StatusDeps) string {
	result := BuildCommandsMessagePaginated(deps, nil)
	return result.Text
}
