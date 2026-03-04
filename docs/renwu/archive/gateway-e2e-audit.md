# 网关 E2E 验证审计报告

**日期**: 2026-02-17
**前置修复**: `gateway-fix-task.md`（Batch A-D 已全部完成）
**验证方式**: 启动后端 + 前端，浏览器访问 `http://localhost:5173/`

---

## 已修复回顾（本轮验证前）

| Batch | 修复内容 | 文件 |
|-------|---------|------|
| A | WS 帧类型 `connect` → `hello-ok` 握手 | `ui/src/ui/gateway.ts` |
| B | HTTP 路由扁平化 + 回调填充 | `backend/internal/gateway/server.go` |
| C | 移除 `sessions.preview` stub + Vite WS 代理 | `server_methods_stubs.go`, `vite.config.ts`, `storage.ts` |
| D | 根路径 `/` 处理器 | `server.go` |
| 追加 | WS URL 自动追加 `/ws` 路径 | `gateway.ts:144` |

---

## 新发现问题

### Issue E2E-1: Token 未自动传递 (P1)

**现象**: 访问 `http://localhost:5173/` → `disconnected (4008): token_missing`

**根因分析**:

```
后端 auth.go:91-106  →  ResolveGatewayAuth() 自动生成 token（AuthModeToken）
后端启动日志打印:      🔑 Dashboard URL: http://localhost:18789/?token=xxxx
前端 storage.ts:29   →  settings.token 默认 ""（空字符串）
前端 gateway.ts:204  →  authToken = this.opts.token → undefined
前端 gateway.ts:215  →  auth = undefined（token 和 password 都 falsy）
后端 ws_server.go:138 →  connectParams.Auth == nil
后端 auth.go:311     →  reject "token_missing"
后端 ws_server.go:161 →  close(4008, "token_missing")
```

**临时验证**: 用 `http://localhost:5173/?token=xxxx` 访问 → 连接成功 ✅

**修复方向**:

| 方案 | 说明 | 文件 |
|------|------|------|
| A. URL token 透传 | `app-settings.ts` 已支持 `?token=` 参数解析(L95,130-133)，验证其是否正确保存到 localStorage | `app-settings.ts` |
| B. 本地免认证 | 后端检测 localhost 直连 + 无 token → 自动放行 | `auth.go` 中 `AuthorizeGatewayConnect` |
| C. 重定向到带 token 的 URL | 后端 `/` 处理器生成带 token 的重定向 URL | `server.go` |

**关键文件**:

- 后端 token 验证: `backend/internal/gateway/auth.go` L294-317 (`AuthorizeGatewayConnect`)
- 后端 WS 认证: `backend/internal/gateway/ws_server.go` L136-165
- 前端 token 来源: `ui/src/ui/app-gateway.ts` L127-128 → `ui/src/ui/storage.ts` L20-24
- 前端 URL 参数解析: `ui/src/ui/app-settings.ts` L95, L130-136
- 本地请求检测: `backend/internal/gateway/auth.go` L228-253 (`IsLocalDirectRequest`)

---

### Issue E2E-2: 聊天字段名不匹配 — `message` vs `text` (P0)

**现象**: 发送聊天消息后，后端日志显示 `text=""`（空字符串），AI 按空消息回复

**根因分析**:

```
前端 chat.ts:126-128:
  await state.client.request("chat.send", {
    message: msg,          // ← 字段名 "message"
  });

后端 server_methods_chat.go:174:
  text, _ := ctx.Params["text"].(string)   // ← 字段名 "text"
```

前端发送 `message` 字段，后端读取 `text` 字段 → 永远为空。

**后端日志证据**:

```
2026/02/17 15:25:21 INFO chat.send: dispatching text="" attachments=0
2026/02/17 15:25:21 INFO chat.send: complete replyLength=74
```

**修复方向**: 统一字段名。建议后端增加 `message` 字段兼容读取：

```go
// server_methods_chat.go:174
text, _ := ctx.Params["text"].(string)
if text == "" {
    text, _ = ctx.Params["message"].(string)  // 兼容前端
}
```

**关键文件**:

- 前端发送: `ui/src/ui/controllers/chat.ts` L126-128
- 后端接收: `backend/internal/gateway/server_methods_chat.go` L174

---

### Issue E2E-3: 用户消息不在聊天框显示 (P1)

**现象**: 发送消息后，聊天框中不显示用户自己发送的消息

**分析**:

前端 `chat.ts:92-99` 在发送前已将 user 消息追加到 `chatMessages`：

```typescript
state.chatMessages = [
  ...state.chatMessages,
  { role: "user", content: contentBlocks, timestamp: now },
];
```

理论上应该显示。可能原因：

1. WS 重连（见 E2E-4）导致组件重渲染，丢失内存中的 `chatMessages`
2. `chat.history` 在重连后重新加载（`app-gateway.ts:147` `refreshActiveTab`），覆盖了刚追加的本地消息
3. 由于 Issue E2E-2（空文本），用户消息虽然被追加到本地数组但可能在重连后被 history 请求覆盖

**关键文件**:

- 消息追加: `ui/src/ui/controllers/chat.ts` L92-99
- 重连后刷新: `ui/src/ui/app-gateway.ts` L132-147 (`onHello`)
- 聊天视图渲染: 需要检查 `ui/src/ui/views/` 下的 chat 视图文件

---

### Issue E2E-4: WS 连接频繁重连 (~15秒间隔) (P1)

**现象**: 后端日志显示大量 `ws: new connection`，约每 15 秒一次，大多数没有 `hello-ok sent`

**后端日志片段**:

```
15:22:30 INFO ws: new connection remote=127.0.0.1:58701
15:22:45 INFO ws: new connection remote=127.0.0.1:58712   ← 15秒
15:22:48 INFO ws: new connection remote=127.0.0.1:58722   ← 3秒
15:23:03 INFO ws: new connection remote=127.0.0.1:58733   ← 15秒
15:23:06 INFO ws: new connection remote=127.0.0.1:58742   ← 3秒
...
15:25:05 INFO ws: hello-ok sent connId=a08a0988  ← 少量成功
15:25:07 INFO ws: client disconnected connId=a08a0988  ← 2秒后断开
```

**分析**:

连接模式呈 15s+3s 交替，可能原因：

1. **Vite HMR 热重载**：Vite 的 HMR WS 连接与应用 WS 连接可能互相干扰
2. **代理心跳超时**：Vite WS proxy 默认心跳超时可能太短
3. **前端重连逻辑**：`gateway.ts:163-164` 退避从小值开始（`backoffMs * 1.7`，最大 15s），如果每次连接都失败则会以 ~15s 间隔重连
4. **Token 缺失导致循环**：大多数连接因 token_missing 被拒 → 断开 → 重连 → 再被拒（无限循环）

**关键文件**:

- 前端重连: `ui/src/ui/gateway.ts` L159-165 (`scheduleReconnect`)
- Vite 代理: `ui/vite.config.ts` proxy 配置
- 后端 WS 处理: `backend/internal/gateway/ws_server.go` L90-165

---

## 修复优先级建议

| 优先级 | Issue | 修复难度 | 依赖 |
|--------|-------|---------|------|
| **P0** | E2E-2: 字段名不匹配 | 低 (1行) | 无 |
| **P1** | E2E-1: Token 未自动传递 | 中 | 需要确定方案 |
| **P1** | E2E-4: WS 重连循环 | 中 | 依赖 E2E-1（token 修复后可能自愈） |
| **P1** | E2E-3: 用户消息不显示 | 中 | 依赖 E2E-4（WS 稳定后可能自愈） |

> [!IMPORTANT]
> E2E-4（WS 重连）很可能是 E2E-1（token_missing）的副作用。修复 token 传递后，连接应该稳定，E2E-3 和 E2E-4 可能自动解决。建议先修 E2E-2 和 E2E-1，然后重新验证 E2E-3 和 E2E-4。

---

## Bootstrap（新窗口使用）

```
请加载以下上下文：
1. docs/renwu/gateway-e2e-audit.md          ← 本审计报告
2. docs/renwu/gateway-fix-task.md            ← 上一轮修复记录
3. docs/renwu/gateway-fix-bootstrap.md       ← 项目 bootstrap
4. skills/acosmi-refactor/references/coding-standards.md  ← 编码规范

按 E2E-2 → E2E-1 → 验证 E2E-4/E2E-3 的顺序修复。
每个 Issue 修复后运行：
  - 后端: cd backend && go build ./... && go vet ./... && go test -race ./internal/gateway/...
  - 前端: cd ui && npx tsc --noEmit
```
