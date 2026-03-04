package config

// Agent 目录管理 — 对应 src/config/agent-dirs.ts (113 行)
//
// 检测多 agent 配置中的目录冲突（多个 agent 共享同一 agentDir 会导致
// auth/session 状态碰撞和 token 失效）。
//
// 依赖:
//   - paths.go: resolveStateDir, resolveUserPath
//   - 内联: normalizeAgentId (来自 src/routing/session-key.ts)
// npm 依赖: 无

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// DefaultAgentID 默认 agent ID
const DefaultAgentID = "main"

// agent ID 正则
var (
	validIDRE      = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	invalidCharsRE = regexp.MustCompile(`[^a-z0-9_-]+`)
	leadingDashRE  = regexp.MustCompile(`^-+`)
	trailingDashRE = regexp.MustCompile(`-+$`)
)

// DuplicateAgentDir 重复的 agent 目录信息
type DuplicateAgentDir struct {
	AgentDir string
	AgentIDs []string
}

// DuplicateAgentDirError 重复 agent 目录错误
type DuplicateAgentDirError struct {
	Duplicates []DuplicateAgentDir
}

func (e *DuplicateAgentDirError) Error() string {
	return FormatDuplicateAgentDirError(e.Duplicates)
}

// NormalizeAgentID 规范化 agent ID (路径安全 + shell 友好)
// 对应 TS: normalizeAgentId(value)
func NormalizeAgentID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultAgentID
	}
	if validIDRE.MatchString(strings.ToLower(trimmed)) {
		return strings.ToLower(trimmed)
	}
	// 最佳努力: 将无效字符替换为 "-"
	result := strings.ToLower(trimmed)
	result = invalidCharsRE.ReplaceAllString(result, "-")
	result = leadingDashRE.ReplaceAllString(result, "")
	result = trailingDashRE.ReplaceAllString(result, "")
	if len(result) > 64 {
		result = result[:64]
	}
	if result == "" {
		return DefaultAgentID
	}
	return result
}

// canonicalizeAgentDir 规范化 agent 目录路径（用于比较）
func canonicalizeAgentDir(agentDir string) string {
	resolved, err := filepath.Abs(agentDir)
	if err != nil {
		resolved = agentDir
	}
	// macOS 和 Windows 大小写不敏感
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.ToLower(resolved)
	}
	return resolved
}

// collectReferencedAgentIDs 从配置中收集所有引用的 agent ID
func collectReferencedAgentIDs(cfg *types.OpenAcosmiConfig) []string {
	seen := make(map[string]bool)
	var ids []string

	addID := func(id string) {
		normalized := NormalizeAgentID(id)
		if !seen[normalized] {
			seen[normalized] = true
			ids = append(ids, normalized)
		}
	}

	// 从 agents 列表收集
	agentsList := cfg.Agents != nil && len(cfg.Agents.List) > 0
	if agentsList {
		// 默认 agent: 第一个 default=true 的 agent, 否则第一个
		defaultID := DefaultAgentID
		for _, agent := range cfg.Agents.List {
			if agent.Default != nil && *agent.Default {
				defaultID = agent.ID
				break
			}
		}
		if defaultID == DefaultAgentID && len(cfg.Agents.List) > 0 && cfg.Agents.List[0].ID != "" {
			defaultID = cfg.Agents.List[0].ID
		}
		addID(defaultID)

		for _, agent := range cfg.Agents.List {
			if agent.ID != "" {
				addID(agent.ID)
			}
		}
	} else {
		addID(DefaultAgentID)
	}

	// 从 bindings 收集
	for _, binding := range cfg.Bindings {
		if id := strings.TrimSpace(binding.AgentID); id != "" {
			addID(id)
		}
	}

	return ids
}

// resolveEffectiveAgentDir 解析 agent 的有效目录
func resolveEffectiveAgentDir(cfg *types.OpenAcosmiConfig, agentID string) string {
	id := NormalizeAgentID(agentID)

	if cfg.Agents != nil {
		for _, agent := range cfg.Agents.List {
			if NormalizeAgentID(agent.ID) == id {
				dir := strings.TrimSpace(agent.AgentDir)
				if dir != "" {
					return resolveUserPath(dir)
				}
			}
		}
	}

	// 默认: stateDir/agents/<id>/agent
	stateDir := ResolveStateDir()
	return filepath.Join(stateDir, "agents", id, "agent")
}

// FindDuplicateAgentDirs 检测配置中的重复 agent 目录
// 对应 TS: findDuplicateAgentDirs(cfg, deps)
func FindDuplicateAgentDirs(cfg *types.OpenAcosmiConfig) []DuplicateAgentDir {
	type entry struct {
		agentDir string
		agentIDs []string
	}
	byDir := make(map[string]*entry)

	for _, agentID := range collectReferencedAgentIDs(cfg) {
		agentDir := resolveEffectiveAgentDir(cfg, agentID)
		key := canonicalizeAgentDir(agentDir)
		if e, ok := byDir[key]; ok {
			e.agentIDs = append(e.agentIDs, agentID)
		} else {
			byDir[key] = &entry{agentDir: agentDir, agentIDs: []string{agentID}}
		}
	}

	var duplicates []DuplicateAgentDir
	for _, e := range byDir {
		if len(e.agentIDs) > 1 {
			duplicates = append(duplicates, DuplicateAgentDir{
				AgentDir: e.agentDir,
				AgentIDs: e.agentIDs,
			})
		}
	}
	return duplicates
}

// FormatDuplicateAgentDirError 格式化重复 agent 目录错误消息
func FormatDuplicateAgentDirError(dups []DuplicateAgentDir) string {
	lines := []string{
		"Duplicate agentDir detected (multi-agent config).",
		"Each agent must have a unique agentDir; sharing it causes auth/session state collisions and token invalidation.",
		"",
		"Conflicts:",
	}
	for _, d := range dups {
		quoted := make([]string, len(d.AgentIDs))
		for i, id := range d.AgentIDs {
			quoted[i] = fmt.Sprintf("%q", id)
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", d.AgentDir, strings.Join(quoted, ", ")))
	}
	lines = append(lines, "")
	lines = append(lines, "Fix: remove the shared agents.list[].agentDir override (or give each agent its own directory).")
	lines = append(lines, "If you want to share credentials, copy auth-profiles.json instead of sharing the entire agentDir.")
	return strings.Join(lines, "\n")
}
