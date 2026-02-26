> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 深度审计报告 #4：Discord `monitor/listeners.ts` ↔ `monitor_listeners.go`

- **TS 文件**: `src/discord/monitor/listeners.ts` (323L)
- **Go 文件**: `backend/internal/channels/discord/monitor_listeners.go` (375L)
- **审计日期**: 2026-02-24
- **优先级**: P1 Monitor 子系统

---

## 1. 架构映射

| TS 构件 | Go 对应 | 状态 |
|---------|---------|------|
| `DISCORD_SLOW_LISTENER_THRESHOLD_MS = 30_000` | `DiscordSlowListenerThresholdMs = 30_000` | ✅ |
| `logSlowDiscordListener()` | `LogSlowDiscordListener()` | ✅ 核心逻辑一致 |
| `registerDiscordListener()` | — | I-009 discordgo 无需去重 |
| `DiscordMessageListener` class | `WrapDiscordMessageHandler()` | ✅ 模式转换正确 |
| `DiscordReactionListener` class | `WrapDiscordReactionAddHandler()` | ✅ |
| `DiscordReactionRemoveListener` class | `WrapDiscordReactionRemoveHandler()` | ✅ |
| `handleDiscordReactionEvent()` (L172-289) | `HandleDiscordReactionEvent()` (L188-346) | ⚠️ 3 项已修复 |
| `DiscordPresenceListener` class (L293-322) | `HandleDiscordPresenceUpdate()` (L355-360) | ✅ |

## 2. 完美对齐的逻辑

| 模块 | 说明 |
|------|------|
| 慢监听器检测 | 阈值 30s，durationMs 判断，warn 日志 — 完全一致 |
| Message handler 包装 | TS class extends → Go 函数包装 + defer，惯用模式转换正确 |
| Reaction handler 包装 | Add/Remove 两个 handler 均正确映射 |
| Presence handler | userId 提取 + cache 更新逻辑对齐 |
| Guild/Channel 解析 | guild entry → channel config → allowed 检查流程一致 |
| Thread 检测 | PublicThread/PrivateThread/AnnouncementThread → `ch.IsThread()` 等价 |

## 3. 发现项与修复

### W-011: reactionNotifications 默认值不一致 — ✅ 已修复

- **TS L246**: `guildInfo?.reactionNotifications ?? "own"` — 默认 `"own"`
- **Go L278 (修复前)**: `reactionMode = "all"` — 默认 `"all"`
- **影响**: Go 会对所有 reaction 发通知，TS 只通知自己消息上的 reaction
- **修复**: 改为 `reactionMode = "own"`

### W-012: "own" 模式缺少 message author 检查 — ✅ 已修复

- **TS L247-260**: fetch message → 获取 author → `shouldEmitDiscordReactionNotification()` 过滤
- **Go (修复前)**: 注释说 "pass through for 'own'"，实际跳过了 message fetch
- **影响**: "own" 模式形同虚设，无法过滤非 bot 消息上的 reaction
- **修复**: 在 step 7 后添加 `monCtx.Session.ChannelMessage()` 获取 author，非 bot 消息跳过

### W-013: contextKey 格式不一致 — ✅ 已修复

- **TS L284**: `discord:reaction:${action}:${data.message_id}:${user.id}:${emojiLabel}`
- **Go L317 (修复前)**: `discord:reaction:%s:%s:%s` → `channelID:messageID:userID`
- **影响**: 去重粒度不同，缺少 action 和 emojiLabel 维度
- **修复**: 改为 `discord:reaction:%s:%s:%s:%s` → `action:messageID:userID:emojiLabel`

### I-009: registerDiscordListener 去重

- TS 有 constructor 去重注册；discordgo 用 `Session.AddHandler` 不需要
- **评估**: 框架差异，无需修复

### I-010: Go 添加 panic recovery

- Go 在所有 Wrap* 函数中添加 `recover()` + stack trace 日志
- **评估**: 正向增强，Go goroutine panic 会导致进程崩溃，recover 是必要的

### I-011: 异步模型差异

- TS: fire-and-forget (`void task.catch().finally()`) — 非阻塞
- Go: 同步调用在 handler goroutine 中 — 但 discordgo 每事件独立 goroutine
- **评估**: 等效，无需修复

### I-012: 系统事件文本格式

- TS: `Discord reaction ${action}: ${emojiLabel} by ${actorLabel} on ${guildSlug} ${channelLabel} msg ${data.message_id}`
- Go: `Reaction %s: %s by %s on message %s in %s` — 缺少 "Discord" 前缀，结构不同
- **评估**: 仅展示用文本，不影响逻辑，标记为 INFO

### I-013: Presence cache 存储粒度

- TS: 存储完整 `GatewayPresenceUpdate` 对象
- Go: 仅存储 `string(p.Status)`
- **评估**: 待 presence-cache 审计时确认是否需要更多字段

## 4. 外部依赖对照

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `@buape/carbon` (MessageCreateListener 等) | `discordgo` (Session.AddHandler) | ✅ 框架转换 |
| `../../infra/format-time/format-duration.ts` | 直接输出 `durationMs` 数值 | ℹ️ 简化 |
| `../../infra/system-events.js` (enqueueSystemEvent) | `monCtx.Deps.EnqueueSystemEvent` | ✅ DI 注入 |
| `../../routing/resolve-route.js` | `monCtx.Deps.ResolveAgentRoute` | ✅ DI 注入 |
| `./allow-list.js` | `monitor_allow_list.go` | ✅ |
| `./format.js` | `monitor_format.go` | ✅ |
| `./presence-cache.js` | `monCtx.PresenceCache` | ✅ |

## 5. 结论

**listeners 模块审计通过**。3 项 WARNING 已全部修复（W-011/W-012/W-013），5 项 INFO 无需处理。

**状态**: 审计完成，已修复
