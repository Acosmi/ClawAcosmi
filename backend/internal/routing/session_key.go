// Package routing 实现会话路由和 session key 管理。
//
// 对齐 TS:
//   - src/routing/session-key.ts (263L)
//   - src/sessions/session-key-utils.ts (76L)
package routing

import (
	"regexp"
	"strings"
)

// ---------- 常量 ----------

const (
	// DefaultAgentID 默认 agent ID。
	DefaultAgentID = "main"
	// DefaultMainKey 默认主 session key。
	DefaultMainKey = "main"
	// DefaultAccountID 默认账户 ID。
	DefaultAccountID = "default"
)

// SessionKeyShape session key 的形状分类。
type SessionKeyShape string

const (
	SessionKeyMissing        SessionKeyShape = "missing"
	SessionKeyAgent          SessionKeyShape = "agent"
	SessionKeyLegacyOrAlias  SessionKeyShape = "legacy_or_alias"
	SessionKeyMalformedAgent SessionKeyShape = "malformed_agent"
)

// ---------- 正则 ----------

var (
	validIDRE      = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9_-]{0,63}$`)
	invalidCharsRE = regexp.MustCompile(`[^a-z0-9_-]+`)
	leadingDashRE  = regexp.MustCompile(`^-+`)
	trailingDashRE = regexp.MustCompile(`-+$`)
)

// ---------- ParsedAgentSessionKey ----------

// ParsedAgentSessionKey 解析后的 agent session key。
type ParsedAgentSessionKey struct {
	AgentID string
	Rest    string
}

// ParseAgentSessionKey 解析 "agent:<agentId>:<rest>" 格式的 session key。
// 对齐 TS: src/sessions/session-key-utils.ts parseAgentSessionKey()
func ParseAgentSessionKey(sessionKey string) *ParsedAgentSessionKey {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return nil
	}
	// 按 ":" 分割，过滤空段
	parts := splitNonEmpty(raw, ":")
	if len(parts) < 3 {
		return nil
	}
	if parts[0] != "agent" {
		return nil
	}
	agentID := strings.TrimSpace(parts[1])
	rest := strings.Join(parts[2:], ":")
	if agentID == "" || rest == "" {
		return nil
	}
	return &ParsedAgentSessionKey{AgentID: agentID, Rest: rest}
}

// ---------- 分类 ----------

// IsSubagentSessionKey 检查是否为 subagent session key。
// 对齐 TS: src/sessions/session-key-utils.ts isSubagentSessionKey()
func IsSubagentSessionKey(sessionKey string) bool {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(raw), "subagent:") {
		return true
	}
	parsed := ParseAgentSessionKey(raw)
	if parsed == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(parsed.Rest), "subagent:")
}

// IsACPSessionKey 检查是否为 ACP session key。
// 对齐 TS: src/sessions/session-key-utils.ts isAcpSessionKey()
func IsACPSessionKey(sessionKey string) bool {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return false
	}
	normalized := strings.ToLower(raw)
	if strings.HasPrefix(normalized, "acp:") {
		return true
	}
	parsed := ParseAgentSessionKey(raw)
	if parsed == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(parsed.Rest), "acp:")
}

// ClassifySessionKeyShape 分类 session key 形状。
// 对齐 TS: src/routing/session-key.ts classifySessionKeyShape()
func ClassifySessionKeyShape(sessionKey string) SessionKeyShape {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return SessionKeyMissing
	}
	if ParseAgentSessionKey(raw) != nil {
		return SessionKeyAgent
	}
	if strings.HasPrefix(strings.ToLower(raw), "agent:") {
		return SessionKeyMalformedAgent
	}
	return SessionKeyLegacyOrAlias
}

// ---------- 标准化 ----------

// NormalizeAgentID 标准化 agent ID。
// 保持 path-safe + shell-friendly。无效字符折叠为 "-"。
// 对齐 TS: src/routing/session-key.ts normalizeAgentId()
func NormalizeAgentID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultAgentID
	}
	if validIDRE.MatchString(trimmed) {
		return strings.ToLower(trimmed)
	}
	// Best-effort fallback
	result := strings.ToLower(trimmed)
	result = invalidCharsRE.ReplaceAllString(result, "-")
	result = leadingDashRE.ReplaceAllString(result, "")
	result = trailingDashRE.ReplaceAllString(result, "")
	if len(result) > 64 {
		result = result[:64]
	}
	if result == "" {
		return DefaultAgentID
	}
	return result
}

// SanitizeAgentID 清理 agent ID（与 NormalizeAgentID 相同逻辑）。
// 对齐 TS: src/routing/session-key.ts sanitizeAgentId()
func SanitizeAgentID(value string) string {
	return NormalizeAgentID(value)
}

// NormalizeMainKey 标准化主 session key。
// 对齐 TS: src/routing/session-key.ts normalizeMainKey()
func NormalizeMainKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultMainKey
	}
	return strings.ToLower(trimmed)
}

// NormalizeAccountID 标准化账户 ID。
// 对齐 TS: src/routing/session-key.ts normalizeAccountId()
func NormalizeAccountID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultAccountID
	}
	if validIDRE.MatchString(trimmed) {
		return strings.ToLower(trimmed)
	}
	result := strings.ToLower(trimmed)
	result = invalidCharsRE.ReplaceAllString(result, "-")
	result = leadingDashRE.ReplaceAllString(result, "")
	result = trailingDashRE.ReplaceAllString(result, "")
	if len(result) > 64 {
		result = result[:64]
	}
	if result == "" {
		return DefaultAccountID
	}
	return result
}

// ---------- Session Key 构建 ----------

// BuildAgentMainSessionKey 构建 agent 的主 session key。
// 格式: "agent:<agentId>:<mainKey>"
// 对齐 TS: src/routing/session-key.ts buildAgentMainSessionKey()
func BuildAgentMainSessionKey(agentID, mainKey string) string {
	return "agent:" + NormalizeAgentID(agentID) + ":" + NormalizeMainKey(mainKey)
}

// ToAgentRequestSessionKey 从 store key 提取 request key。
// 对齐 TS: src/routing/session-key.ts toAgentRequestSessionKey()
func ToAgentRequestSessionKey(storeKey string) string {
	raw := strings.TrimSpace(storeKey)
	if raw == "" {
		return ""
	}
	parsed := ParseAgentSessionKey(raw)
	if parsed != nil {
		return parsed.Rest
	}
	return raw
}

// ToAgentStoreSessionKey 从 request key 构建 store key。
// 对齐 TS: src/routing/session-key.ts toAgentStoreSessionKey()
func ToAgentStoreSessionKey(agentID, requestKey, mainKey string) string {
	raw := strings.TrimSpace(requestKey)
	if raw == "" || raw == DefaultMainKey {
		return BuildAgentMainSessionKey(agentID, mainKey)
	}
	lowered := strings.ToLower(raw)
	if strings.HasPrefix(lowered, "agent:") {
		return lowered
	}
	normalizedAgent := NormalizeAgentID(agentID)
	return "agent:" + normalizedAgent + ":" + lowered
}

// ResolveAgentIDFromSessionKey 从 session key 解析 agent ID。
// 对齐 TS: src/routing/session-key.ts resolveAgentIdFromSessionKey()
func ResolveAgentIDFromSessionKey(sessionKey string) string {
	parsed := ParseAgentSessionKey(sessionKey)
	if parsed != nil {
		return NormalizeAgentID(parsed.AgentID)
	}
	return NormalizeAgentID(DefaultAgentID)
}

// ---------- Peer Session Key ----------

// PeerSessionKeyParams 构建 peer session key 的参数。
type PeerSessionKeyParams struct {
	AgentID       string
	MainKey       string
	Channel       string
	AccountID     string
	PeerKind      string // "direct", "group", "channel"
	PeerID        string
	IdentityLinks map[string][]string
	DMScope       string // "main", "per-peer", "per-channel-peer", "per-account-channel-peer"
}

// BuildAgentPeerSessionKey 构建基于 peer 信息的 session key。
// 对齐 TS: src/routing/session-key.ts buildAgentPeerSessionKey()
func BuildAgentPeerSessionKey(p PeerSessionKeyParams) string {
	peerKind := p.PeerKind
	if peerKind == "" {
		peerKind = "direct"
	}

	normalizedAgent := NormalizeAgentID(p.AgentID)

	if peerKind == "direct" {
		dmScope := p.DMScope
		if dmScope == "" {
			dmScope = "main"
		}

		peerId := strings.TrimSpace(p.PeerID)

		// 尝试 identity link 解析
		if dmScope != "main" {
			if linked := resolveLinkedPeerID(p.IdentityLinks, p.Channel, peerId); linked != "" {
				peerId = linked
			}
		}
		peerId = strings.ToLower(peerId)

		switch dmScope {
		case "per-account-channel-peer":
			if peerId != "" {
				channel := strings.ToLower(strings.TrimSpace(p.Channel))
				if channel == "" {
					channel = "unknown"
				}
				accountID := NormalizeAccountID(p.AccountID)
				return "agent:" + normalizedAgent + ":" + channel + ":" + accountID + ":direct:" + peerId
			}
		case "per-channel-peer":
			if peerId != "" {
				channel := strings.ToLower(strings.TrimSpace(p.Channel))
				if channel == "" {
					channel = "unknown"
				}
				return "agent:" + normalizedAgent + ":" + channel + ":direct:" + peerId
			}
		case "per-peer":
			if peerId != "" {
				return "agent:" + normalizedAgent + ":direct:" + peerId
			}
		}

		return BuildAgentMainSessionKey(p.AgentID, p.MainKey)
	}

	// group / channel
	channel := strings.ToLower(strings.TrimSpace(p.Channel))
	if channel == "" {
		channel = "unknown"
	}
	peerId := strings.ToLower(strings.TrimSpace(p.PeerID))
	if peerId == "" {
		peerId = "unknown"
	}
	return "agent:" + normalizedAgent + ":" + channel + ":" + peerKind + ":" + peerId
}

// resolveLinkedPeerID 从 identity links 查找关联的 peer ID。
// 对齐 TS: src/routing/session-key.ts resolveLinkedPeerId()
func resolveLinkedPeerID(identityLinks map[string][]string, channel, peerID string) string {
	if len(identityLinks) == 0 {
		return ""
	}
	peerId := strings.TrimSpace(peerID)
	if peerId == "" {
		return ""
	}

	candidates := make(map[string]bool)
	rawCandidate := normalizeToken(peerId)
	if rawCandidate != "" {
		candidates[rawCandidate] = true
	}
	ch := normalizeToken(channel)
	if ch != "" {
		scoped := normalizeToken(ch + ":" + peerId)
		if scoped != "" {
			candidates[scoped] = true
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	for canonical, ids := range identityLinks {
		canonicalName := strings.TrimSpace(canonical)
		if canonicalName == "" {
			continue
		}
		for _, id := range ids {
			normalized := normalizeToken(id)
			if normalized != "" && candidates[normalized] {
				return canonicalName
			}
		}
	}
	return ""
}

// ---------- Group History Key ----------

// BuildGroupHistoryKey 构建群组历史记录 key。
// 对齐 TS: src/routing/session-key.ts buildGroupHistoryKey()
func BuildGroupHistoryKey(channel, accountID, peerKind, peerID string) string {
	ch := normalizeToken(channel)
	if ch == "" {
		ch = "unknown"
	}
	acct := NormalizeAccountID(accountID)
	pid := strings.ToLower(strings.TrimSpace(peerID))
	if pid == "" {
		pid = "unknown"
	}
	return ch + ":" + acct + ":" + peerKind + ":" + pid
}

// ---------- Thread Session Key ----------

// ThreadSessionKeyResult 线程 session key 结果。
type ThreadSessionKeyResult struct {
	SessionKey       string
	ParentSessionKey string
}

// ResolveThreadSessionKeys 解析线程 session key。
// 对齐 TS: src/routing/session-key.ts resolveThreadSessionKeys()
func ResolveThreadSessionKeys(baseSessionKey, threadID, parentSessionKey string, useSuffix bool) ThreadSessionKeyResult {
	tid := strings.TrimSpace(threadID)
	if tid == "" {
		return ThreadSessionKeyResult{SessionKey: baseSessionKey}
	}
	normalizedThread := strings.ToLower(tid)
	var sessionKey string
	if useSuffix {
		sessionKey = baseSessionKey + ":thread:" + normalizedThread
	} else {
		sessionKey = baseSessionKey
	}
	return ThreadSessionKeyResult{
		SessionKey:       sessionKey,
		ParentSessionKey: parentSessionKey,
	}
}

// ResolveThreadParentSessionKey 解析线程的父 session key。
// 对齐 TS: src/sessions/session-key-utils.ts resolveThreadParentSessionKey()
func ResolveThreadParentSessionKey(sessionKey string) string {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return ""
	}
	normalized := strings.ToLower(raw)
	markers := []string{":thread:", ":topic:"}

	idx := -1
	for _, marker := range markers {
		candidate := strings.LastIndex(normalized, marker)
		if candidate > idx {
			idx = candidate
		}
	}
	if idx <= 0 {
		return ""
	}
	parent := strings.TrimSpace(raw[:idx])
	return parent
}

// ---------- 内部辅助 ----------

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// splitNonEmpty 按 sep 分割并过滤空段。
func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
