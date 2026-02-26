// =============================================================================
// 文件: backend/internal/sandbox/sandbox_worker.go | 模块: sandbox | 职责: 沙箱异步任务调度器与混合执行引擎
// 审计: V12 2026-02-21 | 适配: 2026-02-23 — 移除 gorm.io/gorm + nexus-backend 依赖
// =============================================================================

// Package sandbox — 异步任务 Worker
//
// [C-2.7] 基于 Go channel 的任务调度器 (轻量级 Asynq 替代)
// 设计: Worker pool + 内存状态跟踪 + 进度回调
//
// 使用 channel-based worker pool 而非外部 Asynq 依赖, 原因:
// 1. 当前无 Redis 强依赖，避免引入新基础设施
// 2. 沙箱任务量级有限 (非海量异步任务)
// 3. 内存 map 存储任务状态, channel 仅做调度
package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// SandboxTask — 本地任务模型 (替代 nexus-backend/internal/model.SandboxTask)
// ============================================================================

// SandboxTaskStatus 任务状态常量
type SandboxTaskStatus string

const (
	SandboxTaskStatusPending   SandboxTaskStatus = "pending"
	SandboxTaskStatusRunning   SandboxTaskStatus = "running"
	SandboxTaskStatusCompleted SandboxTaskStatus = "completed"
	SandboxTaskStatusFailed    SandboxTaskStatus = "failed"
)

// SandboxTask 沙箱任务
type SandboxTask struct {
	ID          string            `json:"id"`
	TaskType    string            `json:"taskType"` // "code_execution" | "data_processing" | "code_interpreter"
	Input       string            `json:"input"`    // JSON 格式输入
	Status      SandboxTaskStatus `json:"status"`
	Output      string            `json:"output,omitempty"`
	Error       string            `json:"error,omitempty"`
	Progress    int               `json:"progress"`
	ProgressMsg string            `json:"progressMsg,omitempty"`
	MemoryLimit int               `json:"memoryLimit,omitempty"` // MB
	TimeoutSec  int               `json:"timeoutSec,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	StartedAt   *time.Time        `json:"startedAt,omitempty"`
	CompletedAt *time.Time        `json:"completedAt,omitempty"`
}

// ============================================================================
// TaskStore — 内存任务存储 (替代 gorm.DB)
// ============================================================================

// TaskStore 内存任务存储
type TaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*SandboxTask
}

// NewTaskStore 创建任务存储
func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks: make(map[string]*SandboxTask),
	}
}

// Get 获取任务
func (s *TaskStore) Get(id string) (*SandboxTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	return task, ok
}

// Save 保存/更新任务
func (s *TaskStore) Save(task *SandboxTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

// Delete 删除任务
func (s *TaskStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, id)
}

// List 获取所有任务
func (s *TaskStore) List() []*SandboxTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*SandboxTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		result = append(result, t)
	}
	return result
}

// ============================================================================
// TaskExecutor + Worker
// ============================================================================

// TaskExecutor 任务执行策略接口
type TaskExecutor interface {
	// Execute 执行任务, 通过 progressFn 报告进度
	Execute(ctx context.Context, task *SandboxTask, progressFn ProgressFunc) (*ExecutionOutput, error)
}

// ProgressFunc 进度回调
type ProgressFunc func(progress int, msg string)

// ExecutionOutput 执行输出
type ExecutionOutput struct {
	Output string
}

// Worker 沙箱任务工作池
type Worker struct {
	store       *TaskStore
	executor    TaskExecutor
	hub         *ProgressHub // WebSocket 进度推送
	taskCh      chan string  // 任务 ID channel
	workerCount int
	wg          sync.WaitGroup
	cancel      context.CancelFunc
}

// WorkerConfig 工作池配置
type WorkerConfig struct {
	// WorkerCount 并发 worker 数量 (default: 2)
	WorkerCount int
	// QueueSize 任务队列缓冲大小 (default: 100)
	QueueSize int
}

// DefaultWorkerConfig 默认配置
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		WorkerCount: 2,
		QueueSize:   100,
	}
}

// NewWorker 创建任务工作池
func NewWorker(store *TaskStore, executor TaskExecutor, hub *ProgressHub, config WorkerConfig) *Worker {
	if config.WorkerCount <= 0 {
		config.WorkerCount = 2
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}

	return &Worker{
		store:       store,
		executor:    executor,
		hub:         hub,
		taskCh:      make(chan string, config.QueueSize),
		workerCount: config.WorkerCount,
	}
}

// Start 启动 worker pool
func (w *Worker) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)

	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.runWorker(ctx, i)
	}

	slog.Info("sandbox worker pool started",
		"workers", w.workerCount,
		"queue_size", cap(w.taskCh),
	)
}

// Stop 优雅关闭 worker pool
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	close(w.taskCh)
	w.wg.Wait()
	slog.Info("sandbox worker pool stopped")
}

// WorkerStatus Worker Pool 当前状态 (用于监控 API)
type WorkerStatus struct {
	WorkerCount  int `json:"workerCount"`
	QueueSize    int `json:"queueSize"`
	QueuePending int `json:"queuePending"`
}

// Status 返回 Worker Pool 内部状态
func (w *Worker) Status() WorkerStatus {
	return WorkerStatus{
		WorkerCount:  w.workerCount,
		QueueSize:    cap(w.taskCh),
		QueuePending: len(w.taskCh),
	}
}

// Submit 提交任务到执行队列
func (w *Worker) Submit(taskID string) error {
	select {
	case w.taskCh <- taskID:
		return nil
	default:
		return fmt.Errorf("sandbox task queue is full")
	}
}

// runWorker 单个 worker 循环
func (w *Worker) runWorker(ctx context.Context, id int) {
	defer w.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("sandbox worker panic recovered",
				"worker_id", id,
				"error", r,
			)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case taskID, ok := <-w.taskCh:
			if !ok {
				return
			}
			w.processTask(ctx, taskID, id)
		}
	}
}

// processTask 处理单个任务
func (w *Worker) processTask(ctx context.Context, taskID string, workerID int) {
	slog.Info("sandbox task started",
		"task_id", taskID,
		"worker_id", workerID,
	)

	// 1. 加载任务
	task, ok := w.store.Get(taskID)
	if !ok {
		slog.Error("sandbox task not found", "task_id", taskID)
		return
	}

	// 2. 标记为 running
	now := time.Now()
	task.Status = SandboxTaskStatusRunning
	task.StartedAt = &now
	w.store.Save(task)

	// 3. 广播进度: started
	w.broadcastEvent(taskID, 0, "execution started", "progress", "")

	// 4. 执行任务 (通过进度回调实时推送)
	progressFn := func(progress int, msg string) {
		task.Progress = progress
		task.ProgressMsg = msg
		w.store.Save(task)
		w.broadcastEvent(taskID, progress, msg, "progress", "")
	}

	output, err := w.executor.Execute(ctx, task, progressFn)

	// 5. 更新最终状态
	completed := time.Now()
	if err != nil {
		task.Status = SandboxTaskStatusFailed
		task.Error = err.Error()
		task.CompletedAt = &completed
		w.store.Save(task)
		w.broadcastEvent(taskID, task.Progress, "failed: "+err.Error(), "error", err.Error())

		slog.Error("sandbox task failed",
			"task_id", taskID,
			"error", err,
		)
	} else {
		task.Status = SandboxTaskStatusCompleted
		task.Progress = 100
		task.Output = output.Output
		task.CompletedAt = &completed
		w.store.Save(task)
		w.broadcastEvent(taskID, 100, "completed", "done", "")

		slog.Info("sandbox task completed",
			"task_id", taskID,
			"duration", completed.Sub(now).String(),
		)
	}
}

// broadcastEvent 通过 WebSocket hub 广播事件
func (w *Worker) broadcastEvent(taskID string, progress int, msg, eventType, output string) {
	if w.hub == nil {
		return
	}
	w.hub.Broadcast(ProgressEvent{
		TaskID:   taskID,
		Progress: progress,
		Message:  msg,
		Type:     eventType,
		Output:   output,
	})
}

// BroadcastOutput 广播 stdout/stderr 输出 (供 Executor 调用)
func (w *Worker) BroadcastOutput(taskID string, outputType string, text string) {
	w.broadcastEvent(taskID, -1, "", outputType, text)
}

// --- 内置执行器: 优先 Wasm, 降级 Docker ---

// HybridExecutor 混合执行器: Wasm 优先, Docker 降级
type HybridExecutor struct {
	dockerRunner *DockerRunner
}

// NewHybridExecutor 创建混合执行器
func NewHybridExecutor() *HybridExecutor {
	return &HybridExecutor{
		dockerRunner: NewDockerRunner(DefaultDockerConfig()),
	}
}

// Execute 执行任务 — 根据 taskType 选择执行策略
func (e *HybridExecutor) Execute(ctx context.Context, task *SandboxTask, progressFn ProgressFunc) (*ExecutionOutput, error) {
	progressFn(10, "preparing execution environment")

	switch task.TaskType {
	case "code_execution":
		return e.executeCode(ctx, task, progressFn)
	case "data_processing":
		return e.executeDataProcessing(ctx, task, progressFn)
	case "code_interpreter":
		return e.executeCodeInterpreter(ctx, task, progressFn)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.TaskType)
	}
}

func (e *HybridExecutor) executeCode(ctx context.Context, task *SandboxTask, progressFn ProgressFunc) (*ExecutionOutput, error) {
	progressFn(30, "executing code in sandbox")

	// Docker fallback — 根据 input 解析语言
	image := "alpine:3.19"
	runtime := "sh"

	// 从 input 中尝试解析语言和代码
	// 期望格式: {"language": "python", "code": "print('hello')"}
	type CodeInput struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	}

	var input CodeInput
	if err := parseJSON(task.Input, &input); err == nil && input.Code != "" {
		switch input.Language {
		case "python", "python3":
			image = "python:3.12-alpine"
			runtime = "python3"
		case "javascript", "js", "node":
			image = "node:20-alpine"
			runtime = "node"
		case "sh", "bash", "shell":
			image = "alpine:3.19"
			runtime = "sh"
		}

		progressFn(50, fmt.Sprintf("running %s code in Docker sandbox", input.Language))

		cfg := DefaultDockerConfig()
		cfg.MemoryLimitMB = task.MemoryLimit
		cfg.TimeoutSecs = task.TimeoutSec
		cfg.DockerImage = image
		runner := NewDockerRunner(cfg)

		result, err := runner.ExecuteScript(ctx, image, input.Code, runtime)
		if err != nil {
			return nil, fmt.Errorf("docker execution failed: %w", err)
		}

		progressFn(90, "collecting results")

		if result.ExitCode != 0 && result.Error != "" {
			return nil, fmt.Errorf("exit code %d: %s", result.ExitCode, result.Error)
		}

		return &ExecutionOutput{
			Output: result.Stdout,
		}, nil
	}

	return nil, fmt.Errorf("invalid code input format")
}

// dataProcessingInput defines the JSON input for data_processing tasks
type dataProcessingInput struct {
	Script       string   `json:"script"`                  // Python script to execute
	InputData    string   `json:"input_data,omitempty"`    // Data to pass via stdin
	OutputFormat string   `json:"output_format,omitempty"` // "json" | "csv" | "text" (default: "text")
	Packages     []string `json:"packages,omitempty"`      // pip packages to install (e.g., ["pandas", "numpy"])
}

// executeDataProcessing runs a Python data processing script in Docker
// [Task 3] Uses DockerRunner with python:3.12-alpine image
func (e *HybridExecutor) executeDataProcessing(ctx context.Context, task *SandboxTask, progressFn ProgressFunc) (*ExecutionOutput, error) {
	progressFn(10, "parsing data processing input")

	var input dataProcessingInput
	if err := parseJSON(task.Input, &input); err != nil {
		return nil, fmt.Errorf("invalid data_processing input: %w", err)
	}

	if input.Script == "" {
		return nil, fmt.Errorf("data_processing requires a non-empty 'script' field")
	}

	progressFn(20, "preparing Docker sandbox")

	// Configure Docker with appropriate limits
	cfg := DefaultDockerConfig()
	if task.MemoryLimit > 0 {
		cfg.MemoryLimitMB = task.MemoryLimit
	} else {
		cfg.MemoryLimitMB = 512 // data processing may need more memory
	}
	if task.TimeoutSec > 0 {
		cfg.TimeoutSecs = task.TimeoutSec
	}
	cfg.DockerImage = "python:3.12-alpine"

	runner := NewDockerRunner(cfg)

	// Build wrapper script that:
	// 1. Installs requested packages (if any)
	// 2. Runs the user script with input_data on stdin
	var scriptBuilder strings.Builder
	scriptBuilder.WriteString("#!/bin/sh\nset -e\n")

	// Install packages if requested
	if len(input.Packages) > 0 {
		scriptBuilder.WriteString("pip install --quiet --no-cache-dir ")
		for i, pkg := range input.Packages {
			if i > 0 {
				scriptBuilder.WriteString(" ")
			}
			// Basic sanitization: only allow alphanumeric, hyphen, underscore, dot, brackets
			sanitized := strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
					r == '-' || r == '_' || r == '.' || r == '[' || r == ']' || r == ',' {
					return r
				}
				return -1
			}, pkg)
			scriptBuilder.WriteString(sanitized)
		}
		scriptBuilder.WriteString(" 2>/dev/null\n")
	}

	// Write user script to a temp file and execute with Python
	scriptBuilder.WriteString("python3 -c '")
	// Escape single quotes in user script
	escaped := strings.ReplaceAll(input.Script, "'", "'\\''")
	scriptBuilder.WriteString(escaped)
	scriptBuilder.WriteString("'\n")

	progressFn(40, "executing data processing script")

	// Execute via Docker with stdin data
	result, err := runner.Execute(ctx, "python:3.12-alpine",
		[]string{"sh", "-c", scriptBuilder.String()},
		input.InputData,
	)
	if err != nil {
		return nil, fmt.Errorf("docker data processing failed: %w", err)
	}

	progressFn(80, "collecting results")

	if result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = result.Error
		}
		return nil, fmt.Errorf("data processing failed (exit %d): %s", result.ExitCode, errMsg)
	}

	progressFn(95, "formatting output")

	return &ExecutionOutput{
		Output: result.Stdout,
	}, nil
}

// codeInterpreterInput Code Interpreter 任务输入格式
type codeInterpreterInput struct {
	Code    string `json:"code"`
	Timeout int    `json:"timeout,omitempty"`
}

// executeCodeInterpreter [Phase C] 通过 Code Interpreter 服务执行 Python 代码
func (e *HybridExecutor) executeCodeInterpreter(ctx context.Context, task *SandboxTask, progressFn ProgressFunc) (*ExecutionOutput, error) {
	progressFn(10, "parsing code interpreter input")

	var input codeInterpreterInput
	if err := parseJSON(task.Input, &input); err != nil {
		return nil, fmt.Errorf("invalid code_interpreter input: %w", err)
	}

	if input.Code == "" {
		return nil, fmt.Errorf("code_interpreter requires a non-empty 'code' field")
	}

	timeout := input.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	_ = timeout // 保留变量用于未来 ContainerPool 集成

	progressFn(20, "preparing Docker sandbox")

	// 降级模式: 当 ContainerPool 未初始化时, 使用 DockerRunner 直接执行
	cfg := DefaultDockerConfig()
	if task.MemoryLimit > 0 {
		cfg.MemoryLimitMB = task.MemoryLimit
	} else {
		cfg.MemoryLimitMB = 512
	}
	if task.TimeoutSec > 0 {
		cfg.TimeoutSecs = task.TimeoutSec
	}
	cfg.DockerImage = "python:3.12-alpine"

	runner := NewDockerRunner(cfg)

	progressFn(40, "executing Python code in sandbox")

	result, err := runner.ExecuteScript(ctx, "python:3.12-alpine", input.Code, "python3")
	if err != nil {
		return nil, fmt.Errorf("code interpreter execution failed: %w", err)
	}

	progressFn(80, "collecting results")

	if result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = result.Error
		}
		return nil, fmt.Errorf("code interpreter failed (exit %d): %s", result.ExitCode, errMsg)
	}

	progressFn(95, "formatting output")

	return &ExecutionOutput{
		Output: result.Stdout,
	}, nil
}

// parseJSON helper
func parseJSON(s string, v interface{}) error {
	if s == "" {
		return fmt.Errorf("empty input")
	}
	return json.Unmarshal([]byte(s), v)
}
