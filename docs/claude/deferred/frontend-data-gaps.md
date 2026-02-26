---
document_type: Deferred
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-frontend-data-gaps-d01-d05.md
skill5_verified: true
---

# 前端数据缺口 — Usage 页未填充字段

> 全部 5 项延迟项已于 2026-02-26 实现。

## ~~F-05: aggregates.byChannel 始终空数组~~ ✅ 已实现 (D-04)

**实现**: `handleSessionsUsage()` 从 `m.storeEntry.Channel` 读取频道标识（空则 `"direct"`），按频道聚合 `usageTotals`，输出 `byChannelArr`。

## ~~F-07: Sessions 不渲染 subject/room/space~~ ✅ 已实现 (D-05)

**实现**: `handleSessionsUsage()` session entry 输出时从 `m.storeEntry` 读取 `Subject` → `"subject"`, `GroupChannel` → `"room"`, `Space` → `"space"`, `Channel` → `"channel"`。

## ~~F-A01: dailyBreakdown.cost / aggregates.daily.cost 始终为 0~~ ✅ 已实现 (D-02)

**实现**: `LookupZenModelCost()` 查询 `zenModelCosts` 定价表（14 个模型），`loadSessionCostFromFile()` 按 model 查表计算 per-message cost（input/output/cacheRead/cacheWrite），累加到 `totals`/`byModel`/`dailyBreakdown`。

## ~~dailyLatency: per-session 延迟统计未填充~~ ✅ 已实现 (D-03)

**实现**: `loadSessionCostFromFile()` 跟踪 `lastUserTs`，user→first assistant 时间差作为近似延迟（上限 600s），按日收集原始值，计算 min/max/avg/P95。全局聚合用 `rawLatencyData` 原始值重新计算（避免 percentile 合并失真）。

## ~~dailyModelUsage: per-session 模型每日用量未填充~~ ✅ 已实现 (D-01)

**实现**: `loadSessionCostFromFile()` 按 `date::provider::model` 二维聚合 tokens/cost/count，`handleSessionsUsage()` 全局合并输出为 `modelDaily` 数组。

---

## 剩余项 (复审新增)

### LOW: SessionsUsageEntry 缺少 subject/room/space TS 类型 (F-06)

**来源**: 归档前复审发现
**描述**: 后端 D-05 输出 `subject/room/space` 到 usage session entry，但前端 `SessionsUsageEntry` (`types.ts:427`) 缺少这 3 个可选字段定义。
**影响**: 数据在 JSON 响应中存在但 TypeScript 无法消费。
**修复**: `types.ts` 的 `SessionsUsageEntry` 补充 `subject?: string; room?: string; space?: string;`
