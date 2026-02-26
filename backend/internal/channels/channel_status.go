package channels

// 频道状态快照构建 — 继承自 src/channels/plugins/status.ts (37L)

// BuildChannelAccountSnapshotParams 状态快照构建参数
type BuildChannelAccountSnapshotParams struct {
	ChannelID  ChannelID
	AccountID  string
	Enabled    *bool
	Configured *bool
	Runtime    *AccountSnapshot
}

// BuildChannelAccountSnapshot 构建频道账户快照
func BuildChannelAccountSnapshot(p BuildChannelAccountSnapshotParams) *AccountSnapshot {
	// 如果有运行时快照，优先使用
	if p.Runtime != nil {
		return p.Runtime
	}
	snap := &AccountSnapshot{
		AccountID: p.AccountID,
	}
	if p.Enabled != nil {
		if *p.Enabled {
			snap.Status = "running"
		} else {
			snap.Status = "stopped"
		}
	}
	return snap
}
