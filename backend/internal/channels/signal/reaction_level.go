package signal

import (
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Signal 反应级别策略 — 继承自 src/signal/reaction-level.ts (72L)

// ResolvedSignalReactionLevel 解析后的反应级别及其含义
type ResolvedSignalReactionLevel struct {
	Level                 types.SignalReactionLevel
	AckEnabled            bool   // 是否启用自动 ACK 反应（如 👀）
	AgentReactionsEnabled bool   // 是否启用 Agent 主动反应
	AgentReactionGuidance string // "minimal"|"extensive"|""
}

// ResolveSignalReactionLevel 解析有效反应级别及其含义
//
// 级别说明：
//   - "off": 不发送任何反应
//   - "ack": 仅自动 ACK 反应（处理中显示 👀），Agent 不主动反应
//   - "minimal": Agent 可反应但保持克制（默认）
//   - "extensive": Agent 可自由反应
func ResolveSignalReactionLevel(cfg *types.OpenAcosmiConfig, accountID string) ResolvedSignalReactionLevel {
	account := ResolveSignalAccount(cfg, accountID)
	level := account.Config.ReactionLevel
	if level == "" {
		level = types.SignalReactionLevelMinimal
	}

	switch level {
	case types.SignalReactionLevelOff:
		return ResolvedSignalReactionLevel{
			Level:                 level,
			AckEnabled:            false,
			AgentReactionsEnabled: false,
		}
	case types.SignalReactionLevelAck:
		return ResolvedSignalReactionLevel{
			Level:                 level,
			AckEnabled:            true,
			AgentReactionsEnabled: false,
		}
	case types.SignalReactionLevelMinimal:
		return ResolvedSignalReactionLevel{
			Level:                 level,
			AckEnabled:            false,
			AgentReactionsEnabled: true,
			AgentReactionGuidance: "minimal",
		}
	case types.SignalReactionLevelExtensive:
		return ResolvedSignalReactionLevel{
			Level:                 level,
			AckEnabled:            false,
			AgentReactionsEnabled: true,
			AgentReactionGuidance: "extensive",
		}
	default:
		// 回退到 minimal 行为
		return ResolvedSignalReactionLevel{
			Level:                 types.SignalReactionLevelMinimal,
			AckEnabled:            false,
			AgentReactionsEnabled: true,
			AgentReactionGuidance: "minimal",
		}
	}
}
