# Phase 5D.7 — Telegram SDK 隐藏依赖深度审计

> 日期：2026-02-13
> 范围：`src/telegram/` (40 TS 文件, 7929L) → `backend/internal/channels/telegram/` (35 Go 文件)
> 方法：按 `/refactor` 工作流步骤 1-6 逐项执行

---

## 步骤 1：提取（Extract）摘要

**40 个 TS 非测试源文件** → **35 个 Go 文件**（5 个合并）

合并映射：

- `index.ts` → Go 包级导出（无需单独文件）
- `fetch.ts` + `proxy.ts` → `http_client.go`
- `network-config.ts` + `network-errors.ts` + `allowed-updates.ts` → `network.go`
- `webhook-set.ts` → `webhook.go`

---

## 步骤 2：依赖图

### 外部 npm 包（grammy 生态）

| 包名 | 用途 | Go 等价方案 |
|------|------|-------------|
| `grammy` | Bot SDK（API 调用、中间件） | 直接 HTTP 调用 Telegram Bot API |
| `@grammyjs/runner` | 并发 getUpdates（sequentialize） | 自实现长轮询 `monitor.go` |
| `@grammyjs/transformer-throttler` | API 限流 | Go 端暂缺（Phase 6 待评估） |
| `@grammyjs/types` | Telegram 类型定义 | `bot_types.go` 自定义结构体 |

### 跨模块依赖统计

| 外部模块 | import 数 | 代表文件 |
|----------|----------|---------|
| `infra/` | 12 | errors, retry-policy, backoff, dedupe, fetch, env, json-file, system-events, channel-activity, diagnostic |
| `config/` | 10 | config, sessions, paths, commands, group-policy, markdown-tables, telegram-custom-commands, io, agent-limits, types |
| `auto-reply/` | 9 | chunk, command-detection, inbound-debounce, commands-registry, skill-commands, status, envelope, history, provider-dispatcher |
| `routing/` | 3 | resolve-route, session-key, bindings |
| `agents/` | 4 | agent-scope, model-selection, model-auth, identity |
| `channels/` | 8 | reply-prefix, typing, logging, session, ack-reactions, command-gating, mention-gating, dock |
| `media/` | 4 | constants, mime, audio, fetch |
| `web/` | 1 | media |
| `markdown/` | 2 | render, ir |
| `logging/` | 3 | subsystem, redact, diagnostic |
| `plugins/` | 1 | commands |
| `pairing/` | 1 | pairing-store |
| `cli/` | 1 | command-format |

---

## 步骤 3：隐藏依赖 7 类检查

### 1️⃣ npm 包黑盒行为

| 子项 | TS 行为 | Go 现状 | 判定 |
|------|---------|---------|------|
| grammy Bot 中间件链 | `bot.use()`, `bot.on()`, `bot.command()` 注册处理器 | Go 用直接 HTTP 调用替代，无中间件链 | ⚠️ **Phase 6 桩** — bot_handlers/bot_native_commands 框架已搭但处理逻辑延迟 |
| grammy `sequentialize()` | 按 chatId 串行化 update 处理 | Go 端未实现串行化 | ⚠️ Phase 6 — 需在 Gateway 层实现 per-chat 串行队列 |
| `apiThrottler()` | 自动限流 API 调用频率 | Go 端无限流 | ⚠️ Phase 6 — 需 `rate.Limiter` 或自实现 |
| grammy `InputFile` | 流式文件上传 | Go `callTelegramAPIMultipart` 已实现 | ✅ 等价 |
| `@grammyjs/runner` `run()` | 并发 getUpdates + backoff + silent 模式 | Go `MonitorTelegramProvider` 已实现长轮询 + 指数退避 | ✅ 等价（但缺并发 sink） |

### 2️⃣ 全局状态/单例

| 子项 | TS 位置 | Go 现状 | 判定 |
|------|---------|---------|------|
| `sentMessageCache` | `sent-message-cache.ts` — 全局 Map 记录 bot 发送的消息 | `sent_message_cache.go` — `sync.RWMutex` 保护 | ✅ 正确 |
| `stickerCacheStore` | `sticker-cache.ts` — JSON 文件持久化 | `sticker_cache.go` — JSON 读写 | ✅ 等价 |
| `updateDedupe` | `bot-updates.ts` — LRU 去重缓存 | `bot_updates.go` — `sync.Mutex` 保护 | ✅ 等价 |
| `groupHistories` | `bot.ts` — Map 存储群聊历史 | Go 端在 bot.go 定义但仅框架 | ⚠️ Phase 6 桩 |
| `textFragmentBuffer` | `bot-handlers.ts` — Map 缓冲分片消息 | Go 端 `bot_handlers.go` 定义但仅框架 | ⚠️ Phase 6 桩 |
| `mediaGroupBuffer` | `bot-handlers.ts` — Map 缓冲媒体组 | Go 端 `bot_handlers.go` 定义但仅框架 | ⚠️ Phase 6 桩 |

### 3️⃣ 事件总线/回调链

| 子项 | TS 位置 | Go 现状 | 判定 |
|------|---------|---------|------|
| `bot.on("message")` | `bot-handlers.ts:668` | 已在 phase5d-task.md 记录为 Phase 6 桩 | ⚠️ Phase 6 |
| `bot.on("callback_query")` | `bot-handlers.ts:279` | 同上 | ⚠️ Phase 6 |
| `bot.on("message_reaction")` | `bot.ts:386` | 同上 | ⚠️ Phase 6 |
| `bot.on("message:migrate_to_chat_id")` | `bot-handlers.ts:617` | 同上 | ⚠️ Phase 6 |
| `inboundDebouncer` | `bot-handlers.ts:86` | 同上 | ⚠️ Phase 6 |
| `setTimeout` 定时器（分片、媒体组） | `bot-handlers.ts:268,830,845` | 同上 | ⚠️ Phase 6 |

### 4️⃣ 环境变量依赖

| 变量 | TS 位置 | Go 现状 | 判定 |
|------|---------|---------|------|
| `TELEGRAM_BOT_TOKEN` | `token.ts:96` | `token.go:103` — `os.Getenv` | ✅ 一致 |
| `OPENACOSMI_STATE_DIR` | `update-offset-store.ts` | `update_offset_store.go:41` | ✅ 一致 |
| `OPENACOSMI_DEBUG_TELEGRAM_ACCOUNTS` | `accounts.ts:9` | `accounts.go:322` | ✅ 一致 |
| `OPENACOSMI_TELEGRAM_ENABLE_AUTO_SELECT_FAMILY` | `network-config.ts` | `network.go:64` | ✅ 一致 |
| `OPENACOSMI_TELEGRAM_DISABLE_AUTO_SELECT_FAMILY` | `network-config.ts` | `network.go:68` | ✅ 一致 |

### 5️⃣ 文件系统约定

| 子项 | TS 行为 | Go 现状 | 判定 |
|------|---------|---------|------|
| update offset 文件 | `.openacosmi/state/telegram-update-offset-{accountId}.json` | `update_offset_store.go` — 同路径 | ✅ 一致 |
| sticker cache 文件 | `.openacosmi/state/sticker-cache-{accountId}.json` | `sticker_cache.go` — 同路径 | ✅ 一致 |

### 6️⃣ 协议/消息格式约定

| 子项 | TS 行为 | Go 现状 | 判定 |
|------|---------|---------|------|
| Bot API Base URL | `https://api.telegram.org` | `network.go` 常量 | ✅ 一致 |
| sendMessage parse_mode=HTML | `send.ts:349` | `send.go:279` | ✅ 一致 |
| HTML 解析错误回退纯文本 | `send.ts:358-378` | `send.go:308-320` | ✅ 等价 |
| thread_not_found 回退 | `send.ts:300-322` `sendWithThreadFallback` | ✅ **已修复 HD-1** — 添加 thread_not_found 回退逻辑 | ✅ 已修复 |
| reply_parameters quote 字段 | `send.ts:263-266` | `send.go:291-295` | ✅ 等价 |
| link_preview_options | `send.ts:334` | `send.go:284-285` | ✅ 等价 |

### 7️⃣ 错误处理约定

| 子项 | TS 行为 | Go 现状 | 判定 |
|------|---------|---------|------|
| `createTelegramRetryRunner` | `send.ts:272-277` — 支持可配置重试策略 | ✅ **已修复 HD-2** — 使用 `pkg/retry.DoWithResult` + `buildTelegramRetryConfig` | ✅ 已修复 |
| `wrapChatNotFound` | `send.ts:287-298` — 友好错误消息 | ✅ **已修复 HD-3** — 包装 chat not found 友好消息 | ✅ 已修复 |
| `isGetUpdatesConflict` | `monitor.ts:60-80` — 完整字段检查 (error_code/description/method) | ✅ **已修复 HD-4** — 增强 409 + conflict/terminated 检查 | ✅ 已修复 |
| `resolveMarkdownTableMode` | `send.ts:325-329` — 从 config 动态解析 | ⏳ **桩 HD-5** — `resolveTableMode()` 已搭框架，待 `OpenAcosmiConfig` 添加 `Markdown` 字段 | ⏳ 延迟 |

---

## 新发现汇总

| ID | 严重程度 | 文件 | 问题 | 状态 |
|----|---------|------|------|------|
| HD-1 | 🟡 中 | `send.go` | 缺 `sendWithThreadFallback` | ✅ 已修复 — 添加 thread_not_found 回退逻辑 |
| HD-2 | 🟡 中 | `send.go` | 缺 retry runner | ✅ 已修复 — 使用 `pkg/retry.DoWithResult` |
| HD-3 | 🟢 低 | `send.go` | 缺 `wrapChatNotFound` | ✅ 已修复 — 包装友好错误消息 |
| HD-4 | 🟢 低 | `monitor.go` | `isGetUpdatesConflict` 简化 | ✅ 已修复 — 增强 409 + conflict 检查 |
| HD-5 | 🟡 中 | `send.go` | `resolveMarkdownTableMode` 硬编码 | ⏳ 桩函数 — 延迟至 `OpenAcosmiConfig` 添加 `Markdown` 字段 |

> **phase5d-task.md 已记录的 5 处 Phase 6 桩**（TG-1~TG-5）均准确，无遗漏。
> **编译验证**：`go build ./...` ✅ 通过。
