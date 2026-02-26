# Coder 工具实时可视化 — 事件管道接通

---
document_type: Tracking
status: Audited
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-coder-tool-event-pipeline.md
skill5_verified: true
---

## Context

**问题**: oa-coder 编程子智能体的可视化卡片（diff 预览、bash 命令、文件写入）在前端不显示。用户只看到纯文本结果。

**根因审计结论**:

| 层 | 状态 | 问题 |
|----|------|------|
| 前端渲染 | ✅ | `coder-tool-cards.ts` + `app-tool-stream.ts` + `tool-display.json` 完整可用 |
| WebSocket 广播 | ✅ | `Broadcaster` 基础设施正常 |
| 全局事件总线 | ✅ | `infra/agent_events.go` EmitAgentEvent/OnAgentEvent 正常 |
| Runner → 事件总线 | 🔴 | `attempt_runner.go` 工具循环不发射事件 |
| WebSocket → 事件总线 | 🔴 | `server_methods_chat.go` / `server_methods_agent_rpc.go` 不订阅事件总线 |

**断裂 1 — Runner 不发射事件**:
- `attempt_runner.go:242-292` 工具循环直接执行 `ExecuteToolCall()` 但不调用 `infra.EmitAgentEvent()`
- `infra.RegisterAgentRunContext()` 从未在生产代码中调用（只有测试）
- `SubscribeContext` 基础设施已建但 `HandleEvent()` 从未在生产代码调用

**断裂 2 — WebSocket 路径不订阅**:
- `server_methods_chat.go:275` goroutine 不订阅 `infra.OnAgentEvent()`
- `server_methods_agent_rpc.go:206` goroutine 同上
- 对比: `openai_http.go:225` **已正确订阅**全局事件总线 (SSE 路径可用)

**前端数据流** (已验证可用):
```
WebSocket event: {type:"event", event:"agent", payload: AgentEventPayload}
  → app-gateway.ts:204 evt.event==="agent" → handleAgentEvent()
  → app-tool-stream.ts:207 payload.stream==="tool" → 按 phase 处理
  → app-tool-stream.ts:96 buildToolStreamMessage() → {type:"toolcall", name, arguments}
  → tool-cards.ts:55 isCoderTool() → renderCoderCard()
  → coder-tool-cards.ts:28 switch(tool) → renderEditCard/WriteCard/BashCard
```

## Online Verification Log

### OpenAI Assistants/Responses API — 3-Phase Tool Streaming
- **Query**: "OpenAI Assistants API run.step.created run.step.delta tool_calls streaming"
- **Source**: https://platform.openai.com/docs/api-reference/assistants-streaming
- **Key finding**: OpenAI 用 `run.step.created` → `run.step.delta` → `run.step.completed` 三阶段。Delta 含 `tool_calls[].id` + 增量 args。我们 `phase: "start"/"update"/"result"` 与此对齐。
- **Verified date**: 2026-02-26

### OpenAI Agents SDK — RunItemStreamEvent
- **Source**: https://openai.github.io/openai-agents-python/streaming/
- **Key finding**: 高级事件层 `tool_call_item` / `tool_call_output_item` 携带完整工具信息，适合 UI 渲染。
- **Verified date**: 2026-02-26

### OpenAI Responses API — Function Call Streaming
- **Source**: https://community.openai.com/t/responses-api-streaming-the-simple-guide-to-events/1363122
- **Key finding**: `response.function_call_arguments.done` 携带完整 `name` + `arguments`。start 事件含完整 args 供 UI 预览。
- **Verified date**: 2026-02-26

### Vercel AI SDK — onToolCall + streamText
- **Source**: https://ai-sdk.dev/docs/reference/ai-sdk-core/stream-text
- **Key finding**: `onToolCall` (执行前, 含 toolName/toolCallId/input) + `onStepFinish` (执行后)。回调注入 + 生命周期钩子。
- **Verified date**: 2026-02-26

### LangGraph — astream_events
- **Source**: https://docs.langchain.com/oss/python/langgraph/streaming
- **Key finding**: `on_tool_start` / `on_tool_end` 细粒度事件。全局事件总线 + 监听器过滤模式。
- **Verified date**: 2026-02-26

### 设计选择

采用 **完整 args** (非增量 delta)，3-phase 生命周期，全局事件总线 + 监听器过滤。

---

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│  EmbeddedAttemptRunner.RunAttempt()  (attempt_runner.go)    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Tool Loop (L242-292):                               │    │
│  │   ① infra.EmitAgentEvent("tool", {phase:"start"})  │    │
│  │   ② ExecuteToolCall(...)                            │    │
│  │   ③ infra.EmitAgentEvent("tool", {phase:"result"}) │    │
│  └─────────────────────────────────────────────────────┘    │
└───────────────────────────┬─────────────────────────────────┘
                            │ infra.EmitAgentEvent()
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  infra/agent_events.go — 全局事件总线 (已有 ✅)               │
└───────────────────────────┬─────────────────────────────────┘
                ┌───────────┴───────────┐
                ▼                       ▼
┌─────────────────────┐  ┌──────────────────────────┐
│  openai_http.go     │  │  server_methods_chat.go   │
│  (SSE, 已有 ✅)      │  │  (WebSocket, 本次新增)     │
└─────────────────────┘  └────────────┬─────────────┘
                                      │ Broadcast("agent", payload)
                                      ▼
┌─────────────────────────────────────────────────────────────┐
│  Frontend (已有 ✅)                                          │
│  app-gateway.ts → handleAgentEvent() → app-tool-stream.ts  │
│  → tool-cards.ts → coder-tool-cards.ts (diff/bash/write)   │
└─────────────────────────────────────────────────────────────┘
```

---

## 实施计划

### Phase 1: Runner 工具事件发射 (~50 LOC) — [x]

**文件**: `backend/internal/agents/runner/attempt_runner.go`

在 `RunAttempt()` 中:

1. 函数开始 (~L116): 注册运行上下文 + 发射 lifecycle start
```go
infra.RegisterAgentRunContext(params.RunID, infra.AgentRunContext{
    SessionKey: params.SessionKey,
})
defer infra.ClearAgentRunContext(params.RunID)
infra.EmitAgentEvent(params.RunID, infra.StreamLifecycle,
    map[string]interface{}{"phase": "start"}, "")
```

2. 工具执行前 (~L242): 发射 tool start (含完整 args 供前端卡片渲染)
```go
toolArgs := make(map[string]interface{})
_ = json.Unmarshal(tc.Input, &toolArgs)
infra.EmitAgentEvent(params.RunID, infra.StreamTool, map[string]interface{}{
    "phase": "start", "name": tc.Name, "toolCallId": tc.ID, "args": toolArgs,
}, "")
```

3. 工具执行后 (~L278): 发射 tool result
```go
resultData := map[string]interface{}{
    "phase": "result", "name": tc.Name, "toolCallId": tc.ID, "isError": toolErr != nil,
}
if output != "" {
    resultData["result"] = truncateForEvent(output, 2048)
}
infra.EmitAgentEvent(params.RunID, infra.StreamTool, resultData, "")
```

4. 工具循环结束: 发射 lifecycle end
```go
infra.EmitAgentEvent(params.RunID, infra.StreamLifecycle,
    map[string]interface{}{"phase": "end"}, "")
```

5. 新增辅助函数:
```go
func truncateForEvent(s string, maxLen int) string {
    if len(s) <= maxLen { return s }
    return s[:maxLen] + "…[truncated]"
}
```

**已确认**: `AttemptParams` 已有 `SessionKey` (L114) 和 `RunID` (L125)，`run.go:140` 已传入。
**已确认**: `attempt_runner.go` 已 import `infra` 包 (L24)。

### Phase 2: WebSocket 事件桥接 (~30 LOC) — [x]

**文件 A**: `backend/internal/gateway/server_methods_chat.go`

在 `go func()` 开始处 (~L275) 添加:
```go
if broadcaster != nil {
    unsubAgentEvents := infra.OnAgentEvent(func(evt infra.AgentEventPayload) {
        if evt.RunID != runId { return }
        broadcaster.Broadcast("agent", evt, &BroadcastOptions{DropIfSlow: true})
    })
    defer unsubAgentEvents()
}
```

**文件 B**: `backend/internal/gateway/server_methods_agent_rpc.go`

在 `go func()` 开始处 (~L206) 添加同样代码。

需要: +import `"github.com/anthropic/open-acosmi/internal/infra"`

### Phase 3: 编译验证 — [x]

```bash
CGO_ENABLED=0 go build ./...
```

---

## 关键文件索引

### 后端 (需修改)
| 文件 | 行号 | 用途 |
|------|------|------|
| `agents/runner/attempt_runner.go` | L107-310 | RunAttempt() 工具循环 — 添加事件发射 |
| `gateway/server_methods_chat.go` | L275 | chat.send goroutine — 添加事件订阅 |
| `gateway/server_methods_agent_rpc.go` | L206 | agent.send goroutine — 添加事件订阅 |

### 后端 (已有, 参考)
| 文件 | 用途 |
|------|------|
| `infra/agent_events.go` | 全局事件总线 (RegisterAgentRunContext/EmitAgentEvent/OnAgentEvent) |
| `gateway/openai_http.go:225` | SSE 路径事件订阅 (参考实现) |
| `gateway/chat.go` | AgentEventHandler (已建但未用, 本次不需要) |
| `gateway/broadcast.go` | Broadcaster.Broadcast() WebSocket 广播 |

### 前端 (无需修改)
| 文件 | 用途 |
|------|------|
| `ui/src/ui/app-gateway.ts:204-212` | 路由 "agent" 事件 → handleAgentEvent() |
| `ui/src/ui/app-tool-stream.ts:207-281` | 处理 tool stream 事件, 构建 tool messages |
| `ui/src/ui/chat/tool-cards.ts:12-51,53-57` | extractToolCards() + isCoderTool() 分支 |
| `ui/src/ui/chat/coder-tool-cards.ts` | renderCoderCard() (edit/write/bash) |
| `ui/src/ui/tool-display.json:33-62` | 6 个 coder 工具注册 |
| `ui/src/styles/chat/coder-cards.css` | diff 红绿样式 |

---

## 总量

| Phase | LOC | 文件 |
|-------|-----|------|
| P1 Runner 事件发射 | ~50 | attempt_runner.go |
| P2 WebSocket 桥接 | ~30 | server_methods_chat.go, server_methods_agent_rpc.go |
| P3 编译验证 | ~0 | - |
| **总计** | **~80** | **3 文件修改, 0 新建** |

## 验证

1. `CGO_ENABLED=0 go build ./...` 编译通过
2. 前端发送消息触发 coder 工具 → 显示 diff/bash/write 卡片
3. DevTools WebSocket → 收到 `event:"agent"` 帧
4. Debug tab Event Log 记录 agent 事件
5. 非 coder 工具也显示默认工具卡片
