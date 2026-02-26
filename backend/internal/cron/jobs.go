package cron

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ============================================================================
// Job 管理 — Job 的创建、补丁、查找、调度计算
// 对应 TS: cron/service/jobs.ts (409L)
// ============================================================================

// --- Job ID 生成 ---

// generateJobID 生成随机 Job ID（UUID v4 格式）
func generateJobID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// --- Job 创建 ---

// CreateJob 从 CronJobCreate 创建 CronJob
func CreateJob(input CronJobCreate, nowMs int64) CronJob {
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	name := NormalizeRequiredName(input.Name)
	if name == "" {
		name = InferLegacyName(input.Schedule, input.Payload)
	}

	desc := NormalizeOptionalText(input.Description, maxJobDescLen)
	agentID := NormalizeOptionalAgentId(input.AgentID)

	job := CronJob{
		ID:             generateJobID(),
		AgentID:        agentID,
		Name:           name,
		Description:    desc,
		Enabled:        enabled,
		DeleteAfterRun: input.DeleteAfterRun,
		CreatedAtMs:    nowMs,
		UpdatedAtMs:    nowMs,
		Schedule:       input.Schedule,
		SessionTarget:  input.SessionTarget,
		WakeMode:       input.WakeMode,
		Payload:        input.Payload,
		Delivery:       input.Delivery,
		State:          CronJobState{},
	}

	if input.State != nil {
		job.State = *input.State
	}

	// 默认值
	if job.SessionTarget == "" {
		job.SessionTarget = defaultSessionTarget
	}
	if job.WakeMode == "" {
		job.WakeMode = defaultWakeMode
	}

	return job
}

// --- Job 补丁 ---

// ApplyJobPatch 将补丁应用到 Job
func ApplyJobPatch(job *CronJob, patch CronJobPatch, nowMs int64) {
	if patch.Name != nil {
		name := NormalizeRequiredName(*patch.Name)
		if name != "" {
			job.Name = name
		}
	}
	if patch.Description != nil {
		job.Description = NormalizeOptionalText(*patch.Description, maxJobDescLen)
	}
	if patch.AgentID != nil {
		job.AgentID = NormalizeOptionalAgentId(*patch.AgentID)
	}
	if patch.Enabled != nil {
		job.Enabled = *patch.Enabled
	}
	if patch.DeleteAfterRun != nil {
		job.DeleteAfterRun = patch.DeleteAfterRun
	}
	if patch.Schedule != nil {
		job.Schedule = *patch.Schedule
	}
	if patch.SessionTarget != nil {
		job.SessionTarget = *patch.SessionTarget
	}
	if patch.WakeMode != nil {
		job.WakeMode = *patch.WakeMode
	}
	if patch.Payload != nil {
		applyPayloadPatch(&job.Payload, *patch.Payload)
	}
	if patch.Delivery != nil {
		applyDeliveryPatch(job, *patch.Delivery)
	}
	if patch.State != nil {
		job.State = *patch.State
	}

	job.UpdatedAtMs = nowMs
}

func applyPayloadPatch(p *CronPayload, patch CronPayloadPatch) {
	if patch.Kind != "" {
		p.Kind = patch.Kind
	}
	if patch.Text != nil {
		p.Text = *patch.Text
	}
	if patch.Message != nil {
		p.Message = *patch.Message
	}
	if patch.Model != nil {
		p.Model = *patch.Model
	}
	if patch.Thinking != nil {
		p.Thinking = *patch.Thinking
	}
	if patch.TimeoutSeconds != nil {
		p.TimeoutSeconds = patch.TimeoutSeconds
	}
	if patch.AllowUnsafeExternalContent != nil {
		p.AllowUnsafeExternalContent = patch.AllowUnsafeExternalContent
	}
	if patch.Deliver != nil {
		p.Deliver = patch.Deliver
	}
	if patch.Channel != nil {
		p.Channel = *patch.Channel
	}
	if patch.To != nil {
		p.To = *patch.To
	}
	if patch.BestEffortDeliver != nil {
		p.BestEffortDeliver = patch.BestEffortDeliver
	}
}

func applyDeliveryPatch(job *CronJob, patch CronDeliveryPatch) {
	if job.Delivery == nil {
		job.Delivery = &CronDelivery{}
	}
	if patch.Mode != nil {
		job.Delivery.Mode = *patch.Mode
	}
	if patch.Channel != nil {
		job.Delivery.Channel = *patch.Channel
	}
	if patch.To != nil {
		job.Delivery.To = *patch.To
	}
	if patch.BestEffort != nil {
		job.Delivery.BestEffort = patch.BestEffort
	}
}

// --- Job 查找 ---

// FindJob 从 jobs 列表中按 ID 查找
func FindJob(jobs []CronJob, id string) (*CronJob, int) {
	id = strings.TrimSpace(id)
	for i := range jobs {
		if jobs[i].ID == id {
			return &jobs[i], i
		}
	}
	return nil, -1
}

// FindJobOrError 查找 Job，不存在返回错误
func FindJobOrError(jobs []CronJob, id string) (*CronJob, int, error) {
	job, idx := FindJob(jobs, id)
	if job == nil {
		return nil, -1, fmt.Errorf("cron job not found: %s", id)
	}
	return job, idx, nil
}

// FindJobByName 按名称查找
func FindJobByName(jobs []CronJob, name string) (*CronJob, int) {
	name = strings.TrimSpace(name)
	for i := range jobs {
		if jobs[i].Name == name {
			return &jobs[i], i
		}
	}
	return nil, -1
}

// --- Job 调度计算 ---

// ComputeJobNextRunAtMs 计算 Job 的下次执行时间
func ComputeJobNextRunAtMs(job *CronJob, nowMs int64) int64 {
	if !job.Enabled {
		return -1
	}

	fromMs := nowMs
	if job.State.LastRunAtMs != nil && *job.State.LastRunAtMs > 0 {
		fromMs = *job.State.LastRunAtMs
	}
	if job.State.RunningAtMs != nil && *job.State.RunningAtMs > 0 {
		// 正在运行，不安排下次
		return -1
	}

	return ComputeNextRunAtMs(job.Schedule, fromMs)
}

// IsJobDue 判断 Job 是否已到期（需要执行）
func IsJobDue(job *CronJob, nowMs int64) bool {
	if !job.Enabled {
		return false
	}
	if job.State.RunningAtMs != nil && *job.State.RunningAtMs > 0 {
		return false // 正在运行
	}

	nextRunAt := job.State.NextRunAtMs
	if nextRunAt == nil || *nextRunAt <= 0 {
		return false
	}
	return *nextRunAt <= nowMs
}

// IsJobStuckRunning 判断 Job 是否卡在运行状态
func IsJobStuckRunning(job *CronJob, nowMs int64, timeoutMs int64) bool {
	if job.State.RunningAtMs == nil || *job.State.RunningAtMs <= 0 {
		return false
	}
	return nowMs-*job.State.RunningAtMs > timeoutMs
}

// MarkJobRunning 标记 Job 为运行中
func MarkJobRunning(job *CronJob, nowMs int64) {
	runningAt := nowMs
	job.State.RunningAtMs = &runningAt
}

// MarkJobFinished 标记 Job 为完成
func MarkJobFinished(job *CronJob, nowMs int64, status CronJobStatus, errMsg string, durationMs int64) {
	lastRun := nowMs
	job.State.LastRunAtMs = &lastRun
	job.State.LastStatus = &status
	job.State.RunningAtMs = nil
	dur := durationMs
	job.State.LastDurationMs = &dur

	if status == JobStatusError {
		if errMsg != "" {
			job.State.LastError = &errMsg
		}
		consecutiveErrors := 1
		if job.State.ConsecutiveErrors != nil {
			consecutiveErrors = *job.State.ConsecutiveErrors + 1
		}
		job.State.ConsecutiveErrors = &consecutiveErrors
	} else {
		job.State.LastError = nil
		zero := 0
		job.State.ConsecutiveErrors = &zero
	}

	job.UpdatedAtMs = nowMs
}

// SetJobNextRunAtMs 设置 Job 的下次执行时间
func SetJobNextRunAtMs(job *CronJob, nextMs int64) {
	if nextMs < 0 {
		job.State.NextRunAtMs = nil
	} else {
		job.State.NextRunAtMs = &nextMs
	}
}

// --- 工具函数 ---

// AssertSessionTarget 确保 payload kind 与 sessionTarget 匹配
// main → systemEvent, isolated → agentTurn
func AssertSessionTarget(job *CronJob) {
	switch job.SessionTarget {
	case SessionTargetMain:
		if job.Payload.Kind != PayloadKindSystemEvent {
			job.Payload.Kind = PayloadKindSystemEvent
		}
	case SessionTargetIsolated:
		if job.Payload.Kind != PayloadKindAgentTurn {
			job.Payload.Kind = PayloadKindAgentTurn
		}
	}
}

// DefaultJobTimeoutMs 默认 Job 超时（5 分钟）
const DefaultJobTimeoutMs = int64(5 * time.Minute / time.Millisecond)

// ErrorBackoffScheduleMs 错误退避阶梯（毫秒）
// 30s → 1m → 2m → 5m → 10m → 30m → 1h
var ErrorBackoffScheduleMs = []int64{
	30_000,
	60_000,
	120_000,
	300_000,
	600_000,
	1_800_000,
	3_600_000,
}

// ComputeErrorBackoffMs 根据连续错误次数计算退避时间
func ComputeErrorBackoffMs(consecutiveErrors int) int64 {
	if consecutiveErrors <= 0 {
		return 0
	}
	idx := consecutiveErrors - 1
	if idx >= len(ErrorBackoffScheduleMs) {
		idx = len(ErrorBackoffScheduleMs) - 1
	}
	return ErrorBackoffScheduleMs[idx]
}
