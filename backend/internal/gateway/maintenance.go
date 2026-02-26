package gateway

import (
	"log/slog"
	"time"
)

// ---------- 维护计时器 ----------
// 对齐 TS server-maintenance.ts: startGatewayMaintenanceTimers()

const (
	// TickIntervalMs 心跳 tick 间隔（毫秒）。
	// 对齐 TS server-constants.ts: TICK_INTERVAL_MS = 30_000。
	TickIntervalMs = 30_000

	// HealthRefreshIntervalMs 健康快照刷新间隔（毫秒）。
	// 对齐 TS server-constants.ts: HEALTH_REFRESH_INTERVAL_MS = 60_000。
	HealthRefreshIntervalMs = 60_000

	// MaintenanceCleanupIntervalMs 维护清理循环间隔（毫秒）。
	// 对齐 TS server-maintenance.ts L75-117: dedupeCleanup 60s 周期。
	MaintenanceCleanupIntervalMs = 60_000

	// AbortedRunTTLMs abortedRuns 条目 TTL（毫秒）。
	// 对齐 TS server-maintenance.ts L108: ABORTED_RUN_TTL_MS = 60 * 60_000。
	AbortedRunTTLMs = 60 * 60_000 // 1 hour
)

// MaintenanceConfig 维护计时器的配置依赖。
type MaintenanceConfig struct {
	Broadcaster       *Broadcaster
	ChatState         *ChatRunState
	HealthRefreshFunc func() // 可选: 调用时刷新 health 快照
	Logger            *slog.Logger
}

// MaintenanceTimers 维护计时器句柄，用于优雅关闭。
type MaintenanceTimers struct {
	stopCh chan struct{}
}

// Stop 停止所有维护计时器。
func (mt *MaintenanceTimers) Stop() {
	select {
	case <-mt.stopCh:
		// 已关闭
	default:
		close(mt.stopCh)
	}
}

// StartMaintenanceTimers 启动网关维护计时器组。
// 对齐 TS server-maintenance.ts: startGatewayMaintenanceTimers()
//
// 启动 3 个周期任务:
//   - tick (30s): 向所有客户端广播心跳信号
//   - health (60s): 刷新健康快照缓存
//   - cleanup (60s): chatAbort 超时清理 + abortedRuns TTL 清理
func StartMaintenanceTimers(cfg MaintenanceConfig) *MaintenanceTimers {
	stopCh := make(chan struct{})
	mt := &MaintenanceTimers{stopCh: stopCh}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// ---------- tick 广播 (30s) ----------
	// 对齐 TS server-maintenance.ts L56-60
	go func() {
		ticker := time.NewTicker(time.Duration(TickIntervalMs) * time.Millisecond)
		defer ticker.Stop()

		logger.Debug("gateway: maintenance tick started", "intervalMs", TickIntervalMs)
		for {
			select {
			case <-stopCh:
				logger.Debug("gateway: maintenance tick stopped")
				return
			case <-ticker.C:
				if cfg.Broadcaster != nil {
					payload := TickEvent{Ts: time.Now().UnixMilli()}
					cfg.Broadcaster.Broadcast("tick", payload, &BroadcastOptions{
						DropIfSlow: true,
					})
				}
			}
		}
	}()

	// ---------- health 刷新 (60s) ----------
	// 对齐 TS server-maintenance.ts L63-67:
	//   const healthInterval = setInterval(() => {
	//     void params.refreshGatewayHealthSnapshot({ probe: true }).catch(...)
	//   }, HEALTH_REFRESH_INTERVAL_MS);
	if cfg.HealthRefreshFunc != nil {
		go func() {
			ticker := time.NewTicker(time.Duration(HealthRefreshIntervalMs) * time.Millisecond)
			defer ticker.Stop()

			// 启动时立即刷新一次（prime cache）
			// 对齐 TS server-maintenance.ts L70-72
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("gateway: initial health refresh panic", "error", r)
					}
				}()
				cfg.HealthRefreshFunc()
			}()

			logger.Debug("gateway: health refresh started", "intervalMs", HealthRefreshIntervalMs)
			for {
				select {
				case <-stopCh:
					logger.Debug("gateway: health refresh stopped")
					return
				case <-ticker.C:
					func() {
						defer func() {
							if r := recover(); r != nil {
								logger.Error("gateway: health refresh panic", "error", r)
							}
						}()
						cfg.HealthRefreshFunc()
					}()
				}
			}
		}()
	}

	// ---------- cleanup 循环 (60s) ----------
	// 对齐 TS server-maintenance.ts L75-117:
	// 合并 chatAbort 超时清理 + abortedRuns TTL 清理
	if cfg.ChatState != nil {
		go func() {
			ticker := time.NewTicker(time.Duration(MaintenanceCleanupIntervalMs) * time.Millisecond)
			defer ticker.Stop()

			logger.Debug("gateway: maintenance cleanup started", "intervalMs", MaintenanceCleanupIntervalMs)
			for {
				select {
				case <-stopCh:
					logger.Debug("gateway: maintenance cleanup stopped")
					return
				case <-ticker.C:
					maintenanceCleanup(cfg, logger)
				}
			}
		}()
	}

	return mt
}

// maintenanceCleanup 执行一次维护清理。
// 对齐 TS server-maintenance.ts L88-116。
func maintenanceCleanup(cfg MaintenanceConfig, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	nowMs := time.Now().UnixMilli()
	cs := cfg.ChatState

	// ---------- chatAbort 超时清理 ----------
	// 对齐 TS server-maintenance.ts L89-106:
	//   for (const [runId, entry] of params.chatAbortControllers) {
	//     if (now <= entry.expiresAtMs) continue;
	//     abortChatRunById(ops, { runId, sessionKey: entry.sessionKey, stopReason: "timeout" });
	//   }
	var expiredAbortRunIDs []string
	cs.AbortControllers.Range(func(key, value interface{}) bool {
		runID := key.(string)
		entry := value.(*ChatAbortControllerEntry)
		if nowMs <= entry.ExpiresAtMs {
			return true // 未过期，跳过
		}
		expiredAbortRunIDs = append(expiredAbortRunIDs, runID)
		return true
	})

	for _, runID := range expiredAbortRunIDs {
		v, loaded := cs.AbortControllers.LoadAndDelete(runID)
		if !loaded {
			continue
		}
		entry := v.(*ChatAbortControllerEntry)

		// 调用 cancel（等价于 TS controller.abort()）
		if entry.Cancel != nil {
			entry.Cancel()
		}

		// 标记为 aborted
		cs.AbortedRuns.Store(runID, nowMs)

		// 清理关联 buffers
		cs.Buffers.Delete(runID)
		cs.DeltaSentAt.Delete(runID)

		// 广播 abort 事件
		if cfg.Broadcaster != nil {
			cfg.Broadcaster.Broadcast("chat.abort", map[string]interface{}{
				"runId":      runID,
				"sessionKey": entry.SessionKey,
				"stopReason": "timeout",
				"ts":         nowMs,
			}, nil)
		}

		logger.Info("maintenance: aborted expired chat run",
			"runId", runID,
			"sessionKey", entry.SessionKey,
		)
	}

	// ---------- abortedRuns TTL 清理 ----------
	// 对齐 TS server-maintenance.ts L108-116:
	//   const ABORTED_RUN_TTL_MS = 60 * 60_000;
	//   for (const [runId, abortedAt] of params.chatRunState.abortedRuns) {
	//     if (now - abortedAt <= ABORTED_RUN_TTL_MS) continue;
	//     params.chatRunState.abortedRuns.delete(runId);
	//     params.chatRunBuffers.delete(runId);
	//     params.chatDeltaSentAt.delete(runId);
	//   }
	var expiredRunIDs []string
	cs.AbortedRuns.Range(func(key, value interface{}) bool {
		runID := key.(string)
		abortedAt := value.(int64)
		if nowMs-abortedAt <= AbortedRunTTLMs {
			return true // 未过期
		}
		expiredRunIDs = append(expiredRunIDs, runID)
		return true
	})

	for _, runID := range expiredRunIDs {
		cs.AbortedRuns.Delete(runID)
		cs.Buffers.Delete(runID)
		cs.DeltaSentAt.Delete(runID)
	}

	if len(expiredAbortRunIDs) > 0 || len(expiredRunIDs) > 0 {
		logger.Debug("maintenance: cleanup complete",
			"abortControllersCleaned", len(expiredAbortRunIDs),
			"abortedRunsCleaned", len(expiredRunIDs),
		)
	}
}

// StartMaintenanceTick 启动网关维护 tick 广播（向后兼容入口）。
// 推荐使用 StartMaintenanceTimers 替代。
func StartMaintenanceTick(broadcaster *Broadcaster) *MaintenanceTimers {
	return StartMaintenanceTimers(MaintenanceConfig{
		Broadcaster: broadcaster,
	})
}
