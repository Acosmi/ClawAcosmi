package cron

import (
	"sync"
	"time"
)

// ============================================================================
// 服务状态 — CronService 的内部状态、依赖注入和事件类型
// 对应 TS: cron/service/state.ts (96L)
// ============================================================================

// --- 依赖注入接口 ---

// CronServiceDeps 外部依赖（由调用方注入）
type CronServiceDeps struct {
	// StorePath cron store 文件路径（如 ~/.config/openacosmi/cron/jobs.json）
	StorePath string

	// Logger 日志接口
	Logger CronLogger

	// OnEvent 事件回调（Job 状态变化通知）
	OnEvent func(event CronEvent)

	// EnqueueSystemEvent 入队系统事件（main session job 触发）
	EnqueueSystemEvent func(text string) error

	// RequestHeartbeatNow 立即请求心跳（唤醒 main session）
	RequestHeartbeatNow func()

	// RunIsolatedAgentJob 执行隔离 agent 任务（isolated session job 触发）
	// C2 完成后实现，目前为占位接口
	RunIsolatedAgentJob func(params IsolatedAgentJobParams) (*IsolatedAgentJobResult, error)

	// NowMs 获取当前时间戳（毫秒），nil 使用 time.Now()
	NowMs func() int64
}

// CronLogger 日志接口
type CronLogger interface {
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// IsolatedAgentJobParams 隔离 agent 任务参数
type IsolatedAgentJobParams struct {
	JobID   string
	Payload CronPayload
	AgentID string
}

// IsolatedAgentJobResult 隔离 agent 任务结果
type IsolatedAgentJobResult struct {
	SessionID  string
	SessionKey string
	Summary    string
}

// --- 服务状态 ---

// CronServiceState 服务运行时状态
type CronServiceState struct {
	Deps CronServiceDeps

	mu    sync.Mutex
	store *CronStoreFile

	// timer 管理
	timer     *time.Timer
	timerStop chan struct{}

	// 运行状态
	running bool
	// 当前执行中的操作名
	op string

	// 加载元数据
	storeLoadedAtMs int64
	storeModTimeMs  int64
	warned          map[string]bool
}

// CreateCronServiceState 创建服务状态
func CreateCronServiceState(deps CronServiceDeps) *CronServiceState {
	return &CronServiceState{
		Deps:    deps,
		warned:  make(map[string]bool),
		store:   nil,
		running: false,
	}
}

// NowMs 获取当前时间戳（毫秒），优先使用注入的获取函数
func (s *CronServiceState) NowMs() int64 {
	if s.Deps.NowMs != nil {
		return s.Deps.NowMs()
	}
	return time.Now().UnixMilli()
}

// --- 事件类型 ---

// CronEventKind 事件类型
type CronEventKind string

const (
	EventKindStarted  CronEventKind = "started"
	EventKindStopped  CronEventKind = "stopped"
	EventKindJobAdded CronEventKind = "jobAdded"
	EventKindJobRun   CronEventKind = "jobRun"
	EventKindJobDone  CronEventKind = "jobDone"
	EventKindJobError CronEventKind = "jobError"
)

// CronEvent 事件
type CronEvent struct {
	Kind  CronEventKind `json:"kind"`
	JobID string        `json:"jobId,omitempty"`
	Error string        `json:"error,omitempty"`
}

// --- 操作结果类型 ---

// CronOpResult 通用操作结果
type CronOpResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CronAddResult 添加操作结果
type CronAddResult struct {
	CronOpResult
	JobID string   `json:"jobId,omitempty"`
	Job   *CronJob `json:"job,omitempty"`
}

// CronStatusResult 状态查询结果
type CronStatusResult struct {
	Running  bool   `json:"running"`
	JobCount int    `json:"jobCount"`
	Op       string `json:"op,omitempty"`
}

// CronRunResult 手动运行结果
type CronRunResult struct {
	CronOpResult
	Status    CronJobStatus `json:"status,omitempty"`
	SessionID string        `json:"sessionId,omitempty"`
	Summary   string        `json:"summary,omitempty"`
}
