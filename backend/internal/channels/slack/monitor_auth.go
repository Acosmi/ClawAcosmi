package slack

// Slack 监控认证 — 继承自 src/slack/monitor/auth.ts (64L)
// Phase 9 实现：pairing store 集成。

// IsSlackSenderAllowListed 检查发送者是否在允许列表中。
func IsSlackSenderAllowListed(allowFrom []string, userID, userName string) bool {
	if len(allowFrom) == 0 {
		return false
	}
	return AllowListMatches(allowFrom, userID, userName)
}

// ResolveSlackEffectiveAllowFrom 解析最终的允许列表。
// 合并静态配置 + pairing store 动态允许。
func ResolveSlackEffectiveAllowFrom(monCtx *SlackMonitorContext) []string {
	base := monCtx.AllowFrom

	// 合并 pairing store
	if monCtx.Deps != nil && monCtx.Deps.ReadAllowFromStore != nil {
		dynamic, err := monCtx.Deps.ReadAllowFromStore("slack")
		if err == nil && len(dynamic) > 0 {
			merged := make([]string, 0, len(base)+len(dynamic))
			merged = append(merged, base...)
			merged = append(merged, dynamic...)
			return merged
		}
	}

	return base
}
