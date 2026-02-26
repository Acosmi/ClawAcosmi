> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 修复报告 #10：Discord 延迟项全量修复（DY-001 ~ DY-011）

修复日期：2026-02-24

---

## 修复总览

| 编号 | 文件 | 问题 | 风险 | 状态 |
|------|------|------|------|------|
| DY-001 | accounts.go + types_discord.go | string 字段合并策略 → *string 指针 | 中低 | ✅ |
| DY-002 | pkg/retry/retry.go | Jitter bool → JitterFactor float64 | 低 | ✅ |
| DY-003 | pkg/retry/retry.go | RetryAfterHint 应用 jitter | 低 | ✅ |
| DY-004 | send_shared.go | 权限探测列表对齐 TS | 中 | ✅ |
| DY-005 | send_shared.go | 添加 retry wrapper | 中 | ✅ |
| DY-006 | send_reactions.go | 并行移除 reactions（bounded semaphore） | 低 | ✅ |
| DY-007 | monitor_message_preflight.go | 补全 role/everyone/reply mention 检测 | 中 | ✅ |
| DY-008 | monitor_message_preflight.go | 补全频道级命令门控 | 低 | ✅ |
| DY-009 | monitor_message_process.go | 补全 auto-thread 回复路由 | 低 | ✅ |
| DY-010 | api.go | mergeRetryConfig 逐字段合并 | 高 | ✅ |
| DY-011 | api.go | HTTP Client 可注入 | 中 | ✅ |

---

## 详细修复内容

### DY-001: *string 指针迁移
- `types_discord.go`: 6 个 string 字段改为 `*string`（Name/Token/GroupPolicy/ChunkMode/ReplyToMode/ResponsePrefix）
- 添加 6 个安全访问器方法 + 3 个指针构造器
- 更新所有调用方：accounts.go、token.go、monitor_provider.go、scope/identity.go、onboarding_discord.go、server_methods_channels.go、plugin_auto_enable.go

### DY-002+003: retry 公共包重构
- `Config.Jitter bool` → `Config.JitterFactor float64`（0~1 系数）
- 新增 `applyJitter(delay, factor)` 函数，对齐 TS 公式
- RetryAfterHint 路径也应用 jitter
- 更新 6 个调用方的 `Jitter: true` → `JitterFactor: 0.1`

### DY-004: 权限探测动态化
- 从固定 5 项改为动态列表：基础 2 项 + 条件添加 SendMessagesInThreads/AttachFiles

### DY-005: send_shared retry wrapper
- 新增 `discordRESTWithRetry`/`discordPOSTWithRetry`/`discordMultipartPOSTWithRetry`
- 3 个关键调用点包裹重试（text/media/dm-channel）

### DY-006: 并行 reaction 移除
- `sync.WaitGroup` + semaphore channel（并发上限 3）
- `Promise.allSettled` 语义：失败静默跳过

### DY-007+008: preflight 补全
- DY-007: 添加 Mentions 数组检查、MentionEveryone/MentionRoles、reply-to-bot 隐式 mention
- DY-008: 补全 ResolveControlCommandGate + ResolveMentionGatingWithBypass 门控链

### DY-009: auto-thread 完整路由
- 补全线程创建（CreateThreadDiscord）+ 回复路由重定向 + SessionKey 覆盖

### DY-010: retry config 逐字段合并
- 新增 `mergeRetryConfig(base, override)` — 非零值覆盖
- 更新 api.go + send_shared.go 两个调用点

### DY-011: HTTP Client 注入
- `DiscordFetchOptions` 新增 `Client *http.Client` 字段
- FetchDiscord 优先使用 opts.Client

---

## 编译验证

`go build ./internal/channels/discord/` ✅ 通过

## 修改文件清单

1. `pkg/retry/retry.go` — DY-002/003
2. `pkg/types/types_discord.go` — DY-001
3. `internal/channels/discord/accounts.go` — DY-001
4. `internal/channels/discord/api.go` — DY-010/011
5. `internal/channels/discord/send_shared.go` — DY-004/005/010
6. `internal/channels/discord/send_reactions.go` — DY-006
7. `internal/channels/discord/send_guild.go` — DY-004
8. `internal/channels/discord/monitor_message_preflight.go` — DY-007/008
9. `internal/channels/discord/monitor_message_process.go` — DY-009
10. `internal/channels/discord/monitor_provider.go` — DY-001
11. `internal/channels/discord/monitor_reply_delivery.go` — DY-002
12. `internal/channels/discord/token.go` — DY-001
13. `internal/agents/scope/identity.go` — DY-001
14. `internal/channels/onboarding_discord.go` — DY-001
15. `internal/gateway/server_methods_channels.go` — DY-001
16. `internal/config/plugin_auto_enable.go` — DY-001
17. `internal/memory/batch_openai.go` — DY-002
18. `internal/memory/batch_voyage.go` — DY-002
