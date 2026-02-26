package config

// 原生命令 / 原生技能启用判断 — 对应 src/config/commands.ts (65 行)
//
// 根据频道 ID + 配置覆盖判断是否启用原生命令/技能。
// Discord 和 Telegram 默认启用，其他频道默认关闭。

import "strings"

// NativeCommandsSetting 原生命令设置值。
// nil = auto (未指定)；*true = 强制启用；*false = 强制禁用
type NativeCommandsSetting = *bool

// resolveAutoDefault 根据频道 ID 返回 auto 模式下的默认值。
func resolveAutoDefault(providerId string) bool {
	id := strings.TrimSpace(strings.ToLower(providerId))
	switch id {
	case "discord", "telegram":
		return true
	default:
		return false
	}
}

// resolveNativeEnabled 通用解析逻辑。
func resolveNativeEnabled(providerId string, providerSetting, globalSetting NativeCommandsSetting) bool {
	setting := globalSetting
	if providerSetting != nil {
		setting = providerSetting
	}
	if setting != nil {
		return *setting
	}
	return resolveAutoDefault(providerId)
}

// ResolveNativeSkillsEnabled 判断是否启用原生技能。
// 对应 TS: resolveNativeSkillsEnabled(params)
func ResolveNativeSkillsEnabled(providerId string, providerSetting, globalSetting NativeCommandsSetting) bool {
	return resolveNativeEnabled(providerId, providerSetting, globalSetting)
}

// ResolveNativeCommandsEnabled 判断是否启用原生命令。
// 对应 TS: resolveNativeCommandsEnabled(params)
func ResolveNativeCommandsEnabled(providerId string, providerSetting, globalSetting NativeCommandsSetting) bool {
	return resolveNativeEnabled(providerId, providerSetting, globalSetting)
}

// IsNativeCommandsExplicitlyDisabled 检查是否被显式禁用。
// 对应 TS: isNativeCommandsExplicitlyDisabled(params)
func IsNativeCommandsExplicitlyDisabled(providerSetting, globalSetting NativeCommandsSetting) bool {
	if providerSetting != nil && !*providerSetting {
		return true
	}
	if providerSetting == nil && globalSetting != nil && !*globalSetting {
		return true
	}
	return false
}
