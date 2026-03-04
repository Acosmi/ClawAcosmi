// Package sessions — 主会话键解析。
//
// 对齐 TS: src/config/sessions/main-session.ts (80L) + session-key.ts (48L)
package sessions

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/routing"
)

// ---------- 主会话键 ----------

// SessionScopeConfig 用于会话键解析的配置子集。
type SessionScopeConfig struct {
	Scope   string // "per-sender" | "global"
	MainKey string
}

// AgentListEntry 代理列表条目。
type AgentListEntry struct {
	ID      string `json:"id,omitempty"`
	Default bool   `json:"default,omitempty"`
}

// ResolveMainSessionKey 解析主会话键。
// 对齐 TS: main-session.ts resolveMainSessionKey()
func ResolveMainSessionKey(sessionCfg *SessionScopeConfig, agents []AgentListEntry) string {
	if sessionCfg != nil && sessionCfg.Scope == "global" {
		return "global"
	}
	defaultAgentID := routing.DefaultAgentID
	if len(agents) > 0 {
		// 先找 default=true，再取第一个
		for _, a := range agents {
			if a.Default && a.ID != "" {
				defaultAgentID = a.ID
				break
			}
		}
		if defaultAgentID == routing.DefaultAgentID && agents[0].ID != "" {
			defaultAgentID = agents[0].ID
		}
	}
	agentID := routing.NormalizeAgentID(defaultAgentID)
	mainKey := routing.DefaultMainKey
	if sessionCfg != nil && sessionCfg.MainKey != "" {
		mainKey = routing.NormalizeMainKey(sessionCfg.MainKey)
	}
	return routing.BuildAgentMainSessionKey(agentID, mainKey)
}

// ResolveAgentMainSessionKey 解析指定代理的主会话键。
// 对齐 TS: main-session.ts resolveAgentMainSessionKey()
func ResolveAgentMainSessionKey(agentID string, mainKey string) string {
	mk := routing.NormalizeMainKey(mainKey)
	return routing.BuildAgentMainSessionKey(agentID, mk)
}

// ResolveExplicitAgentSessionKey 解析显式的代理会话键（如果有代理 ID）。
// 对齐 TS: main-session.ts resolveExplicitAgentSessionKey()
func ResolveExplicitAgentSessionKey(sessionCfg *SessionScopeConfig, agentID string) string {
	id := strings.TrimSpace(agentID)
	if id == "" {
		return ""
	}
	mainKey := ""
	if sessionCfg != nil {
		mainKey = sessionCfg.MainKey
	}
	return ResolveAgentMainSessionKey(id, mainKey)
}

// CanonicalizeMainSessionAlias 将主会话别名规范化。
// 对齐 TS: main-session.ts canonicalizeMainSessionAlias()
func CanonicalizeMainSessionAlias(sessionCfg *SessionScopeConfig, agentID, sessionKey string) string {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return raw
	}

	aid := routing.NormalizeAgentID(agentID)
	mainKey := routing.DefaultMainKey
	if sessionCfg != nil && sessionCfg.MainKey != "" {
		mainKey = routing.NormalizeMainKey(sessionCfg.MainKey)
	}
	agentMainSessionKey := routing.BuildAgentMainSessionKey(aid, mainKey)
	agentMainAliasKey := routing.BuildAgentMainSessionKey(aid, "main")

	isMainAlias := raw == "main" || raw == mainKey ||
		raw == agentMainSessionKey || raw == agentMainAliasKey

	if sessionCfg != nil && sessionCfg.Scope == "global" && isMainAlias {
		return "global"
	}
	if isMainAlias {
		return agentMainSessionKey
	}
	return raw
}

// ---------- 会话键推导 ----------

// DeriveSessionKey 推导会话使用哪个 session bucket。
// 对齐 TS: session-key.ts deriveSessionKey()
func DeriveSessionKey(scope string, ctx MsgContextForGroup) string {
	if scope == "global" {
		return "global"
	}
	resolvedGroup := ResolveGroupSessionKey(ctx)
	if resolvedGroup != nil {
		return resolvedGroup.Key
	}
	from := strings.TrimSpace(ctx.From)
	if from != "" {
		return normalizeE164(from)
	}
	return "unknown"
}

// ResolveSessionKeyFull 解析完整的会话键（含显式覆盖和规范化）。
// 对齐 TS: session-key.ts resolveSessionKey()
func ResolveSessionKeyFull(scope string, ctx MsgContextForGroup, explicitSessionKey, mainKey string) string {
	explicit := strings.ToLower(strings.TrimSpace(explicitSessionKey))
	if explicit != "" {
		return explicit
	}
	raw := DeriveSessionKey(scope, ctx)
	if scope == "global" {
		return raw
	}
	canonicalMainKey := routing.NormalizeMainKey(mainKey)
	canonical := routing.BuildAgentMainSessionKey(routing.DefaultAgentID, canonicalMainKey)
	isGroup := strings.Contains(raw, ":group:") || strings.Contains(raw, ":channel:")
	if !isGroup {
		return canonical
	}
	return "agent:" + routing.DefaultAgentID + ":" + raw
}

// normalizeE164 简单的电话号码规范化。
func normalizeE164(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToLower(value)
}
