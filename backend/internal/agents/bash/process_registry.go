// bash/process_registry.go — 全局进程注册表。
// TS 参考：src/agents/bash-process-registry.ts (275L)
//
// 管理运行中和已结束的 ProcessSession，提供输出缓冲、TTL 清理。
package bash

import (
	"math"
	"os"
	"strconv"
	"sync"
	"time"
)

// ---------- 常量 ----------

const (
	DefaultJobTTLMs         = 30 * 60 * 1000     // 30 minutes
	MinJobTTLMs             = 60 * 1000          // 1 minute
	MaxJobTTLMs             = 3 * 60 * 60 * 1000 // 3 hours
	DefaultPendingOutputCap = 30_000
	DefaultTailLen          = 2000
)

func clampTTL(value int) int {
	if value <= 0 {
		return DefaultJobTTLMs
	}
	if value < MinJobTTLMs {
		return MinJobTTLMs
	}
	if value > MaxJobTTLMs {
		return MaxJobTTLMs
	}
	return value
}

// ---------- 状态类型 ----------

// ProcessStatus 进程状态。
type ProcessStatus string

const (
	StatusRunning   ProcessStatus = "running"
	StatusCompleted ProcessStatus = "completed"
	StatusFailed    ProcessStatus = "failed"
	StatusKilled    ProcessStatus = "killed"
)

// SessionStdin 会话标准输入接口。
type SessionStdin interface {
	Write(data string) error
	End()
	IsDestroyed() bool
}

// ProcessSession 运行中的进程会话。
// TS 参考: bash-process-registry.ts L26-52
type ProcessSession struct {
	ID                    string
	Command               string
	ScopeKey              string
	SessionKey            string
	NotifyOnExit          bool
	ExitNotified          bool
	Stdin                 SessionStdin
	PID                   int
	StartedAt             int64 // milliseconds since epoch
	Cwd                   string
	MaxOutputChars        int
	PendingMaxOutputChars int
	TotalOutputChars      int
	PendingStdout         []string
	PendingStderr         []string
	PendingStdoutChars    int
	PendingStderrChars    int
	Aggregated            string
	Tail                  string
	ExitCode              *int
	ExitSignal            string
	Exited                bool
	Truncated             bool
	Backgrounded          bool
}

// FinishedSession 已完成的进程会话。
// TS 参考: bash-process-registry.ts L54-68
type FinishedSession struct {
	ID               string        `json:"id"`
	Command          string        `json:"command"`
	ScopeKey         string        `json:"scopeKey,omitempty"`
	StartedAt        int64         `json:"startedAt"`
	EndedAt          int64         `json:"endedAt"`
	Cwd              string        `json:"cwd,omitempty"`
	Status           ProcessStatus `json:"status"`
	ExitCode         *int          `json:"exitCode,omitempty"`
	ExitSignal       string        `json:"exitSignal,omitempty"`
	Aggregated       string        `json:"aggregated"`
	Tail             string        `json:"tail"`
	Truncated        bool          `json:"truncated"`
	TotalOutputChars int           `json:"totalOutputChars"`
}

// ---------- Registry ----------

// ProcessRegistry 全局进程注册表。
// 线程安全，管理运行中和已完成的进程会话。
type ProcessRegistry struct {
	mu           sync.RWMutex
	running      map[string]*ProcessSession
	finished     map[string]*FinishedSession
	jobTTLMs     int
	sweepStop    chan struct{}
	sweepStarted bool
}

// NewProcessRegistry 创建新的进程注册表。
func NewProcessRegistry() *ProcessRegistry {
	ttl := DefaultJobTTLMs
	if raw := os.Getenv("PI_BASH_JOB_TTL_MS"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			ttl = clampTTL(v)
		}
	}
	return &ProcessRegistry{
		running:  make(map[string]*ProcessSession),
		finished: make(map[string]*FinishedSession),
		jobTTLMs: ttl,
	}
}

// AddSession 注册新会话。
func (r *ProcessRegistry) AddSession(session *ProcessSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running[session.ID] = session
	r.startSweeper()
}

// GetSession 获取运行中的会话。
func (r *ProcessRegistry) GetSession(id string) *ProcessSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running[id]
}

// GetFinishedSession 获取已完成的会话。
func (r *ProcessRegistry) GetFinishedSession(id string) *FinishedSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.finished[id]
}

// DeleteSession 删除会话（运行中和已完成）。
func (r *ProcessRegistry) DeleteSession(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.running, id)
	delete(r.finished, id)
}

// IsSessionIDTaken 检查 ID 是否已使用。
func (r *ProcessRegistry) IsSessionIDTaken(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, a := r.running[id]
	_, b := r.finished[id]
	return a || b
}

// AppendOutput 追加进程输出到缓冲区。
// TS 参考: bash-process-registry.ts L101-129
func (r *ProcessRegistry) AppendOutput(session *ProcessSession, stream string, chunk string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if session.PendingStdout == nil {
		session.PendingStdout = make([]string, 0)
	}
	if session.PendingStderr == nil {
		session.PendingStderr = make([]string, 0)
	}

	var buffer *[]string
	var bufferChars *int
	if stream == "stdout" {
		buffer = &session.PendingStdout
		bufferChars = &session.PendingStdoutChars
	} else {
		buffer = &session.PendingStderr
		bufferChars = &session.PendingStderrChars
	}

	pendingCap := session.PendingMaxOutputChars
	if pendingCap <= 0 {
		pendingCap = DefaultPendingOutputCap
	}
	if pendingCap > session.MaxOutputChars && session.MaxOutputChars > 0 {
		pendingCap = session.MaxOutputChars
	}

	*buffer = append(*buffer, chunk)
	pendingChars := *bufferChars + len(chunk)

	if pendingChars > pendingCap {
		session.Truncated = true
		pendingChars = capPendingBuffer(buffer, pendingChars, pendingCap)
	}
	*bufferChars = pendingChars

	session.TotalOutputChars += len(chunk)
	aggregated := TrimWithCap(session.Aggregated+chunk, session.MaxOutputChars)
	if len(aggregated) < len(session.Aggregated)+len(chunk) {
		session.Truncated = true
	}
	session.Aggregated = aggregated
	session.Tail = Tail(session.Aggregated, DefaultTailLen)
}

// DrainSession 提取并清空待处理输出缓冲。
// TS 参考: bash-process-registry.ts L131-139
func (r *ProcessRegistry) DrainSession(session *ProcessSession) (stdout, stderr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stdout = joinAndClear(&session.PendingStdout)
	stderr = joinAndClear(&session.PendingStderr)
	session.PendingStdoutChars = 0
	session.PendingStderrChars = 0
	return
}

func joinAndClear(buffer *[]string) string {
	if len(*buffer) == 0 {
		return ""
	}
	result := ""
	for _, s := range *buffer {
		result += s
	}
	*buffer = (*buffer)[:0]
	return result
}

// MarkExited 标记会话退出。
// TS 参考: bash-process-registry.ts L141-152
func (r *ProcessRegistry) MarkExited(session *ProcessSession, exitCode *int, exitSignal string, status ProcessStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session.Exited = true
	session.ExitCode = exitCode
	session.ExitSignal = exitSignal
	session.Tail = Tail(session.Aggregated, DefaultTailLen)
	r.moveToFinished(session, status)
}

// MarkBackgrounded 标记会话为后台。
func (r *ProcessRegistry) MarkBackgrounded(session *ProcessSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session.Backgrounded = true
}

func (r *ProcessRegistry) moveToFinished(session *ProcessSession, status ProcessStatus) {
	delete(r.running, session.ID)
	if !session.Backgrounded {
		return
	}
	r.finished[session.ID] = &FinishedSession{
		ID:               session.ID,
		Command:          session.Command,
		ScopeKey:         session.ScopeKey,
		StartedAt:        session.StartedAt,
		EndedAt:          time.Now().UnixMilli(),
		Cwd:              session.Cwd,
		Status:           status,
		ExitCode:         session.ExitCode,
		ExitSignal:       session.ExitSignal,
		Aggregated:       session.Aggregated,
		Tail:             session.Tail,
		Truncated:        session.Truncated,
		TotalOutputChars: session.TotalOutputChars,
	}
}

// ListRunningSessions 列出所有后台运行中的会话。
func (r *ProcessRegistry) ListRunningSessions() []*ProcessSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*ProcessSession
	for _, s := range r.running {
		if s.Backgrounded {
			result = append(result, s)
		}
	}
	return result
}

// ListFinishedSessions 列出所有已完成的会话。
func (r *ProcessRegistry) ListFinishedSessions() []*FinishedSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*FinishedSession, 0, len(r.finished))
	for _, s := range r.finished {
		result = append(result, s)
	}
	return result
}

// ClearFinished 清除所有已完成的会话。
func (r *ProcessRegistry) ClearFinished() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finished = make(map[string]*FinishedSession)
}

// SetJobTTLMs 设置 TTL（毫秒）。
func (r *ProcessRegistry) SetJobTTLMs(value int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobTTLMs = clampTTL(value)
	r.stopSweeper()
	r.startSweeper()
}

// ResetForTests 重置注册表（仅测试使用）。
func (r *ProcessRegistry) ResetForTests() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = make(map[string]*ProcessSession)
	r.finished = make(map[string]*FinishedSession)
	r.stopSweeper()
}

// ---------- 内部清理 ----------

func (r *ProcessRegistry) startSweeper() {
	if r.sweepStarted {
		return
	}
	r.sweepStarted = true
	r.sweepStop = make(chan struct{})
	interval := time.Duration(math.Max(30_000, float64(r.jobTTLMs)/6)) * time.Millisecond
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.pruneFinished()
			case <-r.sweepStop:
				return
			}
		}
	}()
}

func (r *ProcessRegistry) stopSweeper() {
	if !r.sweepStarted {
		return
	}
	close(r.sweepStop)
	r.sweepStarted = false
}

func (r *ProcessRegistry) pruneFinished() {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().UnixMilli() - int64(r.jobTTLMs)
	for id, session := range r.finished {
		if session.EndedAt < cutoff {
			delete(r.finished, id)
		}
	}
}

// ---------- 工具函数 ----------

// Tail 返回字符串末尾 max 字符。
func Tail(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[len(text)-max:]
}

// TrimWithCap 保留字符串末尾 max 字符。
func TrimWithCap(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[len(text)-max:]
}

func capPendingBuffer(buffer *[]string, pendingChars, cap int) int {
	if pendingChars <= cap {
		return pendingChars
	}
	buf := *buffer
	if len(buf) > 0 {
		last := buf[len(buf)-1]
		if len(last) >= cap {
			*buffer = []string{last[len(last)-cap:]}
			return cap
		}
	}
	for len(buf) > 0 && pendingChars-len(buf[0]) >= cap {
		pendingChars -= len(buf[0])
		buf = buf[1:]
	}
	if len(buf) > 0 && pendingChars > cap {
		overflow := pendingChars - cap
		buf[0] = buf[0][overflow:]
		pendingChars = cap
	}
	*buffer = buf
	return pendingChars
}

// DefaultRegistry 全局默认进程注册表。
var DefaultRegistry = NewProcessRegistry()
