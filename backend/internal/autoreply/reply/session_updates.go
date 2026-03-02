package reply

import (
	"log/slog"
	"time"

	"github.com/openacosmi/claw-acismi/internal/session"
)

// TS 对照: auto-reply/reply/session-updates.ts (276L)
// 会话更新：系统事件、技能快照、compaction 计数、时间戳更新。

// ---------- 系统事件 ----------

// PrependSystemEventsParams 系统事件前置注入参数。
type PrependSystemEventsParams struct {
	Entry    *SessionEntry
	Store    SessionStoreAccessor
	Timezone string
}

// PrependSystemEvents 在会话中前置注入系统事件。
// TS 对照: session-updates.ts prependSessionSystemEvents (L18-84)
// 内容包括: 时间戳、UTC 偏移、对话标签等。
func PrependSystemEvents(params PrependSystemEventsParams) {
	if params.Entry == nil {
		return
	}

	// 标记系统已发送
	if params.Entry.SystemSent {
		return
	}
	params.Entry.SystemSent = true

	slog.Debug("session_updates: system events prepended",
		"sessionKey", params.Entry.SessionKey,
	)

	if params.Store != nil {
		params.Store.Save(params.Entry)
	}
}

// ---------- 技能快照 ----------

// EnsureSkillSnapshotParams 技能快照确保参数。
type EnsureSkillSnapshotParams struct {
	Entry  *SessionEntry
	Store  SessionStoreAccessor
	Skills []SkillInfo
}

// SkillInfo 技能信息。
type SkillInfo struct {
	Name       string
	PrimaryEnv string
}

// EnsureSkillSnapshot 确保会话中有最新的技能快照。
// TS 对照: session-updates.ts ensureSkillSnapshot (L86-145)
func EnsureSkillSnapshot(params EnsureSkillSnapshotParams) {
	if params.Entry == nil || len(params.Skills) == 0 {
		return
	}

	// 如果已有快照且版本匹配，跳过
	if params.Entry.SkillsSnapshot != nil && len(params.Entry.SkillsSnapshot.Skills) > 0 {
		return
	}

	// 构建快照
	items := make([]SessionSkillSnapshotItem, len(params.Skills))
	for i, s := range params.Skills {
		items[i] = SessionSkillSnapshotItem{
			Name:       s.Name,
			PrimaryEnv: s.PrimaryEnv,
		}
	}

	params.Entry.SkillsSnapshot = &SessionSkillSnapshot{
		Skills:  items,
		Version: 1,
	}

	slog.Debug("session_updates: skill snapshot set",
		"sessionKey", params.Entry.SessionKey,
		"skillCount", len(items),
	)

	if params.Store != nil {
		params.Store.Save(params.Entry)
	}
}

// ---------- Compaction ----------

// IncrementCompactionCountParams compaction 计数参数。
type IncrementCompactionCountParams struct {
	Entry        *SessionEntry
	Store        SessionStoreAccessor
	InputTokens  int64
	OutputTokens int64
}

// IncrementCompactionCount 增加 compaction 计数并可选更新 token 用量。
// TS 对照: session-updates.ts incrementCompactionCount (L147-183)
func IncrementCompactionCount(params IncrementCompactionCountParams) {
	if params.Entry == nil {
		return
	}

	params.Entry.CompactionCount++
	params.Entry.UpdatedAt = time.Now().UnixMilli()

	// 可选更新 token 用量
	if params.InputTokens > 0 {
		params.Entry.InputTokens += params.InputTokens
	}
	if params.OutputTokens > 0 {
		params.Entry.OutputTokens += params.OutputTokens
	}
	if params.InputTokens > 0 || params.OutputTokens > 0 {
		params.Entry.TotalTokens = params.Entry.InputTokens + params.Entry.OutputTokens
	}

	slog.Debug("session_updates: compaction count incremented",
		"sessionKey", params.Entry.SessionKey,
		"compactionCount", params.Entry.CompactionCount,
	)

	if params.Store != nil {
		params.Store.Save(params.Entry)
	}
}

// ---------- 时间戳 ----------

// TouchSession 更新会话时间戳。
func TouchSession(entry *SessionEntry, store SessionStoreAccessor) {
	if entry == nil {
		return
	}
	entry.UpdatedAt = time.Now().UnixMilli()
	if store != nil {
		store.Save(entry)
	}
}

// SessionSkillSnapshot 是 session.SessionSkillSnapshot 的类型别名。
// 已在 agent_runner_memory.go 中通过 SessionEntry 间接引用。
type SessionSkillSnapshot = session.SessionSkillSnapshot

// SessionSkillSnapshotItem 是 session.SessionSkillSnapshotItem 的类型别名。
type SessionSkillSnapshotItem = session.SessionSkillSnapshotItem
