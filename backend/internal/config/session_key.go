package config

// session_key.go — 会话 key 解析入口
// TS 参考: src/config/sessions/session-key.ts (48L)
//
// 高层 session key 推断函数，依赖 routing 包的标准化函数和
// 本包的 ResolveGroupSessionKey。

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/routing"
)

// DeriveSessionKey 根据 scope 和消息上下文推断 session key。
// scope=="global" 返回 "global"；群组消息返回群组 key；否则返回发送者标识。
// 对齐 TS: src/config/sessions/session-key.ts deriveSessionKey()
func DeriveSessionKey(scope SessionScope, ctx MsgContext) string {
	if scope == SessionScopeGlobal {
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

// ResolveSessionKey 解析最终 session key（含显式 SessionKey 覆盖和 canonical main key 折叠）。
// 对齐 TS: src/config/sessions/session-key.ts resolveSessionKey()
func ResolveSessionKey(scope SessionScope, ctx MsgContext, mainKey string) string {
	// 显式 SessionKey 优先
	if explicit := strings.TrimSpace(ctx.SessionKey); explicit != "" {
		return strings.ToLower(explicit)
	}

	raw := DeriveSessionKey(scope, ctx)
	if scope == SessionScopeGlobal {
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

// normalizeE164 尝试 E.164 格式标准化（简化版——小写并去空白）。
// 完整版在 channels/normalize.go，此处仅做基本标准化。
func normalizeE164(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}
