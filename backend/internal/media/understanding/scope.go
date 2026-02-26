package understanding

import "strings"

// TS 对照: media-understanding/scope.ts (65L)

// ScopeDecision 作用域决策。
// TS 对照: scope.ts L8-14
type ScopeDecision struct {
	Allowed bool
	Reason  string
}

// ScopeParams 作用域决策参数。
// TS 对照: scope.ts L16-25
type ScopeParams struct {
	Channel    string
	ChatType   string // "dm", "group", "channel"
	SessionKey string
	Kind       Kind
	// 配置中的作用域规则
	AllowedChannels  []string
	AllowedChatTypes []string
	DeniedChannels   []string
}

// ResolveMediaUnderstandingScope 解析媒体理解作用域。
// 返回是否允许执行指定能力。
// TS 对照: scope.ts L27-65
func ResolveMediaUnderstandingScope(params ScopeParams) ScopeDecision {
	// 默认允许
	if len(params.AllowedChannels) == 0 && len(params.AllowedChatTypes) == 0 && len(params.DeniedChannels) == 0 {
		return ScopeDecision{Allowed: true}
	}

	// 检查拒绝列表
	for _, denied := range params.DeniedChannels {
		if strings.EqualFold(denied, params.Channel) {
			return ScopeDecision{
				Allowed: false,
				Reason:  "channel " + params.Channel + " is in deny list",
			}
		}
	}

	// 检查允许的频道
	if len(params.AllowedChannels) > 0 {
		found := false
		for _, allowed := range params.AllowedChannels {
			if strings.EqualFold(allowed, params.Channel) {
				found = true
				break
			}
		}
		if !found {
			return ScopeDecision{
				Allowed: false,
				Reason:  "channel " + params.Channel + " not in allow list",
			}
		}
	}

	// 检查允许的聊天类型
	if len(params.AllowedChatTypes) > 0 {
		found := false
		for _, ct := range params.AllowedChatTypes {
			if strings.EqualFold(ct, params.ChatType) {
				found = true
				break
			}
		}
		if !found {
			return ScopeDecision{
				Allowed: false,
				Reason:  "chat type " + params.ChatType + " not in allow list",
			}
		}
	}

	return ScopeDecision{Allowed: true}
}
