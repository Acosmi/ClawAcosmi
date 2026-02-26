package cron

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ============================================================================
// 服务层持久化 — 加载/保存/迁移 cron store
// 对应 TS: cron/service/store.ts (509L)
// ============================================================================

// EnsureLoaded 确保 store 已加载（延迟加载 + 自动迁移）
func EnsureLoaded(state *CronServiceState) error {
	if state.store != nil {
		return nil
	}

	storePath := state.Deps.StorePath
	if storePath == "" {
		return fmt.Errorf("cron: store path is empty")
	}

	store, err := LoadCronStore(storePath)
	if err != nil {
		return fmt.Errorf("cron: load store failed: %w", err)
	}

	// 迁移和规范化所有 jobs
	migrated := migrateAllJobs(state, store)
	if migrated {
		// 有迁移发生，立即保存
		if err := Persist(state, store); err != nil {
			if state.Deps.Logger != nil {
				state.Deps.Logger.Warn("cron: failed to persist migrated store",
					"error", err)
			}
		}
	}

	state.store = store
	state.storeLoadedAtMs = state.NowMs()

	// 记录文件修改时间
	if info, err := os.Stat(storePath); err == nil {
		state.storeModTimeMs = info.ModTime().UnixMilli()
	}

	return nil
}

// Persist 保存当前 store 到文件
func Persist(state *CronServiceState, store *CronStoreFile) error {
	if store == nil {
		store = state.store
	}
	if store == nil {
		return fmt.Errorf("cron: no store to persist")
	}

	storePath := state.Deps.StorePath
	if storePath == "" {
		return fmt.Errorf("cron: store path is empty")
	}

	if err := SaveCronStore(storePath, store); err != nil {
		return err
	}

	// 更新文件修改时间记录
	if info, err := os.Stat(storePath); err == nil {
		state.storeModTimeMs = info.ModTime().UnixMilli()
	}

	return nil
}

// PersistCurrent 保存当前 state 中的 store
func PersistCurrent(state *CronServiceState) error {
	return Persist(state, nil)
}

// --- 迁移逻辑 ---

// migrateAllJobs 迁移所有 jobs 的遗留字段
func migrateAllJobs(state *CronServiceState, store *CronStoreFile) bool {
	if store == nil || len(store.Jobs) == 0 {
		return false
	}

	nowMs := state.NowMs()
	anyMigrated := false

	for i := range store.Jobs {
		job := &store.Jobs[i]
		if migrateJob(state, job, nowMs) {
			anyMigrated = true
		}
	}

	return anyMigrated
}

// migrateJob 迁移单个 job 的遗留字段
func migrateJob(state *CronServiceState, job *CronJob, nowMs int64) bool {
	mutated := false

	// 确保有 ID
	if strings.TrimSpace(job.ID) == "" {
		job.ID = generateJobID()
		mutated = true
	}

	// 规范化名称
	name := NormalizeRequiredName(job.Name)
	if name == "" {
		name = InferLegacyName(job.Schedule, job.Payload)
		mutated = true
	}
	if name != job.Name {
		job.Name = name
		mutated = true
	}

	// 规范化描述
	desc := NormalizeOptionalText(job.Description, maxJobDescLen)
	if desc != job.Description {
		job.Description = desc
		mutated = true
	}

	// 规范化 agentId
	agentID := NormalizeOptionalAgentId(job.AgentID)
	if agentID != job.AgentID {
		job.AgentID = agentID
		mutated = true
	}

	// 确保 sessionTarget 有默认值
	if job.SessionTarget == "" {
		job.SessionTarget = defaultSessionTarget
		mutated = true
	}

	// 确保 wakeMode 有默认值
	if job.WakeMode == "" {
		job.WakeMode = defaultWakeMode
		mutated = true
	}

	// 断言 sessionTarget vs payload.kind 一致性
	expectedKind := PayloadKindSystemEvent
	if job.SessionTarget == SessionTargetIsolated {
		expectedKind = PayloadKindAgentTurn
	}
	if job.Payload.Kind != expectedKind {
		job.Payload.Kind = expectedKind
		mutated = true
	}

	// 遗留负载迁移
	if MigrateLegacyCronPayloadTyped(&job.Payload) {
		mutated = true
	}

	// 确保有创建时间
	if job.CreatedAtMs <= 0 {
		job.CreatedAtMs = nowMs
		mutated = true
	}
	if job.UpdatedAtMs <= 0 {
		job.UpdatedAtMs = nowMs
		mutated = true
	}

	return mutated
}

// --- store 检查 ---

// IsStoreStale 检查 store 文件是否被外部修改
func IsStoreStale(state *CronServiceState) bool {
	if state.store == nil || state.Deps.StorePath == "" {
		return false
	}

	info, err := os.Stat(state.Deps.StorePath)
	if err != nil {
		return false
	}

	return info.ModTime().UnixMilli() != state.storeModTimeMs
}

// ReloadIfStale 如果 store 文件被外部修改，重新加载
func ReloadIfStale(state *CronServiceState) error {
	if !IsStoreStale(state) {
		return nil
	}

	// 标记为未加载，下次 EnsureLoaded 会重新加载
	state.store = nil
	state.storeLoadedAtMs = 0

	if state.Deps.Logger != nil {
		state.Deps.Logger.Info("cron: store file changed externally, reloading")
	}
	return EnsureLoaded(state)
}

// GetJobs 获取当前 store 中的 jobs（需已加载）
func GetJobs(state *CronServiceState) []CronJob {
	if state.store == nil {
		return nil
	}
	return state.store.Jobs
}

// SetJobs 设置 store 中的 jobs（用于删除等操作）
func SetJobs(state *CronServiceState, jobs []CronJob) {
	if state.store == nil {
		state.store = &CronStoreFile{Version: storeVersion}
	}
	state.store.Jobs = jobs
}

// --- 工具 ---

// StoreLastLoadDuration 计算上次加载耗时
func StoreLastLoadDuration(state *CronServiceState) time.Duration {
	if state.storeLoadedAtMs <= 0 {
		return 0
	}
	return time.Duration(state.NowMs()-state.storeLoadedAtMs) * time.Millisecond
}
