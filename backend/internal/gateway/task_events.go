package gateway

// task_events.go — task.* WS 事件结构体
//
// 定义任务看板系统所需的 WebSocket 事件类型。
// 这些事件与现有 channel.message.incoming 广播并行工作，
// 提供结构化的任务状态数据，供前端看板 UI 消费。
//
// 事件生命周期:
//   task.queued    → 任务入队（Phase 2: async=true 时广播）
//   task.started   → 任务开始执行
//   task.progress  → 工具调用进度更新
//   task.completed → 任务执行完成
//   task.failed    → 任务执行失败

// ---------- 事件名常量 ----------

const (
	EventTaskQueued    = "task.queued"
	EventTaskStarted   = "task.started"
	EventTaskProgress  = "task.progress"
	EventTaskCompleted = "task.completed"
	EventTaskFailed    = "task.failed"
)

// ---------- 事件载荷结构体 ----------

// TaskQueuedEvent 任务入队事件。
// Phase 2 中 async=true 时广播，表示任务已加入后台执行队列。
type TaskQueuedEvent struct {
	TaskID     string `json:"taskId"`          // 任务 ID（= runId）
	SessionKey string `json:"sessionKey"`      // 发起方 session
	Text       string `json:"text"`            // 用户消息摘要
	Ts         int64  `json:"ts"`              // 入队时间戳 (UnixMilli)
	Async      bool   `json:"async,omitempty"` // 是否异步执行
}

// TaskStartedEvent 任务开始执行事件。
type TaskStartedEvent struct {
	TaskID     string `json:"taskId"`     // 任务 ID（= runId）
	SessionKey string `json:"sessionKey"` // 发起方 session
	Ts         int64  `json:"ts"`         // 开始时间戳 (UnixMilli)
}

// TaskProgressEvent 任务进度事件（工具调用粒度）。
type TaskProgressEvent struct {
	TaskID     string `json:"taskId"`             // 任务 ID（= runId）
	SessionKey string `json:"sessionKey"`         // 发起方 session
	ToolName   string `json:"toolName"`           // 工具名称（bash, edit, read 等）
	ToolID     string `json:"toolId,omitempty"`   // 工具调用 ID
	Phase      string `json:"phase"`              // "start" | "end"
	Text       string `json:"text"`               // 人类可读的进度描述
	IsError    bool   `json:"isError,omitempty"`  // 工具执行是否失败（仅 phase=end）
	Duration   int64  `json:"duration,omitempty"` // 工具执行耗时 ms（仅 phase=end）
	Ts         int64  `json:"ts"`                 // 事件时间戳 (UnixMilli)
}

// TaskCompletedEvent 任务完成事件。
type TaskCompletedEvent struct {
	TaskID     string `json:"taskId"`            // 任务 ID（= runId）
	SessionKey string `json:"sessionKey"`        // 发起方 session
	Summary    string `json:"summary,omitempty"` // 结果摘要
	Ts         int64  `json:"ts"`                // 完成时间戳 (UnixMilli)
}

// TaskFailedEvent 任务失败事件。
type TaskFailedEvent struct {
	TaskID     string `json:"taskId"`     // 任务 ID（= runId）
	SessionKey string `json:"sessionKey"` // 发起方 session
	Error      string `json:"error"`      // 错误描述
	Ts         int64  `json:"ts"`         // 失败时间戳 (UnixMilli)
}
