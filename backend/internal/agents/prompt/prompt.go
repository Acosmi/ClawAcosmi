package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ---------- 系统提示词构建器 ----------

// TS 参考: src/agents/system-prompt.ts (649 行) + system-prompt-params.ts (116 行)

// PromptMode 控制提示词中包含的硬编码段落级别。
type PromptMode string

const (
	PromptModeFull    PromptMode = "full"    // 所有段落（主 agent）
	PromptModeMinimal PromptMode = "minimal" // 精简段落（子 agent）
	PromptModeNone    PromptMode = "none"    // 仅基础身份行
)

// SilentReplyToken 静默回复标记。TS 对应: auto-reply/tokens.ts → SILENT_REPLY_TOKEN
const SilentReplyToken = "NO_REPLY"

// ---------- 新增类型 ----------

// SandboxInfo 沙箱环境信息。
type SandboxInfo struct {
	Enabled             bool
	WorkspaceDir        string
	WorkspaceAccess     string // "none"|"ro"|"rw"
	AgentWorkspaceMount string
	BrowserBridgeURL    string
	BrowserNoVncURL     string
	HostBrowserAllowed  *bool // nil=unknown
	Elevated            *SandboxElevated
}

// SandboxElevated 沙箱提权配置。
type SandboxElevated struct {
	Allowed      bool
	DefaultLevel string // "on"|"off"|"ask"|"full"
}

// ContextFile 注入的上下文文件。
type ContextFile struct {
	Path    string
	Content string
}

// ReactionGuidance 反应指导配置。
type ReactionGuidance struct {
	Level   string // "minimal"|"extensive"
	Channel string
}

// ---------- Runtime 参数 ----------

// RuntimeInfo 运行时信息。
// TS 参考: system-prompt-params.ts → RuntimeInfoInput
type RuntimeInfo struct {
	AgentID      string   `json:"agentId,omitempty"`
	Host         string   `json:"host"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	GoVersion    string   `json:"goVersion"`
	Model        string   `json:"model"`
	DefaultModel string   `json:"defaultModel,omitempty"`
	Shell        string   `json:"shell,omitempty"`
	Channel      string   `json:"channel,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	RepoRoot     string   `json:"repoRoot,omitempty"`
}

// SystemPromptParams 系统提示词构建参数。
type SystemPromptParams struct {
	RuntimeInfo  RuntimeInfo
	UserTimezone string
	UserTime     string
}

// BuildSystemPromptParams 构建系统提示词运行时参数。
// TS 参考: system-prompt-params.ts → buildSystemPromptParams()
func BuildSystemPromptParams(agentID string, rt RuntimeInfo, workspaceDir, cwd, userTimezone string) SystemPromptParams {
	// 解析 repo root
	repoRoot := resolveRepoRoot(workspaceDir, cwd)
	if repoRoot != "" {
		rt.RepoRoot = repoRoot
	}
	rt.AgentID = agentID

	// 时区
	tz := userTimezone
	if tz == "" {
		tz = resolveLocalTimezone()
	}

	// 当前时间
	userTime := formatUserTime(time.Now(), tz)

	return SystemPromptParams{
		RuntimeInfo:  rt,
		UserTimezone: tz,
		UserTime:     userTime,
	}
}

// ---------- 提示词构建 ----------

// BuildParams 构建系统提示词的完整参数集。
type BuildParams struct {
	Mode              PromptMode
	WorkspaceDir      string
	ExtraSystemPrompt string            // 用户自定义追加
	SkillsPrompt      string            // 技能注入段
	OwnerLine         string            // 用户身份行
	ToolNames         []string          // 可用工具名称
	ToolSummaries     map[string]string // 工具名→描述映射
	ModelAliasLines   []string          // 模型别名行
	HeartbeatPrompt   string            // 心跳提示词
	DocsPath          string            // 文档路径
	WorkspaceNotes    []string          // 工作区备注
	TTSHint           string            // TTS 提示
	SandboxInfo       *SandboxInfo      // 沙箱信息
	ContextFiles      []ContextFile     // 注入的上下文文件
	ReasoningTagHint  bool              // 是否启用 <think>/<final> 标签
	ReasoningLevel    string            // off|on|stream
	MessageToolHints  []string          // 消息工具附加提示
	ReactionGuidance  *ReactionGuidance // 反应指导
	MemoryCitations   string            // off|on
	RuntimeInfo       *RuntimeInfo
	UserTimezone      string
	ThinkLevel        string // "off"|"low"|"medium"|"high"
}

// BuildAgentSystemPrompt 构建 Agent 系统提示词。
// TS 参考: system-prompt.ts → buildAgentSystemPrompt() (649L)
func BuildAgentSystemPrompt(params BuildParams) string {
	mode := params.Mode
	if mode == "" {
		mode = PromptModeFull
	}
	isMinimal := mode == PromptModeMinimal || mode == PromptModeNone

	// "none" 模式: 仅返回身份行
	if mode == PromptModeNone {
		return "You are a personal assistant running inside OpenAcosmi."
	}

	// 构建可用工具集
	available := make(map[string]bool)
	for _, t := range params.ToolNames {
		available[strings.ToLower(strings.TrimSpace(t))] = true
	}
	hasGateway := available["gateway"]
	readToolName := "read"

	var sections []string
	add := func(s string) {
		if s != "" {
			sections = append(sections, s)
		}
	}

	// 1. 身份行
	add("You are a personal assistant running inside OpenAcosmi.")
	// 2. Tooling
	add(buildToolingSection(params.ToolNames, params.ToolSummaries))
	// 3. Tool Call Style
	add(buildToolCallStyleSection())
	// 4. Safety
	add(buildSafetySection())
	// 5. CLI Quick Reference
	add(buildCLISection())
	// 6. Skills
	add(buildSkillsSectionFull(params.SkillsPrompt, isMinimal, readToolName))
	// 7. Memory Recall
	add(buildMemorySectionFull(available, params.MemoryCitations))
	// 8. Self-Update
	add(buildSelfUpdateSection(hasGateway, isMinimal))
	// 9. Model Aliases
	add(buildModelAliasesSection(params.ModelAliasLines, isMinimal))
	// 10. Workspace
	if params.WorkspaceDir != "" {
		ws := fmt.Sprintf("## Workspace\nYour working directory is: %s\nTreat this directory as the single global workspace.", params.WorkspaceDir)
		for _, note := range params.WorkspaceNotes {
			if n := strings.TrimSpace(note); n != "" {
				ws += "\n" + n
			}
		}
		add(ws)
	}
	// 11. Docs
	add(buildDocsSection(params.DocsPath, isMinimal))
	// 12. Sandbox
	add(buildSandboxSection(params.SandboxInfo))
	// 13. User Identity
	if params.OwnerLine != "" && !isMinimal {
		add(buildUserIdentitySection(params.OwnerLine))
	}
	// 14. Time
	if params.UserTimezone != "" {
		add(buildTimeSection(params.UserTimezone))
	}
	// 14b. Workspace Files (injected) — TS L504-506
	if len(params.ContextFiles) > 0 {
		add("## Workspace Files (injected)\n" +
			"These user-editable files are loaded by OpenAcosmi and included below in Project Context.")
	}
	// 15. Reply Tags
	add(buildReplyTagsSection(isMinimal))
	// 16. Messaging
	add(buildMessagingSection(isMinimal, available, params.MessageToolHints))
	// 17. Voice/TTS
	add(buildVoiceSection(isMinimal, params.TTSHint))

	// Extra system prompt
	if params.ExtraSystemPrompt != "" {
		header := "## Group Chat Context"
		if mode == PromptModeMinimal {
			header = "## Subagent Context"
		}
		add(header + "\n" + params.ExtraSystemPrompt)
	}

	// Reactions
	add(buildReactionsSection(params.ReactionGuidance))
	// Reasoning Format
	add(buildReasoningFormatSection(params.ReasoningTagHint))
	// Context Files
	add(buildContextFilesSection(params.ContextFiles))

	// Silent Replies (full mode only)
	if !isMinimal {
		add(buildSilentRepliesSection())
	}
	// Heartbeats (full mode only)
	if !isMinimal {
		add(buildHeartbeatsSection(params.HeartbeatPrompt))
	}

	// Runtime (always last)
	if params.RuntimeInfo != nil {
		add(buildRuntimeLine(params.RuntimeInfo, params.ThinkLevel))
	}
	reasoningLevel := params.ReasoningLevel
	if reasoningLevel == "" {
		reasoningLevel = "off"
	}
	add(fmt.Sprintf("Reasoning: %s (hidden unless on/stream).", reasoningLevel))

	return joinSections(sections)
}

// ---------- 段落构建器 ----------

func buildUserIdentitySection(ownerLine string) string {
	return fmt.Sprintf("## User Identity\n%s", ownerLine)
}

func buildTimeSection(timezone string) string {
	t := time.Now()
	loc, err := time.LoadLocation(timezone)
	if err == nil {
		t = t.In(loc)
	}
	return fmt.Sprintf("## Current Time\n%s (%s)", t.Format("2006-01-02 15:04:05"), timezone)
}

func buildRuntimeLine(rt *RuntimeInfo, thinkLevel string) string {
	parts := []string{}
	if rt.Host != "" {
		parts = append(parts, fmt.Sprintf("Host: %s", rt.Host))
	}
	if rt.OS != "" {
		parts = append(parts, fmt.Sprintf("OS: %s/%s", rt.OS, rt.Arch))
	}
	if rt.Model != "" {
		parts = append(parts, fmt.Sprintf("Model: %s", rt.Model))
	}
	if rt.Shell != "" {
		parts = append(parts, fmt.Sprintf("Shell: %s", rt.Shell))
	}
	if rt.RepoRoot != "" {
		parts = append(parts, fmt.Sprintf("Repo: %s", rt.RepoRoot))
	}
	if thinkLevel != "" && thinkLevel != "off" {
		parts = append(parts, fmt.Sprintf("Thinking: %s", thinkLevel))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("## Runtime\n%s", strings.Join(parts, " | "))
}

// ---------- RepoRoot 解析 ----------

// ResolveRepoRoot 从配置和工作路径中解析 Git 仓库根目录。
// TS 参考: system-prompt-params.ts → resolveRepoRoot()
func resolveRepoRoot(workspaceDir, cwd string) string {
	candidates := []string{workspaceDir, cwd}
	seen := make(map[string]bool)

	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		root := FindGitRoot(abs)
		if root != "" {
			return root
		}
	}
	return ""
}

// FindGitRoot 向上查找 .git 目录。
// TS 参考: system-prompt-params.ts → findGitRoot()
func FindGitRoot(startDir string) string {
	current, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}
	for i := 0; i < 12; i++ {
		gitPath := filepath.Join(current, ".git")
		info, err := os.Stat(gitPath)
		if err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}

func resolveLocalTimezone() string {
	return time.Now().Location().String()
}

func formatUserTime(t time.Time, timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return t.Format("2006-01-02 15:04:05")
	}
	return t.In(loc).Format("2006-01-02 15:04:05")
}

func joinSections(sections []string) string {
	return strings.Join(sections, "\n\n")
}

// DefaultRuntimeInfo 创建带默认值的运行时信息。
func DefaultRuntimeInfo() RuntimeInfo {
	hostname, _ := os.Hostname()
	shell := os.Getenv("SHELL")
	return RuntimeInfo{
		Host:      hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
		Shell:     shell,
	}
}
