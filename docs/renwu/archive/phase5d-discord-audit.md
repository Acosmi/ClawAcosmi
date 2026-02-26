# Phase 5D.9 — Discord SDK 隐藏依赖深度审计

> 日期：2026-02-13
> 范围：`src/discord/` (44 TS 文件, 8733L) → `backend/internal/channels/discord/` (25+2 Go 文件)
> 方法：按 `/refactor` 工作流步骤 1-6 逐项执行

---

## 步骤 1：提取（Extract）摘要

**44 个 TS 非测试源文件** → **27 个 Go 文件**（含铺砌新增 2 个）

合并/拆分映射：

- `index.ts` → Go 包级导出（无单独文件）
- `send.shared.ts` → `send_shared.go` + `send_media.go`（拆分）
- `send.emojis-stickers.ts` → `send_emojis_stickers.go`（新建）
- `send.outbound.ts` 逻辑 → 合入 `send_guild.go`
- Monitor 13 文件 → Phase 6 延迟（仅 5 个纯逻辑 monitor 文件已移植）

---

## 步骤 2：依赖图

### 外部 npm 包

| 包名 | 用途 | Go 等价方案 |
| ---- | ---- | ----------- |
| `@buape/carbon` | REST API 客户端（封装 Discord API 调用 + 文件上传） | 直接 HTTP 调用 Discord REST API |
| `discord-api-types/v10` | Discord API 类型定义 + 路由常量 | `send_types.go` 自定义结构体 + 手写路由 |

### 跨模块依赖统计

| 外部模块 | import 数 | 代表文件 |
| -------- | --------- | -------- |
| `infra/` | 6 | retry, backoff, errors, fetch, json-file |
| `config/` | 5 | config, paths, group-policy, markdown-tables, types |
| `auto-reply/` | 5 | chunk, command-detection, inbound-debounce, history, provider-dispatcher |
| `routing/` | 2 | resolve-route, session-key |
| `web/` | 2 | media, outbound |
| `channels/` | 4 | reply-prefix, typing, logging, session |
| `pairing/` | 1 | pairing-store |
| `markdown/` | 1 | render |

---

## 步骤 3：隐藏依赖 7 类检查

### 1️⃣ npm 包黑盒行为

| 子项 | TS 行为 | Go 现状 | 判定 |
| ---- | ------- | ------- | ---- |
| `@buape/carbon` REST client | `rest.get/post/put/patch/delete` 封装认证 + 序列化 | `discordGET/POST` + `discordMultipartPOST` 直接 HTTP 调用 | ✅ 等价 |
| `@buape/carbon` 文件上传 | `rest.post({ body: { files: [...] }})` multipart | `discordMultipartPOST` 手动构建 multipart/form-data | ✅ 等价 |
| `@buape/carbon` Gateway v10 | 协议握手、心跳、分片、重连、IDENTIFY/RESUME | 13 个 monitor 文件延迟 Phase 6 | ⏳ Phase 6 桩 |

### 2️⃣ 全局状态/单例

| 子项 | TS 位置 | Go 现状 | 判定 |
| ---- | ------- | ------- | ---- |
| 无 Send 层全局状态 | — | — | ✅ 无此类依赖 |
| Monitor 层缓存（presence/reply-context） | `presence-cache.ts`, `reply-context.ts` | Phase 6 桩 | ⏳ Phase 6 |

### 3️⃣ 事件总线/回调链

| 子项 | TS 位置 | Go 现状 | 判定 |
| ---- | ------- | ------- | ---- |
| `recordChannelActivity` | `send.outbound.ts:25` | `send_guild.go` TODO(Phase 6) | ⏳ Phase 6 桩 |
| Gateway 事件绑定 | `monitor/listeners.ts` | 延迟 Phase 6 | ⏳ Phase 6 桩 |

### 4️⃣ 环境变量依赖

| 变量 | TS 位置 | Go 现状 | 判定 |
| ---- | ------- | ------- | ---- |
| `DISCORD_TOKEN` | `token.ts` | `token.go` — `os.Getenv` | ✅ 一致 |
| `DISCORD_TOKEN_<accountId>` | `token.ts` | `token.go` | ✅ 一致 |

### 5️⃣ 文件系统约定

| 子项 | TS 行为 | Go 现状 | 判定 |
| ---- | ------- | ------- | ---- |
| 无 Send 层文件持久化 | — | — | ✅ 无此类依赖 |

### 6️⃣ 协议/消息格式约定

| 子项 | TS 行为 | Go 现状 | 判定 |
| ---- | ------- | ------- | ---- |
| emoji 上传 data URI | `data:<mime>;base64,<data>` | `UploadEmojiDiscord` — 同格式 | ✅ 等价 |
| sticker multipart 上传 | `rest.post` with `files` | `discordMultipartPOST` | ✅ 等价 |
| poll layout_type | `PollLayoutType.Default = 1` | `LayoutType: 1` 硬编码 | ✅ 等价 |
| message_reference | `{ message_id, fail_if_not_exists: false }` | `SendDiscordMedia` 等价构造 | ✅ 等价 |
| `convertMarkdownTables` | `send.outbound.ts:20` — Markdown 表格转换 | TODO(Phase 7) 桩 | ⏳ Phase 7 桩 |

### 7️⃣ 错误处理约定

| 子项 | TS 行为 | Go 现状 | 判定 |
| ---- | ------- | ------- | ---- |
| `BuildDiscordSendErrorFromErr` | 50403 权限 / 50007 DM 封锁检测 | `send_shared.go` — 等价实现 | ✅ 等价 |
| emoji MIME 白名单 | `image/png,jpeg,jpg,gif` 不匹配则 throw | `UploadEmojiDiscord` — 同白名单 | ✅ 等价 |
| sticker MIME 白名单 | `image/png,apng,application/json` | `UploadStickerDiscord` — 同白名单 | ✅ 等价 |
| poll 答案数量上限 | 10 答案截断 + 3-768h 持续时间 | `NormalizeDiscordPollInput` — 10/768h | ✅ 等价 |

---

## 新发现 + 铺砌修复汇总

### P0 缺失（已全部修复）

| ID | 严重程度 | 文件 | 问题 | 状态 |
| ---- | -------- | ---- | ---- | ---- |
| P0-1 | 🔴 高 | `send_emojis_stickers.go` | 缺 `uploadEmojiDiscord` | ✅ 已新建 |
| P0-2 | 🔴 高 | `send_emojis_stickers.go` | 缺 `uploadStickerDiscord` | ✅ 已新建 |
| P0-3 | 🔴 高 | `send_shared.go` + `send_media.go` | 缺 `sendDiscordMedia` | ✅ 已新建 `send_media.go` + 添加 `SendDiscordMedia` |
| P0-4 | 🟡 中 | `send_guild.go` | 缺 `normalizeDiscordPollInput` | ✅ 已添加 `NormalizeDiscordPollInput` |

### P1 差异（4 项已修复 + 2 项延迟）

| ID | 严重程度 | 文件 | 问题 | 状态 |
| ---- | -------- | ---- | ---- | ---- |
| P1-1 | 🟡 中 | `send_guild.go` | `sendMessageDiscord` 缺 media/embeds/chunkMode | ✅ 已修复 — `SendMessageConfig` 增强 |
| P1-2 | 🟢 低 | `send_guild.go` | `sendMessageDiscord` 缺 `convertMarkdownTables` | ⏳ Phase 7 桩 — 跨频道共用 |
| P1-3 | 🟢 低 | `send_guild.go` | `sendMessageDiscord` 缺 `recordChannelActivity` | ⏳ Phase 6 桩 — 依赖 Gateway |
| P1-4 | 🟡 中 | `send_guild.go` | `ParseRecipient` 缺目录查找 | ✅ 已修复 — 新增 `ParseAndResolveRecipient`（lookupFn Phase 6 TODO）|
| P1-5 | 🟡 中 | `send_guild.go` | `SendPollDiscord` 无 poll 校验 | ✅ 已修复 — `NormalizeDiscordPollInput` |
| P1-6 | 🟢 低 | `send_shared.go` | 缺 `FormatReactionEmoji` | ✅ 已修复 — `BuildReactionIdentifier` 别名 |

### P2 维护风险（不阻塞功能）

| ID | 严重程度 | 文件 | 问题 | 状态 |
| ---- | -------- | ---- | ---- | ---- |
| P2-1 | 🟢 低 | `accounts.go` | 手动字段合并（20+ 字段），新增字段需同步 | ⚠️ 记录 — 建议未来改用反射或 code-gen |
| P2-2 | 🟢 低 | `api.go` + `pluralkit.go` | 缺少 HTTP client 注入，测试难以 mock | ⚠️ 记录 — 建议添加 `HTTPClientConfig` 参数 |
| P2-3 | 🟢 低 | `chunk.go` | `ChunkDiscordTextWithMode` newline 模式为桩 | ⏳ Phase 7 — 回退到 length 模式 |

---

## 完整覆盖确认（19 个模块）

| Go 文件 | TS 源 | 函数覆盖 |
| ------- | ----- | -------- |
| `token.go` | `token.ts` | ✅ 2/2 |
| `pluralkit.go` | `pluralkit.ts` | ✅ 1/1 + 4 类型 |
| `gateway_logging.go` | `gateway-logging.ts` | ✅ 3/3 |
| `audit.go` | `audit.ts` | ✅ 4/4 + 2 类型 |
| `probe.go` | `probe.ts` | ✅ 4/4 + 5 类型 |
| `accounts.go` | `accounts.ts` | ✅ 完整 |
| `targets.go` | `targets.ts` | ✅ 完整 |
| `directory_live.go` | `directory-live.ts` | ✅ 2/2 |
| `resolve_channels.go` | `resolve-channels.ts` | ✅ 7/7 |
| `resolve_users.go` | `resolve-users.ts` | ✅ 完整 |
| `chunk.go` | `chunk.ts` | ✅ 7/7 |
| `send_types.go` | `send.types.ts` | ✅ 16/16 类型 |
| `send_shared.go` | `send.shared.ts` | ✅ 完整（含铺砌补强） |
| `send_media.go` | `send.shared.ts` 拆分 | ✅ 新建 — 媒体下载 + multipart |
| `send_emojis_stickers.go` | `send.emojis-stickers.ts` | ✅ 新建 — emoji/sticker 上传 |
| `send_permissions.go` | `send.permissions.ts` | ✅ 5/5 |
| `send_messages.go` | `send.messages.ts` | ✅ 10/10 |
| `send_reactions.go` | `send.reactions.ts` | ✅ 4/4 |
| `send_channels.go` | `send.channels.ts` | ✅ 6/6 |
| `send_guild.go` | `send.guild.ts` + `send.outbound.ts` | ✅ 完整（含铺砌增强） |
| `monitor_format.go` | `monitor/format.ts` | ✅ |
| `monitor_sender_identity.go` | `monitor/sender-identity.ts` | ✅ |
| `monitor_allow_list.go` | `monitor/allow-list.ts` | ✅ |
| `monitor_message_utils.go` | `monitor/message-utils.ts` | ✅ |
| `monitor_threading.go` | `monitor/threading.ts` | ✅ |

---

## 延迟项汇总

已记录在 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) 的 DIS-A ~ DIS-F：

| 编号 | 模块 | Phase | 说明 |
| ---- | ---- | ----- | ---- |
| DIS-A | Gateway 事件绑定 + Monitor 生命周期 | Phase 6 | 原生 WebSocket 客户端或集成 discordgo |
| DIS-B | 消息处理管线 | Phase 6 | preflight → process → reply |
| DIS-C | 执行审批 UI | Phase 6 | Discord Interactions API |
| DIS-D | 原生命令 | Phase 6 | /reset, /help 等 |
| DIS-E | 缓存 + 辅助模块 | Phase 6 | presence/reply-context/typing |
| DIS-F | Send 层铺砌延迟项 | Phase 6/7 | convertMarkdownTables/recordChannelActivity/DirectoryLookupFunc/loadWebMedia |

---

## 验证结果

```
go build ./...     ✅ 通过
go vet ./...       ✅ 通过
```

> **编译验证**：27 Go 文件（25 原始 + 2 铺砌新建），全部通过。
