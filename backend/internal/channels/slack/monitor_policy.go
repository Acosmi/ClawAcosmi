package slack

// Slack 频道策略 — 继承自 src/slack/monitor/policy.ts (17L)

// IsSlackChannelAllowedByPolicy 判断频道是否满足策略要求。
func IsSlackChannelAllowedByPolicy(groupPolicy string, channelAllowlistConfigured, channelAllowed bool) bool {
	switch groupPolicy {
	case "disabled":
		return false
	case "open":
		return true
	default: // "allowlist"
		if !channelAllowlistConfigured {
			return false
		}
		return channelAllowed
	}
}
