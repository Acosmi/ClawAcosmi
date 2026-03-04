# Phase 5D.8 Slack SDK — 隐藏依赖深度审计报告

> 日期：2026-02-14
> 范围：`src/slack/` 65 TS 文件 ↔ `backend/internal/channels/slack/` 37 Go 文件 (3591L)

---

## 审计总结

| 类别 | ✅ 无需处理 | ⚠️ 已记录/桩 | ❌ 需修复 |
|------|-----------|-------------|----------|
| #1 npm 包黑盒 | 1 | 1 | 0 |
| #2 全局状态/单例 | 0 | 3 | 0 |
| #3 事件总线/回调链 | 0 | 2 | 0 |
| #4 环境变量 | 2 | 0 | 0 |
| #5 文件系统约定 | 1 | 0 | 0 |
| #6 协议/消息格式 | 2 | 3 | 0 |
| #7 错误处理约定 | 1 | 1 | 2 |
| **合计** | **7** | **10** | **2** |

---

## #1 npm 包黑盒行为

### 1a. `@slack/web-api` WebClient 重试/rate-limit ✅

- **TS 行为**：`client.ts:3-9` — `SLACK_DEFAULT_RETRY_OPTIONS { retries:2, factor:2, minTimeout:500, maxTimeout:3000, randomize:true }`
- **Go 实现**：`client.go:31-38` — `DefaultSlackRetryOptions` 完整复刻，指数退避 + jitter
- **评估**：✅ 等价实现完备

### 1b. `@slack/bolt` App 生命周期 ⚠️ Phase 6 桩

- **TS 行为**：`provider.ts:25-145` — `App` 构造（Socket Mode / HTTPReceiver）、`app.start()`/`app.stop()`、`app.event()`/`app.command()` 注册
- **隐式行为**：Socket Mode 自动心跳、envelope ack、WebSocket 重连；HTTPReceiver 签名验证
- **Go 实现**：`monitor_provider.go` — Phase 6 桩，`TODO(phase6)`
- **评估**：⚠️ 需 Phase 6 实现（SLK-A/SLK-B）

---

## #2 全局状态/单例

### 2a. `THREAD_STARTER_CACHE` 模块级 Map ⚠️ Phase 6

- **TS 位置**：`monitor/media.ts:173` — `const THREAD_STARTER_CACHE = new Map<string, SlackThreadStarter>()`
- **行为**：进程生命周期内缓存线程起始消息，无 TTL/大小限制
- **Go 实现**：`monitor_media.go` — Phase 6 桩
- **方案**：Phase 6 用 `sync.Map` + 可选 TTL 驱逐

### 2b. `createDedupeCache` 去重缓存 ⚠️ Phase 6

- **TS 位置**：`monitor/context.ts:172` — `createDedupeCache({ ttlMs: 60_000, maxSize: 500 })`
- **行为**：60s TTL + 500 条上限，用于消息去重 `markMessageSeen`
- **Go 实现**：`monitor_context.go` — Phase 6 桩（SLK-E）
- **方案**：Phase 6 实现等价 LRU + TTL

### 2c. `channelCache` + `userCache` 模块级 Map ⚠️ Phase 6

- **TS 位置**：`monitor/context.ts:162-171`
- **行为**：按 channelId/userId 缓存 API 查询结果，无 TTL
- **Go 实现**：`monitor_context.go` — Phase 6 桩（SLK-E）
- **方案**：Phase 6 用 `sync.Map` 实现

---

## #3 事件总线/回调链

### 3a. `app.event()` / `app.command()` 注册链 ⚠️ Phase 6

- **TS**：`monitor/events.ts` + `monitor/slash.ts` — `@slack/bolt` middleware 链
- **Go**：Phase 6 桩（SLK-C/D/G）→ 自建事件分发路由

### 3b. `abortSignal` 生命周期管理 ⚠️ Phase 6

- **TS**：`provider.ts:353-378` — AbortSignal → app.stop() + handler 注销
- **Go**：Phase 6 桩（SLK-A）→ 用 `context.WithCancel` 替代

---

## #4 环境变量依赖

### 4a. `SLACK_BOT_TOKEN` ✅

- **TS**：`accounts.ts:84` — `process.env.SLACK_BOT_TOKEN`
- **Go**：`accounts.go:267` — `os.Getenv("SLACK_BOT_TOKEN")` ✅

### 4b. `SLACK_APP_TOKEN` ✅

- **TS**：`accounts.ts:85` — `process.env.SLACK_APP_TOKEN`
- **Go**：`accounts.go:272` — `os.Getenv("SLACK_APP_TOKEN")` ✅

---

## #5 文件系统约定

- ✅ 无硬编码路径。媒体存储通过 `saveMediaBuffer` 抽象层（Phase 6 桩 SLK-I）

---

## #6 协议/消息格式约定

### 6a. Slack mrkdwn angle-bracket 转义 ✅

- **TS**：`format.ts:7-53` — `escapeSlackMrkdwnSegment` + `isAllowedSlackAngleToken`
- **Go**：`format.go:14-65` — 完整复刻 ✅

### 6b. `conversations.replies` parent 消息过滤 ✅

- **TS**：`actions.ts:209` — `filter(m => m.ts !== opts.threadId)`
- **Go**：`actions.go:236-241` — 等价过滤 ✅

### 6c. `files.uploadV2` 多阶段 vs legacy API ⚠️

- **TS**：`send.ts:113` — 使用 `client.files.uploadV2()`（v2 三阶段上传）
- **Go**：`client.go:221` — 使用 legacy `files.upload`（单阶段）
- **风险**：Slack 计划废弃 `files.upload`，Phase 7 需迁移到 v2 流程

### 6d. `assertSlackFileUrl` HTTPS + domain 白名单 ⚠️ Phase 6

- **TS**：`media.ts:29-45` — 仅允许 `*.slack.com`/`*.slack-edge.com`/`*.slack-files.com`
- **Go**：Phase 6 桩（SLK-I）— 缺少安全校验
- **风险**：缺少此检查可能导致 token 泄露到非 Slack 域名

### 6e. `assistant.threads.setStatus` 非标 API ⚠️ Phase 6

- **TS**：`context.ts:261-294` — 尝试 `client.assistant.threads.setStatus` → fallback `apiCall`
- **Go**：Phase 6 桩（SLK-F）

---

## #7 错误处理约定

### 7a. `auth.test` 失败非致命降级 ✅

- **TS**：`provider.ts:158-165` — catch 后继续，fallback 到 regex mention
- **Go**：概念存在于 `monitor_provider.go` 桩 ✅

### 7b. `resolveSlackSendToken` 使用 `panic` ✅ 已修复

- **Go 位置**：`send.go:101`
- **修复**：改为返回 `(string, error)`，调用方已同步更新

### 7c. `ResolveSlackChannelID` 使用 `panic` ✅ 已修复

- **Go 位置**：`targets.go:106`
- **修复**：改为返回 `(string, error)`

### 7d. `already_reacted` 错误码静默处理 ⚠️

- **TS 行为**：`@slack/web-api` 收到 `already_reacted` 会抛出错误
- **TS 使用**：`prepare.ts:365-374` — 对 react 失败做 catch 并 logVerbose
- **Go 实现**：`actions.go:57-69` — 未处理 `already_reacted` 错误码
- **风险**：重复 react 时会返回不必要的错误

---

## 当前可修复项（Phase 5D 内直接修复）

| # | 问题 | 文件 | 修复方案 |
|---|------|------|----------|
| 7b | `panic` in `resolveSlackSendToken` | `send.go:106` | 返回 `("", error)` |
| 7c | `panic` in `ResolveSlackChannelID` | `targets.go:109` | 返回 `("", error)` |

## Phase 6/7 延迟项汇总

已有桩 SLK-A ~ SLK-I + SLK-P7-A/B 覆盖。本次审计确认：

- 6c — `files.uploadV2` v2 迁移 → 归入 SLK-P7-B
- 6d — `assertSlackFileUrl` 安全校验 → 归入 SLK-I
- 7d — `already_reacted` 静默处理 → 归入 SLK-C（事件处理时一并处理）
