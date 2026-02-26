package cron

// ============================================================================
// Cron 类型定义 — 定时任务系统核心数据结构
// 对应 TS: cron/types.ts (97L)
// ============================================================================

// CronScheduleKind 调度类型
type CronScheduleKind string

const (
	ScheduleKindAt    CronScheduleKind = "at"
	ScheduleKindEvery CronScheduleKind = "every"
	ScheduleKindCron  CronScheduleKind = "cron"
)

// CronSchedule 定时调度配置
// TS 使用 tagged union，Go 使用 Kind + 可选字段
type CronSchedule struct {
	Kind CronScheduleKind `json:"kind"`
	// kind=at: ISO-8601 时间字符串
	At string `json:"at,omitempty"`
	// kind=every: 间隔毫秒数
	EveryMs int64 `json:"everyMs,omitempty"`
	// kind=every: 锚点时间戳（毫秒）
	AnchorMs *int64 `json:"anchorMs,omitempty"`
	// kind=cron: cron 表达式
	Expr string `json:"expr,omitempty"`
	// kind=cron: 时区（IANA 格式）
	Tz string `json:"tz,omitempty"`
}

// CronSessionTarget 会话目标类型
type CronSessionTarget string

const (
	SessionTargetMain     CronSessionTarget = "main"
	SessionTargetIsolated CronSessionTarget = "isolated"
)

// CronWakeMode 唤醒模式
type CronWakeMode string

const (
	WakeModeNextHeartbeat CronWakeMode = "next-heartbeat"
	WakeModeNow           CronWakeMode = "now"
)

// CronMessageChannel 投递渠道（频道ID 或 "last"）
type CronMessageChannel = string

// CronDeliveryMode 投递模式
type CronDeliveryMode string

const (
	DeliveryModeNone     CronDeliveryMode = "none"
	DeliveryModeAnnounce CronDeliveryMode = "announce"
)

// CronDelivery 投递配置
type CronDelivery struct {
	Mode       CronDeliveryMode   `json:"mode"`
	Channel    CronMessageChannel `json:"channel,omitempty"`
	To         string             `json:"to,omitempty"`
	BestEffort *bool              `json:"bestEffort,omitempty"`
}

// CronDeliveryPatch 投递配置补丁
type CronDeliveryPatch struct {
	Mode       *CronDeliveryMode `json:"mode,omitempty"`
	Channel    *string           `json:"channel,omitempty"`
	To         *string           `json:"to,omitempty"`
	BestEffort *bool             `json:"bestEffort,omitempty"`
}

// CronPayloadKind 负载类型
type CronPayloadKind string

const (
	PayloadKindSystemEvent CronPayloadKind = "systemEvent"
	PayloadKindAgentTurn   CronPayloadKind = "agentTurn"
)

// CronPayload 任务负载
type CronPayload struct {
	Kind    CronPayloadKind `json:"kind"`
	Text    string          `json:"text,omitempty"`    // kind=systemEvent
	Message string          `json:"message,omitempty"` // kind=agentTurn
	// 以下字段仅 kind=agentTurn 有效
	Model                      string `json:"model,omitempty"`
	Thinking                   string `json:"thinking,omitempty"`
	TimeoutSeconds             *int   `json:"timeoutSeconds,omitempty"`
	AllowUnsafeExternalContent *bool  `json:"allowUnsafeExternalContent,omitempty"`
	Deliver                    *bool  `json:"deliver,omitempty"`
	Channel                    string `json:"channel,omitempty"`
	To                         string `json:"to,omitempty"`
	BestEffortDeliver          *bool  `json:"bestEffortDeliver,omitempty"`
}

// CronPayloadPatch 负载补丁
type CronPayloadPatch struct {
	Kind                       CronPayloadKind `json:"kind"`
	Text                       *string         `json:"text,omitempty"`
	Message                    *string         `json:"message,omitempty"`
	Model                      *string         `json:"model,omitempty"`
	Thinking                   *string         `json:"thinking,omitempty"`
	TimeoutSeconds             *int            `json:"timeoutSeconds,omitempty"`
	AllowUnsafeExternalContent *bool           `json:"allowUnsafeExternalContent,omitempty"`
	Deliver                    *bool           `json:"deliver,omitempty"`
	Channel                    *string         `json:"channel,omitempty"`
	To                         *string         `json:"to,omitempty"`
	BestEffortDeliver          *bool           `json:"bestEffortDeliver,omitempty"`
}

// CronJobStatus Job 执行状态
type CronJobStatus string

const (
	JobStatusOk      CronJobStatus = "ok"
	JobStatusError   CronJobStatus = "error"
	JobStatusSkipped CronJobStatus = "skipped"
)

// CronJobState Job 运行时状态
type CronJobState struct {
	NextRunAtMs    *int64         `json:"nextRunAtMs,omitempty"`
	RunningAtMs    *int64         `json:"runningAtMs,omitempty"`
	LastRunAtMs    *int64         `json:"lastRunAtMs,omitempty"`
	LastStatus     *CronJobStatus `json:"lastStatus,omitempty"`
	LastError      *string        `json:"lastError,omitempty"`
	LastDurationMs *int64         `json:"lastDurationMs,omitempty"`
	// 连续执行错误次数（成功时重置）。用于退避。
	ConsecutiveErrors *int `json:"consecutiveErrors,omitempty"`
}

// CronJob 定时任务
type CronJob struct {
	ID             string            `json:"id"`
	AgentID        string            `json:"agentId,omitempty"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Enabled        bool              `json:"enabled"`
	DeleteAfterRun *bool             `json:"deleteAfterRun,omitempty"`
	CreatedAtMs    int64             `json:"createdAtMs"`
	UpdatedAtMs    int64             `json:"updatedAtMs"`
	Schedule       CronSchedule      `json:"schedule"`
	SessionTarget  CronSessionTarget `json:"sessionTarget"`
	WakeMode       CronWakeMode      `json:"wakeMode"`
	Payload        CronPayload       `json:"payload"`
	Delivery       *CronDelivery     `json:"delivery,omitempty"`
	State          CronJobState      `json:"state"`
}

// CronStoreFile 持久化文件格式
type CronStoreFile struct {
	Version int       `json:"version"`
	Jobs    []CronJob `json:"jobs"`
}

// CronJobCreate 创建任务入参（无 id/createdAtMs/updatedAtMs/state）
type CronJobCreate struct {
	AgentID        string            `json:"agentId,omitempty"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Enabled        *bool             `json:"enabled,omitempty"`
	DeleteAfterRun *bool             `json:"deleteAfterRun,omitempty"`
	Schedule       CronSchedule      `json:"schedule"`
	SessionTarget  CronSessionTarget `json:"sessionTarget"`
	WakeMode       CronWakeMode      `json:"wakeMode"`
	Payload        CronPayload       `json:"payload"`
	Delivery       *CronDelivery     `json:"delivery,omitempty"`
	State          *CronJobState     `json:"state,omitempty"`
}

// CronJobPatch 更新任务入参（部分字段可选）
type CronJobPatch struct {
	AgentID        *string            `json:"agentId,omitempty"`
	Name           *string            `json:"name,omitempty"`
	Description    *string            `json:"description,omitempty"`
	Enabled        *bool              `json:"enabled,omitempty"`
	DeleteAfterRun *bool              `json:"deleteAfterRun,omitempty"`
	Schedule       *CronSchedule      `json:"schedule,omitempty"`
	SessionTarget  *CronSessionTarget `json:"sessionTarget,omitempty"`
	WakeMode       *CronWakeMode      `json:"wakeMode,omitempty"`
	Payload        *CronPayloadPatch  `json:"payload,omitempty"`
	Delivery       *CronDeliveryPatch `json:"delivery,omitempty"`
	State          *CronJobState      `json:"state,omitempty"`
}
