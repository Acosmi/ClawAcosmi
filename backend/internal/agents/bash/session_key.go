// bash/session_key.go — 会话键解析与 Agent ID 管理。
// TS 参考：src/routing/session-key.ts (263L) + src/sessions/session-key-utils.ts (76L)
//
// 解析、构建和规范化 Agent 会话键。
package bash

import (
	"regexp"
	"strings"
)

// ---------- 常量 ----------

const (
	DefaultAgentID   = "main"
	DefaultMainKey   = "main"
	DefaultAccountID = "default"
)

// SessionKeyShape 会话键形状。
type SessionKeyShape string

const (
	SessionKeyMissing        SessionKeyShape = "missing"
	SessionKeyAgent          SessionKeyShape = "agent"
	SessionKeyLegacyOrAlias  SessionKeyShape = "legacy_or_alias"
	SessionKeyMalformedAgent SessionKeyShape = "malformed_agent"
)

// ---------- 正则 ----------

var (
	validIDRe      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
	invalidCharsRe = regexp.MustCompile(`[^a-z0-9_-]+`)
	leadingDashRe  = regexp.MustCompile(`^-+`)
	trailingDashRe = regexp.MustCompile(`-+$`)
)

// ---------- ParsedAgentSessionKey ----------

// ParsedAgentSessionKey 解析后的 agent 会话键。
// TS 参考: session-key-utils.ts ParsedAgentSessionKey
type ParsedAgentSessionKey struct {
	AgentID string
	Rest    string
}

// ParseAgentSessionKey 解析 "agent:ID:rest" 格式的会话键。
// TS 参考: session-key-utils.ts parseAgentSessionKey L6-26
func ParseAgentSessionKey(sessionKey string) *ParsedAgentSessionKey {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ":")
	// 过滤空段
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) < 3 {
		return nil
	}
	if filtered[0] != "agent" {
		return nil
	}

	agentID := strings.TrimSpace(filtered[1])
	rest := strings.Join(filtered[2:], ":")
	if agentID == "" || rest == "" {
		return nil
	}

	return &ParsedAgentSessionKey{AgentID: agentID, Rest: rest}
}

// IsSubagentSessionKey 检查是否是 subagent 会话键。
// TS 参考: session-key-utils.ts isSubagentSessionKey L28-38
func IsSubagentSessionKey(sessionKey string) bool {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(raw), "subagent:") {
		return true
	}
	parsed := ParseAgentSessionKey(raw)
	return parsed != nil && strings.HasPrefix(strings.ToLower(parsed.Rest), "subagent:")
}

// IsAcpSessionKey 检查是否是 ACP 会话键。
// TS 参考: session-key-utils.ts isAcpSessionKey L40-51
func IsAcpSessionKey(sessionKey string) bool {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return false
	}
	normalized := strings.ToLower(raw)
	if strings.HasPrefix(normalized, "acp:") {
		return true
	}
	parsed := ParseAgentSessionKey(raw)
	return parsed != nil && strings.HasPrefix(strings.ToLower(parsed.Rest), "acp:")
}

// ResolveThreadParentSessionKey 解析线程父会话键。
// TS 参考: session-key-utils.ts resolveThreadParentSessionKey L55-75
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
	if parent == "" {
		return ""
	}
	return parent
}

// ---------- 规范化函数 ----------

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// NormalizeMainKey 规范化主键。
// TS 参考: session-key.ts normalizeMainKey L26-29
func NormalizeMainKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultMainKey
	}
	return strings.ToLower(trimmed)
}

// NormalizeAgentId 规范化 agent ID。
// TS 参考: session-key.ts normalizeAgentId L74-92
func NormalizeAgentId(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultAgentID
	}
	if validIDRe.MatchString(trimmed) {
		return strings.ToLower(trimmed)
	}
	// 回退：将无效字符替换为 "-"
	result := strings.ToLower(trimmed)
	result = invalidCharsRe.ReplaceAllString(result, "-")
	result = leadingDashRe.ReplaceAllString(result, "")
	result = trailingDashRe.ReplaceAllString(result, "")
	if len(result) > 64 {
		result = result[:64]
	}
	if result == "" {
		return DefaultAgentID
	}
	return result
}

// SanitizeAgentId 净化 agent ID。
// TS 参考: session-key.ts sanitizeAgentId L94-110
func SanitizeAgentId(value string) string {
	return NormalizeAgentId(value) // 逻辑完全一致
}

// NormalizeAccountId 规范化账户 ID。
// TS 参考: session-key.ts normalizeAccountId L112-128
func NormalizeAccountId(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultAccountID
	}
	if validIDRe.MatchString(trimmed) {
		return strings.ToLower(trimmed)
	}
	result := strings.ToLower(trimmed)
	result = invalidCharsRe.ReplaceAllString(result, "-")
	result = leadingDashRe.ReplaceAllString(result, "")
	result = trailingDashRe.ReplaceAllString(result, "")
	if len(result) > 64 {
		result = result[:64]
	}
	if result == "" {
		return DefaultAccountID
	}
	return result
}

// ---------- 会话键操作 ----------

// ResolveAgentIdFromSessionKey 从会话键提取 agent ID。
// TS 参考: session-key.ts resolveAgentIdFromSessionKey L58-61
func ResolveAgentIdFromSessionKey(sessionKey string) string {
	parsed := ParseAgentSessionKey(sessionKey)
	if parsed != nil {
		return NormalizeAgentId(parsed.AgentID)
	}
	return NormalizeAgentId(DefaultAgentID)
}

// ClassifySessionKeyShape 分类会话键形状。
// TS 参考: session-key.ts classifySessionKeyShape L63-72
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

// ToAgentRequestSessionKey 将 store key 转为 request key。
// TS 参考: session-key.ts toAgentRequestSessionKey L31-37
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

// ToAgentStoreSessionKey 将 request key 转为 store key。
// TS 参考: session-key.ts toAgentStoreSessionKey L39-56
func ToAgentStoreSessionKey(agentID, requestKey, mainKey string) string {
	raw := strings.TrimSpace(requestKey)
	if raw == "" || raw == DefaultMainKey {
		return BuildAgentMainSessionKey(agentID, mainKey)
	}
	lowered := strings.ToLower(raw)
	if strings.HasPrefix(lowered, "agent:") {
		return lowered
	}
	normalizedAgent := NormalizeAgentId(agentID)
	return "agent:" + normalizedAgent + ":" + lowered
}

// BuildAgentMainSessionKey 构建 agent 主会话键。
// TS 参考: session-key.ts buildAgentMainSessionKey L130-137
func BuildAgentMainSessionKey(agentID, mainKey string) string {
	return "agent:" + NormalizeAgentId(agentID) + ":" + NormalizeMainKey(mainKey)
}

// BuildAgentPeerSessionKey 构建 agent peer 会话键。
// TS 参考: session-key.ts buildAgentPeerSessionKey L139-186
func BuildAgentPeerSessionKey(
	agentID, mainKey, channel, accountID, peerKind, peerId, dmScope string,
	identityLinks map[string][]string,
) string {
	if peerKind == "" {
		peerKind = "direct"
	}

	if peerKind == "direct" {
		if dmScope == "" {
			dmScope = "main"
		}

		peerIdTrimmed := strings.TrimSpace(peerId)

		// 尝试身份链接解析
		if dmScope != "main" && peerIdTrimmed != "" {
			if linked := resolveLinkedPeerId(identityLinks, channel, peerIdTrimmed); linked != "" {
				peerIdTrimmed = linked
			}
		}
		peerIdLower := strings.ToLower(peerIdTrimmed)

		normalizedAgent := NormalizeAgentId(agentID)
		chanLower := strings.ToLower(strings.TrimSpace(channel))
		if chanLower == "" {
			chanLower = "unknown"
		}

		switch dmScope {
		case "per-account-channel-peer":
			if peerIdLower != "" {
				acctId := NormalizeAccountId(accountID)
				return "agent:" + normalizedAgent + ":" + chanLower + ":" + acctId + ":direct:" + peerIdLower
			}
		case "per-channel-peer":
			if peerIdLower != "" {
				return "agent:" + normalizedAgent + ":" + chanLower + ":direct:" + peerIdLower
			}
		case "per-peer":
			if peerIdLower != "" {
				return "agent:" + normalizedAgent + ":direct:" + peerIdLower
			}
		}

		return BuildAgentMainSessionKey(agentID, mainKey)
	}

	// 非 direct (group/channel)
	chanLower := strings.ToLower(strings.TrimSpace(channel))
	if chanLower == "" {
		chanLower = "unknown"
	}
	peerIdLower := strings.ToLower(strings.TrimSpace(peerId))
	if peerIdLower == "" {
		peerIdLower = "unknown"
	}
	return "agent:" + NormalizeAgentId(agentID) + ":" + chanLower + ":" + peerKind + ":" + peerIdLower
}

// resolveLinkedPeerId 解析身份链接。
// TS 参考: session-key.ts resolveLinkedPeerId L188-232
func resolveLinkedPeerId(identityLinks map[string][]string, channel, peerId string) string {
	if identityLinks == nil {
		return ""
	}
	peerIdTrimmed := strings.TrimSpace(peerId)
	if peerIdTrimmed == "" {
		return ""
	}

	candidates := make(map[string]bool)
	rawCandidate := normalizeToken(peerIdTrimmed)
	if rawCandidate != "" {
		candidates[rawCandidate] = true
	}
	chanNorm := normalizeToken(channel)
	if chanNorm != "" {
		scopedCandidate := normalizeToken(chanNorm + ":" + peerIdTrimmed)
		if scopedCandidate != "" {
			candidates[scopedCandidate] = true
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

// BuildGroupHistoryKey 构建群历史键。
// TS 参考: session-key.ts buildGroupHistoryKey L234-244
func BuildGroupHistoryKey(channel, accountID, peerKind, peerId string) string {
	chanLower := normalizeToken(channel)
	if chanLower == "" {
		chanLower = "unknown"
	}
	acctId := NormalizeAccountId(accountID)
	peerIdLower := strings.ToLower(strings.TrimSpace(peerId))
	if peerIdLower == "" {
		peerIdLower = "unknown"
	}
	return chanLower + ":" + acctId + ":" + peerKind + ":" + peerIdLower
}

// ResolveThreadSessionKeys 解析线程会话键。
// TS 参考: session-key.ts resolveThreadSessionKeys L246-262
func ResolveThreadSessionKeys(baseSessionKey, threadId, parentSessionKey string, useSuffix bool) (sessionKey, parent string) {
	threadIdTrimmed := strings.TrimSpace(threadId)
	if threadIdTrimmed == "" {
		return baseSessionKey, ""
	}
	normalizedThreadId := strings.ToLower(threadIdTrimmed)
	if useSuffix {
		return baseSessionKey + ":thread:" + normalizedThreadId, parentSessionKey
	}
	return baseSessionKey, parentSessionKey
}
