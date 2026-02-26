package reply

import "strings"

// TS 对照: auto-reply/reply/queue/normalize.ts (45L)
// 队列模式和丢弃策略的别名归一化。

// NormalizeQueueMode 将各种别名归一化为标准 QueueMode。
func NormalizeQueueMode(raw string) QueueMode {
	if raw == "" {
		return ""
	}
	cleaned := strings.TrimSpace(strings.ToLower(raw))
	switch cleaned {
	case "queue", "queued":
		return QueueModeSteer
	case "interrupt", "interrupts", "abort":
		return QueueModeInterrupt
	case "steer", "steering":
		return QueueModeSteer
	case "followup", "follow-ups", "followups":
		return QueueModeFollowup
	case "collect", "coalesce":
		return QueueModeCollect
	case "steer+backlog", "steer-backlog", "steer_backlog":
		return QueueModeSteerBacklog
	default:
		return ""
	}
}

// NormalizeQueueDropPolicy 将各种别名归一化为标准 QueueDropPolicy。
func NormalizeQueueDropPolicy(raw string) QueueDropPolicy {
	if raw == "" {
		return ""
	}
	cleaned := strings.TrimSpace(strings.ToLower(raw))
	switch cleaned {
	case "old", "oldest":
		return QueueDropOld
	case "new", "newest":
		return QueueDropNew
	case "summarize", "summary":
		return QueueDropSummarize
	default:
		return ""
	}
}
