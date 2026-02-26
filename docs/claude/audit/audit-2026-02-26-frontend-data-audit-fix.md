---
document_type: Audit
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
---

# 审计报告: 前端数据审计修复 (F-01/F-03/F-04/F-06)

## Scope

4 个文件的修改，修复 7 个审计发现 (含复核发现):
- `backend/internal/gateway/server_methods.go` — F-01 授权修复
- `backend/internal/gateway/server_methods_usage.go` — F-03 时间字段 + F-04 daily 聚合 + RC-01/RC-02 字段对齐 + F-A02 消息计数
- `backend/internal/gateway/ws_server.go` — F-06 UptimeMs
- `backend/internal/gateway/server.go` — F-06 BootedAt 初始化

## Findings

### RC-01 [CRITICAL] — `dailyBreakdown` 字段结构与前端不匹配 ✅ 已修复

**Location**: `server_methods_usage.go` dailyUsageEntry 结构体
**Issue**: 初始实现返回 `{ date, totals: { input, output, totalTokens, ... } }` 嵌套对象，但前端 `types.ts:465` 期望 `{ date: string; tokens: number; cost: number }`。`usage.ts:2676` 读 `day.tokens` 和 `day.cost` 会得到 `undefined`。
**Impact**: HIGH — per-session dailyBreakdown 数据完全无效，前端热力图和日筛选功能不工作。
**Fix**: `dailyUsageEntry` 改为扁平 `{ Date, Tokens, Cost }` 结构，对齐前端类型。

### RC-02 [CRITICAL] — `aggregates.daily` 字段结构与前端不匹配 ✅ 已修复

**Location**: `server_methods_usage.go` globalDaily 聚合 + dailyArr 构建
**Issue**: 初始实现返回 `{ date, totals: {...} }`，但前端 `types.ts:607-614` 期望 `{ date, tokens, cost, messages, toolCalls, errors }`。`usage.ts:2809` 读 `day.messages` 和 `day.errors` 会得到 `undefined`。
**Impact**: HIGH — Daily 柱状图和错误分析完全无数据。
**Fix**: `globalDailyEntry` 使用扁平结构体 `{ Tokens, Cost, Messages, ToolCalls, Errors }`，dailyArr 输出扁平字段。

### F-A02 [MEDIUM] — aggregates.daily 的 messages/toolCalls/errors 始终为 0 ✅ 已修复

**Location**: `server_methods_usage.go` loadSessionCostFromFile + 全局聚合
**Issue**: 初始实现无 per-day 消息计数，`globalDaily.Messages`/`ToolCalls`/`Errors` 始终为 0。
**Fix**:
1. 新增 `dailyMsgCountEntry` 结构体 + `DailyMessageCounts` 字段 (对齐前端 `types.ts:466-473`)
2. 循环中同时统计 per-day user/assistant/toolCalls/toolResults/errors
3. 利用 `is_error` 标记 (`llmclient/types.go:44`) 检测 tool_result 错误
4. 全局聚合中合并 per-session DailyMessageCounts → globalDaily.Messages/ToolCalls/Errors
5. 附带修复: `counts.Errors` 之前从未递增，现在正确统计 `is_error: true` 的 tool_result

### F-A01 [INFO] — dailyBreakdown.cost / aggregates.daily.cost 始终为 0

**Location**: `server_methods_usage.go` dailyMap 聚合
**Issue**: 系统级无 pricing table，所有 cost 字段为 0 (包括 `usageTotals.TotalCost`)。JSONL transcript 中 cost 对象也硬编码为 0 (`transcript.go:169-181`)。
**Risk**: LOW — 前端对 `cost: 0` 显示为空/0，不报错。属于系统级 pricing 模块缺失，非本次修复范围。

### F-A03 [INFO] — BootedAt 零值保护

**Location**: `ws_server.go:347`
**Issue**: 如果 `cfg.BootedAt` 为零值（测试代码未设置），`time.Since()` 会返回非常大的值。
**Risk**: NEGLIGIBLE — 仅影响测试场景，生产始终设置 `time.Now()`。

### F-A04 [INFO] — dateSet 与 dailyMap 语义不同，保持分离正确

**Location**: `server_methods_usage.go` loadSessionCostFromFile
**Issue**: `dateSet` 条件 `msgTs > 0`（所有有 timestamp 的消息），`dailyMap` 条件 `msgTs > 0 && (msgInput > 0 || msgOutput > 0)`（仅有 token 的消息）。二者语义不同。
**Risk**: NEGLIGIBLE — 保持分离是正确的。

## Correctness Verification

### 字段交叉比对 (后端 → 前端)

| 后端结构 | 前端类型 | 匹配 |
|---|---|---|
| `dailyUsageEntry{Date, Tokens, Cost}` | `types.ts:465` `{date, tokens, cost}` | ✅ |
| `dailyMsgCountEntry{Date, Total, User, Assistant, ToolCalls, ToolResults, Errors}` | `types.ts:466-473` `{date, total, user, assistant, toolCalls, toolResults, errors}` | ✅ |
| `globalDailyEntry → {date, tokens, cost, messages, toolCalls, errors}` | `types.ts:607-614` `{date, tokens, cost, messages, toolCalls, errors}` | ✅ |

### 逻辑验证

- **F-01**: `"memory.stats"` 正确加入 `readMethods` ✅
- **F-03**: timestamp `float64` → `int64` 无精度损失 (< 2^53) ✅
- **F-03**: `firstTs`/`lastTs` 初始化为 0，首条有效 timestamp 正确设置 ✅
- **F-03**: `activityDates` / `dailyBreakdown` / `dailyMessageCounts` 均排序输出 ✅
- **F-04**: globalDaily 在 cost 循环中收集，避免二次 IO ✅
- **F-A02**: `is_error` 检测用 `bm["is_error"].(bool)` 类型安全 (零值 false) ✅
- **F-A02**: `counts.Errors` 从 tool_result is_error 统计，与 `dailyMsgMap` errors 保持一致 ✅
- **F-A02**: per-day 消息计数条件 `msgTs > 0 && role != ""` — header 行无 role 自动跳过 ✅
- **F-06**: `BootedAt` 在 server.go 中 `time.Now()` 初始化 ✅
- **编译**: `go build ./...` 通过 ✅

## Verdict

**PASS** — 2 CRITICAL + 1 MEDIUM 已修复，2 INFO 记录（1 为系统级 pricing 缺失）。编译通过。所有字段精确对齐前端 TypeScript 类型。
