package autoreply

import (
	"strings"
)

// TS 对照: auto-reply/skill-commands.ts (141L)

// ---------- DI 接口 ----------

// SkillCommandDeps 技能命令外部依赖。
type SkillCommandDeps interface {
	// BuildWorkspaceSkillSpecs 构建工作区技能规格列表。
	BuildWorkspaceSkillSpecs(workspaceDir string) []SkillCommandSpec
	// ListAgentIDs 列出所有 Agent ID。
	ListAgentIDs() []string
	// ResolveAgentWorkspaceDir 解析 Agent 的工作区目录。
	ResolveAgentWorkspaceDir(agentID string) string
}

// ---------- 类型定义 ----------

// SkillCommandSpec 技能命令规格。
// TS 对照: skill-commands.ts SkillCommandSpec
type SkillCommandSpec struct {
	Name        string
	Description string
	WorkspaceID string
	AgentID     string
	IsRemote    bool
}

// ---------- 名称标准化 ----------

// NormalizeSkillCommandLookup 标准化技能命令名称用于查找。
// 小写 + 空格/下划线 → 连字符。
// TS 对照: skill-commands.ts normalizeSkillCommandLookup
func NormalizeSkillCommandLookup(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, " ", "-")
	n = strings.ReplaceAll(n, "_", "-")
	return n
}

// ---------- 保留名称解析 ----------

// ResolveReservedCommandNames 从命令注册表提取保留名称集合。
// TS 对照: skill-commands.ts resolveReservedCommandNames
func ResolveReservedCommandNames() map[string]struct{} {
	reserved := make(map[string]struct{})
	for _, cmd := range ListChatCommands() {
		if cmd.NativeName != "" {
			reserved[strings.ToLower(cmd.NativeName)] = struct{}{}
		}
		for _, alias := range cmd.TextAliases {
			trimmed := strings.TrimSpace(alias)
			if !strings.HasPrefix(trimmed, "/") {
				continue
			}
			reserved[strings.ToLower(trimmed[1:])] = struct{}{}
		}
	}
	return reserved
}

// ---------- 技能命令发现 ----------

// ListSkillCommandsForWorkspace 单工作区技能命令发现。
// 过滤掉与内置命令同名的技能。
// TS 对照: skill-commands.ts listSkillCommandsForWorkspace
func ListSkillCommandsForWorkspace(deps SkillCommandDeps, workspaceDir string) []SkillCommandSpec {
	if deps == nil || workspaceDir == "" {
		return nil
	}
	specs := deps.BuildWorkspaceSkillSpecs(workspaceDir)
	if len(specs) == 0 {
		return nil
	}

	reserved := ResolveReservedCommandNames()
	var result []SkillCommandSpec
	for _, spec := range specs {
		normalized := NormalizeSkillCommandLookup(spec.Name)
		if _, isReserved := reserved[normalized]; isReserved {
			continue
		}
		result = append(result, spec)
	}
	return result
}

// ListSkillCommandsForAgents 多 Agent 技能命令发现（去重）。
// TS 对照: skill-commands.ts listSkillCommandsForAgents
func ListSkillCommandsForAgents(deps SkillCommandDeps) []SkillCommandSpec {
	if deps == nil {
		return nil
	}
	agentIDs := deps.ListAgentIDs()
	if len(agentIDs) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	var result []SkillCommandSpec
	for _, agentID := range agentIDs {
		wsDir := deps.ResolveAgentWorkspaceDir(agentID)
		if wsDir == "" {
			continue
		}
		specs := ListSkillCommandsForWorkspace(deps, wsDir)
		for _, spec := range specs {
			key := NormalizeSkillCommandLookup(spec.Name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			spec.AgentID = agentID
			result = append(result, spec)
		}
	}
	return result
}

// ---------- 查找 + 调用解析 ----------

// FindSkillCommand 从列表中查找技能命令（模糊匹配）。
// TS 对照: skill-commands.ts findSkillCommand
func FindSkillCommand(specs []SkillCommandSpec, name string) *SkillCommandSpec {
	if name == "" || len(specs) == 0 {
		return nil
	}
	normalized := NormalizeSkillCommandLookup(name)
	for i := range specs {
		if NormalizeSkillCommandLookup(specs[i].Name) == normalized {
			return &specs[i]
		}
	}
	return nil
}

// SkillCommandInvocation 技能命令调用解析结果。
type SkillCommandInvocation struct {
	Spec *SkillCommandSpec
	Args string
}

// ResolveSkillCommandInvocation 解析技能命令调用。
// 支持："/skill <name> [args]" 和 "/<skillName> [args]"。
// TS 对照: skill-commands.ts resolveSkillCommandInvocation
func ResolveSkillCommandInvocation(specs []SkillCommandSpec, body string) *SkillCommandInvocation {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	// 去掉前导 /
	rest := trimmed[1:]

	// "/skill <name> [args]" 格式
	if strings.HasPrefix(strings.ToLower(rest), "skill ") {
		afterSkill := strings.TrimSpace(rest[6:])
		spaceIdx := strings.IndexByte(afterSkill, ' ')
		name := afterSkill
		args := ""
		if spaceIdx >= 0 {
			name = afterSkill[:spaceIdx]
			args = strings.TrimSpace(afterSkill[spaceIdx+1:])
		}
		spec := FindSkillCommand(specs, name)
		if spec != nil {
			return &SkillCommandInvocation{Spec: spec, Args: args}
		}
		return nil
	}

	// "/<skillName> [args]" 格式
	spaceIdx := strings.IndexByte(rest, ' ')
	cmdName := rest
	args := ""
	if spaceIdx >= 0 {
		cmdName = rest[:spaceIdx]
		args = strings.TrimSpace(rest[spaceIdx+1:])
	}

	spec := FindSkillCommand(specs, cmdName)
	if spec != nil {
		return &SkillCommandInvocation{Spec: spec, Args: args}
	}
	return nil
}
