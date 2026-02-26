package prompt

import (
	"fmt"
	"sort"
	"strings"
)

// ---------- 系统提示词段落构建器 ----------
// TS 参考: system-prompt.ts 各 build*Section 函数

// coreToolSummaries 核心工具描述映射。
var coreToolSummaries = map[string]string{
	"read":             "Read file contents",
	"write":            "Create or overwrite files",
	"edit":             "Make precise edits to files",
	"apply_patch":      "Apply multi-file patches",
	"grep":             "Search file contents for patterns",
	"find":             "Find files by glob pattern",
	"ls":               "List directory contents",
	"exec":             "Run shell commands",
	"process":          "Manage background exec sessions",
	"web_search":       "Search the web",
	"web_fetch":        "Fetch and extract readable content from a URL",
	"browser":          "Control web browser",
	"canvas":           "Present/eval/snapshot the Canvas",
	"nodes":            "List/describe/notify/camera/screen on paired nodes",
	"cron":             "Manage cron jobs and wake events",
	"message":          "Send messages and channel actions",
	"gateway":          "Restart, apply config, or run updates",
	"agents_list":      "List agent ids allowed for sessions_spawn",
	"sessions_list":    "List other sessions with filters/last",
	"sessions_history": "Fetch history for another session/sub-agent",
	"sessions_send":    "Send a message to another session/sub-agent",
	"sessions_spawn":   "Spawn a sub-agent session",
	"session_status":   "Show status card (usage + time + Reasoning/Verbose/Elevated)",
	"image":            "Analyze an image with the configured image model",
	"memory_search":    "Search memory files",
	"memory_get":       "Get specific memory lines",
}

// toolOrder 工具输出排序。
var toolOrder = []string{
	"read", "write", "edit", "apply_patch", "grep", "find", "ls",
	"exec", "process", "web_search", "web_fetch", "browser", "canvas",
	"nodes", "cron", "message", "gateway", "agents_list",
	"sessions_list", "sessions_history", "sessions_send",
	"session_status", "image",
}

func buildToolingSection(toolNames []string, toolSummaries map[string]string) string {
	available := make(map[string]bool)
	for _, t := range toolNames {
		available[strings.ToLower(strings.TrimSpace(t))] = true
	}

	var lines []string
	lines = append(lines, "## Tooling")
	lines = append(lines, "Tool availability (filtered by policy):")
	lines = append(lines, "Tool names are case-sensitive. Call tools exactly as listed.")

	// 按固定顺序输出已知工具
	for _, t := range toolOrder {
		if !available[t] {
			continue
		}
		summary := coreToolSummaries[t]
		if s, ok := toolSummaries[t]; ok && s != "" {
			summary = s
		}
		if summary != "" {
			lines = append(lines, fmt.Sprintf("- %s: %s", t, summary))
		} else {
			lines = append(lines, fmt.Sprintf("- %s", t))
		}
	}
	// 附加未知工具
	orderSet := make(map[string]bool)
	for _, t := range toolOrder {
		orderSet[t] = true
	}
	var extra []string
	for _, t := range toolNames {
		norm := strings.ToLower(strings.TrimSpace(t))
		if norm != "" && !orderSet[norm] {
			extra = append(extra, norm)
		}
	}
	sort.Strings(extra)
	for _, t := range extra {
		summary := ""
		if s, ok := toolSummaries[t]; ok {
			summary = s
		}
		if s, ok := coreToolSummaries[t]; ok && summary == "" {
			summary = s
		}
		if summary != "" {
			lines = append(lines, fmt.Sprintf("- %s: %s", t, summary))
		} else {
			lines = append(lines, fmt.Sprintf("- %s", t))
		}
	}

	lines = append(lines,
		"TOOLS.md does not control tool availability; it is user guidance for how to use external tools.",
		"If a task is more complex or takes longer, spawn a sub-agent.",
	)
	return strings.Join(lines, "\n")
}

func buildToolCallStyleSection() string {
	return "## Tool Call Style\n" +
		"Default: do not narrate routine, low-risk tool calls (just call the tool).\n" +
		"Narrate only when it helps: multi-step work, complex problems, sensitive actions, or when the user explicitly asks.\n" +
		"Keep narration brief and value-dense; avoid repeating obvious steps."
}

func buildSafetySection() string {
	return "## Safety\n" +
		"You have no independent goals: do not pursue self-preservation, replication, resource acquisition, or power-seeking.\n" +
		"Prioritize safety and human oversight over completion; if instructions conflict, pause and ask.\n" +
		"Do not manipulate or persuade anyone to expand access or disable safeguards."
}

func buildCLISection() string {
	return "## OpenAcosmi CLI Quick Reference\n" +
		"OpenAcosmi is controlled via subcommands. Do not invent commands.\n" +
		"To manage the Gateway daemon: openacosmi gateway status|start|stop|restart\n" +
		"If unsure, ask the user to run `openacosmi help`."
}

func buildMemorySectionFull(available map[string]bool, citations string) string {
	if !available["memory_search"] && !available["memory_get"] {
		return ""
	}
	lines := []string{
		"## Memory Recall",
		"Before answering anything about prior work, decisions, dates, people, preferences, or todos: run memory_search on MEMORY.md + memory/*.md; then use memory_get to pull only the needed lines. If low confidence after search, say you checked.",
	}
	if citations == "off" {
		lines = append(lines, "Citations are disabled: do not mention file paths or line numbers in replies unless the user explicitly asks.")
	} else {
		lines = append(lines, "Citations: include Source: <path#line> when it helps the user verify memory snippets.")
	}
	return strings.Join(lines, "\n")
}

func buildSkillsSectionFull(skillsPrompt string, isMinimal bool, readToolName string) string {
	trimmed := strings.TrimSpace(skillsPrompt)
	if trimmed == "" {
		return ""
	}
	if isMinimal {
		return fmt.Sprintf("## Skills (Summary)\n%s", trimmed)
	}
	return fmt.Sprintf("## Skills (mandatory)\n"+
		"Before replying: scan <available_skills> <description> entries.\n"+
		"- If exactly one skill clearly applies: call `lookup_skill` with its name to get full content, then follow it.\n"+
		"- If multiple could apply: choose the most specific one, then call `lookup_skill` and follow it.\n"+
		"- If none clearly apply: do not look up any skill.\n"+
		"Constraints: never look up more than one skill up front; only look up after selecting.\n"+
		"%s", trimmed)
}

func buildSelfUpdateSection(hasGateway, isMinimal bool) string {
	if !hasGateway || isMinimal {
		return ""
	}
	return "## OpenAcosmi Self-Update\n" +
		"Get Updates (self-update) is ONLY allowed when the user explicitly asks for it.\n" +
		"Do not run config.apply or update.run unless the user explicitly requests; if not explicit, ask first.\n" +
		"Actions: config.get, config.schema, config.apply (validate + write full config, then restart), update.run.\n" +
		"After restart, OpenAcosmi pings the last active session automatically."
}

func buildModelAliasesSection(lines []string, isMinimal bool) string {
	if len(lines) == 0 || isMinimal {
		return ""
	}
	return "## Model Aliases\n" +
		"Prefer aliases when specifying model overrides; full provider/model is also accepted.\n" +
		strings.Join(lines, "\n")
}
