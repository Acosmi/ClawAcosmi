package whatsapp

import (
	"math"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// WhatsApp 重连策略 — 继承自 src/web/reconnect.ts (54L)

// DefaultHeartbeatSeconds 默认心跳间隔
const DefaultHeartbeatSeconds = 60

// ReconnectPolicy 重连退避策略
type ReconnectPolicy struct {
	InitialMs   float64 `json:"initialMs"`
	MaxMs       float64 `json:"maxMs"`
	Factor      float64 `json:"factor"`
	Jitter      float64 `json:"jitter"`
	MaxAttempts int     `json:"maxAttempts"`
}

// DefaultReconnectPolicy 默认重连策略
var DefaultReconnectPolicy = ReconnectPolicy{
	InitialMs:   2000,
	MaxMs:       30000,
	Factor:      1.8,
	Jitter:      0.25,
	MaxAttempts: 12,
}

// ResolveHeartbeatSeconds 解析心跳间隔（秒）
func ResolveHeartbeatSeconds(cfg *types.OpenAcosmiConfig, overrideSeconds *int) int {
	if overrideSeconds != nil && *overrideSeconds > 0 {
		return *overrideSeconds
	}
	if cfg.Web != nil && cfg.Web.HeartbeatSeconds != nil && *cfg.Web.HeartbeatSeconds > 0 {
		return *cfg.Web.HeartbeatSeconds
	}
	return DefaultHeartbeatSeconds
}

// ResolveReconnectPolicy 解析重连策略（合并默认值+配置+覆盖）
func ResolveReconnectPolicy(cfg *types.OpenAcosmiConfig, overrides *ReconnectPolicy) ReconnectPolicy {
	merged := DefaultReconnectPolicy

	// 合并配置层
	if cfg.Web != nil && cfg.Web.Reconnect != nil {
		applyReconnectOverrides(&merged, cfg.Web.Reconnect)
	}

	// 合并参数覆盖层
	if overrides != nil {
		if overrides.InitialMs > 0 {
			merged.InitialMs = overrides.InitialMs
		}
		if overrides.MaxMs > 0 {
			merged.MaxMs = overrides.MaxMs
		}
		if overrides.Factor > 0 {
			merged.Factor = overrides.Factor
		}
		if overrides.Jitter > 0 {
			merged.Jitter = overrides.Jitter
		}
		if overrides.MaxAttempts > 0 {
			merged.MaxAttempts = overrides.MaxAttempts
		}
	}

	// 约束合法范围
	merged.InitialMs = math.Max(250, merged.InitialMs)
	merged.MaxMs = math.Max(merged.InitialMs, merged.MaxMs)
	merged.Factor = clamp(merged.Factor, 1.1, 10)
	merged.Jitter = clamp(merged.Jitter, 0, 1)
	if merged.MaxAttempts < 0 {
		merged.MaxAttempts = 0
	}

	return merged
}

// applyReconnectOverrides 应用 WebConfig.Reconnect 覆盖
func applyReconnectOverrides(target *ReconnectPolicy, src *types.WebReconnectConfig) {
	if src.InitialMs != nil {
		target.InitialMs = float64(*src.InitialMs)
	}
	if src.MaxMs != nil {
		target.MaxMs = float64(*src.MaxMs)
	}
	if src.Factor != nil {
		target.Factor = *src.Factor
	}
	if src.Jitter != nil {
		target.Jitter = *src.Jitter
	}
	if src.MaxAttempts != nil {
		target.MaxAttempts = *src.MaxAttempts
	}
}

func clamp(val, min, max float64) float64 {
	return math.Max(min, math.Min(max, val))
}
