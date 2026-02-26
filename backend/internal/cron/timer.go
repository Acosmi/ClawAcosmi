package cron

import (
	"sort"
	"time"
)

// ============================================================================
// 定时器管理 — Job 调度、触发、错误退避
// 对应 TS: cron/service/timer.ts (534L)
// ============================================================================

// ArmTimer 设置/重置定时器，在下一个 due job 时间点触发
func ArmTimer(state *CronServiceState) {
	// 取消现有定时器
	cancelTimer(state)

	if !state.running || state.store == nil {
		return
	}

	nowMs := state.NowMs()
	nextMs := findNextDueMs(state.store.Jobs, nowMs)
	if nextMs < 0 {
		return
	}

	delay := time.Duration(nextMs-nowMs) * time.Millisecond
	if delay < 0 {
		delay = 0
	}

	stopCh := make(chan struct{})
	state.timerStop = stopCh

	state.timer = time.AfterFunc(delay, func() {
		select {
		case <-stopCh:
			return // 定时器已被取消
		default:
		}
		OnTimer(state)
	})
}

// cancelTimer 取消现有定时器
func cancelTimer(state *CronServiceState) {
	if state.timerStop != nil {
		close(state.timerStop)
		state.timerStop = nil
	}
	if state.timer != nil {
		state.timer.Stop()
		state.timer = nil
	}
}

// findNextDueMs 查找最近的 due 时间
func findNextDueMs(jobs []CronJob, nowMs int64) int64 {
	var earliest int64 = -1

	for i := range jobs {
		job := &jobs[i]
		if !job.Enabled {
			continue
		}
		if job.State.RunningAtMs != nil && *job.State.RunningAtMs > 0 {
			continue
		}
		nextRunAt := job.State.NextRunAtMs
		if nextRunAt == nil || *nextRunAt <= 0 {
			continue
		}
		if earliest < 0 || *nextRunAt < earliest {
			earliest = *nextRunAt
		}
	}

	return earliest
}

// OnTimer 定时器触发回调 — 执行所有 due 和 missed 的 jobs
func OnTimer(state *CronServiceState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.running || state.store == nil {
		return
	}

	nowMs := state.NowMs()

	// 清理 stuck 运行
	cleanupStuckJobs(state, nowMs)

	// 收集 due jobs
	dueJobs := collectDueJobs(state.store.Jobs, nowMs)
	if len(dueJobs) == 0 {
		ArmTimer(state)
		return
	}

	// 按到期时间排序（最早优先）
	sort.Slice(dueJobs, func(i, j int) bool {
		iNext := dueJobs[i].State.NextRunAtMs
		jNext := dueJobs[j].State.NextRunAtMs
		if iNext == nil {
			return true
		}
		if jNext == nil {
			return false
		}
		return *iNext < *jNext
	})

	// 逐个执行
	for _, idx := range dueJobIndices(state.store.Jobs, dueJobs) {
		job := &state.store.Jobs[idx]
		executeJob(state, job, nowMs)
	}

	// 持久化
	if err := PersistCurrent(state); err != nil {
		if state.Deps.Logger != nil {
			state.Deps.Logger.Error("cron: failed to persist after timer tick", "error", err)
		}
	}

	// 重新设置定时器
	ArmTimer(state)
}

// collectDueJobs 收集所有 due 的 jobs
func collectDueJobs(jobs []CronJob, nowMs int64) []CronJob {
	var due []CronJob
	for i := range jobs {
		if IsJobDue(&jobs[i], nowMs) {
			due = append(due, jobs[i])
		}
	}
	return due
}

// dueJobIndices 返回 due jobs 在 store 中的索引
func dueJobIndices(allJobs []CronJob, dueJobs []CronJob) []int {
	idSet := make(map[string]bool)
	for _, j := range dueJobs {
		idSet[j.ID] = true
	}
	var indices []int
	for i, j := range allJobs {
		if idSet[j.ID] {
			indices = append(indices, i)
		}
	}
	return indices
}

// cleanupStuckJobs 清理卡在运行状态的 jobs
func cleanupStuckJobs(state *CronServiceState, nowMs int64) {
	for i := range state.store.Jobs {
		job := &state.store.Jobs[i]
		if IsJobStuckRunning(job, nowMs, DefaultJobTimeoutMs) {
			if state.Deps.Logger != nil {
				state.Deps.Logger.Warn("cron: clearing stuck job",
					"jobId", job.ID, "jobName", job.Name)
			}
			MarkJobFinished(job, nowMs, JobStatusError, "job execution timed out (stuck)", 0)

			// 重新计算下次运行时间
			nextMs := ComputeJobNextRunAtMs(job, nowMs)
			SetJobNextRunAtMs(job, nextMs)

			emitEvent(state, CronEvent{Kind: EventKindJobError, JobID: job.ID, Error: "stuck timeout"})
		}
	}
}

// executeJob 执行单个 job
func executeJob(state *CronServiceState, job *CronJob, nowMs int64) {
	// 标记运行中
	MarkJobRunning(job, nowMs)
	emitEvent(state, CronEvent{Kind: EventKindJobRun, JobID: job.ID})

	startMs := state.NowMs()
	var status CronJobStatus
	var errMsg string
	var sessionID string
	var summary string

	switch job.SessionTarget {
	case SessionTargetMain:
		status, errMsg = executeMainSessionJob(state, job)
	case SessionTargetIsolated:
		status, errMsg, sessionID, summary = executeIsolatedSessionJob(state, job)
	default:
		status = JobStatusError
		errMsg = "unknown session target: " + string(job.SessionTarget)
	}

	endMs := state.NowMs()
	durationMs := endMs - startMs

	// 记录完成
	MarkJobFinished(job, endMs, status, errMsg, durationMs)

	// 计算下次执行时间
	nextMs := ComputeJobNextRunAtMs(job, endMs)

	// 应用错误退避
	if status == JobStatusError && job.State.ConsecutiveErrors != nil && *job.State.ConsecutiveErrors > 0 {
		backoffMs := ComputeErrorBackoffMs(*job.State.ConsecutiveErrors)
		if backoffMs > 0 {
			backoffNext := endMs + backoffMs
			if nextMs < 0 || backoffNext > nextMs {
				nextMs = backoffNext
			}
		}
	}

	SetJobNextRunAtMs(job, nextMs)

	// 运行日志
	logEntry := CronRunLogEntry{
		Ts:         endMs,
		JobID:      job.ID,
		Action:     "finished",
		Status:     string(status),
		RunAtMs:    &startMs,
		DurationMs: &durationMs,
	}
	if nextMs > 0 {
		logEntry.NextRunAtMs = &nextMs
	}
	if errMsg != "" {
		logEntry.Error = errMsg
	}
	if sessionID != "" {
		logEntry.SessionID = sessionID
	}
	if summary != "" {
		logEntry.Summary = summary
	}

	logPath := ResolveCronRunLogPath(state.Deps.StorePath, job.ID)
	if err := AppendCronRunLog(logPath, logEntry, 0, 0); err != nil {
		if state.Deps.Logger != nil {
			state.Deps.Logger.Warn("cron: failed to append run log", "error", err)
		}
	}

	// 事件通知
	if status == JobStatusError {
		emitEvent(state, CronEvent{Kind: EventKindJobError, JobID: job.ID, Error: errMsg})
	} else {
		emitEvent(state, CronEvent{Kind: EventKindJobDone, JobID: job.ID})
	}

	// delete-after-run
	if job.DeleteAfterRun != nil && *job.DeleteAfterRun && status != JobStatusError {
		removeJobByID(state, job.ID)
	}
}

// executeMainSessionJob 执行 main session job（系统事件）
func executeMainSessionJob(state *CronServiceState, job *CronJob) (CronJobStatus, string) {
	text := job.Payload.Text
	if text == "" {
		return JobStatusSkipped, "empty payload text"
	}

	// 入队系统事件
	if state.Deps.EnqueueSystemEvent != nil {
		if err := state.Deps.EnqueueSystemEvent(text); err != nil {
			return JobStatusError, err.Error()
		}
	}

	// 请求心跳唤醒
	if job.WakeMode == WakeModeNow && state.Deps.RequestHeartbeatNow != nil {
		state.Deps.RequestHeartbeatNow()
	}

	return JobStatusOk, ""
}

// executeIsolatedSessionJob 执行 isolated session job（agent turn）
// 完整实现在 isolated_agent.go → RunCronIsolatedAgentTurn()
// 桥接：通过 NewRunIsolatedAgentJobFunc(deps) 在服务启动时注入到 state.Deps.RunIsolatedAgentJob
func executeIsolatedSessionJob(state *CronServiceState, job *CronJob) (CronJobStatus, string, string, string) {
	if state.Deps.RunIsolatedAgentJob == nil {
		return JobStatusError, "isolated agent runner not wired (inject via NewRunIsolatedAgentJobFunc at startup)", "", ""
	}

	result, err := state.Deps.RunIsolatedAgentJob(IsolatedAgentJobParams{
		JobID:   job.ID,
		Payload: job.Payload,
		AgentID: job.AgentID,
	})
	if err != nil {
		return JobStatusError, err.Error(), "", ""
	}
	if result == nil {
		return JobStatusOk, "", "", ""
	}

	return JobStatusOk, "", result.SessionID, result.Summary
}

// removeJobByID 按 ID 从 store 中删除 job
func removeJobByID(state *CronServiceState, id string) {
	if state.store == nil {
		return
	}
	jobs := state.store.Jobs
	for i, j := range jobs {
		if j.ID == id {
			state.store.Jobs = append(jobs[:i], jobs[i+1:]...)
			return
		}
	}
}

// emitEvent 发送事件通知
func emitEvent(state *CronServiceState, event CronEvent) {
	if state.Deps.OnEvent != nil {
		state.Deps.OnEvent(event)
	}
}

// ComputeAllNextRunTimes 重新计算所有 jobs 的下次运行时间
func ComputeAllNextRunTimes(state *CronServiceState) {
	if state.store == nil {
		return
	}
	nowMs := state.NowMs()
	for i := range state.store.Jobs {
		job := &state.store.Jobs[i]
		if job.State.RunningAtMs != nil && *job.State.RunningAtMs > 0 {
			continue // 正在运行的 job 不重新计算
		}
		nextMs := ComputeJobNextRunAtMs(job, nowMs)
		SetJobNextRunAtMs(job, nextMs)
	}
}
