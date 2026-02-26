// config.go — 沙箱配置解析与哈希。
//
// TS 对照: agents/sandbox/config.ts (173L),
//
//	agents/sandbox/config-hash.ts (65L),
//	agents/sandbox/shared.ts (47L)
//
// 提供沙箱配置合并、模式判断、配置哈希、
// 会话键 slug 化以及工作区路径解析。
package sandbox

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ---------- shared.ts 工具函数 ----------

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

// SlugifySessionKey 将会话键转为容器安全的 slug。
// TS 对照: shared.ts slugifySessionKey()
func SlugifySessionKey(sessionKey string) string {
	s := strings.ToLower(strings.TrimSpace(sessionKey))
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
	}
	if s == "" {
		s = "default"
	}
	return s
}

// ResolveSandboxWorkspaceDir 解析沙箱工作区目录。
// TS 对照: shared.ts resolveSandboxWorkspaceDir()
func ResolveSandboxWorkspaceDir(stateDir, agentID string, scope SandboxScope, sessionKey string) string {
	base := filepath.Join(stateDir, "sandbox", "workspaces")
	switch scope {
	case ScopeSession:
		return filepath.Join(base, "sessions", SlugifySessionKey(sessionKey))
	case ScopeAgent:
		return filepath.Join(base, "agents", agentID)
	case ScopeShared:
		return filepath.Join(base, "shared")
	default:
		return filepath.Join(base, "sessions", SlugifySessionKey(sessionKey))
	}
}

// ResolveSandboxAgentId 从会话键中提取 agentID。
// TS 对照: shared.ts resolveSandboxAgentId()
func ResolveSandboxAgentId(sessionKey string) string {
	// sessionKey 格式：agentId/sessionId 或 agentId
	parts := strings.SplitN(sessionKey, "/", 2)
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "default"
}

// ---------- config.ts 配置合并 ----------

// ResolveSandboxConfigForAgent 为指定 agent 解析沙箱配置。
// 合并全局配置与 agent 特定覆盖。
// TS 对照: config.ts resolveSandboxConfigForAgent()
func ResolveSandboxConfigForAgent(global SandboxConfig, agentOverride *SandboxConfig) SandboxConfig {
	cfg := global

	// 应用默认值
	if cfg.Docker.Image == "" {
		cfg.Docker.Image = DefaultImage
	}
	if cfg.Docker.Workdir == "" {
		cfg.Docker.Workdir = DefaultWorkdir
	}
	if cfg.Docker.MemoryMB == 0 {
		cfg.Docker.MemoryMB = DefaultMemoryMB
	}
	if cfg.Docker.CPUs == 0 {
		cfg.Docker.CPUs = DefaultCPUs
	}
	if cfg.Scope == "" {
		cfg.Scope = ScopeSession
	}
	if cfg.Workspace == "" {
		cfg.Workspace = AccessReadWrite
	}
	if cfg.Prune.IdleHours == 0 {
		cfg.Prune.IdleHours = DefaultIdleHours
	}
	if cfg.Prune.MaxAgeDays == 0 {
		cfg.Prune.MaxAgeDays = DefaultMaxAgeDays
	}
	if cfg.Browser.Image == "" {
		cfg.Browser.Image = DefaultBrowserImage
	}
	if cfg.Browser.CDPPort == 0 {
		cfg.Browser.CDPPort = DefaultCDPPort
	}
	if cfg.Browser.VNCPort == 0 {
		cfg.Browser.VNCPort = DefaultVNCPort
	}
	if cfg.Browser.NoVNCPort == 0 {
		cfg.Browser.NoVNCPort = DefaultNoVNCPort
	}
	if cfg.Browser.IdleTimeout == 0 {
		cfg.Browser.IdleTimeout = DefaultBrowserIdleTimeout
	}
	if cfg.Browser.AutoStartTimeoutMs == 0 {
		cfg.Browser.AutoStartTimeoutMs = DefaultBrowserAutoStartTimeoutMs
	}

	// SB-03：无覆盖时也注入 LANG 默认值。
	if cfg.Docker.Env == nil {
		cfg.Docker.Env = map[string]string{"LANG": "C.UTF-8"}
	} else if _, hasLang := cfg.Docker.Env["LANG"]; !hasLang {
		cfg.Docker.Env["LANG"] = "C.UTF-8"
	}

	// 合并 agent-specific override
	if agentOverride == nil {
		return cfg
	}

	if agentOverride.Docker.Image != "" {
		cfg.Docker.Image = agentOverride.Docker.Image
	}
	if agentOverride.Docker.Workdir != "" {
		cfg.Docker.Workdir = agentOverride.Docker.Workdir
	}
	if agentOverride.Docker.MemoryMB != 0 {
		cfg.Docker.MemoryMB = agentOverride.Docker.MemoryMB
	}
	if agentOverride.Docker.CPUs != 0 {
		cfg.Docker.CPUs = agentOverride.Docker.CPUs
	}
	if agentOverride.Docker.Network != "" {
		cfg.Docker.Network = agentOverride.Docker.Network
	}
	if agentOverride.Docker.User != "" {
		cfg.Docker.User = agentOverride.Docker.User
	}
	if agentOverride.Docker.SeccompPolicy != "" {
		cfg.Docker.SeccompPolicy = agentOverride.Docker.SeccompPolicy
	}
	if agentOverride.Docker.ApparmorProfile != "" {
		cfg.Docker.ApparmorProfile = agentOverride.Docker.ApparmorProfile
	}
	if agentOverride.Docker.ReadOnlyRoot {
		cfg.Docker.ReadOnlyRoot = true
	}
	if len(agentOverride.Docker.Tmpfs) > 0 {
		cfg.Docker.Tmpfs = agentOverride.Docker.Tmpfs
	}
	if len(agentOverride.Docker.Capabilities) > 0 {
		cfg.Docker.Capabilities = agentOverride.Docker.Capabilities
	}
	if len(agentOverride.Docker.Env) > 0 {
		if cfg.Docker.Env == nil {
			cfg.Docker.Env = make(map[string]string)
		}
		for k, v := range agentOverride.Docker.Env {
			cfg.Docker.Env[k] = v
		}
	}

	// SB-03：注入 LANG 默认值（与 TS config.ts:48 对齐）。
	// TS 中 globalDocker?.env ?? { LANG: "C.UTF-8" } 当无用户 env 时注入 LANG。
	if cfg.Docker.Env == nil {
		cfg.Docker.Env = map[string]string{"LANG": "C.UTF-8"}
	} else if _, hasLang := cfg.Docker.Env["LANG"]; !hasLang {
		cfg.Docker.Env["LANG"] = "C.UTF-8"
	}

	if agentOverride.Scope != "" {
		cfg.Scope = agentOverride.Scope
	}
	if agentOverride.Workspace != "" {
		cfg.Workspace = agentOverride.Workspace
	}
	if len(agentOverride.Tools.Allow) > 0 {
		cfg.Tools.Allow = agentOverride.Tools.Allow
	}
	if len(agentOverride.Tools.Deny) > 0 {
		cfg.Tools.Deny = agentOverride.Tools.Deny
	}
	if agentOverride.Browser.Image != "" {
		cfg.Browser.Image = agentOverride.Browser.Image
	}
	if agentOverride.Browser.ContainerPrefix != "" {
		cfg.Browser.ContainerPrefix = agentOverride.Browser.ContainerPrefix
	}
	if agentOverride.Browser.CDPPort != 0 {
		cfg.Browser.CDPPort = agentOverride.Browser.CDPPort
	}
	if agentOverride.Browser.VNCPort != 0 {
		cfg.Browser.VNCPort = agentOverride.Browser.VNCPort
	}
	if agentOverride.Browser.NoVNCPort != 0 {
		cfg.Browser.NoVNCPort = agentOverride.Browser.NoVNCPort
	}
	if agentOverride.Browser.AutoStartTimeoutMs != 0 {
		cfg.Browser.AutoStartTimeoutMs = agentOverride.Browser.AutoStartTimeoutMs
	}
	// bool 字段：agent override 显式设置时覆盖
	if agentOverride.Browser.Headless {
		cfg.Browser.Headless = true
	}
	if agentOverride.Browser.EnableNoVNC {
		cfg.Browser.EnableNoVNC = true
	}
	if agentOverride.Browser.AllowHostControl {
		cfg.Browser.AllowHostControl = true
	}
	if agentOverride.Browser.AutoStart {
		cfg.Browser.AutoStart = true
	}

	return cfg
}

// ResolveSandboxMode 判断沙箱模式。
// TS 对照: config.ts resolveSandboxMode()
func ResolveSandboxMode(cfg SandboxConfig) string {
	if !cfg.Enabled {
		return "off"
	}
	return "enforced"
}

// ---------- config-hash.ts 配置哈希 ----------

// NormalizeAndHashConfig 对沙箱配置进行规范化并生成 SHA-256 哈希。
// 用于检测配置变更，决定是否需要重建容器。
// TS 对照: config-hash.ts normalizeAndHashConfig()
func NormalizeAndHashConfig(cfg SandboxConfig) string {
	normalized := normalizeValue(cfg)
	data, _ := json.Marshal(normalized)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// normalizeValue 递归规范化配置值。
// TS 对照: config-hash.ts normalizeValue()
func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return normalizeMap(val)
	case []interface{}:
		return normalizeSlice(val)
	default:
		// 先序列化为 JSON 再反序列化，确保类型统一
		data, err := json.Marshal(v)
		if err != nil {
			return v
		}
		var result interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return v
		}
		return normalizeGeneric(result)
	}
}

func normalizeGeneric(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return normalizeMap(val)
	case []interface{}:
		return normalizeSlice(val)
	default:
		return v
	}
}

func normalizeMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result[k] = normalizeGeneric(m[k])
	}
	return result
}

func normalizeSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = normalizeGeneric(v)
	}
	// 对纯字符串切片排序
	allStrings := true
	for _, v := range result {
		if _, ok := v.(string); !ok {
			allStrings = false
			break
		}
	}
	if allStrings {
		strs := make([]string, len(result))
		for i, v := range result {
			strs[i] = v.(string)
		}
		sort.Strings(strs)
		for i, s := range strs {
			result[i] = s
		}
	}
	return result
}

// ResolveContainerName 生成容器名称。
func ResolveContainerName(sessionKey string, scope SandboxScope, agentID string) string {
	switch scope {
	case ScopeShared:
		return DefaultContainerPrefix + "shared"
	case ScopeAgent:
		return DefaultContainerPrefix + agentID
	default:
		return DefaultContainerPrefix + SlugifySessionKey(sessionKey)
	}
}

// ResolveBrowserContainerName 生成浏览器容器名称。
func ResolveBrowserContainerName(sessionKey string) string {
	return DefaultBrowserContainerPrefix + SlugifySessionKey(sessionKey)
}
