// Package sessions — 会话重置策略。
//
// 对齐 TS: src/config/sessions/reset.ts (177L)
package sessions

import (
	"math"
	"strings"
	"time"
)

// ---------- 类型定义 ----------

// SessionResetMode 重置模式。
type SessionResetMode string

const (
	ResetModeDaily SessionResetMode = "daily"
	ResetModeIdle  SessionResetMode = "idle"
)

// SessionResetType 重置关联的会话类型。
type SessionResetType string

const (
	ResetTypeDirect SessionResetType = "direct"
	ResetTypeGroup  SessionResetType = "group"
	ResetTypeThread SessionResetType = "thread"
)

// SessionResetPolicy 会话重置策略。
type SessionResetPolicy struct {
	Mode        SessionResetMode
	AtHour      int
	IdleMinutes *int // nil 表示未设置
}

// SessionFreshness 会话新鲜度检测结果。
type SessionFreshness struct {
	Fresh         bool
	DailyResetAt  *int64 // Unix ms
	IdleExpiresAt *int64 // Unix ms
}

// SessionResetConfig 对齐 TS SessionResetConfig。
type SessionResetConfig struct {
	Mode        string `json:"mode,omitempty"`
	AtHour      *int   `json:"atHour,omitempty"`
	IdleMinutes *int   `json:"idleMinutes,omitempty"`
}

// ---------- 常量 ----------

const (
	DefaultResetMode   = ResetModeDaily
	DefaultResetAtHour = 4
	DefaultIdleMinutes = 60
)

var (
	threadSessionMarkers = []string{":thread:", ":topic:"}
	groupSessionMarkers  = []string{":group:", ":channel:"}
)

// ---------- 分类函数 ----------

// IsThreadSessionKey 判断是否为线程会话键。
// 对齐 TS: reset.ts isThreadSessionKey()
func IsThreadSessionKey(sessionKey string) bool {
	normalized := strings.ToLower(strings.TrimSpace(sessionKey))
	if normalized == "" {
		return false
	}
	for _, marker := range threadSessionMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// ResolveSessionResetType 解析会话重置类型。
// 对齐 TS: reset.ts resolveSessionResetType()
func ResolveSessionResetType(sessionKey string, isGroup, isThread bool) SessionResetType {
	if isThread || IsThreadSessionKey(sessionKey) {
		return ResetTypeThread
	}
	if isGroup {
		return ResetTypeGroup
	}
	normalized := strings.ToLower(sessionKey)
	for _, marker := range groupSessionMarkers {
		if strings.Contains(normalized, marker) {
			return ResetTypeGroup
		}
	}
	return ResetTypeDirect
}

// ResolveThreadFlag 判断是否为线程消息。
// 对齐 TS: reset.ts resolveThreadFlag()
func ResolveThreadFlag(params ThreadFlagParams) bool {
	if params.MessageThreadID != "" {
		return true
	}
	if strings.TrimSpace(params.ThreadLabel) != "" {
		return true
	}
	if strings.TrimSpace(params.ThreadStarterBody) != "" {
		return true
	}
	if strings.TrimSpace(params.ParentSessionKey) != "" {
		return true
	}
	return IsThreadSessionKey(params.SessionKey)
}

// ThreadFlagParams 线程判定参数。
type ThreadFlagParams struct {
	SessionKey        string
	MessageThreadID   string
	ThreadLabel       string
	ThreadStarterBody string
	ParentSessionKey  string
}

// ---------- 时间计算 ----------

// ResolveDailyResetAtMs 计算当天（或前一天）的重置时间点。
// 对齐 TS: reset.ts resolveDailyResetAtMs()
func ResolveDailyResetAtMs(nowMs int64, atHour int) int64 {
	normalizedHour := normalizeResetAtHour(atHour)
	now := time.UnixMilli(nowMs)
	resetAt := time.Date(now.Year(), now.Month(), now.Day(),
		normalizedHour, 0, 0, 0, now.Location())
	if nowMs < resetAt.UnixMilli() {
		resetAt = resetAt.AddDate(0, 0, -1)
	}
	return resetAt.UnixMilli()
}

// ---------- 策略解析 ----------

// SessionConfig 对齐 TS SessionConfig 中的 session 部分（仅 reset 相关字段）。
type SessionConfig struct {
	Reset          *SessionResetConfig            `json:"reset,omitempty"`
	ResetByType    map[string]*SessionResetConfig `json:"resetByType,omitempty"`
	ResetByChannel map[string]*SessionResetConfig `json:"resetByChannel,omitempty"`
	IdleMinutes    *int                           `json:"idleMinutes,omitempty"`
}

// ResolveSessionResetPolicy 解析完整的重置策略。
// 对齐 TS: reset.ts resolveSessionResetPolicy()
func ResolveSessionResetPolicy(sessionCfg *SessionConfig, resetType SessionResetType, resetOverride *SessionResetConfig) SessionResetPolicy {
	var baseReset *SessionResetConfig
	if resetOverride != nil {
		baseReset = resetOverride
	} else if sessionCfg != nil {
		baseReset = sessionCfg.Reset
	}

	// typeReset: 仅在无 override 时查找 resetByType
	var typeReset *SessionResetConfig
	if resetOverride == nil && sessionCfg != nil && sessionCfg.ResetByType != nil {
		typeReset = sessionCfg.ResetByType[string(resetType)]
		// 向后兼容: "dm" 作为 "direct" 的别名
		if typeReset == nil && resetType == ResetTypeDirect {
			typeReset = sessionCfg.ResetByType["dm"]
		}
	}

	hasExplicitReset := baseReset != nil || (sessionCfg != nil && sessionCfg.ResetByType != nil)

	var legacyIdleMinutes *int
	if resetOverride == nil && sessionCfg != nil {
		legacyIdleMinutes = sessionCfg.IdleMinutes
	}

	// 模式解析
	mode := DefaultResetMode
	if typeReset != nil && typeReset.Mode != "" {
		mode = SessionResetMode(typeReset.Mode)
	} else if baseReset != nil && baseReset.Mode != "" {
		mode = SessionResetMode(baseReset.Mode)
	} else if !hasExplicitReset && legacyIdleMinutes != nil {
		mode = ResetModeIdle
	}

	// atHour 解析
	atHour := DefaultResetAtHour
	if typeReset != nil && typeReset.AtHour != nil {
		atHour = normalizeResetAtHour(*typeReset.AtHour)
	} else if baseReset != nil && baseReset.AtHour != nil {
		atHour = normalizeResetAtHour(*baseReset.AtHour)
	}

	// idleMinutes 解析
	var idleMinutes *int
	if typeReset != nil && typeReset.IdleMinutes != nil {
		v := clampIdleMinutes(*typeReset.IdleMinutes)
		idleMinutes = &v
	} else if baseReset != nil && baseReset.IdleMinutes != nil {
		v := clampIdleMinutes(*baseReset.IdleMinutes)
		idleMinutes = &v
	} else if legacyIdleMinutes != nil {
		v := clampIdleMinutes(*legacyIdleMinutes)
		idleMinutes = &v
	} else if mode == ResetModeIdle {
		v := DefaultIdleMinutes
		idleMinutes = &v
	}

	return SessionResetPolicy{Mode: mode, AtHour: atHour, IdleMinutes: idleMinutes}
}

// ResolveChannelResetConfig 解析频道级别的重置配置。
// 对齐 TS: reset.ts resolveChannelResetConfig()
func ResolveChannelResetConfig(sessionCfg *SessionConfig, channel string) *SessionResetConfig {
	if sessionCfg == nil || sessionCfg.ResetByChannel == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(channel))
	if key == "" {
		return nil
	}
	if cfg, ok := sessionCfg.ResetByChannel[key]; ok {
		return cfg
	}
	return nil
}

// EvaluateSessionFreshness 评估会话是否仍然新鲜。
// 对齐 TS: reset.ts evaluateSessionFreshness()
func EvaluateSessionFreshness(updatedAtMs, nowMs int64, policy SessionResetPolicy) SessionFreshness {
	var dailyResetAt *int64
	if policy.Mode == ResetModeDaily {
		v := ResolveDailyResetAtMs(nowMs, policy.AtHour)
		dailyResetAt = &v
	}

	var idleExpiresAt *int64
	if policy.IdleMinutes != nil {
		v := updatedAtMs + int64(*policy.IdleMinutes)*60_000
		idleExpiresAt = &v
	}

	staleDaily := dailyResetAt != nil && updatedAtMs < *dailyResetAt
	staleIdle := idleExpiresAt != nil && nowMs > *idleExpiresAt

	return SessionFreshness{
		Fresh:         !(staleDaily || staleIdle),
		DailyResetAt:  dailyResetAt,
		IdleExpiresAt: idleExpiresAt,
	}
}

// ---------- 内部工具 ----------

func normalizeResetAtHour(value int) int {
	v := int(math.Floor(float64(value)))
	if v < 0 {
		return 0
	}
	if v > 23 {
		return 23
	}
	return v
}

func clampIdleMinutes(v int) int {
	if v < 1 {
		return 1
	}
	return v
}
