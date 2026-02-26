package telegram

import "github.com/anthropic/open-acosmi/pkg/types"

// Telegram 反应级别解析 — 继承自 src/telegram/reaction-level.ts (65L)

// TelegramReactionLevel 反应级别
type TelegramReactionLevel string

const (
	ReactionLevelOff       TelegramReactionLevel = "off"
	ReactionLevelAck       TelegramReactionLevel = "ack"
	ReactionLevelMinimal   TelegramReactionLevel = "minimal"
	ReactionLevelExtensive TelegramReactionLevel = "extensive"
)

// ResolvedReactionLevel 解析后的反应级别及其含义
type ResolvedReactionLevel struct {
	Level                 TelegramReactionLevel
	AckEnabled            bool   // 是否启用确认反应（如处理中 👀）
	AgentReactionsEnabled bool   // 是否启用 agent 控制的反应
	AgentReactionGuidance string // 反应引导级别（"minimal" 或 "extensive"）
}

// ResolveTelegramReactionLevel 解析 Telegram 账户的有效反应级别。
func ResolveTelegramReactionLevel(cfg *types.OpenAcosmiConfig, accountID string) ResolvedReactionLevel {
	account := ResolveTelegramAccount(cfg, accountID)
	level := TelegramReactionLevel(account.Config.ReactionLevel)
	if level == "" {
		level = ReactionLevelMinimal
	}

	switch level {
	case ReactionLevelOff:
		return ResolvedReactionLevel{
			Level:      level,
			AckEnabled: false,
		}
	case ReactionLevelAck:
		return ResolvedReactionLevel{
			Level:      level,
			AckEnabled: true,
		}
	case ReactionLevelMinimal:
		return ResolvedReactionLevel{
			Level:                 level,
			AgentReactionsEnabled: true,
			AgentReactionGuidance: "minimal",
		}
	case ReactionLevelExtensive:
		return ResolvedReactionLevel{
			Level:                 level,
			AgentReactionsEnabled: true,
			AgentReactionGuidance: "extensive",
		}
	default:
		// 回退到 ack 行为
		return ResolvedReactionLevel{
			Level:      ReactionLevelAck,
			AckEnabled: true,
		}
	}
}
