package autoreply

import "strings"

// TS 对照: auto-reply/command-auth.ts (271L) — 简化版

// CommandAuthorization 命令授权结果。
// TS 对照: command-auth.ts L10-30
type CommandAuthorization struct {
	Authorized    bool
	Reason        string   // 拒绝原因（如有）
	ProviderID    string   // F2: 频道 provider ID（如 "telegram", "slack"）
	OwnerList     []string // F2: 频道所有者列表
	SenderID      string   // F2: 发送者 ID
	SenderIsOwner bool     // F2: 发送者是否为频道所有者
	From          string   // F2: 来源标识
	To            string   // F2: 目标标识
}

// CommandAuthParams 命令授权参数。
type CommandAuthParams struct {
	SenderID     string
	ChannelType  string
	IsGroup      bool
	IsBotOwner   bool
	AllowedUsers []string // 允许执行命令的用户 ID 列表
	DenyUsers    []string // 拒绝执行命令的用户 ID 列表
}

// ResolveCommandAuthorization 解析命令授权。
// TS 对照: command-auth.ts L40-120（简化）
func ResolveCommandAuthorization(params *CommandAuthParams) CommandAuthorization {
	if params == nil {
		return CommandAuthorization{Authorized: false, Reason: "no params"}
	}

	// Bot 所有者始终被授权
	if params.IsBotOwner {
		return CommandAuthorization{Authorized: true}
	}

	// 检查拒绝列表
	if len(params.DenyUsers) > 0 {
		for _, deny := range params.DenyUsers {
			if strings.EqualFold(deny, params.SenderID) {
				return CommandAuthorization{
					Authorized: false,
					Reason:     "sender in deny list",
				}
			}
		}
	}

	// 检查允许列表（如果设置了允许列表，则仅允许列表中的用户）
	if len(params.AllowedUsers) > 0 {
		for _, allowed := range params.AllowedUsers {
			if strings.EqualFold(allowed, params.SenderID) {
				return CommandAuthorization{Authorized: true}
			}
		}
		return CommandAuthorization{
			Authorized: false,
			Reason:     "sender not in allow list",
		}
	}

	// 无显式列表时：私聊默认授权，群组默认拒绝
	if !params.IsGroup {
		return CommandAuthorization{Authorized: true}
	}

	return CommandAuthorization{
		Authorized: false,
		Reason:     "group chat command not authorized by default",
	}
}

// ResolveFullCommandAuthorization 解析命令授权（含 provider-aware 字段）。
// TS 对照: command-auth.ts resolveCommandAuthorization (完整版)
func ResolveFullCommandAuthorization(params *CommandAuthParams) CommandAuthorization {
	base := ResolveCommandAuthorization(params)
	if params == nil {
		return base
	}

	// 填充 F2 扩展字段
	base.SenderID = params.SenderID
	base.SenderIsOwner = params.IsBotOwner

	// OwnerList: 如果 AllowedUsers 非空，使用它作为 owner list
	if len(params.AllowedUsers) > 0 {
		base.OwnerList = make([]string, len(params.AllowedUsers))
		copy(base.OwnerList, params.AllowedUsers)
	}

	return base
}
