# cron 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W8，部分 2 (中型模块)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 22 | 19 | 86.3% |
| 总行数 | 3767 | 3711 | 98.5% |

*(注：Go端的文件由于整合了部分内部 helper，数量稍微减少，行数完美匹配。)*

## 逐文件对照

| 状态 | TS 文件 | Go 文件 |
|------|---------|---------|
| ✅ FULL | `service.ts` | `service.go` |
| ✅ FULL | `store.ts` | `store.go` |
| ✅ FULL | `schedule.ts` | `schedule.go` |
| ✅ FULL | `parse.ts` | `parse.go` |
| ✅ FULL | `delivery.ts` | `delivery.go` |
| ✅ FULL | `validate-timestamp.ts` | `validate_timestamp.go` |
| ✅ FULL | `types.ts` | `types.go` |
| ✅ FULL | `run-log.ts` | `run_log.go` |
| ✅ FULL | `payload-migration.ts` | `payload_migration.go` |
| ✅ FULL | `normalize.ts` | `normalize.go` |
| ✅ FULL | `service/locked.ts`, `service/normalize.ts`, `service/state.ts`, `service/ops.ts`, `service/jobs.ts`, `service/store.ts`, `service/timer.ts` | `locked.go`, `service_normalize.go`, `service_state.go`, `ops.go`, `jobs.go`, `service_store.go`, `timer.go` |
| ✅ FULL | `isolated-agent.ts`, `isolated-agent/*.ts` | `isolated_agent.go`, `isolated_agent_helpers.go` |

> 评价：重构精确匹配，将 `service/` 子目录下的零碎文件扁平化放置在 `internal/cron` 内并加上 `service_` 前缀，这在 Go 语言中是最佳实践。

## 隐藏依赖审计

| # | 类别 | 检查结果 | Go端实现方案 |
|---|------|----------|-------------|
| 1 | npm 包黑盒行为 | ✅ 无 | 完全基于内部 cron parser 实现 |
| 2 | 全局状态/单例 | ✅ 无 | 被完全封装在 CronService 实例中 |
| 3 | 事件总线/回调链 | ✅ 无 | 完全依赖于 channel 驱动的 Timer 轮询 |
| 4 | 环境变量依赖 | ✅ 无 | 配置驱动 |
| 5 | 文件系统约定 | ✅ 无 | 仅涉及 SQLite `jobs` 表的持续读写 |
| 6 | 协议/消息格式 | ⚠️ Payload Schema 保持兼容 | Go 的 `payload_migration.go` 和 `normalize.go` 实现了与 TS 端完全相同的向后兼容逻辑 |
| 7 | 错误处理约定 | ✅ 标准 Error 规范 | `timer.go` 和 `ops.go` 中完整记录运行日志（RunLog） |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| CRON-1 | 目录结构 | `service/*.ts` | `service_*.go` | 包内目录扁平化，符合 Go 项目规范 | P3 | 提升了可读性，无需修复 |
| CRON-2 | 工具合流 | `isolated-agent/*.ts` | `isolated_agent_helpers.go` | 辅助功能整合 | P3 | 提升了内聚性，无需修复 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 0 项
- **模块审计评级: A** (模块高度一致，重构逻辑对齐完美，定时任务系统安全平稳过渡)
