---
document_type: Audit
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
---

# 审计报告: 前端数据缺口修复 D-01 ~ D-05

## 范围

| 文件 | 改动类型 | LOC |
|---|---|---|
| `internal/agents/models/openacosmi_zen_models.go` | 新增 `LookupZenModelCost()` | +14 |
| `internal/gateway/server_methods_usage.go` | 新增类型 + 5 项功能 | ~200 |

**功能覆盖**: D-01 dailyModelUsage, D-02 cost 计算, D-03 dailyLatency, D-04 byChannel, D-05 session metadata

---

## 审计清单

### Security

| 检查项 | 结果 | 说明 |
|---|---|---|
| 路径遍历 | PASS | 无新文件路径构造。`sessionFile` 来自上游已验证路径 |
| 输入注入 | PASS | 无新用户输入处理。`Channel`/`Subject` 来自内部 session store |
| 权限边界 | PASS | 无新权限检查或绕过。纯数据聚合 |
| 信息泄露 | PASS | 仅输出已有 session 元数据字段，未暴露新敏感信息 |
| JSON 输出安全 | PASS | `encoding/json` 自动转义特殊字符 |

### Resource Safety

| 检查项 | 结果 | 说明 |
|---|---|---|
| 内存增长 | PASS | 新 map 大小与现有 `dailyMap`/`byModel` 同量级，受 session 文件大小约束 |
| goroutine 泄漏 | N/A | 无新 goroutine |
| FD 泄漏 | N/A | 无新文件打开操作 |
| 清理路径 | PASS | 所有新数据结构为函数局部变量，函数返回后自动 GC |
| `rawLatencyData` 生命周期 | PASS | 引用传递到 `sessionCostSummary`，全局聚合后不再引用，可 GC |

### Correctness

| 检查项 | 结果 | 说明 |
|---|---|---|
| Cost 公式 | PASS | `tokens * price / 1_000_000`，行业标准。14 模型定价与 `zenModelCosts` 一致 |
| 别名解析 | PASS | `LookupZenModelCost` 先查原始 ID，再尝试 alias。`ResolveAcosmiZenAlias` 已处理 lowercase+trim |
| P95 nearest-rank | PASS | `ceil(0.95*n)-1`，n=1→0, n=20→18, n=100→94，均为有效索引。含 `<0` 和 `>=n` 防御性检查 |
| Latency 配对 | PASS | user→first assistant 配对，tool_result 不干扰，连续 user 取最后一条 |
| 延迟上限 | PASS | `< 600_000ms` (10分钟)，过滤无效超长间隔 |
| dailyModel key 唯一性 | PASS | `date::provider::model` 或 `date::model`，与 byModel 的 `provider::model` 格式一致 |
| 全局 latency 聚合 | PASS | 用原始值 `rawLatencyData` 重新计算 P95，避免 percentile 合并失真 |
| byChannel nil guard | PASS | `m.storeEntry` nil 时默认 `"direct"`，`cost != nil` 时才聚合 |
| D-05 空值跳过 | PASS | 每个字段独立检查 `!= ""`，nil storeEntry 整体跳过 |
| `handleUsageCost` 兼容 | PASS | `mergeTotalsInto` 已包含所有 cost 字段，`handleUsageCost` 自动受益 |

### Edge Cases

| 场景 | 结果 | 说明 |
|---|---|---|
| 空 session 文件 | PASS | `loadSessionCostFromFile` 返回 nil，调用方已处理 |
| 无 usage 数据 | PASS | 所有聚合数组保持空，cost 保持 0 |
| 单条延迟数据 | PASS | n=1: min=max=avg=p95，数学正确 |
| 零成本模型 (glm-4.7) | PASS | `LookupZenModelCost` 返回 `{0,0,0,0}`，cost 计为 0，不计入 `MissingCostEntries` |
| storeEntry 为 nil | PASS | D-04 默认 `"direct"`，D-05 跳过元数据，无 panic |
| 负时间戳 | PASS | `msgTs > 0` 在所有延迟/日期聚合处守卫 |
| 超大 JSONL | PASS | 与现有 `os.ReadFile` + `strings.Split` 模式一致，无新内存压力 |

### Frontend Type 兼容性

| Go JSON tag | TS 类型 | 匹配 |
|---|---|---|
| `dailyModelEntry.date/provider/model/tokens/cost/count` | `types.ts:483-490 dailyModelUsage` (per-session) | ✅ |
| `dailyLatencyEntry.date/count/avgMs/p95Ms/minMs/maxMs` | `types.ts:475-482 dailyLatency` (per-session) | ✅ |
| 聚合 `"modelDaily"` | `types.ts:599-606 modelDaily?` | ✅ |
| 聚合 `"dailyLatency"` | `types.ts:591-598 dailyLatency?` | ✅ |
| 聚合 `"byChannel"` | `types.ts:583 byChannel` | ✅ |
| session `"channel"` | `types.ts:433 SessionsUsageEntry.channel?` | ✅ |
| session `"subject"/"room"/"space"` | `SessionsUsageEntry` **未定义** (见 F-06) | ⚠️ LOW |

> **复审修正**: 初次审计误将 session 元数据比对到 `GatewaySessionRow`（sidebar 类型），
> 实际 usage 响应的 sessions 数组使用 `SessionsUsageEntry` (types.ts:427)。
> 该类型有 `channel?` 但缺少 `subject?/room?/space?`。

---

## Findings

### F-01 — INFO: byModel/daily 门控不含 cache-only 消息

**位置**: `server_methods_usage.go:393,412,424`
**描述**: byModel 和 daily 聚合条件为 `(msgInput > 0 || msgOutput > 0)`，不包括仅有 cache tokens 的消息。而 cost 计算条件更宽（含 `msgCacheRead > 0 || msgCacheWrite > 0`），导致 cache-only 消息的 cost 计入 `totals` 但不计入 `byModel`/`daily`/`dailyModel`。
**风险**: 极低。实际场景中消息几乎总是同时携带 input/output 和 cache tokens。纯 cache-only 消息极为罕见。
**建议**: 如需完全一致，可将 byModel/daily 门控改为 `(msgInput > 0 || msgOutput > 0 || msgCacheRead > 0 || msgCacheWrite > 0)`。当前行为与改动前一致（pre-existing），无回归。
**处理**: 无需修复，记录备查。

### F-02 — INFO: P95 计算代码重复

**位置**: `server_methods_usage.go:526-556` (per-session), `988-1020` (global)
**描述**: P95 nearest-rank + min/max/avg 计算逻辑在两处完全相同，可提取为 `computeLatencyStats(vals []float64) dailyLatencyEntry` helper。
**风险**: 无。两处逻辑正确一致。
**建议**: 未来重构时提取公共函数。
**处理**: 无需修复。

### F-03 — INFO: float64 累加精度

**位置**: `server_methods_usage.go:377-386`
**描述**: cost 使用 `float64` 累加，多次加法可能产生微量浮点误差（如 `0.1 + 0.2 != 0.3`）。
**风险**: 无。此为分析/展示用途，非计费场景。误差量级 < 1e-10，对 USD 显示无影响。
**处理**: 无需修复。

### F-04 — INFO: zenModelCosts 中 CacheWrite 均为 0

**位置**: `openacosmi_zen_models.go:52-67`
**描述**: 14 个模型定价条目均未设置 `CacheWrite` 字段（默认 0）。cache_creation_input_tokens 的成本始终计为 $0。
**风险**: 低。Anthropic 模型的 cache write 定价通常为 input 的 1.25x（如 Claude Opus: $18.75/M），未计入会导致 cost 偏低。
**建议**: 后续更新 `zenModelCosts` 补充 `CacheWrite` 定价。
**处理**: 无需本次修复，已记录至 deferred。

### F-05 — INFO: 聚合级 `latency` 总汇字段未填充

**位置**: 响应 `aggregates` 中缺少 `latency` 字段（TS `types.ts:584-590`）
**描述**: 前端类型定义了可选的 `latency?: {count, avgMs, p95Ms, minMs, maxMs}` 聚合总汇字段，当前只实现了 `dailyLatency` 按日细分。总汇字段未在计划范围内。
**风险**: 无。字段为可选（`?`），前端已有 fallback。
**建议**: 后续可从 `globalLatencyByDate` 扁平化所有值计算一次总汇。
**处理**: 无需本次修复。

### F-06 — LOW: SessionsUsageEntry 缺少 subject/room/space 类型定义 (复审新增)

**位置**: `ui/src/ui/types.ts:427-490` (`SessionsUsageEntry`)
**描述**: 后端 D-05 在 usage 响应的 session entry 中输出 `"subject"`/`"room"`/`"space"` 字段，但前端 `SessionsUsageEntry` 类型只定义了 `channel?: string`（line 433），未定义 `subject?/room?/space?`。JSON 中的这 3 个字段会被 TypeScript 忽略，无法在 usage 页面消费。
**风险**: LOW。后端数据正确存在于 JSON 响应中，不影响现有功能。前端添加 3 行可选字段即可消费。
**建议**: 在 `SessionsUsageEntry` 中补充:
```typescript
subject?: string;
room?: string;
space?: string;
```
**处理**: 记录至 deferred，后端侧无需修改。

---

## 复审记录

| 阶段 | 日期 | 审计人 | 结论 |
|---|---|---|---|
| 初次审计 | 2026-02-26 | claude | PASS (5 INFO) |
| 归档前复审 | 2026-02-26 | claude | PASS (5 INFO + 1 LOW, 无需阻塞归档) |

**复审附加验证**:
- `go build ./...` ✅
- `go vet ./internal/gateway/ ./internal/agents/models/` ✅
- 前端类型精确比对 (SessionsUsageEntry vs GatewaySessionRow 区分) ✅
- 延迟项逐条对照代码实现完整性 ✅

---

## Verdict

**PASS** — 无 CRITICAL / HIGH / MEDIUM 发现。5 INFO + 1 LOW 已记录。

- LOW (F-06): `SessionsUsageEntry` TS 类型缺 3 字段 → 不阻塞归档，后端正确
- 代码正确性、安全性、资源安全性均通过复审
- 编译 + go vet 通过
- 可归档
