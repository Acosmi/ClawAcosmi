---
document_type: Audit
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: self
skill5_verified: true
---

# 审计报告: 飞书远程聊天持久化回归 Bug

## 1. 范围

| 项目 | 内容 |
|------|------|
| **审计目标** | 飞书（及钉钉/企微）远程频道聊天消息持久化失败 |
| **审计类型** | 回归 Bug 分析 + 解决方案验证 |
| **现象** | 电脑网页端对话持久化正常；飞书远程聊天刷新页面后消息全部丢失 |
| **发现时间** | 2026-02-25（今日修复若干问题后发现） |
| **严重等级** | **HIGH** — 数据丢失、用户可见 |

---

## 2. 根因分析

### 2.1 消息持久化架构总览

系统采用 **后端 JSONL Transcript + SessionStore** 双层持久化：

```
用户发消息 → Backend 处理
  ├── 1. SessionStore.Save()     → sessions.json (session 注册)
  ├── 2. AppendUserTranscript()  → {sessionId}.jsonl (用户消息)
  ├── 3. Pipeline dispatch       → AI 生成回复
  ├── 4. AppendAssistantTranscript() → {sessionId}.jsonl (AI 回复)
  └── 5. WebSocket Broadcast     → 前端实时显示
```

前端刷新时通过 `chat.history` RPC 从 JSONL transcript 文件加载历史消息。

### 2.2 Web Chat 路径（正常 ✅）

文件: `backend/internal/gateway/server_methods_chat.go:173-412`

`handleChatSend` 完整实现了全部 5 个步骤：

| 步骤 | 代码位置 | 说明 |
|------|----------|------|
| SessionEntry 创建 | L282-297 | `store.LoadSessionEntry()` → `store.Save(entry)` |
| 用户消息持久化 | L303-308 | `AppendUserTranscriptMessage(...)` |
| Pipeline dispatch | L327-335 | `DispatchInboundMessage(...)` |
| AI 回复持久化 | L373-378 | `AppendAssistantTranscriptMessage(...)` |
| WebSocket 广播 | L397-404 | `broadcaster.Broadcast("chat", ...)` |

### 2.3 飞书路径（缺失 ❌）

文件: `backend/internal/gateway/server.go:471-522`

飞书 DispatchFunc **仅实现了步骤 3 和 5**，缺失步骤 1、2、4：

```go
// server.go:471-522
feishuPlugin.DispatchFunc = func(ctx context.Context, channel, accountID, chatID, userID, text string) string {
    sessionKey := fmt.Sprintf("feishu:%s", chatID)  // L472
    msgCtx := &autoreply.MsgContext{...}             // L473-480

    // ❌ 缺失: SessionStore.Save() — session 未注册到 sessions.json
    // ❌ 缺失: AppendUserTranscriptMessage() — 用户消息未写入 JSONL

    bc.Broadcast("chat.message", ...)                // L485-493 (仅 WebSocket 广播)

    result := DispatchInboundMessage(ctx, ...)       // L496-500 (Pipeline OK)

    // ❌ 缺失: AppendAssistantTranscriptMessage() — AI 回复未写入 JSONL

    bc.Broadcast("chat.message", ...)                // L511-520 (仅 WebSocket 广播)
    return reply
}
```

### 2.4 DispatchInboundMessage 不负责持久化

文件: `backend/internal/gateway/dispatch_inbound.go:57-104`

经审计确认，`DispatchInboundMessage()` 纯粹做管线路由（构建 `GetReplyOptions` → 调用 `PipelineDispatcher`），**不做任何 transcript 写入或 session 存储**。持久化完全由调用方负责。

同样，`autoreply/reply/` 包内也无 transcript 写入逻辑（仅有 `inbound_context.go:82` 的模板字段）。

### 2.5 前端侧影响

文件: `ui/src/ui/app-gateway.ts:264-287`

```typescript
// Bug C fix: 远程频道（飞书/钉钉/企微）聊天消息
if (evt.event === "chat.message") {
    // ...
    app.chatMessages = [...app.chatMessages, msg];  // L281: 纯内存追加
}
```

- 通过 WebSocket 收到 `chat.message` 事件后追加到内存数组
- **刷新页面 → 内存清空 → `chat.history` RPC 读 JSONL → 文件不存在 → 空列表**
- **额外问题**: 不检查 `payload.sessionKey` 是否匹配当前 session，所有飞书消息混入当前 chat

### 2.6 影响范围

| 渠道 | SessionStore 注册 | Transcript 写入 | WebSocket 广播 | 刷新后持久化 |
|------|-------------------|-----------------|---------------|-------------|
| Web Chat | ✅ `Save()` | ✅ JSONL | ✅ `chat` | ✅ |
| Discord | ✅ `RecordSessionMeta()` | ❌ 缺失 | ✅ `chat.message` | ❌ |
| Feishu | ❌ 缺失 | ❌ 缺失 | ✅ `chat.message` | ❌ |
| DingTalk | ❌ 缺失 | ❌ 缺失 | ✅ `chat.message` | ❌ |
| WeCom | ❌ 缺失 | ❌ 缺失 | ✅ `chat.message` | ❌ |

**注意**: Discord 通过 `channel_deps_adapter.go:104` 注册了 session metadata，但也没写 transcript。所有远程频道都存在持久化缺失。

### 2.7 回归触发点

- `server.go` 最后修改: **15:22 今天**
- `app-gateway.ts:264` 标注 "Bug C fix" 说明是近期新增
- 最可能场景：今天修复问题时重写了飞书 DispatchFunc，新版本遗漏持久化步骤

---

## 3. Online Verification Log (Skill 5)

### 3.1 JSONL Append-Only 持久化模式

- **Query**: `JSONL append-only log message persistence Go concurrent file write safety`
- **Source**: [Switch to JSONL for chat session storage — google-gemini/gemini-cli #15292](https://github.com/google-gemini/gemini-cli/issues/15292)
- **Key finding**: Gemini CLI 从单一 JSON 文件迁移到 JSONL 流格式，采用 append-only 写入模式。每条消息独立一行追加，避免全文件重写。这与本项目 `transcript.go` 的 `O_APPEND` 写入模式完全一致。JSONL 格式在进程崩溃时最多丢失最后一条未完成写入，不会损坏整个文件。
- **Verified date**: 2026-02-25

### 3.2 Go 并发文件写入安全性

- **Query**: `Go os.OpenFile O_APPEND concurrent goroutine write safe atomic POSIX guarantee`
- **Source**: [os: atomicity guarantees of os.File.Write — golang/go #49877](https://github.com/golang/go/issues/49877)
- **Source**: [Concurrency safe file access in Go](https://echorand.me/posts/go-file-mutex/)
- **Key finding**: Go 官方文档声明 "The methods of File are safe for concurrent use"，但未明确保证 `Write` 的原子性。POSIX `O_APPEND` 在 OS 层提供原子追加保证，但 Go 层面建议额外使用 `sync.Mutex` 保护。本项目 `transcript.go:195` 使用 `O_APPEND|O_WRONLY` 但无进程内锁；而 `SessionStore` 使用 `sync.RWMutex`（sessions.go:43）。解决方案应保持一致：飞书路径的 transcript 写入复用已有的 `AppendUserTranscriptMessage` / `AppendAssistantTranscriptMessage`，这些函数已遵循项目既有的写入模式。
- **Verified date**: 2026-02-25

### 3.3 Gorilla WebSocket 多频道 Session 管理

- **Query**: `gorilla websocket golang session store persistence pattern multi-channel chat`
- **Source**: [gorilla/websocket — Go Packages](https://pkg.go.dev/github.com/gorilla/websocket)
- **Source**: [Extending the chat example to have multiple rooms — gorilla/websocket #343](https://github.com/gorilla/websocket/issues/343)
- **Key finding**: Gorilla WebSocket 官方 chat example 采用 Hub 模式管理多客户端广播。对于多频道/多房间支持，推荐使用 `map[string]*Room` 结构按 channel/session 路由消息。本项目 `Broadcaster` 已实现此模式。关键是：**广播（实时展示）和持久化（刷新恢复）必须是两个独立步骤**，不能仅靠广播代替持久化。
- **Verified date**: 2026-02-25

### 3.4 飞书 Webhook 无内置持久化

- **Query**: `Feishu Lark bot message webhook session persistence chat history store`
- **Source**: [go-lark/lark — GitHub](https://github.com/go-lark/lark)
- **Source**: [Message FAQ — Feishu Open Platform](https://open.feishu.cn/document/server-docs/im-v1/faq)
- **Key finding**: 飞书 Webhook 机制是无状态事件传递（stateless event delivery）。SDK 只负责消息收发，不提供内置的 session 持久化或聊天历史存储。持久化必须由应用层自行实现——这正是本项目 `SessionStore` + JSONL transcript 的职责。
- **Verified date**: 2026-02-25

### 3.5 Go sync.Mutex 文件持久化 Session Store 模式

- **Query**: `sync.Mutex file append Go transcript session persistence pattern production`
- **Source**: [How to Use Mutex in Go: Patterns and Best Practices](https://oneuptime.com/blog/post/2026-01-23-go-mutex/view)
- **Source**: [Concurrency safe file access in Go](https://echorand.me/posts/go-file-mutex/)
- **Key finding**: 生产环境推荐的 session store 模式：`sync.Mutex` 保护内存 map + 原子文件写入（tmp+rename）。本项目 `SessionStore`（sessions.go:42-54）已正确实现此模式（`sync.RWMutex` + `saveToDisk` 使用 tmp+rename）。解决方案可直接复用 `SessionStore.Save()` 和 `SessionStore.RecordSessionMeta()`。
- **Verified date**: 2026-02-25

---

## 4. 解决方案

### 4.1 后端修复: 飞书 DispatchFunc 补充持久化 (HIGH)

**文件**: `backend/internal/gateway/server.go:471-522`

**修复逻辑**: 在现有 DispatchFunc 中补充 3 个持久化步骤，对齐 `handleChatSend` 的实现模式。

**伪代码** (对照 `server_methods_chat.go:278-308` 和 `L360-394`):

```go
feishuPlugin.DispatchFunc = func(ctx context.Context, channel, accountID, chatID, userID, text string) string {
    sessionKey := fmt.Sprintf("feishu:%s", chatID)

    // ===== 步骤 1: 确保 session 注册到 SessionStore =====
    // (参考 server_methods_chat.go:282-297)
    store := state.SessionStore()
    var resolvedSessionId string
    if store != nil {
        entry := store.LoadSessionEntry(sessionKey)
        if entry == nil {
            newId := fmt.Sprintf("session_%d", time.Now().UnixNano())
            entry = &SessionEntry{
                SessionKey: sessionKey,
                SessionId:  newId,
                Label:      fmt.Sprintf("飞书:%s", chatID),
                Channel:    "feishu",
                ChatID:     chatID,
            }
            store.Save(entry)
        }
        resolvedSessionId = entry.SessionId

        // 同时记录 session metadata
        store.RecordSessionMeta(sessionKey, InboundMeta{
            Channel:     "feishu",
            DisplayName: userID,
        })
    }

    // ===== 步骤 2: 持久化用户消息到 transcript =====
    // (参考 server_methods_chat.go:303-308)
    storePath := /* 从 config 获取 store path */
    if resolvedSessionId != "" {
        AppendUserTranscriptMessage(AppendTranscriptParams{
            Message:         text,
            SessionID:       resolvedSessionId,
            StorePath:       storePath,
            CreateIfMissing: true,
        })
    }

    // (广播 + Pipeline dispatch — 保持现有代码不变)
    // ...

    // ===== 步骤 4: 持久化 AI 回复到 transcript =====
    // (参考 server_methods_chat.go:373-378)
    if resolvedSessionId != "" && reply != "" {
        AppendAssistantTranscriptMessage(AppendTranscriptParams{
            Message:         reply,
            SessionID:       resolvedSessionId,
            StorePath:       storePath,
            CreateIfMissing: true,
        })
    }

    return reply
}
```

**技术依据**:
- 复用项目已有的 `SessionStore.Save()` (sessions.go:207-215) 和 `AppendUserTranscriptMessage` / `AppendAssistantTranscriptMessage` (transcript.go:220-267, 133-206)
- `SessionStore` 使用 `sync.RWMutex` 保证并发安全 (sessions.go:43)
- transcript 写入使用 `O_APPEND|O_WRONLY` 模式 (transcript.go:195, 256)
- JSONL append-only 模式已被 Gemini CLI 等项目验证为 chat session 存储的行业最佳实践 ([gemini-cli #15292](https://github.com/google-gemini/gemini-cli/issues/15292))

### 4.2 后端修复: 钉钉/企微 DispatchFunc 同步补充 (HIGH)

**文件**: `backend/internal/gateway/server.go` (钉钉/企微的 DispatchFunc 区域)

与飞书相同的 3 步持久化缺失，需同步修复。检查 DingTalk 和 WeCom 插件的 DispatchFunc 并补充：
1. `SessionStore.Save()` — session 注册
2. `AppendUserTranscriptMessage()` — 用户消息
3. `AppendAssistantTranscriptMessage()` — AI 回复

### 4.3 后端修复: Discord transcript 补充 (MEDIUM)

**文件**: `backend/internal/gateway/channel_deps_adapter.go:74-98`

Discord 的 `buildDiscordDispatcher` 已有 `RecordSessionMeta`（L104），但也缺少 transcript 写入。需在 dispatch 前后补充 `AppendUserTranscriptMessage` 和 `AppendAssistantTranscriptMessage`。

同理 Telegram (L184) 和 Slack (L323) 的 dispatcher 也需要补充。

### 4.4 前端修复: chat.message sessionKey 过滤 (MEDIUM)

**文件**: `ui/src/ui/app-gateway.ts:264-287`

当前 `chat.message` 事件处理器不检查 sessionKey，所有远程频道消息不分 session 全部追加到当前 chat：

```typescript
// 当前代码 (有问题)
if (evt.event === "chat.message") {
    // ... 无 sessionKey 检查
    app.chatMessages = [...app.chatMessages, msg];  // 所有消息混入
}
```

**修复方向**:

```typescript
// 修复后
if (evt.event === "chat.message") {
    const payload = evt.payload as {...};
    if (payload?.text && payload?.sessionKey === app.sessionKey) {
        // 只显示当前 session 的消息
        const msg = {...};
        app.chatMessages = [...app.chatMessages, msg];
    }
}
```

或者更好的方案：在收到飞书消息时自动切换到对应 session 并触发 `loadChatHistory()` 重新加载。

### 4.5 可选优化: 提取通用 DispatchFunc 工厂 (LOW)

将 5 个步骤（session 注册 → 用户消息持久化 → pipeline dispatch → AI 回复持久化 → 广播）抽象为通用函数，供所有频道复用，避免重复代码和遗漏：

```go
func channelDispatchWithPersistence(params ChannelDispatchParams) string {
    // 1. SessionStore.Save()
    // 2. AppendUserTranscriptMessage()
    // 3. DispatchInboundMessage()
    // 4. AppendAssistantTranscriptMessage()
    // 5. Broadcast("chat.message", ...)
}
```

参考 Gorilla WebSocket 多房间设计模式 ([gorilla/websocket #343](https://github.com/gorilla/websocket/issues/343))：将频道路由与持久化解耦，通过统一 dispatcher 保证一致性。

---

## 5. 修复优先级

| # | 修复项 | 严重等级 | 文件 | 说明 |
|---|--------|---------|------|------|
| F-01 | 飞书 DispatchFunc 补充 3 步持久化 | **HIGH** | server.go:471-522 | 根因修复 |
| F-02 | 钉钉/企微 DispatchFunc 同步补充 | **HIGH** | server.go (DingTalk/WeCom 区域) | 同类缺陷 |
| F-03 | Discord dispatcher 补充 transcript | **MEDIUM** | channel_deps_adapter.go:74-98 | 同类缺陷 |
| F-04 | Telegram/Slack dispatcher 补充 transcript | **MEDIUM** | channel_deps_adapter.go:184,323 | 同类缺陷 |
| F-05 | 前端 chat.message sessionKey 过滤 | **MEDIUM** | app-gateway.ts:264-287 | 消息串台 |
| F-06 | 提取通用 channelDispatchWithPersistence | **LOW** | 新文件或 dispatch_inbound.go | 防止未来遗漏 |

---

## 6. 验证方法

### 6.1 手动验证

1. 启动网关，配置飞书频道
2. 通过飞书发送消息，确认后端日志出现 `session_store: saved` 和 `transcript: appended`
3. 检查 `~/.openacosmi/store/sessions.json` 包含 `feishu:*` 条目
4. 检查 `~/.openacosmi/store/{sessionId}.jsonl` 包含用户消息和 AI 回复
5. 刷新网页前端，确认聊天历史正确加载
6. 对钉钉/企微重复以上步骤

### 6.2 自动化测试

- 单元测试: mock SessionStore + transcript 函数，验证飞书 DispatchFunc 调用顺序
- 集成测试: 发送消息 → 检查 JSONL 文件内容 → 调用 `chat.history` RPC → 验证返回消息

---

## 7. 参考源

- [Switch to JSONL for chat session storage — google-gemini/gemini-cli #15292](https://github.com/google-gemini/gemini-cli/issues/15292) — JSONL append-only session 存储行业实践
- [os: atomicity guarantees of os.File.Write — golang/go #49877](https://github.com/golang/go/issues/49877) — Go 并发文件写入原子性讨论
- [Concurrency safe file access in Go](https://echorand.me/posts/go-file-mutex/) — sync.Mutex 文件持久化模式
- [gorilla/websocket](https://pkg.go.dev/github.com/gorilla/websocket) — WebSocket 多频道架构参考
- [gorilla/websocket #343 — Multiple rooms](https://github.com/gorilla/websocket/issues/343) — 多房间消息路由设计
- [go-lark/lark](https://github.com/go-lark/lark) — 飞书 SDK（无内置持久化，需应用层实现）
- [Message FAQ — Feishu Open Platform](https://open.feishu.cn/document/server-docs/im-v1/faq) — 飞书消息 API 官方文档
- [How to Use Mutex in Go: Patterns and Best Practices](https://oneuptime.com/blog/post/2026-01-23-go-mutex/view) — Go Mutex 生产环境最佳实践
- [JSON Lines](https://jsonlines.org/) — JSONL 格式官方规范

---

## 8. 判定

**FAIL** — 飞书（及钉钉/企微/Discord）远程频道聊天消息未写入持久化存储，刷新后丢失。需按 F-01 ~ F-06 修复后重新审计。
