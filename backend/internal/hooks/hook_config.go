package hooks

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ============================================================================
// 钩子配置解析 + 资格检查
// 对应 TS: config.ts
// ============================================================================

// DefaultConfigValues 默认配置值（当路径未定义时使用）
var DefaultConfigValues = map[string]bool{
	"browser.enabled":         true,
	"browser.evaluateEnabled": true,
	"workspace.dir":           true,
}

// ResolveConfigPath 按点号路径访问配置对象
// 对应 TS: config.ts resolveConfigPath
func ResolveConfigPath(config map[string]interface{}, pathStr string) interface{} {
	if config == nil {
		return nil
	}
	parts := strings.Split(pathStr, ".")
	var current interface{} = config
	for _, part := range parts {
		if part == "" {
			continue
		}
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

// IsConfigPathTruthy 检查配置路径的值是否为真
// 对应 TS: config.ts isConfigPathTruthy
func IsConfigPathTruthy(config map[string]interface{}, pathStr string) bool {
	value := ResolveConfigPath(config, pathStr)
	if value == nil {
		if def, ok := DefaultConfigValues[pathStr]; ok {
			return def
		}
		return false
	}
	return isTruthy(value)
}

// ResolveHookConfigEntry 获取指定 hook 的配置
// 对应 TS: config.ts resolveHookConfig
func ResolveHookConfigEntry(config map[string]interface{}, hookKey string) *HookConfig {
	if config == nil {
		return nil
	}
	hooks, _ := config["hooks"].(map[string]interface{})
	if hooks == nil {
		return nil
	}
	internal, _ := hooks["internal"].(map[string]interface{})
	if internal == nil {
		return nil
	}
	entries, _ := internal["entries"].(map[string]interface{})
	if entries == nil {
		return nil
	}
	entry, ok := entries[hookKey].(map[string]interface{})
	if !ok {
		return nil
	}
	hc := &HookConfig{}
	if e, ok := entry["enabled"].(bool); ok {
		hc.Enabled = &e
	}
	if m, ok := entry["messages"].(float64); ok {
		mi := int(m)
		hc.Messages = &mi
	}
	if envMap, ok := entry["env"].(map[string]interface{}); ok {
		hc.Env = make(map[string]string)
		for k, v := range envMap {
			if s, ok := v.(string); ok {
				hc.Env[k] = s
			}
		}
	}
	return hc
}

// ResolveRuntimePlatform 获取当前运行平台
// 对应 TS: config.ts resolveRuntimePlatform
func ResolveRuntimePlatform() string {
	return runtime.GOOS
}

// HasBinary 检查 PATH 中是否存在指定二进制文件
// 对应 TS: config.ts hasBinary
func HasBinary(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// ShouldIncludeHook 检查钩子是否应被包含
// 对应 TS: config.ts shouldIncludeHook
func ShouldIncludeHook(entry *HookEntry, config map[string]interface{}, eligibility *HookEligibilityContext) bool {
	hookKey := ResolveHookKey(entry.Hook.Name, entry)
	hookConfig := ResolveHookConfigEntry(config, hookKey)
	pluginManaged := entry.Hook.Source == HookSourcePlugin

	// Check if explicitly disabled
	if !pluginManaged && hookConfig != nil && hookConfig.Enabled != nil && !*hookConfig.Enabled {
		return false
	}

	// Check OS requirement
	osList := entry.Metadata.osOrEmpty()
	if len(osList) > 0 {
		currentOS := ResolveRuntimePlatform()
		osMatched := containsStr(osList, currentOS)
		if !osMatched && eligibility != nil && eligibility.Remote != nil {
			for _, platform := range eligibility.Remote.Platforms {
				if containsStr(osList, platform) {
					osMatched = true
					break
				}
			}
		}
		if !osMatched {
			return false
		}
	}

	// If marked as 'always', bypass all other checks
	if entry.Metadata != nil && entry.Metadata.Always != nil && *entry.Metadata.Always {
		return true
	}

	// Check required binaries (all must be present)
	requiredBins := entry.Metadata.binsOrEmpty()
	for _, bin := range requiredBins {
		if HasBinary(bin) {
			continue
		}
		if eligibility != nil && eligibility.Remote != nil && eligibility.Remote.HasBin != nil && eligibility.Remote.HasBin(bin) {
			continue
		}
		return false
	}

	// Check anyBins (at least one must be present)
	anyBins := entry.Metadata.anyBinsOrEmpty()
	if len(anyBins) > 0 {
		found := false
		for _, bin := range anyBins {
			if HasBinary(bin) {
				found = true
				break
			}
		}
		if !found && eligibility != nil && eligibility.Remote != nil && eligibility.Remote.HasAnyBin != nil {
			found = eligibility.Remote.HasAnyBin(anyBins)
		}
		if !found {
			return false
		}
	}

	// Check required environment variables
	requiredEnv := entry.Metadata.envOrEmpty()
	for _, envName := range requiredEnv {
		if os.Getenv(envName) != "" {
			continue
		}
		if hookConfig != nil && hookConfig.Env != nil && hookConfig.Env[envName] != "" {
			continue
		}
		return false
	}

	// Check required config paths
	requiredConfig := entry.Metadata.configOrEmpty()
	for _, configPath := range requiredConfig {
		if !IsConfigPathTruthy(config, configPath) {
			return false
		}
	}

	return true
}

// --- helpers ---

func isTruthy(value interface{}) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		return strings.TrimSpace(v) != ""
	}
	return true
}

func containsStr(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

// HookMetadata 辅助方法
func (m *HookMetadata) osOrEmpty() []string {
	if m == nil {
		return nil
	}
	return m.OS
}

func (m *HookMetadata) binsOrEmpty() []string {
	if m == nil || m.Requires == nil {
		return nil
	}
	return m.Requires.Bins
}

func (m *HookMetadata) anyBinsOrEmpty() []string {
	if m == nil || m.Requires == nil {
		return nil
	}
	return m.Requires.AnyBins
}

func (m *HookMetadata) envOrEmpty() []string {
	if m == nil || m.Requires == nil {
		return nil
	}
	return m.Requires.Env
}

func (m *HookMetadata) configOrEmpty() []string {
	if m == nil || m.Requires == nil {
		return nil
	}
	return m.Requires.Config
}
