# Audit Report: Coder 工具实时可视化事件管道

---
document_type: Audit
status: Complete
created: 2026-02-26
last_updated: 2026-02-26
scope: P1-P3 全 3 Phase 代码审计
verdict: **PASS** (4 findings, 2 已修复, 2 INFO 接受)
---

## Scope

审计范围覆盖事件管道接通计划全部 3 Phase 的代码变更:

| Phase | 文件 | LOC |
|-------|------|-----|
| P1 Runner 事件发射 | `agents/runner/attempt_runner.go` | ~50 |
| P2 WebSocket 桥接 | `gateway/server_methods_chat.go`, `gateway/server_methods_agent_rpc.go` | ~30 |
| P3 编译验证 | (无代码变更) | 0 |

**新建文件**: 0
**修改文件**: 3
**总 LOC**: ~80

---

## Findings

### F-01 [MEDIUM] `truncateForEvent` 按字节截断可能切断多字节 UTF-8 — **已修复**

**位置**: `agents/runner/attempt_runner.go` `truncateForEvent()`
**风险**: `s[:maxLen]` 按字节截断。若 2048 字节边界恰好落在多字节 UTF-8 字符中间（如中文 3 字节），会产生无效 UTF-8 字符串。JSON 序列化可能失败或前端收到乱码。
**与 L0/L1/L2 审计 F-01 相同模式**: 该审计已确认此类截断在中文场景下必现。
**修复**: 改用 rune-aware 截断 — 逐 rune 累计字节数，在超过 maxLen 前停止，保证输出始终是有效 UTF-8。

### F-02 [LOW] 审批重试路径缺少 tool 事件发射 — **已修复**

**位置**: `agents/runner/attempt_runner.go` L350-386 (审批通过后重试工具执行)
**风险**: 当全部工具因权限被拒绝后等待审批、审批通过后重新执行工具时，重试路径未发射 `tool.start` 和 `tool.result` 事件。
**影响**: 前端 tool 卡片停留在 "Permission denied" 状态，不会更新为实际执行结果。
**修复**: 在重试路径的工具执行前后添加相同的 `EmitAgentEvent` 调用。前端 `app-tool-stream.ts` 对相同 `toolCallId` 的重复事件会覆盖更新已有 entry，因此重试后卡片会正确刷新。

### F-03 [INFO] `lifecycle end` 事件在提前退出路径中缺失 — **接受**

**位置**: `agents/runner/attempt_runner.go` L131 (API key 错误), L206 (超时), L215 (LLM 错误)
**风险**: 这些 `return` 路径绕过 L420 的 `lifecycle end` 发射。前端可能观察到 `lifecycle start` 无配对的 `lifecycle end`。
**评估**: 可接受。这些路径在工具循环之前退出，不会有任何 tool 事件。前端仅在 `stream === "tool"` 时处理事件 (`app-tool-stream.ts:218`)，lifecycle 事件用于未来扩展。`ClearAgentRunContext` 通过 `defer` 正确清理。
**决定**: 接受现状。未来扩展 lifecycle 功能时再考虑 deferred emission。

### F-04 [INFO] `OnAgentEvent` 索引取消订阅可能失效 (预存问题) — **接受**

**位置**: `infra/agent_events.go:142-153` (预存代码，非本次变更)
**风险**: 取消订阅函数捕获注册时的 `idx`。若其他监听器先取消（导致 slice 移位），`idx` 可能指向错误的监听器或越界。
**评估**: 可接受。本次使用模式 (goroutine 开始订阅 + defer 取消) 在请求隔离的场景下风险极低。`DropIfSlow` + `runId` 过滤提供了额外安全网 — 即使事件发到错误的监听器，`runId` 不匹配会被过滤丢弃。
**决定**: 接受现状。此为 `infra/agent_events.go` 的预存设计问题，超出本次审计修复范围。

---

## 前端兼容性验证

| 后端发射字段 | 前端读取位置 | 匹配 |
|------------|------------|------|
| `AgentEventPayload.Stream = "tool"` | `app-tool-stream.ts:218 payload.stream === "tool"` | ✅ |
| `Data.toolCallId` | `app-tool-stream.ts:237 data.toolCallId` | ✅ |
| `Data.name` | `app-tool-stream.ts:241 data.name` | ✅ |
| `Data.phase` ("start"/"result") | `app-tool-stream.ts:242 data.phase` | ✅ |
| `Data.args` (start phase) | `app-tool-stream.ts:243 phase === "start" ? data.args` | ✅ |
| `Data.result` (result phase) | `app-tool-stream.ts:248 data.result` | ✅ |
| `Data.isError` | `app-tool-stream.ts` (未直接读取, 可扩展) | ✅ |
| `AgentEventPayload.RunID` | `app-tool-stream.ts:229 payload.runId !== host.chatRunId` | ✅ |
| `AgentEventPayload.SessionKey` | `app-tool-stream.ts:222 payload.sessionKey !== host.sessionKey` | ✅ |
| Broadcast event name `"agent"` | `app-gateway.ts:204 evt.event === "agent"` | ✅ |

## 安全性检查

- [x] 无路径遍历风险 (事件只含工具名和参数，不含文件系统操作)
- [x] 无敏感信息泄露 (`truncateForEvent` 截断大输出，args 为 LLM 生成的 JSON)
- [x] `DropIfSlow: true` 防止慢 WebSocket 客户端阻塞事件总线
- [x] `runId` 过滤防止跨会话事件泄露
- [x] `defer ClearAgentRunContext` 防止全局注册表泄漏

## 资源安全检查

- [x] `defer infra.ClearAgentRunContext(params.RunID)` — 所有退出路径清理
- [x] `defer unsubAgentEvents()` — goroutine 退出时取消订阅
- [x] `EmitAgentEvent` 内部 `recover()` 防止监听器 panic 传播
- [x] 无 goroutine 泄漏 (事件发射是同步调用，无新 goroutine)

## 正确性检查

- [x] `json.Unmarshal(tc.Input, &toolArgs)` 的 `_` 忽略错误 — 可接受: best-effort 解析，失败时 `toolArgs` 为空 map，前端渲染为无参数卡片
- [x] `truncateForEvent` rune-aware 截断 — 保证有效 UTF-8
- [x] 重试路径事件发射 — 使用相同 `toolCallId`，前端覆盖更新
- [x] `EmitAgentEvent` 空 sessionKey 自动从 `RegisterAgentRunContext` 补全

---

## Summary

| 级别 | 总数 | 已修复 | 接受 |
|------|------|--------|------|
| CRITICAL | 0 | - | - |
| HIGH | 0 | - | - |
| MEDIUM | 1 | 1 (F-01) | - |
| LOW | 1 | 1 (F-02) | - |
| INFO | 2 | - | 2 (F-03, F-04) |

## Verdict

**PASS** — 4 findings, 2 已修复, 2 INFO 接受

- 0 CRITICAL / 0 HIGH / 0 未修复
- 1 MEDIUM (UTF-8 截断) 已修复
- 1 LOW (重试路径缺事件) 已修复
- 2 INFO (lifecycle 缺失 + 预存索引问题) 接受
- `CGO_ENABLED=0 go build ./...` 编译通过
- 前端 payload 结构 100% 兼容 (10/10 字段匹配)
- 安全/资源/正确性检查全部通过
