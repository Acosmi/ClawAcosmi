---
document_type: Tracking
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-frontend-data-audit-fix.md
skill5_verified: true
---

# 前端数据审计修复 — 空数据 + 时间戳缺失

## Online Verification Log

### Go JSONL Timestamp Extraction
- **Query**: Go JSONL file parse timestamp best practices
- **Source**: Go stdlib encoding/json; 现有 `loadTimeseriesFromFile()` (L676-724)
- **Key finding**: 现有 `loadSessionCostFromFile()` 用 `os.ReadFile` + `strings.Split` + `json.Unmarshal`，只需在同循环中额外提取 `timestamp` 字段。
- **Verified date**: 2026-02-26

### 时间戳约定
- **Source**: `server_methods_usage.go:626` (`.UnixMilli()`), `server_methods_memory.go:453` (`.Unix()`)
- **Key finding**: Session JSONL `timestamp` 是 Unix 毫秒。新字段必须用毫秒。
- **Verified date**: 2026-02-26

### Cost 计算可行性
- **Query**: 项目 pricing table / cost 计算逻辑
- **Source**: 全局代码搜索 + `transcript.go:169-181` + `internal/infra/cost/`
- **Key finding**: 系统无 pricing table，所有 cost 字段硬编码为 0。`dailyBreakdown.cost` 为 0 是系统级限制，非本次修复范围。
- **Verified date**: 2026-02-26

## Tasks

### 已修复 (7 项)

- [x] **F-01** [HIGH]: `memory.stats` 加入 `readMethods` → 403 Forbidden 解决
- [x] **F-03** [HIGH]: `sessionCostSummary` 扩展时间字段 + timestamp 提取 + daily 聚合
- [x] **F-04** [MEDIUM]: `aggregates.daily` 全局聚合，替换空数组
- [x] **F-06** [LOW]: `SnapshotData.UptimeMs` — WsServerConfig.BootedAt + time.Since
- [x] **RC-01** [CRITICAL]: `dailyBreakdown` 改为扁平 `{date, tokens, cost}` 对齐前端
- [x] **RC-02** [CRITICAL]: `aggregates.daily` 改为扁平 `{date, tokens, cost, messages, toolCalls, errors}`
- [x] **F-A02** [MEDIUM]: `dailyMessageCounts` + `counts.Errors` 按日消息计数 + is_error 检测
- [x] 编译验证: `go build ./...` ✅
- [x] 字段交叉比对: 后端 JSON tag ↔ 前端 TypeScript 类型 全部匹配 ✅

### Deferred (无数据源，记录于 `docs/claude/deferred/frontend-data-gaps.md`)

- [x] **F-05** [MEDIUM]: `aggregates.byChannel` 始终空数组
  - **根因**: JSONL `SessionHeader` 无 `channel` 字段
  - **后续**: channel plugin 实现后填充

- [x] **F-07** [INFO]: Sessions 不渲染 subject/room/space
  - **根因**: JSONL header 无对应字段
  - **后续**: 丰富 session 元数据时补充

- [x] **F-A01** [INFO]: dailyBreakdown.cost / aggregates.daily.cost 始终为 0
  - **根因**: 系统无 pricing table，所有 cost 字段为 0
  - **后续**: 实现 pricing 模块后统一处理

- [x] **dailyLatency** [INFO]: per-session 延迟统计未填充
  - **根因**: JSONL 无 latency 字段，需 runner 层记录 per-turn 延迟
  - **前端**: `?? []` fallback，延迟图表不渲染
  - **后续**: runner 记录请求延迟 → transcript → loadSessionCostFromFile 按日聚合

- [x] **dailyModelUsage** [INFO]: per-session 模型每日用量未填充
  - **根因**: 已有全局 byModel 聚合，但未按 date+model 二维聚合
  - **前端**: `?? []` fallback，模型堆叠图不渲染
  - **后续**: 在 loadSessionCostFromFile 循环中按 dateStr+modelKey 聚合（实现难度低）

### 无需修复

- [x] **F-08** [INFO]: Instances 依赖 `system-event` 推送 — 正常行为

## 修改文件

| 文件 | 改动 |
|---|---|
| `server_methods.go` | +1 行: `"memory.stats"` 加入 readMethods |
| `server_methods_usage.go` | ~120 LOC: 时间字段 + 扁平 daily 结构 + dailyMessageCounts + is_error 检测 + 全局聚合 |
| `ws_server.go` | ~3 行: WsServerConfig.BootedAt 字段 + UptimeMs 计算 |
| `server.go` | +1 行: BootedAt: time.Now() |

## 审计

- **报告**: `docs/claude/audit/audit-2026-02-26-frontend-data-audit-fix.md`
- **结果**: PASS (2 CRITICAL + 1 MEDIUM 已修复, 2 INFO 记录)
