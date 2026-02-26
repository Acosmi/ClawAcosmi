package cron

import (
	"fmt"
)

// ============================================================================
// CRUD 操作 — 公共 API 实现
// 对应 TS: cron/service/ops.ts (212L)
// ============================================================================

// storeLock 全局 store 锁
var storeLock = &CronStoreLock{}

// Start 启动 cron 服务
func Start(state *CronServiceState) error {
	return storeLock.Locked(func() error {
		if state.running {
			return nil // 已在运行
		}

		if err := EnsureLoaded(state); err != nil {
			return fmt.Errorf("cron: start failed: %w", err)
		}

		// 计算所有 jobs 的下次运行时间
		ComputeAllNextRunTimes(state)

		// 持久化（确保 nextRunAtMs 已保存）
		if err := PersistCurrent(state); err != nil {
			if state.Deps.Logger != nil {
				state.Deps.Logger.Warn("cron: failed to persist on start", "error", err)
			}
		}

		state.running = true
		state.op = "running"

		// 启动定时器
		ArmTimer(state)

		emitEvent(state, CronEvent{Kind: EventKindStarted})

		if state.Deps.Logger != nil {
			jobCount := 0
			if state.store != nil {
				jobCount = len(state.store.Jobs)
			}
			state.Deps.Logger.Info("cron: service started",
				"jobCount", jobCount)
		}

		return nil
	})
}

// Stop 停止 cron 服务
func Stop(state *CronServiceState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.running {
		return
	}

	cancelTimer(state)
	state.running = false
	state.op = "stopped"

	emitEvent(state, CronEvent{Kind: EventKindStopped})

	if state.Deps.Logger != nil {
		state.Deps.Logger.Info("cron: service stopped")
	}
}

// Status 查询服务状态
func Status(state *CronServiceState) CronStatusResult {
	state.mu.Lock()
	defer state.mu.Unlock()

	result := CronStatusResult{
		Running: state.running,
		Op:      state.op,
	}

	if state.store != nil {
		result.JobCount = len(state.store.Jobs)
	}

	return result
}

// List 列出 jobs
func List(state *CronServiceState, includeDisabled bool) ([]CronJob, error) {
	return LockedValue(storeLock, func() ([]CronJob, error) {
		if err := EnsureLoaded(state); err != nil {
			return nil, err
		}

		jobs := GetJobs(state)
		if includeDisabled {
			result := make([]CronJob, len(jobs))
			copy(result, jobs)
			return result, nil
		}

		// 仅返回启用的 jobs
		var enabled []CronJob
		for _, j := range jobs {
			if j.Enabled {
				enabled = append(enabled, j)
			}
		}
		return enabled, nil
	})
}

// Add 添加新 job
func Add(state *CronServiceState, input CronJobCreate) (*CronAddResult, error) {
	return LockedValue(storeLock, func() (*CronAddResult, error) {
		if err := EnsureLoaded(state); err != nil {
			return nil, err
		}

		nowMs := state.NowMs()
		job := CreateJob(input, nowMs)

		// 确保 sessionTarget 与 payload.kind 一致
		AssertSessionTarget(&job)

		// 计算 nextRunAtMs
		nextMs := ComputeJobNextRunAtMs(&job, nowMs)
		SetJobNextRunAtMs(&job, nextMs)

		// 添加到 store
		state.store.Jobs = append(state.store.Jobs, job)

		// 持久化
		if err := PersistCurrent(state); err != nil {
			return nil, fmt.Errorf("cron: add failed to persist: %w", err)
		}

		// 重新设置定时器
		if state.running {
			ArmTimer(state)
		}

		emitEvent(state, CronEvent{Kind: EventKindJobAdded, JobID: job.ID})

		return &CronAddResult{
			CronOpResult: CronOpResult{OK: true, Message: "job added"},
			JobID:        job.ID,
			Job:          &job,
		}, nil
	})
}

// Update 更新 job
func Update(state *CronServiceState, id string, patch CronJobPatch) (*CronOpResult, error) {
	return LockedValue(storeLock, func() (*CronOpResult, error) {
		if err := EnsureLoaded(state); err != nil {
			return nil, err
		}

		job, _, err := FindJobOrError(GetJobs(state), id)
		if err != nil {
			return &CronOpResult{OK: false, Error: err.Error()}, nil
		}

		nowMs := state.NowMs()
		ApplyJobPatch(job, patch, nowMs)

		// 重新断言 sessionTarget
		AssertSessionTarget(job)

		// 重新计算 nextRunAtMs
		nextMs := ComputeJobNextRunAtMs(job, nowMs)
		SetJobNextRunAtMs(job, nextMs)

		// 持久化
		if err := PersistCurrent(state); err != nil {
			return nil, fmt.Errorf("cron: update failed to persist: %w", err)
		}

		// 重新设置定时器
		if state.running {
			ArmTimer(state)
		}

		return &CronOpResult{OK: true, Message: "job updated"}, nil
	})
}

// Remove 删除 job
func Remove(state *CronServiceState, id string) (*CronOpResult, error) {
	return LockedValue(storeLock, func() (*CronOpResult, error) {
		if err := EnsureLoaded(state); err != nil {
			return nil, err
		}

		_, idx, err := FindJobOrError(GetJobs(state), id)
		if err != nil {
			return &CronOpResult{OK: false, Error: err.Error()}, nil
		}

		jobs := state.store.Jobs
		state.store.Jobs = append(jobs[:idx], jobs[idx+1:]...)

		// 持久化
		if err := PersistCurrent(state); err != nil {
			return nil, fmt.Errorf("cron: remove failed to persist: %w", err)
		}

		// 重新设置定时器
		if state.running {
			ArmTimer(state)
		}

		return &CronOpResult{OK: true, Message: "job removed"}, nil
	})
}

// Run 手动运行 job
func Run(state *CronServiceState, id string, mode string) (*CronRunResult, error) {
	return LockedValue(storeLock, func() (*CronRunResult, error) {
		if err := EnsureLoaded(state); err != nil {
			return nil, err
		}

		job, _, err := FindJobOrError(GetJobs(state), id)
		if err != nil {
			return &CronRunResult{
				CronOpResult: CronOpResult{OK: false, Error: err.Error()},
			}, nil
		}

		// mode=due: 仅当 due 时执行
		if mode == "due" && !IsJobDue(job, state.NowMs()) {
			return &CronRunResult{
				CronOpResult: CronOpResult{OK: true, Message: "job not due"},
				Status:       JobStatusSkipped,
			}, nil
		}

		nowMs := state.NowMs()

		// 直接执行（不启动定时器循环）
		MarkJobRunning(job, nowMs)
		startMs := state.NowMs()

		var status CronJobStatus
		var errMsg, sessionID, summary string

		switch job.SessionTarget {
		case SessionTargetMain:
			status, errMsg = executeMainSessionJob(state, job)
		case SessionTargetIsolated:
			status, errMsg, sessionID, summary = executeIsolatedSessionJob(state, job)
		default:
			status = JobStatusError
			errMsg = "unknown session target"
		}

		endMs := state.NowMs()
		durationMs := endMs - startMs
		MarkJobFinished(job, endMs, status, errMsg, durationMs)

		// 重新计算 nextRunAtMs
		nextMs := ComputeJobNextRunAtMs(job, endMs)
		SetJobNextRunAtMs(job, nextMs)

		// 持久化
		if err := PersistCurrent(state); err != nil {
			if state.Deps.Logger != nil {
				state.Deps.Logger.Warn("cron: run failed to persist", "error", err)
			}
		}

		// 重新设置定时器
		if state.running {
			ArmTimer(state)
		}

		result := &CronRunResult{
			CronOpResult: CronOpResult{OK: true, Message: "job executed"},
			Status:       status,
			SessionID:    sessionID,
			Summary:      summary,
		}
		if errMsg != "" {
			result.Error = errMsg
		}

		return result, nil
	})
}

// WakeNow 立即唤醒（触发系统事件 + heartbeat）
func WakeNow(state *CronServiceState, mode string, text string) *CronOpResult {
	if text == "" {
		return &CronOpResult{OK: false, Error: "text is required"}
	}

	// 入队系统事件
	if state.Deps.EnqueueSystemEvent != nil {
		if err := state.Deps.EnqueueSystemEvent(text); err != nil {
			return &CronOpResult{OK: false, Error: err.Error()}
		}
	}

	// 按 mode 决定唤醒方式
	if mode == "now" && state.Deps.RequestHeartbeatNow != nil {
		state.Deps.RequestHeartbeatNow()
	}

	return &CronOpResult{OK: true, Message: "wake triggered"}
}
