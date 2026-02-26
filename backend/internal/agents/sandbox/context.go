// context.go — 沙箱运行时状态、工具策略与上下文编排。
//
// TS 对照: agents/sandbox/runtime-status.ts (139L),
//
//	agents/sandbox/tool-policy.ts (143L),
//	agents/sandbox/context.ts (156L)
//
// 运行时状态判定、工具策略编译与匹配、
// 沙箱上下文（容器 + 浏览器 + 工作区）完整编排。
package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ---------- 工具策略 ----------

// CompiledToolPolicy 已编译的工具策略（用于快速匹配）。
// TS 对照: tool-policy.ts CompiledToolPolicy
type CompiledToolPolicy struct {
	AllowExact    map[string]bool
	AllowPrefixes []string
	AllowAll      bool
	DenyExact     map[string]bool
	DenyPrefixes  []string
	DenyAll       bool
}

// CompileToolPolicy 编译工具策略为快速匹配结构。
// TS 对照: tool-policy.ts compileToolPolicy()
func CompileToolPolicy(policy SandboxToolPolicy) *CompiledToolPolicy {
	compiled := &CompiledToolPolicy{
		AllowExact: make(map[string]bool),
		DenyExact:  make(map[string]bool),
	}

	for _, pattern := range policy.Allow {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == "*" {
			compiled.AllowAll = true
			continue
		}
		if strings.HasSuffix(pattern, "*") {
			compiled.AllowPrefixes = append(compiled.AllowPrefixes, strings.TrimSuffix(pattern, "*"))
		} else {
			compiled.AllowExact[pattern] = true
		}
	}

	for _, pattern := range policy.Deny {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == "*" {
			compiled.DenyAll = true
			continue
		}
		if strings.HasSuffix(pattern, "*") {
			compiled.DenyPrefixes = append(compiled.DenyPrefixes, strings.TrimSuffix(pattern, "*"))
		} else {
			compiled.DenyExact[pattern] = true
		}
	}

	return compiled
}

// IsToolAllowed 检查工具是否被策略允许。
// 逻辑：先检查 deny（deny 优先），再检查 allow。
// TS 对照: tool-policy.ts isToolAllowed()
func IsToolAllowed(compiled *CompiledToolPolicy, toolName string) bool {
	if compiled == nil {
		return true
	}

	// 检查 deny
	if compiled.DenyAll {
		return false
	}
	if compiled.DenyExact[toolName] {
		return false
	}
	for _, prefix := range compiled.DenyPrefixes {
		if strings.HasPrefix(toolName, prefix) {
			return false
		}
	}

	// 检查 allow
	if compiled.AllowAll {
		return true
	}
	if compiled.AllowExact[toolName] {
		return true
	}
	for _, prefix := range compiled.AllowPrefixes {
		if strings.HasPrefix(toolName, prefix) {
			return true
		}
	}

	// 没有匹配任何 allow → 默认拒绝（如果有 allow 规则）
	if len(compiled.AllowExact) > 0 || len(compiled.AllowPrefixes) > 0 {
		return false
	}

	// 没有 allow 规则 → 默认允许
	return true
}

// ResolveToolPolicyForAgent 为 agent 解析合并后的工具策略。
// TS 对照: tool-policy.ts resolveToolPolicyForAgent()
func ResolveToolPolicyForAgent(global SandboxConfig, agentOverride *SandboxConfig) SandboxToolPolicy {
	result := SandboxToolPolicy{
		Allow: global.Tools.Allow,
		Deny:  global.Tools.Deny,
	}

	// 应用默认值
	if len(result.Allow) == 0 {
		result.Allow = DefaultToolAllow
	}
	if len(result.Deny) == 0 {
		result.Deny = DefaultToolDeny
	}

	// Agent 覆盖
	if agentOverride != nil {
		if len(agentOverride.Tools.Allow) > 0 {
			result.Allow = agentOverride.Tools.Allow
		}
		if len(agentOverride.Tools.Deny) > 0 {
			result.Deny = agentOverride.Tools.Deny
		}
	}

	return result
}

// FormatToolBlockedMessage 格式化工具被沙箱策略阻止的消息。
// TS 对照: runtime-status.ts formatToolBlockedMessage()
func FormatToolBlockedMessage(toolName string) string {
	return fmt.Sprintf(
		"Tool %q is blocked by the sandbox tool policy. "+
			"The sandbox restricts which tools can be used to prevent unintended system access. "+
			"To allow this tool, update the sandbox.tools configuration.",
		toolName,
	)
}

// ---------- 运行时状态 ----------

// ResolveSandboxRuntimeStatus 解析沙箱运行时状态。
// TS 对照: runtime-status.ts resolveSandboxRuntimeStatus()
func ResolveSandboxRuntimeStatus(cfg SandboxConfig, agentID string, agentOverride *SandboxConfig) SandboxRuntimeStatus {
	mode := ResolveSandboxMode(cfg)
	toolPolicy := ResolveToolPolicyForAgent(cfg, agentOverride)

	return SandboxRuntimeStatus{
		AgentID:     agentID,
		Mode:        mode,
		IsSandboxed: mode == "enforced",
		ToolPolicy:  &toolPolicy,
	}
}

// ---------- 上下文编排 ----------

// ResolveSandboxContextParams 上下文编排参数。
type ResolveSandboxContextParams struct {
	SessionKey    string
	AgentID       string
	StateDir      string
	SourceDir     string // 用于种子工作区的源目录
	GlobalConfig  SandboxConfig
	AgentOverride *SandboxConfig
}

// ResolveSandboxContext 编排完整的沙箱上下文。
// 确保容器运行、浏览器就绪、工作区准备。
// TS 对照: context.ts resolveSandboxContext()
func ResolveSandboxContext(params ResolveSandboxContextParams) (*SandboxContext, error) {
	cfg := ResolveSandboxConfigForAgent(params.GlobalConfig, params.AgentOverride)

	if !cfg.Enabled {
		return nil, nil
	}

	containerName := ResolveContainerName(params.SessionKey, cfg.Scope, params.AgentID)
	registryPath := filepath.Join(params.StateDir, "sandbox", RegistryFilename)

	// 1. 确保镜像存在
	if err := EnsureSandboxImage(cfg.Docker.Image); err != nil {
		return nil, fmt.Errorf("ensuring sandbox image: %w", err)
	}

	// 2. 检查现有容器状态
	state := DockerContainerState(containerName)
	now := time.Now().UnixMilli()

	if state.Exists && state.Running {
		// 检查配置变更
		currentHash, _ := ReadContainerLabel(containerName, "openacosmi.config-hash")
		newHash := NormalizeAndHashConfig(cfg)
		if currentHash != "" && currentHash != newHash {
			// 配置变更 → 重建容器
			_ = RemoveContainer(containerName)
			state.Exists = false
		}
	}

	if state.Exists && !state.Running {
		// 存在但停止 → 尝试启动
		if _, err := ExecDocker([]string{"start", containerName}, &ExecDockerOpts{AllowFailure: true}); err != nil {
			_ = RemoveContainer(containerName)
			state.Exists = false
		}
	}

	if !state.Exists {
		// 创建并启动新容器
		if err := CreateAndStartContainer(cfg, containerName, params.SessionKey); err != nil {
			return nil, fmt.Errorf("creating sandbox container: %w", err)
		}
	}

	// 更新注册表
	entry := RegistryEntry{
		ContainerName: containerName,
		SessionKey:    params.SessionKey,
		CreatedAtMs:   now,
		LastUsedAtMs:  now,
		Image:         cfg.Docker.Image,
		ConfigHash:    NormalizeAndHashConfig(cfg),
	}
	_ = UpdateRegistryEntry(registryPath, entry)

	// 3. 工作区准备
	workspaceDir := ResolveSandboxWorkspaceDir(params.StateDir, params.AgentID, cfg.Scope, params.SessionKey)
	if cfg.Workspace != AccessNone {
		if err := EnsureSandboxWorkspace(workspaceDir, params.SourceDir); err != nil {
			// 非致命错误
			_ = err
		}

		// 挂载工作区到容器（如果容器支持）
		// 注意：实际挂载在 docker create 时通过 -v 参数完成。
		// 这里仅确保目录存在。
	}

	// 4. 浏览器准备
	var browserCtx *SandboxBrowserContext
	if cfg.Browser.Enabled {
		bc, err := EnsureSandboxBrowser(cfg, params.SessionKey, workspaceDir, workspaceDir, params.StateDir)
		if err != nil {
			// 浏览器失败不阻止沙箱运行
			_ = err
		} else {
			browserCtx = bc
		}
	}

	return &SandboxContext{
		ContainerName: containerName,
		SessionKey:    params.SessionKey,
		WorkspaceDir:  workspaceDir,
		AgentID:       params.AgentID,
		Browser:       browserCtx,
	}, nil
}
