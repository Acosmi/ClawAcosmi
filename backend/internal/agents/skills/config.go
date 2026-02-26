package skills

// config.go — 技能配置加载入口
// 对应 TS: agents/skills/config.ts (部分)
//
// 提供 LoadSkillConfig / ResolveBundledAllowlist /
// ResolveSkillsInstallPreferences 等配置级便利函数。
//
// 注意：核心判定函数 (ShouldIncludeSkill / ResolveConfigPath /
// IsConfigPathTruthy / ResolveSkillConfig / HasBinary) 在 eligibility.go 中。

import (
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// SkillsInstallPreferences 技能安装偏好。
// 对应 TS: resolveSkillsInstallPreferences
type SkillsInstallPreferences struct {
	NodeManager string // "npm" | "pnpm" | "yarn" | "bun"
}

// LoadSkillConfig 加载指定技能的配置（含默认值填充）。
// 对应 TS: config.ts → resolveSkillConfig + 默认合并
//
// 如果 config 中不存在该 skillKey 的配置，返回零值 SkillConfig。
func LoadSkillConfig(config *types.OpenAcosmiConfig, skillKey string) *types.SkillConfig {
	sc := ResolveSkillConfig(config, skillKey)
	if sc != nil {
		return sc
	}
	// 返回零值配置
	return &types.SkillConfig{}
}

// ResolveBundledAllowlist 获取内置技能白名单。
// 对应 TS: config.ts → resolveBundledAllowlist
func ResolveBundledAllowlist(config *types.OpenAcosmiConfig) []string {
	if config == nil || config.Skills == nil {
		return nil
	}
	raw := config.Skills.AllowBundled
	if len(raw) == 0 {
		return nil
	}
	var normalized []string
	for _, s := range raw {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// ResolveSkillsInstallPreferences 解析技能安装偏好。
// 对应 TS: skills.ts → resolveSkillsInstallPreferences
//
// 从配置中读取偏好的 Node 包管理器，默认为 "npm"。
func ResolveSkillsInstallPreferences(config *types.OpenAcosmiConfig) SkillsInstallPreferences {
	prefs := SkillsInstallPreferences{NodeManager: "npm"}
	if config == nil || config.Skills == nil || config.Skills.Install == nil {
		return prefs
	}
	if config.Skills.Install.NodeManager != "" {
		nm := strings.TrimSpace(strings.ToLower(config.Skills.Install.NodeManager))
		switch nm {
		case "pnpm", "yarn", "bun":
			prefs.NodeManager = nm
		default:
			prefs.NodeManager = "npm"
		}
	}
	return prefs
}
