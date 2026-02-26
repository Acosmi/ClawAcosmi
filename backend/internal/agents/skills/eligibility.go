package skills

// eligibility.go — 技能适用性判定 + 配置路径解析
// 对应 TS: agents/skills/config.ts (192L)
//
// 提供 shouldIncludeSkill / hasBinary / resolveConfigPath /
// resolveSkillConfig / isBundledSkillAllowed 等能力。

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// 默认配置值
var defaultConfigValues = map[string]bool{
	"browser.enabled":         true,
	"browser.evaluateEnabled": true,
}

// ResolveConfigPath 按点分路径解析配置值。
// 对应 TS: resolveConfigPath
func ResolveConfigPath(config *types.OpenAcosmiConfig, pathStr string) interface{} {
	if config == nil {
		return nil
	}
	// 简化实现：将 config 序列化为 map 后递归解析
	// 实际使用中通过反射处理更高效，这里保持功能对等
	parts := strings.Split(pathStr, ".")
	var current interface{} = configToMap(config)
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

// IsConfigPathTruthy 检查配置路径值是否为 truthy。
// 对应 TS: isConfigPathTruthy
func IsConfigPathTruthy(config *types.OpenAcosmiConfig, pathStr string) bool {
	value := ResolveConfigPath(config, pathStr)
	if value == nil {
		if v, ok := defaultConfigValues[pathStr]; ok {
			return v
		}
		return false
	}
	return isTruthy(value)
}

// ResolveSkillConfig 获取指定 skill 的配置。
// 对应 TS: resolveSkillConfig
func ResolveSkillConfig(config *types.OpenAcosmiConfig, skillKey string) *types.SkillConfig {
	if config == nil || config.Skills == nil || config.Skills.Entries == nil {
		return nil
	}
	entry, ok := config.Skills.Entries[skillKey]
	if !ok || entry == nil {
		return nil
	}
	return entry
}

// HasBinary 检查 PATH 中是否存在指定二进制。
// 对应 TS: hasBinary
func HasBinary(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// ResolveRuntimePlatform 获取运行时平台。
// 对应 TS: resolveRuntimePlatform
func ResolveRuntimePlatform() string {
	return runtime.GOOS
}

// IsBundledSkillAllowed 检查捆绑技能是否在白名单中。
func IsBundledSkillAllowed(entry SkillEntry, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	skillKey := ResolveSkillKey(entry.Skill.Name, entry.Metadata)
	for _, name := range allowlist {
		if name == skillKey || name == entry.Skill.Name {
			return true
		}
	}
	return false
}

// ShouldIncludeSkill 判定技能是否应包含。
// 对应 TS: shouldIncludeSkill — 核心适用性判定函数
func ShouldIncludeSkill(entry SkillEntry, config *types.OpenAcosmiConfig, eligibility *SkillEligibilityContext) bool {
	skillKey := ResolveSkillKey(entry.Skill.Name, entry.Metadata)
	skillConfig := ResolveSkillConfig(config, skillKey)

	// 1) 配置级禁用
	if skillConfig != nil && skillConfig.Enabled != nil && !*skillConfig.Enabled {
		return false
	}

	// 2) 捆绑白名单
	var allowBundled []string
	if config != nil && config.Skills != nil {
		allowBundled = config.Skills.AllowBundled
	}
	if !IsBundledSkillAllowed(entry, allowBundled) {
		return false
	}

	// 3) OS 过滤
	if entry.Metadata != nil && len(entry.Metadata.OS) > 0 {
		platform := ResolveRuntimePlatform()
		found := false
		for _, os := range entry.Metadata.OS {
			if os == platform {
				found = true
				break
			}
		}
		// 也检查远程平台
		if !found && eligibility != nil && eligibility.Remote != nil {
			for _, rp := range eligibility.Remote.Platforms {
				for _, os := range entry.Metadata.OS {
					if os == rp {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
		if !found {
			return false
		}
	}

	// 4) always 标志
	if entry.Metadata != nil && entry.Metadata.Always != nil && *entry.Metadata.Always {
		return true
	}

	// 5) requires.bins — 全部必须
	if entry.Metadata != nil && entry.Metadata.Requires != nil {
		for _, bin := range entry.Metadata.Requires.Bins {
			if HasBinary(bin) {
				continue
			}
			if eligibility != nil && eligibility.Remote != nil && eligibility.Remote.HasBin != nil {
				if eligibility.Remote.HasBin(bin) {
					continue
				}
			}
			return false
		}

		// 6) requires.anyBins — 任一满足
		if len(entry.Metadata.Requires.AnyBins) > 0 {
			anyFound := false
			for _, bin := range entry.Metadata.Requires.AnyBins {
				if HasBinary(bin) {
					anyFound = true
					break
				}
			}
			if !anyFound && eligibility != nil && eligibility.Remote != nil && eligibility.Remote.HasAnyBin != nil {
				anyFound = eligibility.Remote.HasAnyBin(entry.Metadata.Requires.AnyBins)
			}
			if !anyFound {
				return false
			}
		}

		// 7) requires.env
		for _, envName := range entry.Metadata.Requires.Env {
			if os.Getenv(envName) != "" {
				continue
			}
			if skillConfig != nil && skillConfig.Env != nil {
				if skillConfig.Env[envName] != "" {
					continue
				}
			}
			if skillConfig != nil && skillConfig.APIKey != "" && entry.Metadata.PrimaryEnv == envName {
				continue
			}
			return false
		}

		// 8) requires.config
		for _, configPath := range entry.Metadata.Requires.Config {
			if !IsConfigPathTruthy(config, configPath) {
				return false
			}
		}
	}

	return true
}

// ---------- helpers ----------

func isTruthy(value interface{}) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return true
	}
}

func configToMap(config *types.OpenAcosmiConfig) map[string]interface{} {
	if config == nil {
		return nil
	}
	// 简化实现 — 只映射常用顶层字段
	result := make(map[string]interface{})
	if config.Browser != nil {
		browser := make(map[string]interface{})
		if config.Browser.Enabled != nil {
			browser["enabled"] = *config.Browser.Enabled
		}
		result["browser"] = browser
	}
	if config.Skills != nil {
		skills := make(map[string]interface{})
		if config.Skills.AllowBundled != nil {
			skills["allowBundled"] = config.Skills.AllowBundled
		}
		result["skills"] = skills
	}
	return result
}

// RemoteEligibility 远程适用性上下文。
type RemoteEligibility struct {
	Platforms []string
	HasBin    func(string) bool
	HasAnyBin func([]string) bool
	Note      string
}

// ExpandedSkillEligibilityContext 扩展的适用性上下文。
type ExpandedSkillEligibilityContext struct {
	Remote *RemoteEligibility
}
