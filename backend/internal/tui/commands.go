// commands.go — TUI 命令注册表 + 处理器
//
// 对齐 TS: src/tui/commands.ts(164L) + src/tui/tui-command-handlers.ts(503L)
// 差异 CMD-01/C-01 (P1): 20+ slash 命令完整移植。
//
// W4 产出文件 #1。
package tui

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// ---------- 类型定义 ----------

// SlashCommand slash 命令定义。
type SlashCommand struct {
	Name        string
	Description string
	// GetArgumentCompletions 参数补全函数（可选）。
	GetArgumentCompletions func(prefix string) []SlashCommandCompletion
}

// SlashCommandCompletion 命令参数补全项。
type SlashCommandCompletion struct {
	Value string
	Label string
}

// SlashCommandOptions 命令列表选项。
type SlashCommandOptions struct {
	Provider string
	Model    string
}

// ParsedCommand 命令解析结果。
type ParsedCommand struct {
	Name string
	Args string
}

// ---------- 常量 ----------

var verboseLevels = []string{"on", "off"}
var reasoningLevels = []string{"on", "off"}
var elevatedLevels = []string{"on", "off", "ask", "full"}
var activationLevels = []string{"mention", "always"}
var usageFooterLevels = []string{"off", "tokens", "full"}

// commandAliases 命令别名映射。
var commandAliases = map[string]string{
	"elev": "elevated",
}

// ---------- 命令解析 ----------

// ParseCommand 解析 slash 命令。
// TS 参考: commands.ts L27-38
func ParseCommand(input string) ParsedCommand {
	trimmed := strings.TrimSpace(strings.TrimPrefix(input, "/"))
	if trimmed == "" {
		return ParsedCommand{}
	}
	parts := strings.SplitN(trimmed, " ", 2)
	name := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	if alias, ok := commandAliases[name]; ok {
		name = alias
	}
	return ParsedCommand{Name: name, Args: args}
}

// ---------- 命令注册表 ----------

// GetSlashCommands 获取所有可用 slash 命令。
// TS 参考: commands.ts L40-139
func GetSlashCommands(opts SlashCommandOptions) []SlashCommand {
	thinkLevels := autoreply.ListThinkingLevelLabels(opts.Provider, opts.Model)

	commands := []SlashCommand{
		{Name: "help", Description: "Show slash command help"},
		{Name: "status", Description: "Show gateway status summary"},
		{Name: "agent", Description: "Switch agent (or open picker)"},
		{Name: "agents", Description: "Open agent picker"},
		{Name: "session", Description: "Switch session (or open picker)"},
		{Name: "sessions", Description: "Open session picker"},
		{Name: "model", Description: "Set model (or open picker)"},
		{Name: "models", Description: "Open model picker"},
		{
			Name:        "think",
			Description: "Set thinking level",
			GetArgumentCompletions: func(prefix string) []SlashCommandCompletion {
				return filterCompletions(thinkLevels, prefix)
			},
		},
		{
			Name:        "verbose",
			Description: "Set verbose on/off",
			GetArgumentCompletions: func(prefix string) []SlashCommandCompletion {
				return filterCompletions(verboseLevels, prefix)
			},
		},
		{
			Name:        "reasoning",
			Description: "Set reasoning on/off",
			GetArgumentCompletions: func(prefix string) []SlashCommandCompletion {
				return filterCompletions(reasoningLevels, prefix)
			},
		},
		{
			Name:        "usage",
			Description: "Toggle per-response usage line",
			GetArgumentCompletions: func(prefix string) []SlashCommandCompletion {
				return filterCompletions(usageFooterLevels, prefix)
			},
		},
		{
			Name:        "elevated",
			Description: "Set elevated on/off/ask/full",
			GetArgumentCompletions: func(prefix string) []SlashCommandCompletion {
				return filterCompletions(elevatedLevels, prefix)
			},
		},
		{
			Name:        "elev",
			Description: "Alias for /elevated",
			GetArgumentCompletions: func(prefix string) []SlashCommandCompletion {
				return filterCompletions(elevatedLevels, prefix)
			},
		},
		{
			Name:        "activation",
			Description: "Set group activation",
			GetArgumentCompletions: func(prefix string) []SlashCommandCompletion {
				return filterCompletions(activationLevels, prefix)
			},
		},
		{Name: "abort", Description: "Abort active run"},
		{Name: "new", Description: "Reset the session"},
		{Name: "reset", Description: "Reset the session"},
		{Name: "settings", Description: "Open settings"},
		{Name: "exit", Description: "Exit the TUI"},
		{Name: "quit", Description: "Exit the TUI"},
	}

	// 追加 gateway 自定义命令
	seen := make(map[string]struct{})
	for _, cmd := range commands {
		seen[cmd.Name] = struct{}{}
	}
	gatewayCommands := autoreply.ListChatCommands()
	for _, cmd := range gatewayCommands {
		aliases := cmd.TextAliases
		if len(aliases) == 0 {
			aliases = []string{"/" + cmd.Key}
		}
		for _, alias := range aliases {
			name := strings.TrimSpace(strings.TrimPrefix(alias, "/"))
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			commands = append(commands, SlashCommand{
				Name:        name,
				Description: cmd.Description,
			})
		}
	}

	return commands
}

// filterCompletions 按前缀过滤补全项。
func filterCompletions(levels []string, prefix string) []SlashCommandCompletion {
	lowerPrefix := strings.ToLower(prefix)
	var result []SlashCommandCompletion
	for _, v := range levels {
		if strings.HasPrefix(v, lowerPrefix) {
			result = append(result, SlashCommandCompletion{Value: v, Label: v})
		}
	}
	return result
}

// HelpText 生成帮助文本。
// TS 参考: commands.ts L141-163
func HelpText(opts SlashCommandOptions) string {
	thinkLevels := autoreply.FormatThinkingLevels(opts.Provider, opts.Model, "|")
	return strings.Join([]string{
		"Slash commands:",
		"/help",
		"/commands",
		"/status",
		"/agent <id> (or /agents)",
		"/session <key> (or /sessions)",
		"/model <provider/model> (or /models)",
		fmt.Sprintf("/think <%s>", thinkLevels),
		"/verbose <on|off>",
		"/reasoning <on|off>",
		"/usage <off|tokens|full>",
		"/elevated <on|off|ask|full>",
		"/elev <on|off|ask|full>",
		"/activation <mention|always>",
		"/new or /reset",
		"/abort",
		"/settings",
		"/exit",
	}, "\n")
}

// ---------- 命令处理消息类型 ----------

// CommandResultMsg 命令异步结果消息。
type CommandResultMsg struct {
	SystemMessages []string
	Err            error
}

// ---------- Model 方法: 命令处理器 ----------

// handleCommand 处理 slash 命令。
// TS 参考: tui-command-handlers.ts L240-463
func (m *Model) handleCommand(raw string) tea.Cmd {
	parsed := ParseCommand(raw)
	if parsed.Name == "" {
		return nil
	}

	switch parsed.Name {
	case "help":
		m.chatLog.AddSystem(HelpText(SlashCommandOptions{
			Provider: m.sessionInfo.ModelProvider,
			Model:    m.sessionInfo.Model,
		}))
		return nil

	case "status":
		return m.handleStatusCommand()

	case "agent":
		if parsed.Args == "" {
			return m.openAgentSelectorCmd()
		}
		return m.handleSetAgent(parsed.Args)

	case "agents":
		return m.openAgentSelectorCmd()

	case "session":
		if parsed.Args == "" {
			return m.openSessionSelectorCmd()
		}
		return m.handleSetSession(parsed.Args)

	case "sessions":
		return m.openSessionSelectorCmd()

	case "model":
		if parsed.Args == "" {
			return m.openModelSelectorCmd()
		}
		return m.handlePatchSession("model", parsed.Args)

	case "models":
		return m.openModelSelectorCmd()

	case "think":
		if parsed.Args == "" {
			levels := autoreply.FormatThinkingLevels(
				m.sessionInfo.ModelProvider,
				m.sessionInfo.Model,
				"|",
			)
			m.chatLog.AddSystem(fmt.Sprintf("usage: /think <%s>", levels))
			return nil
		}
		return m.handlePatchSession("thinkingLevel", parsed.Args)

	case "verbose":
		if parsed.Args == "" {
			m.chatLog.AddSystem("usage: /verbose <on|off>")
			return nil
		}
		return m.handlePatchSessionAndReload("verboseLevel", parsed.Args)

	case "reasoning":
		if parsed.Args == "" {
			m.chatLog.AddSystem("usage: /reasoning <on|off>")
			return nil
		}
		return m.handlePatchSession("reasoningLevel", parsed.Args)

	case "usage":
		return m.handleUsageCommand(parsed.Args)

	case "elevated":
		if parsed.Args == "" {
			m.chatLog.AddSystem("usage: /elevated <on|off|ask|full>")
			return nil
		}
		validLevels := map[string]bool{"on": true, "off": true, "ask": true, "full": true}
		if !validLevels[parsed.Args] {
			m.chatLog.AddSystem("usage: /elevated <on|off|ask|full>")
			return nil
		}
		return m.handlePatchSession("elevatedLevel", parsed.Args)

	case "activation":
		if parsed.Args == "" {
			m.chatLog.AddSystem("usage: /activation <mention|always>")
			return nil
		}
		value := "mention"
		if parsed.Args == "always" {
			value = "always"
		}
		return m.handlePatchSession("groupActivation", value)

	case "new", "reset":
		return m.handleResetSession()

	case "abort":
		return m.handleAbortActive()

	case "settings":
		return m.openSettingsSelectorCmd()

	case "exit", "quit":
		m.client.Stop()
		return tea.Quit

	default:
		// 未知命令 → 作为普通消息发送
		return m.sendMessage(raw)
	}
}

// ---------- 命令辅助方法 ----------

// handleStatusCommand 处理 /status 命令。
func (m *Model) handleStatusCommand() tea.Cmd {
	return func() tea.Msg {
		status, err := m.client.GetStatus()
		if err != nil {
			return CommandResultMsg{
				SystemMessages: []string{fmt.Sprintf("status failed: %s", err)},
			}
		}
		if statusMap, ok := status.(map[string]interface{}); ok {
			lines := FormatStatusSummary(statusMap)
			return CommandResultMsg{SystemMessages: lines}
		}
		if statusStr, ok := status.(string); ok {
			return CommandResultMsg{SystemMessages: []string{statusStr}}
		}
		return CommandResultMsg{SystemMessages: []string{"status: unknown response"}}
	}
}

// handleSetAgent 处理 /agent <id> 命令。
func (m *Model) handleSetAgent(id string) tea.Cmd {
	m.currentAgentID = id
	newKey := m.resolveSessionKey("")
	m.currentSessionKey = newKey
	m.chatLog.AddSystem(fmt.Sprintf("agent set to %s, session %s", id, m.formatSessionKey(newKey)))
	return m.refreshAfterSessionChange()
}

// handleSetSession 处理 /session <key> 命令。
func (m *Model) handleSetSession(rawKey string) tea.Cmd {
	newKey := m.resolveSessionKey(rawKey)
	m.currentSessionKey = newKey
	m.chatLog.AddSystem(fmt.Sprintf("session set to %s", m.formatSessionKey(newKey)))
	return m.refreshAfterSessionChange()
}

// buildPatchParams 构建 SessionsPatchParams（通用辅助）。
func buildPatchParams(key, field, value string) SessionsPatchParams {
	p := SessionsPatchParams{Key: key}
	switch field {
	case "thinkingLevel":
		p.ThinkingLevel = &value
	case "verboseLevel":
		p.VerboseLevel = &value
	case "reasoningLevel":
		p.ReasoningLevel = &value
	case "model":
		p.Model = &value
	case "responseUsage":
		p.ResponseUsage = &value
	case "elevatedLevel":
		p.SendPolicy = &value
	case "groupActivation":
		p.Label = &value // 复用 label 字段传 activation（gateway 端解析）
	}
	return p
}

// handlePatchSession 通用 session patch（单字段）。
func (m *Model) handlePatchSession(field, value string) tea.Cmd {
	key := m.currentSessionKey
	return func() tea.Msg {
		params := buildPatchParams(key, field, value)
		_, err := m.client.PatchSession(params)
		if err != nil {
			return CommandResultMsg{
				SystemMessages: []string{fmt.Sprintf("%s failed: %s", field, err)},
			}
		}
		return CommandResultMsg{
			SystemMessages: []string{fmt.Sprintf("%s set to %s", field, value)},
		}
	}
}

// handlePatchSessionAndReload patch session 后重新加载 history。
func (m *Model) handlePatchSessionAndReload(field, value string) tea.Cmd {
	key := m.currentSessionKey
	return func() tea.Msg {
		params := buildPatchParams(key, field, value)
		_, err := m.client.PatchSession(params)
		if err != nil {
			return CommandResultMsg{
				SystemMessages: []string{fmt.Sprintf("%s failed: %s", field, err)},
			}
		}
		return CommandResultMsg{
			SystemMessages: []string{fmt.Sprintf("%s set to %s", field, value)},
		}
	}
}

// handleUsageCommand 处理 /usage 命令。
// TS 参考: tui-command-handlers.ts L369-391
func (m *Model) handleUsageCommand(args string) tea.Cmd {
	if args != "" {
		normalized, ok := autoreply.NormalizeUsageDisplay(args)
		if !ok {
			m.chatLog.AddSystem("usage: /usage <off|tokens|full>")
			return nil
		}
		return m.handlePatchSession("responseUsage", string(normalized))
	}

	// 无参数 → 循环切换 off → tokens → full → off
	current := autoreply.ResolveResponseUsageMode(m.sessionInfo.ResponseUsage)
	var next string
	switch current {
	case autoreply.UsageOff:
		next = string(autoreply.UsageTokens)
	case autoreply.UsageTokens:
		next = string(autoreply.UsageFull)
	default:
		next = string(autoreply.UsageOff)
	}

	value := next
	if next == string(autoreply.UsageOff) {
		value = "" // null in TS → empty string to clear
	}

	key := m.currentSessionKey
	return func() tea.Msg {
		params := SessionsPatchParams{Key: key, ResponseUsage: &value}
		_, err := m.client.PatchSession(params)
		if err != nil {
			return CommandResultMsg{
				SystemMessages: []string{fmt.Sprintf("usage failed: %s", err)},
			}
		}
		return CommandResultMsg{
			SystemMessages: []string{fmt.Sprintf("usage footer: %s", next)},
		}
	}
}

// handleResetSession 处理 /new 和 /reset 命令。
// TS 参考: tui-command-handlers.ts L430-445
func (m *Model) handleResetSession() tea.Cmd {
	// 立即清除 token 计数
	m.sessionInfo.InputTokens = nil
	m.sessionInfo.OutputTokens = nil
	m.sessionInfo.TotalTokens = nil

	return func() tea.Msg {
		err := m.client.ResetSession(m.currentSessionKey)
		if err != nil {
			return CommandResultMsg{
				SystemMessages: []string{fmt.Sprintf("reset failed: %s", err)},
			}
		}
		return CommandResultMsg{
			SystemMessages: []string{
				fmt.Sprintf("session %s reset", m.currentSessionKey),
			},
		}
	}
}

// handleAbortActive 中止活动 run。
func (m *Model) handleAbortActive() tea.Cmd {
	if m.activeChatRunID == "" {
		m.chatLog.AddSystem("no active run to abort")
		return nil
	}
	runID := m.activeChatRunID
	sessionKey := m.currentSessionKey
	m.activeChatRunID = ""
	m.activityStatus = "aborted"

	return func() tea.Msg {
		err := m.client.AbortChat(sessionKey, runID)
		if err != nil {
			return CommandResultMsg{
				SystemMessages: []string{fmt.Sprintf("abort failed: %s", err)},
			}
		}
		return CommandResultMsg{
			SystemMessages: []string{"run aborted"},
		}
	}
}

// ---------- 消息发送 ----------

// sendMessage 发送普通消息到 gateway。
// TS 参考: tui-command-handlers.ts L465-491
func (m *Model) sendMessage(text string) tea.Cmd {
	m.chatLog.AddUser(text)
	runID := uuid.New().String()
	m.noteLocalRunID(runID)
	m.activeChatRunID = runID
	m.activityStatus = "sending"

	sessionKey := m.currentSessionKey
	thinking := m.opts.Thinking
	deliver := m.opts.Deliver
	timeoutMs := m.opts.TimeoutMs

	return func() tea.Msg {
		_, err := m.client.SendChat(ChatSendOptions{
			SessionKey: sessionKey,
			Message:    text,
			Thinking:   thinking,
			Deliver:    deliver,
			TimeoutMs:  timeoutMs,
			RunID:      runID,
		})
		if err != nil {
			return CommandResultMsg{
				SystemMessages: []string{fmt.Sprintf("send failed: %s", err)},
				Err:            err,
			}
		}
		return CommandResultMsg{}
	}
}

// refreshAfterSessionChange 刷新 session 后的通用操作。
func (m *Model) refreshAfterSessionChange() tea.Cmd {
	m.syncSessionKey()
	m.historyLoaded = false
	m.chatLog.ClearAll()
	// History 和 session info 会在下次事件循环中刷新
	return nil
}
