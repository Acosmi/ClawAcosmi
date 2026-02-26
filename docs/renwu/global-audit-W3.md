# W3 审计报告：channels 消息通道模块

> 审计日期：2026-02-19 | 审计窗口：W3
>
> **W3 全量补全修复完成**：2026-02-21 | P0/P1 全部清零，P2 剩余 2 项推迟

---

## 各通道覆盖率总览

| 通道 | TS 文件数 | Go 文件数 | 关键路径覆盖 | 评级 |
|------|-----------|-----------|-------------|------|
| channels 基础框架 | ~77 | 35 | 95% | A |
| Discord | 44 | 38 | 95% | A |
| Slack | 43 | 39 | 95% | A |
| Telegram | 40 | 38 | 93% | A |
| WhatsApp | 1+43(web/) | 17 | 90% | B+ |
| Signal | 14 | 14 | 92% | A |
| iMessage | 12 | 13 | 88% | B+ |
| LINE | 21 | 16 | **85%** | **B+** ✅ (修复前 C → 修复后 B+) |
| Web (WA层) | 43 | 17 | 75% | B |

---

## P0 差异（LINE 通道核心管线缺失）— ✅ 全部已修复

### P0-1：LINE bot-handlers 完全缺失 — ✅ 已实现

**TS 文件**：`src/line/bot-handlers.ts`（346行）→ **Go 已实现**: `bot_handlers.go` (509L)

已实现功能：

- ✅ `ShouldProcessLineEvent`（DM/群组策略、pairing 门控）
- ✅ `handleMessageEvent`、`handlePostbackEvent` 等所有事件处理函数
- ✅ `LineHandlerContext`、`PairingStore` 接口、pairing 回复

### P0-2：LINE monitor 主入口缺失 — ✅ 已实现

**TS 文件**：`src/line/monitor.ts` → **Go 已实现**: `monitor.go` (351L)

已实现功能：

- ✅ LINE 监控主入口 `MonitorLineProvider`（HTTP 路由注册、bot 初始化、processMessage 管线绑定）
- ✅ `LineRuntimeState` 运行时状态管理
- ✅ Loading keepalive 动画
- ✅ `HTTPRegistry` 路由注入接口

### P0-3：LINE 自动回复投递管线缺失 — ✅ 已实现

**TS 文件**：`src/line/auto-reply-delivery.ts` → **Go 已实现**: `auto_reply_delivery.go` (423L)

已实现功能：

- ✅ 完整的 LINE 自动回复投递管线 `DeliverLineAutoReply`
- ✅ Flex Message、分块投递、快速回复按钮
- ✅ 模板消息构建 `buildTemplateMessage`
- ✅ 依赖注入 `AutoReplyDeps` 接口

---

## P1 差异 — ✅ 全部已修复

### P1-1：iMessage 地址规范化逻辑差异 — ✅ 已修复

- **Go**：`normalize.go` L320-340 `normalizeIMessageHandle()` 已实现 email lowercase + E.164 规范化
- **修复确认**：email 类 iMessage 地址 `strings.Contains(handle, "@") → strings.ToLower(handle)`

### P1-2：Discord 规范化简化 — ✅ 已修复

- **Go**：`normalize.go` L99-151 `NormalizeDiscordMessagingTarget()` 已实现完整解析
- **修复确认**：含 `<@123>` / `<@!123>` mention 展开、user: vs channel: 区分、裸 ID 默认 channel

### P1-3：WhatsApp 群组激活模式缺失 — ✅ 已修复

- **Go**：`monitor_inbound.go` L111-124 已实现 `always` + `silent_token` 激活模式
- **修复确认**：`ResolveSilentToken()` 在 `accounts.go` 中实现

### P1-4：LINE 媒体/功能文件缺失 — ✅ 已实现

- ✅ `download.go` (141L) → 媒体下载完整实现
- ✅ `rich_menu.go` (425L) → Rich Menu API 12+ 方法
- ✅ `auto_reply_delivery.go` → 模板消息（含 `buildTemplateMessage`）
- ✅ `bot_access.go` (90L) → 访问控制 `IsSenderAllowedLine`
- ✅ `client.go` (234L) → Bot 生命周期管理

---

## P2 差异

### P2-1：Telegram webhook secret 等时比较缺失 — ✅ 已修复

- **Go**：`webhook.go` L90 已使用 `hmac.Equal([]byte(headerSecret), []byte(cfg.Secret))`

### P2-2：LINE webhook 签名验证错误码不一致 — ✅ 已修复

- **Go**：`client.go` L170 使用 `http.StatusUnauthorized` (401)，与 TS 一致

### P2-3：速率限制缺失（Telegram/Slack/LINE）— ⏭️ 推迟至 W3-D1

- **TS 端**：Telegram 使用 grammy throttler（有测试文件证明）
- **Go 端**：对应实现均无重试/限速封装
- **推迟原因**：需引入令牌桶限速器，涉及架构设计

### P2-4：`mergeAllowlist` 去重逻辑分散 — ⏭️ 推迟至 W3-D2

- **当前状态**：`normalize.go` 已实现统一 `MergeAllowlist`（case-insensitive 去重）
- **差异**：各通道尚未全量接入统一函数
- **推迟原因**：需逐通道排查引用并替换

### P2-5：LINE markdown_to_line.go 死代码 — ✅ 已修复

- `markdown_to_line.go` L106-126：`rowBox` 已通过 `newFlexBoxComponent(rowBox)` 正确转换为嵌入式 FlexComponent
- 表格 Flex 渲染已实现逐列布局（每行为 horizontal FlexBox，每列 `Flex=1` 等宽）

---

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 状态 |
|----|------|---------|---------|------|--------|------|
| W3-01 | MISSING | src/line/bot-handlers.ts | bot_handlers.go | LINE 事件处理和策略门控 | P0 | ✅ |
| W3-02 | MISSING | src/line/monitor.ts | monitor.go | LINE 监控主入口 | P0 | ✅ |
| W3-03 | MISSING | src/line/auto-reply-delivery.ts | auto_reply_delivery.go | LINE 自动回复投递管线 | P0 | ✅ |
| W3-04 | PARTIAL | normalize/imessage.ts | channels/normalize.go | iMessage email 规范化 | P1 | ✅ |
| W3-05 | PARTIAL | normalize/discord.ts | channels/normalize.go | Discord mention 解析 | P1 | ✅ |
| W3-06 | MISSING | group-activation.ts | whatsapp/monitor_inbound.go | WhatsApp always/silent_token | P1 | ✅ |
| W3-07 | MISSING | src/line/download.ts | line/download.go | LINE 媒体下载 | P1 | ✅ |
| W3-08 | MISSING | src/line/rich-menu.ts | line/rich_menu.go | LINE Rich Menu API | P1 | ✅ |
| W3-09 | BUG | 无 | telegram/webhook.go | timing attack (hmac.Equal) | P2 | ✅ |
| W3-10 | PARTIAL | src/line/webhook.ts | line/client.go | webhook 401 vs 403 | P2 | ✅ |
| W3-11 | MISSING | grammy throttler | 无 | 速率限制缺失 | P2 | ⏭️ |
| W3-12 | PARTIAL | allowlists/resolve-utils.ts | 各通道独立 | 白名单去重不统一 | P2 | ⏭️ |
| W3-13 | BUG | 无 | line/markdown_to_line.go | 死代码/表格渲染 | P2 | ✅ |

---

## 总结

- **P0 差异**：3 项 → ✅ 全部已修复
- **P1 差异**：4 项（含 P1-4 拆分）→ ✅ 全部已修复
- **P2 差异**：5 项 → ✅ 3 项已修复，⏭️ 2 项推迟
- **LINE 通道评级**：C → **B+**（核心运行时管线已完整）
- **其他通道评级**：A（Discord/Slack/Telegram/Signal 达到 A 级）
- **模块总体评级**：**A-**（大幅提升，仅剩 2 项 P2 延迟）
