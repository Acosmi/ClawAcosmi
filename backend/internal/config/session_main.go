package config

// session_main.go — 主会话 key 推断
// TS 参考: src/config/sessions/main-session.ts

import (
	"github.com/Acosmi/ClawAcosmi/internal/routing"
)

// ---------- 主会话 key 推断 ----------

// MainSessionConfig 主会话 key 推断所需的配置子集
type MainSessionConfig struct {
	Session *SessionConfig    `json:"session,omitempty"`
	Agents  *AgentsListConfig `json:"agents,omitempty"`
}

// SessionConfig session 配置子集
type SessionConfig struct {
	Scope   SessionScope `json:"scope,omitempty"`
	MainKey string       `json:"mainKey,omitempty"`
}

// AgentsListConfig agents 列表配置子集
type AgentsListConfig struct {
	List []AgentItemConfig `json:"list,omitempty"`
}

// AgentItemConfig 单个 agent 配置项
type AgentItemConfig struct {
	ID      string `json:"id,omitempty"`
	Default bool   `json:"default,omitempty"`
}

// ResolveMainSessionKey 从配置推断主会话 key。
// TS 参考: src/config/sessions/main-session.ts resolveMainSessionKey
func ResolveMainSessionKey(cfg *MainSessionConfig) string {
	if cfg != nil && cfg.Session != nil && cfg.Session.Scope == SessionScopeGlobal {
		return "global"
	}

	var agentList []AgentItemConfig
	if cfg != nil && cfg.Agents != nil {
		agentList = cfg.Agents.List
	}

	defaultAgentID := routing.DefaultAgentID
	for _, a := range agentList {
		if a.Default && a.ID != "" {
			defaultAgentID = a.ID
			break
		}
	}
	if defaultAgentID == routing.DefaultAgentID && len(agentList) > 0 && agentList[0].ID != "" {
		defaultAgentID = agentList[0].ID
	}

	agentID := routing.NormalizeAgentID(defaultAgentID)

	mainKey := routing.DefaultMainKey
	if cfg != nil && cfg.Session != nil && cfg.Session.MainKey != "" {
		mainKey = routing.NormalizeMainKey(cfg.Session.MainKey)
	}

	return routing.BuildAgentMainSessionKey(agentID, mainKey)
}

// ResolveAgentMainSessionKey 推断指定 agentId 的主会话 key。
// TS 参考: src/config/sessions/main-session.ts resolveAgentMainSessionKey
func ResolveAgentMainSessionKey(cfg *MainSessionConfig, agentID string) string {
	mainKey := routing.DefaultMainKey
	if cfg != nil && cfg.Session != nil && cfg.Session.MainKey != "" {
		mainKey = routing.NormalizeMainKey(cfg.Session.MainKey)
	}
	return routing.BuildAgentMainSessionKey(routing.NormalizeAgentID(agentID), mainKey)
}

// ResolveExplicitAgentSessionKey 如果 agentId 非空，返回对应主会话 key，否则返回空字符串。
// TS 参考: src/config/sessions/main-session.ts resolveExplicitAgentSessionKey
func ResolveExplicitAgentSessionKey(cfg *MainSessionConfig, agentID string) string {
	trimmed := routing.NormalizeAgentID(agentID)
	if trimmed == "" || trimmed == routing.DefaultAgentID {
		// 如果 agentID 本身就是空/默认，视调用方意图而定；若原始值为空则返回空
		if agentID == "" {
			return ""
		}
	}
	return ResolveAgentMainSessionKey(cfg, agentID)
}

// CanonicalizeMainSessionAlias 将主会话的各种别名（"main"、配置 mainKey 等）
// 规范化为标准 session key。
// TS 参考: src/config/sessions/main-session.ts canonicalizeMainSessionAlias
func CanonicalizeMainSessionAlias(cfg *MainSessionConfig, agentID, sessionKey string) string {
	raw := sessionKey
	if raw == "" {
		return raw
	}

	normalizedAgentID := routing.NormalizeAgentID(agentID)

	mainKey := routing.DefaultMainKey
	if cfg != nil && cfg.Session != nil && cfg.Session.MainKey != "" {
		mainKey = routing.NormalizeMainKey(cfg.Session.MainKey)
	}

	agentMainSessionKey := routing.BuildAgentMainSessionKey(normalizedAgentID, mainKey)
	agentMainAliasKey := routing.BuildAgentMainSessionKey(normalizedAgentID, "main")

	isMainAlias := raw == "main" ||
		raw == mainKey ||
		raw == agentMainSessionKey ||
		raw == agentMainAliasKey

	if cfg != nil && cfg.Session != nil && cfg.Session.Scope == SessionScopeGlobal && isMainAlias {
		return "global"
	}
	if isMainAlias {
		return agentMainSessionKey
	}
	return raw
}
