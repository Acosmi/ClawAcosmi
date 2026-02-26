> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 深度审计报告 #6：Discord Monitor P1 批次B

审计范围：sender-identity, threading, typing, system-events（共 4 对）
审计日期：2026-02-24

---

## 1. sender-identity.ts (83L) ↔ monitor_sender_identity.go (117L)

**对齐项**：DiscordSenderIdentity 类型、PK 路径 memberID/memberName/label、普通用户 nickname→globalName→username 优先级

**WARNING (1项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-030 | `resolveDiscordWebhookId` 丢失双字段合并（webhookId + webhook_id） | LOW |

**INFO**：senderLabel 空串 fallback 与 TS `??` 语义微差（Go 行为更合理，属改进）

---

## 2. threading.ts (347L) ↔ monitor_threading.go (150L) — **重灾区**

**WARNING (4项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-031 | `ReplyToMode` 枚举值不对齐: TS "off"/"all" vs Go "never"/"always"，配置文件用 TS 值时 Go 走 default 分支 | **HIGH** |
| W-032 | `sanitizeDiscordThreadName` 丢失 mention 正则清理、截断80→100差异、fallback 缺 "Thread " 前缀 | **HIGH** |
| W-033 | `resolveDiscordAutoThreadContext` 的 From/To/OriginatingTo/SessionKey 格式**完全不对齐**，session 路由将失败 | **CRITICAL** |
| W-034 | `resolveDiscordReplyDeliveryPlan` 丢失 replyTarget 更新、ReplyReferencePlanner 有状态逻辑、allowReference 机制 | **HIGH** |

**INFO**：Phase 7 延迟的异步函数（resolveDiscordThreadStarter 等）缺失可接受

---

## 3. typing.ts (11L) ↔ monitor_typing.go (14L)

**对齐项**：核心 typing API 调用对齐

**WARNING (1项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-035 | 丢失 channel 存在性检查和 triggerTyping 方法检测（Go 返回 error 是改进） | LOW |

---

## 4. system-events.ts (55L) ↔ monitor_system_events.go (136L)

**WARNING (2项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-036 | 事件文案不一致: "user joined" vs "joined the server"，ThreadCreated 文案差异 | MEDIUM |
| W-037 | `buildSystemEventText` 丢失 `location` 参数 + 作者标签用 username 非 formatDiscordUserTag | **HIGH** |

---

## 跨文件汇总

| 严重度 | 数量 | 编号 |
|--------|------|------|
| **CRITICAL** | 1 | W-033 (threading SessionKey 格式) |
| **HIGH** | 4 | W-031, W-032, W-034, W-037 |
| MEDIUM | 1 | W-036 |
| LOW | 2 | W-030, W-035 |
