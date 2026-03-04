# ACP 隐藏依赖深度审计报告

> 最后更新：2026-02-14
> 审计范围：`src/acp/*.ts` (10 文件) ↔ `backend/internal/acp/*.go` (9 文件)

## 一、7 类隐藏依赖评估结果

| # | 类别 | 结果 | 说明 |
|---|------|------|------|
| 1 | npm 包黑盒行为 | ⚠️ | `@agentclientprotocol/sdk` 的 `ndJsonStream`、`ClientSideConnection`、`AgentSideConnection` — Go 端自建等价 ndJSON 实现 |
| 2 | 全局状态/单例 | ✅ | `defaultAcpSessionStore` → Go `DefaultSessionStore` 已实现 |
| 3 | 事件总线/回调链 | ⚠️ | TS Gateway `onEvent`/`onHelloOk`/`onClose` 回调 → Go 通过 `HandleGatewayEvent` 接口处理，但缺 reconnect/disconnect |
| 4 | 环境变量依赖 | ⚠️ | TS server.ts 读 `OPENACOSMI_GATEWAY_TOKEN`/`OPENACOSMI_GATEWAY_PASSWORD` + `process.env` → Go 端 defer to CLI 层 |
| 5 | 文件系统约定 | ✅ | 无硬编码路径 |
| 6 | 协议/消息格式 | ⚠️ | 3 处格式差异需修复（见下文 P0 清单） |
| 7 | 错误处理约定 | ⚠️ | chat.error → TS 用 `refusal` stopReason，Go 用 `end_turn` |

---

## 二、发现的缺陷和差异（按优先级排序）

### P0 — 功能逻辑缺失（必须立即修复）

| ID | 文件 | 问题 | TS 行为 | Go 差异 |
|----|------|------|---------|--------|
| P0-1 | `translator.go` | ❌ `prefixCwd` 逻辑完全缺失 | `message = prefixCwd ? "[Working directory: ${cwd}]\n\n${text}" : text` (L243-244) | Go 直接传 `promptText`，未前缀 CWD |
| P0-2 | `translator.go` | ❌ chat **delta** 事件解析逻辑不正确 | TS delta 使用增量切片：`fullText.slice(sentSoFar)` (L412-419) — Gateway 发送的是累积全文 | Go 假设 payload 直接包含 delta 文本 |
| P0-3 | `translator.go` | ❌ Gateway 事件路由结构不正确 | TS 使用 `evt.event === "chat"` + `payload.state`；`evt.event === "agent"` + `payload.stream` (L90-97) | Go 使用扁平事件名（`chat.delta`、`agent.tool_call.start`），与 Gateway EventFrame 格式不匹配 |
| P0-4 | `translator.go` | ❌ `idempotencyKey`/`runId` 匹配缺失 | TS 在 prompt 中生成 `randomUUID()` 作为 `idempotencyKey`，chat 事件用 `runId` 匹配 (L379) | Go 未传 `idempotencyKey` 到 chat.send，未在事件中校验 |
| P0-5 | `translator.go` | ❌ pending 查找按 sessionKey 而非 sessionId | TS 使用 `pendingPrompts.set(sessionId, ...)` 但 `findPendingBySessionKey` 遍历所有 pending (L436-443) | Go 直接按 sessionKey 存取 pending，语义正确但丢失了 `sessionId` → pending 的主索引 |
| P0-6 | `session_mapper.go` | ❌ `requireExistingSession` 验证缺失 | TS 在 sessionKey 和 default key 路径都检查 `requireExisting` 并调 `sessions.resolve` 验证 (L48-82) | Go 跳过 requireExisting，直接返回 key |
| P0-7 | `translator.go` | ❌ `sessions.patch` 参数错误 | TS 发送 `{ key, thinkingLevel: modeId }` (L215-218) | Go 发送 `{ key, mode: modeId }` |

### P1 — 功能缺失（应修复）

| ID | 文件 | 问题 | TS 行为 | Go 差异 |
|----|------|------|---------|--------|
| P1-1 | `translator.go` | `loadSession` 未重新解析 sessionKey | TS 重新调 `parseSessionMeta` + `resolveSessionKey` + `resetSessionIfNeeded` + `createSession`(L153-179) | Go 仅更新 CWD |
| P1-2 | `translator.go` | 缺 `handleGatewayReconnect`/`handleGatewayDisconnect` | TS 在 disconnect 时 reject 所有 pending (L82-88) | Go 无此方法 |
| P1-3 | `translator.go` | 缺 `authenticate` handler | TS 返回 `{}` (L202-204) | Go AcpServerHandler 接口无此方法 |
| P1-4 | `translator.go` | chat.error stopReason 不一致 | TS 使用 `StopReason.Refusal` (L397) | Go 使用 `StopReasonEndTurn` |
| P1-5 | `translator.go` | tool 结果用 `tool_call` 而非 `tool_call_update` | TS tool result 发 `sessionUpdate: "tool_call_update"` (L349-358) | Go 统一发 `tool_call` |
| P1-6 | `session.go` | 缺 `clearActiveRun` 方法 | TS `clearActiveRun` 清理 run 但不调 cancel (区别于 `cancelActiveRun`) | Go 只有 `CancelActiveRun` |
| P1-7 | `session_mapper.go` | `parseSessionMeta` 键名不完整 | TS: sessionKey → `["sessionKey", "session", "key"]`; sessionLabel → `["sessionLabel", "label"]`; requireExisting → `["requireExistingSession", "requireExisting"]`; resetSession → `["resetSession", "reset"]` | Go 使用不同的 alias 列表 |
| P1-8 | `translator.go` | 缺 toolCalls 去重 Set | TS 使用 `pending.toolCalls = new Set()` 防重复 (L325-331) | Go 无去重逻辑 |

### P2 — 次要差异（延迟处理可接受）

| ID | 文件 | 问题 |
|----|------|------|
| P2-1 | `session.go` | TS 使用 `randomUUID()` 生成 sessionId，Go 使用 `acp-session-N` 计数器 |
| P2-2 | `translator.go` | TS `newSession` 发 `sendAvailableCommands` 在 create 之后、return 之前；Go 把 commands 放在 prompt 中 |
| P2-3 | `types.go` | `AcpServerOptions` 缺少 `gatewayUrl`/`gatewayToken`/`gatewayPassword` 字段（server.ts L14-34 需要） |
| P2-4 | `translator.go` | TS `ListSessions` 使用 `req._meta.limit`；Go 硬编码 100 |
| P2-5 | `translator.go` | TS delta 追踪 `sentText` 完整文本用于调试 |

---

## 三、P0 修复计划

> 以下 P0 项目需立即修复

### P0-1: 添加 `prefixCwd` 逻辑

- 在 `Prompt()` 中解析 `prefixCwd` meta + opts 默认值
- 如果为 true（默认 true），前缀 `[Working directory: <cwd>]\n\n`

### P0-2 + P0-3: 修正 Gateway 事件路由

- `HandleGatewayEvent` 签名改为接收 `EventFrame` 类型
- chat 事件：`evt.event == "chat"` → 按 `payload.state` 分发
- agent 事件：`evt.event == "agent"` → 按 `payload.stream + payload.data.phase` 分发
- delta 处理改为增量切片模式（追踪 `sentTextLength`）

### P0-4: 添加 `idempotencyKey`

- 在 `pendingPrompt` 中存 `idempotencyKey`（UUID）
- chat.send 请求添加 `idempotencyKey` 参数
- chat 事件校验 `payload.runId == pending.idempotencyKey`

### P0-5: 修正 pending 查找

- `pendingPrompts` 改为按 `sessionId` 索引
- 添加 `findPendingBySessionKey` 遍历方法

### P0-6: 补全 `requireExistingSession`

- 在 `ResolveSessionKey` 中添加 `requireExisting` 分支
- 使用 `sessions.resolve` 验证 key 存在性

### P0-7: 修正 `setSessionMode`

- `sessions.patch` 参数从 `mode` 改为 `thinkingLevel`
