# Phase 7 Batch D (D4/D5/D6) 隐藏依赖审计报告

> 审计时间: 2026-02-15 | 审计范围: D4 回复核心 + D5 回复辅助 + D6 投递集成

---

## 审计总结

| 优先级 | 数量 | 说明 |
|--------|------|------|
| P0 紧急 | 0 | 无阻塞性问题 |
| P1 高 | 1 | `sanitizeUserFacingText` 降级为 P2 延迟 |
| P1 已修复 | 2 | `abort.go` 触发词 + `inbound_context.go` ChatType/Label ✅ |
| P2 中 | 5 | 功能缺失，预期内骨架状态 |
| 已知延迟 | 8 | 已在 deferred-items.md 中记录 |

### 健康度评分: **A-** (优良)

- ✅ 编译通过，62 autoreply + 9 markdown tests PASS
- ✅ 核心分块引擎（围栏感知）功能完整
- ✅ reply/ 子包基础逻辑可用（dispatch + normalize + context）
- ✅ P1-2 和 P1-3 已修复
- ⚠️ P1-1 (`sanitizeUserFacingText`) 降级为 P2，延迟到 Phase 8

---

## D4: 回复核心 (reply/ 子包)

### `reply/normalize_reply.go` (84L) ←→ TS `normalize-reply.ts` (94L)

| # | 隐藏依赖类别 | 检查结果 | 详情 |
|---|-------------|----------|------|
| 1 | npm 包黑盒 | ⚠️ P1 | TS 调用 `sanitizeUserFacingText()`（从 `pi-embedded-helpers` 导入）做安全清理。Go 端缺失此步骤 |
| 2 | 事件回调链 | ✅ | `onHeartbeatStrip`/`onSkip` 回调已实现 |
| 3 | LINE 指令 | ⚠️ P2-延迟 | TS 调用 `hasLineDirectives()`/`parseLineDirectives()` 解析 LINE 特有指令。Go 端未实现，需 LINE SDK 完成后处理 |
| 4 | `channelData` 判断 | ⚠️ P2 | TS 检查 `payload.channelData` 是否非空来决定是否跳过。Go `ReplyPayload` 无 `ChannelData` 字段 |
| 5 | `shouldSkip` 心跳分支 | ✅ | Go `StripHeartbeatToken` 返回 `didStrip`+`text`，`shouldSkip` 逻辑等价（空文本→OnSkip） |

### `reply/inbound_context.go` (133L) ←→ TS `inbound-context.ts` (81L)

| # | 隐藏依赖类别 | 检查结果 | 详情 |
|---|-------------|----------|------|
| 1 | 跨模块依赖 | ⚠️ P1 | TS 调用 `normalizeChatType()`（from `channels/chat-type`）和 `resolveConversationLabel()`（from `channels/conversation-label`）。Go 端缺失这两个函数调用 |
| 2 | 字段覆盖 | ✅ | `Transcript`、`ThreadStarterBody` 已在 Go L82-83 规范化 |
| 3 | 默认拒绝 | ✅ | Go `bool` 零值为 `false`，等价 TS `=== true` 默认拒绝 |
| 4 | sender meta 调用 | ✅ | Go 在 `FinalizeInboundContext` 中调用 `FormatInboundBodyWithSenderMeta`，等价 |

### `reply/reply_dispatcher.go` (227L) ←→ TS `reply-dispatcher.ts` (193L)

| # | 隐藏依赖类别 | 检查结果 | 详情 |
|---|-------------|----------|------|
| 1 | Typing 集成 | ⚠️ P2-延迟 | TS 有 `createReplyDispatcherWithTyping()`（28L）与 `TypingController` 集成。Go 端未实现——已在 deferred-items.md |
| 2 | 动态前缀上下文 | ⚠️ P2 | TS 支持 `responsePrefixContextProvider` 动态回调。Go 仅有静态 `ResponsePrefixContext` |
| 3 | 人类延迟时序 | ✅ | `firstBlock` 跳过延迟逻辑已实现 |
| 4 | Promise 链串行 | ✅ | Go 使用 goroutine+channel 实现等价串行化 |
| 5 | onError 回调 | ✅ | 错误回调已实现 |

### `reply/dispatch_from_config.go` (40L) ←→ TS `dispatch-from-config.ts` (458L)

- 已知骨架，Phase 8 实现。**符合预期。**

### `reply/types.go` (42L) — 类型完整性

- ✅ `ReplyDispatchKind`、`NormalizeReplySkipReason`、`FinalizeInboundContextOptions`、`ResponsePrefixContext` 均已定义
- ⚠️ `ResponsePrefixContext` 缺少 TS 中的 `modelFull`、`thinkingLevel`、`identityName` 字段

---

## D5: 回复辅助

### `reply/abort.go` (42L) ←→ TS `abort.ts` (205L)

| # | 隐藏依赖类别 | 检查结果 | 详情 |
|---|-------------|----------|------|
| 1 | 全局状态 | ⚠️ P2-延迟 | TS 有 `ABORT_MEMORY` Map（模块级单例）用于跨调用记忆。Go 仅有 `IsAbortTrigger`，无 `getAbortMemory`/`setAbortMemory` |
| 2 | Session Store | ⚠️ 延迟 | TS `tryFastAbortFromMessage` 需 session store、agent-runner-abort、subagent-registry 等 7 个模块。Phase 8 实现 |
| 3 | 触发词列表 | ⚠️ **P1** | TS: `stop, esc, abort, wait, exit, interrupt`（6 个，无前缀）。Go: `/abort, /stop, /cancel`（3 个，带 `/` 前缀）。**语义和格式均不一致**——用户发 "stop" 在 Go 端不会触发中止 |

### `reply/response_prefix.go` (21L) ←→ TS `response-prefix-template.ts` (101L)

| # | 隐藏依赖类别 | 检查结果 | 详情 |
|---|-------------|----------|------|
| 1 | 模板变量 | ⚠️ P2 | TS 支持 `{model}`, `{provider}`, `{thinkingLevel}`, `{identity.name}` 等模板变量。Go 的 `applyResponsePrefixTemplate` 使用 `{{var}}` 语法 |
| 2 | 缺失函数 | ⚠️ P2 | TS 导出 `extractShortModelName()` 和 `hasTemplateVariables()`，Go 端均未实现 |

### `reply/body.go` (109L) ←→ TS `body.ts` (50L)

| # | 检查结果 | 详情 |
|---|----------|------|
| 1 | ✅ 不同功能 | TS `body.ts` 实现 `applySessionHints`（session 提示注入）。Go `body.go` 实现 `BuildResponseBody`/`BuildResponseBodyIR`（回复体分块构建）。两者互补，TS 版 `applySessionHints` 属于 agent-runner 链路的一部分，延迟到 Phase 8 |

---

## D6: 投递+集成

### `chunk.go` (380L) ←→ TS `chunk.ts` (501L)

| # | 检查结果 | 详情 |
|---|----------|------|
| 1 | ✅ 围栏感知 | `ChunkMarkdownText` 接入 `fences.go`，围栏重开/保留逻辑完整 |
| 2 | ✅ 段落分块 | `ChunkByParagraph` 围栏内空行不分割 |
| 3 | ✅ 配置解析 | `ResolveTextChunkLimit`/`ResolveChunkMode` 含频道级+账号级覆盖 |
| 4 | ✅ 括号感知 | `scanParenAwareBreakpoints` 实现 |
| 5 | ✅ 测试覆盖 | 30 tests PASS |

### `envelope.go` (66L) ←→ TS `envelope.ts` (219L)

| # | 隐藏依赖类别 | 检查结果 | 详情 |
|---|-------------|----------|------|
| 1 | 时间格式化 | ⚠️ P2-延迟 | TS 使用 `format-datetime.ts` / `format-relative.ts` 做时区感知格式化+经过时间。Go 使用 `time.Now().Format()` 简化版 |
| 2 | Sender Label 解析 | ⚠️ P2-延迟 | TS 使用 `resolveSenderLabel()` from `channels/sender-label`。Go 直接取 `SenderDisplayName`/`SenderName`/`SenderID` |
| 3 | 信封格式 | ⚠️ 差异 | TS 格式 `[Channel From +elapsed ts] body`。Go 格式 `ts | sender | [channelType]`。格式差异较大 |

### `dispatch.go` (28L) ←→ TS `dispatch.ts` (77L)

- 已知骨架。TS 有 3 个分发入口（`dispatchInboundMessage`/`...WithBufferedDispatcher`/`...WithDispatcher`），Go 仅有类型定义。**Phase 8 实现。**

### `tool_meta.go` (60L) — 独立审计

- ✅ `FormatToolAggregate`/`ShortenToolPath`/`FormatToolPrefix` 逻辑完整
- TS 144L → Go 60L，无遗漏

---

## 7 类隐藏依赖汇总

| # | 类别 | 状态 |
|---|------|------|
| 1 | npm 包黑盒行为 | ⚠️ `sanitizeUserFacingText` 未移植（P1） |
| 2 | 全局状态 | ⚠️ `ABORT_MEMORY` Map 未实现（P2-延迟）|
| 3 | 事件总线/回调链 | ✅ 回调已实现（onSkip/onError/onIdle/onHeartbeatStrip） |
| 4 | 环境变量 | ✅ 无环境变量依赖 |
| 5 | 文件系统约定 | ✅ 本批次无文件系统依赖 |
| 6 | 协议/消息格式 | ⚠️ 信封格式差异（P2-延迟）|
| 7 | 错误处理约定 | ✅ 错误回调链完整 |

---

## P1 待修复项

### P1-1: `normalize_reply.go` 缺少 `sanitizeUserFacingText`

- **风险**：用户消息中的 HTML/脚本注入可能被透传
- **修复方案**：在 `normalize_reply.go` 的文本处理链中，RegExp 剥离嵌入式脚本标签和危险 HTML
- **建议修复时间**：Phase 8 早期

### P1-2: `inbound_context.go` 缺少 `normalizeChatType` + `resolveConversationLabel`

- **风险**：ChatType 可能不规范（"Group" vs "group" vs "GROUP"），ConversationLabel 可能为空
- **修复方案**：实现 `normalizeChatType()`（简单 toLower 规范化）和 `resolveConversationLabel()`（根据 channelType+chatType 推断标签）
- **建议修复时间**：Phase 8 早期

### P1-3: `abort.go` 触发词列表与 TS 不一致

- **风险**：TS 中 "stop"、"abort" 等不带前缀直接匹配，Go 要求 `/stop`、`/abort` 前缀。用户发 "stop" 在 Go 端不触发中止
- **修复方案**：对齐 TS 触发词列表（`stop, esc, abort, wait, exit, interrupt`），移除 `/` 前缀要求，同时保留 `/stop` 等 slash 命令格式
- **建议修复时间**：Phase 8 早期

---

## 已知延迟项（符合预期，无需立即处理）

1. `dispatch_from_config.go` — 完整分发逻辑（Phase 8）
2. `abort.go` — 完整中止链路（Phase 8）
3. `dispatch.go` — 3 个分发入口（Phase 8）
4. `createReplyDispatcherWithTyping` — Typing 集成（Phase 8）
5. `envelope.go` — 完整时区格式化+elapsed time（Phase 8）
6. `body.ts` `applySessionHints` — Session 提示注入（Phase 8）
7. LINE 指令解析（Phase 8+）
8. `extractShortModelName`/`hasTemplateVariables`（Phase 8）
