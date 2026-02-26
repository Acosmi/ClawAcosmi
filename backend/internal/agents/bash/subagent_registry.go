// bash/subagent_registry.go — 子代理注册表。
// TS 参考：src/agents/subagent-registry.ts (431L) + subagent-registry.store.ts (119L)
//
// 管理子代理运行记录：注册、查询、持久化、清理、恢复。
package bash

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ---------- 常量 ----------

const (
	SubagentAnnounceTimeoutMs = 120_000
	DefaultArchiveAfterMin    = 60
	SubagentSweepInterval     = 60 * time.Second
)

// ---------- 类型 ----------

// SubagentRunOutcome 子代理运行结果。
type SubagentRunOutcome struct {
	Status string `json:"status"` // "ok" | "error"
	Error  string `json:"error,omitempty"`
}

// DeliveryContext 投递上下文（规范化后）。
type DeliveryContext struct {
	Channel   string `json:"channel,omitempty"`
	ChannelID string `json:"channelId,omitempty"`
	Platform  string `json:"platform,omitempty"`
	To        string `json:"to,omitempty"`
	AccountID string `json:"accountId,omitempty"`
	ThreadID  string `json:"threadId,omitempty"`
}

// SubagentRunRecord 子代理运行记录。
// TS 参考: subagent-registry.ts L12-28
type SubagentRunRecord struct {
	RunID               string              `json:"runId"`
	ChildSessionKey     string              `json:"childSessionKey"`
	RequesterSessionKey string              `json:"requesterSessionKey"`
	RequesterOrigin     *DeliveryContext    `json:"requesterOrigin,omitempty"`
	RequesterDisplayKey string              `json:"requesterDisplayKey"`
	Task                string              `json:"task"`
	Cleanup             string              `json:"cleanup"` // "delete" | "keep"
	Label               string              `json:"label,omitempty"`
	CreatedAt           int64               `json:"createdAt"`
	StartedAt           *int64              `json:"startedAt,omitempty"`
	EndedAt             *int64              `json:"endedAt,omitempty"`
	Outcome             *SubagentRunOutcome `json:"outcome,omitempty"`
	ArchiveAtMs         *int64              `json:"archiveAtMs,omitempty"`
	CleanupCompletedAt  *int64              `json:"cleanupCompletedAt,omitempty"`
	CleanupHandled      bool                `json:"cleanupHandled,omitempty"`
}

// SubagentRegistryConfig 注册表配置。
type SubagentRegistryConfig struct {
	DataDir         string // 持久化目录
	ArchiveAfterMin int    // 归档延迟（分钟）
}

// ---------- Registry ----------

// SubagentRegistry 子代理注册表（线程安全）。
type SubagentRegistry struct {
	mu               sync.RWMutex
	runs             map[string]*SubagentRunRecord
	dataDir          string
	archiveAfterMs   int64
	sweepStop        chan struct{}
	sweepStarted     bool
	restoreAttempted bool
}

// NewSubagentRegistry 创建新注册表。
func NewSubagentRegistry(cfg SubagentRegistryConfig) *SubagentRegistry {
	archiveMin := cfg.ArchiveAfterMin
	if archiveMin <= 0 {
		archiveMin = DefaultArchiveAfterMin
	}
	return &SubagentRegistry{
		runs:           make(map[string]*SubagentRunRecord),
		dataDir:        cfg.DataDir,
		archiveAfterMs: int64(archiveMin) * 60_000,
	}
}

// RegisterRun 注册子代理运行。
// TS 参考: subagent-registry.ts L282-321
func (r *SubagentRegistry) RegisterRun(params SubagentRunRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UnixMilli()
	params.CreatedAt = now
	startedAt := now
	params.StartedAt = &startedAt
	params.CleanupHandled = false

	if r.archiveAfterMs > 0 {
		archiveAt := now + r.archiveAfterMs
		params.ArchiveAtMs = &archiveAt
	}

	r.runs[params.RunID] = &params
	r.persistLocked()

	if params.ArchiveAtMs != nil {
		r.startSweeperLocked()
	}
}

// GetRun 获取运行记录。
func (r *SubagentRegistry) GetRun(runID string) *SubagentRunRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runs[runID]
}

// ReleaseRun 释放运行记录。
// TS 参考: subagent-registry.ts L410-418
func (r *SubagentRegistry) ReleaseRun(runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	deleted := false
	if _, ok := r.runs[runID]; ok {
		delete(r.runs, runID)
		deleted = true
	}
	if deleted {
		r.persistLocked()
	}
	if len(r.runs) == 0 {
		r.stopSweeperLocked()
	}
}

// ListForRequester 列出请求者的所有运行。
// TS 参考: subagent-registry.ts L420-426
func (r *SubagentRegistry) ListForRequester(requesterSessionKey string) []*SubagentRunRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*SubagentRunRecord
	for _, entry := range r.runs {
		if entry.RequesterSessionKey == requesterSessionKey {
			result = append(result, entry)
		}
	}
	return result
}

// MarkEnded 标记运行结束。
func (r *SubagentRegistry) MarkEnded(runID string, endedAt int64, outcome SubagentRunOutcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.runs[runID]
	if !ok {
		return
	}
	entry.EndedAt = &endedAt
	entry.Outcome = &outcome
	r.persistLocked()
}

// MarkStarted 标记运行开始。
func (r *SubagentRegistry) MarkStarted(runID string, startedAt int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.runs[runID]
	if !ok {
		return
	}
	entry.StartedAt = &startedAt
	r.persistLocked()
}

// BeginCleanup 开始清理。
// TS 参考: subagent-registry.ts L266-280
func (r *SubagentRegistry) BeginCleanup(runID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.runs[runID]
	if !ok || entry.CleanupCompletedAt != nil || entry.CleanupHandled {
		return false
	}
	entry.CleanupHandled = true
	r.persistLocked()
	return true
}

// FinalizeCleanup 完成清理。
// TS 参考: subagent-registry.ts L246-264
func (r *SubagentRegistry) FinalizeCleanup(runID, cleanup string, didAnnounce bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.runs[runID]
	if !ok {
		return
	}
	if !didAnnounce {
		entry.CleanupHandled = false
		r.persistLocked()
		return
	}
	if cleanup == "delete" {
		delete(r.runs, runID)
		r.persistLocked()
		return
	}
	now := time.Now().UnixMilli()
	entry.CleanupCompletedAt = &now
	r.persistLocked()
}

// Init 初始化（恢复磁盘数据）。
// TS 参考: subagent-registry.ts L428-430
func (r *SubagentRegistry) Init() {
	r.restoreFromDisk()
}

// ResetForTests 测试重置。
func (r *SubagentRegistry) ResetForTests() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = make(map[string]*SubagentRunRecord)
	r.stopSweeperLocked()
	r.restoreAttempted = false
	r.persistLocked()
}

// ---------- 持久化 ----------

const registryFilename = "subagent-registry.json"

type registryFileV2 struct {
	Version int                  `json:"version"`
	Runs    []*SubagentRunRecord `json:"runs"`
}

func (r *SubagentRegistry) persistLocked() {
	if r.dataDir == "" {
		return
	}
	filePath := filepath.Join(r.dataDir, registryFilename)
	records := make([]*SubagentRunRecord, 0, len(r.runs))
	for _, entry := range r.runs {
		records = append(records, entry)
	}
	data := registryFileV2{Version: 2, Runs: records}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		slog.Debug("subagent registry persist failed", "err", err)
		return
	}
	_ = os.MkdirAll(r.dataDir, 0755)
	_ = os.WriteFile(filePath, b, 0644)
}

func (r *SubagentRegistry) restoreFromDisk() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.restoreAttempted {
		return
	}
	r.restoreAttempted = true

	if r.dataDir == "" {
		return
	}
	filePath := filepath.Join(r.dataDir, registryFilename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return // file doesn't exist, that's OK
	}

	// 尝试 v2 格式
	var v2 registryFileV2
	if err := json.Unmarshal(data, &v2); err == nil && v2.Version == 2 {
		for _, entry := range v2.Runs {
			if entry.RunID == "" {
				continue
			}
			if _, exists := r.runs[entry.RunID]; !exists {
				r.runs[entry.RunID] = entry
			}
		}
		r.maybeStartSweeperLocked()
		return
	}

	// 尝试 v1 格式 (plain map)
	var v1 map[string]*SubagentRunRecord
	if err := json.Unmarshal(data, &v1); err == nil {
		for runID, entry := range v1 {
			if runID == "" || entry == nil {
				continue
			}
			entry.RunID = runID
			if _, exists := r.runs[runID]; !exists {
				r.runs[runID] = entry
			}
		}
		// Migrate to v2
		r.persistLocked()
		r.maybeStartSweeperLocked()
	}
}

// ---------- 清理 ----------

func (r *SubagentRegistry) maybeStartSweeperLocked() {
	for _, entry := range r.runs {
		if entry.ArchiveAtMs != nil {
			r.startSweeperLocked()
			return
		}
	}
}

func (r *SubagentRegistry) startSweeperLocked() {
	if r.sweepStarted {
		return
	}
	r.sweepStarted = true
	r.sweepStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(SubagentSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.sweep()
			case <-r.sweepStop:
				return
			}
		}
	}()
}

func (r *SubagentRegistry) stopSweeperLocked() {
	if !r.sweepStarted {
		return
	}
	close(r.sweepStop)
	r.sweepStarted = false
}

func (r *SubagentRegistry) sweep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UnixMilli()
	mutated := false
	for runID, entry := range r.runs {
		if entry.ArchiveAtMs == nil || *entry.ArchiveAtMs > now {
			continue
		}
		delete(r.runs, runID)
		mutated = true
		// Note: session deletion via gateway would happen here in production
	}
	if mutated {
		r.persistLocked()
	}
	if len(r.runs) == 0 {
		r.stopSweeperLocked()
	}
}

// NormalizeDeliveryContext 规范化投递上下文。
// TS 参考: utils/delivery-context.ts normalizeDeliveryContext
func NormalizeDeliveryContext(ctx *DeliveryContext) *DeliveryContext {
	if ctx == nil {
		return nil
	}
	return &DeliveryContext{
		Channel:   ctx.Channel,
		ChannelID: ctx.ChannelID,
		Platform:  ctx.Platform,
		To:        ctx.To,
		AccountID: ctx.AccountID,
		ThreadID:  ctx.ThreadID,
	}
}

// DeliveryContextKey 生成投递上下文的唯一键。
// TS 参考: utils/delivery-context.ts deliveryContextKey L132-140
func DeliveryContextKey(ctx *DeliveryContext) string {
	normalized := NormalizeDeliveryContext(ctx)
	if normalized == nil || normalized.Channel == "" || normalized.To == "" {
		return ""
	}
	threadID := ""
	if normalized.ThreadID != "" {
		threadID = normalized.ThreadID
	}
	return normalized.Channel + "|" + normalized.To + "|" + normalized.AccountID + "|" + threadID
}

// ListAllRuns 列出所有运行记录（调试用）。
func (r *SubagentRegistry) ListAllRuns() []*SubagentRunRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*SubagentRunRecord, 0, len(r.runs))
	for _, entry := range r.runs {
		result = append(result, entry)
	}
	return result
}

// Count 返回注册表中的运行数量。
func (r *SubagentRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.runs)
}

// DefaultSubagentRegistry 全局默认子代理注册表。
var DefaultSubagentRegistry *SubagentRegistry

// InitDefaultSubagentRegistry 初始化全局注册表。
func InitDefaultSubagentRegistry(dataDir string) {
	if DefaultSubagentRegistry != nil {
		return
	}
	DefaultSubagentRegistry = NewSubagentRegistry(SubagentRegistryConfig{
		DataDir: dataDir,
	})
	DefaultSubagentRegistry.Init()
}

// RegisterSubagentRun 便捷函数：注册到全局注册表。
func RegisterSubagentRun(params SubagentRunRecord) {
	if DefaultSubagentRegistry == nil {
		InitDefaultSubagentRegistry("")
	}
	DefaultSubagentRegistry.RegisterRun(params)
}

// ListSubagentRunsForRequester 便捷函数：列出请求者的运行。
func ListSubagentRunsForRequester(requesterSessionKey string) []*SubagentRunRecord {
	if DefaultSubagentRegistry == nil {
		return nil
	}
	return DefaultSubagentRegistry.ListForRequester(requesterSessionKey)
}

// Format 返回运行记录的简短描述。
func (r *SubagentRunRecord) Format() string {
	status := "pending"
	if r.EndedAt != nil {
		if r.Outcome != nil && r.Outcome.Status == "error" {
			status = "error"
		} else {
			status = "completed"
		}
	} else if r.StartedAt != nil {
		status = "running"
	}
	return fmt.Sprintf("[%s] %s (child=%s)", status, r.Task, r.ChildSessionKey)
}
