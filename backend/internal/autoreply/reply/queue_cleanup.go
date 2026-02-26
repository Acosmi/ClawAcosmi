package reply

import "strings"

// TS 对照: auto-reply/reply/queue/cleanup.ts (30L)
// 队列清理：批量清除指定 session key 的 followup 队列。

// ClearSessionQueueResult 清理结果。
type ClearSessionQueueResult struct {
	FollowupCleared int
	LaneCleared     int
	Keys            []string
}

// ClearSessionQueues 批量清除指定 session key 的 followup 队列和 command-lane。
// TS: clearSessionQueues(keys: Array<string | undefined>)
func ClearSessionQueues(keys []string) ClearSessionQueueResult {
	seen := make(map[string]struct{})
	var followupCleared int
	var laneCleared int
	var clearedKeys []string

	for _, key := range keys {
		cleaned := strings.TrimSpace(key)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		clearedKeys = append(clearedKeys, cleaned)
		followupCleared += ClearFollowupQueue(cleaned)
		// TS: laneCleared += clearCommandLane(resolveEmbeddedSessionLane(cleaned))
		laneCleared += ClearCommandLane(resolveEmbeddedSessionLane(cleaned))
	}

	return ClearSessionQueueResult{
		FollowupCleared: followupCleared,
		LaneCleared:     laneCleared,
		Keys:            clearedKeys,
	}
}
