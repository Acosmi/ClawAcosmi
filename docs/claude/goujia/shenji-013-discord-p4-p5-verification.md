> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 验证报告 #13：Discord P4 Go 独有文件 + P5 TS 未映射确认

验证日期：2026-02-24

---

## P4 — Go 独有文件验证（6 项）

| # | 文件 | 来源类型 | 对应 TS | 需对齐审计 |
|---|------|---------|---------|-----------|
| 1 | `account_id.go` | 迁移 | `src/routing/session-key.ts` L112-128 | 否 — 100% 对齐 |
| 2 | `monitor_deps.go` | Go 独有 | 无（Go DI 接口定义） | 否 — 签名已确认一致 |
| 3 | `monitor_message_dispatch.go` | 迁移（简化版） | `monitor/message-handler.process.ts` | **是** — 缺少线程/forum/历史/ack/reply 等功能 |
| 4 | `send_media.go` | 多源合并 | `send.shared.ts` + `send.emojis-stickers.ts` | 部分 — multipart 是 Go 原生实现，缺少 chunk 分段 |
| 5 | `webhook_verify.go` | Go 独有 | 无（TS 用 Gateway 不用 HTTP Endpoint） | 否 — Go 独有功能 |
| 6 | `bridge/discord_actions*.go` | 一对一迁移 | `src/agents/tools/discord-actions*.ts` | 部分 — action 覆盖完整，细节实现需后续审查 |

### P4 关键发现

**monitor_message_dispatch.go**（DY-028）：
Go 版本是 TS `processDiscordMessage` 的精简骨架（229 行 vs 450 行）。缺失功能：
- 线程处理（thread starter, auto-thread reply plan）
- Forum 频道支持（forum parent slug）
- Guild 历史消息上下文
- Ack reaction 确认反应
- Reply context 引用消息
- Envelope 格式化
- 频道权限检测
- Markdown table 转换
- 频道级 systemPrompt / ownerAllowFrom

**send_media.go**（DY-029）：
Go multipart 实现正确但缺少 TS 的 chunk 分段发送逻辑（先发带附件首条，再分段发剩余文本）。

**bridge/discord_actions*.go**：
5 个文件一对一迁移，action 覆盖完整。需后续审查的细节：
- `removeOwnReactions` 分页遍历
- `uploadEmoji`/`uploadSticker` base64 vs multipart 方式差异
- 各 action 错误返回格式

---

## P5 — TS 未映射文件确认（4 项）

| # | TS 文件 | 性质 | Go 对应 | 需审计 |
|---|--------|------|---------|--------|
| 1 | `index.ts` / `send.ts` / `monitor.ts` | 纯 re-export 聚合 | 不需要（Go 包机制） | 否 |
| 2 | `send.outbound.ts` | 业务逻辑 153L | `send_shared.go` + `send_media.go` | 否 — 已覆盖 |
| 3 | `monitor/message-handler.ts` | 业务逻辑 146L | `monitor_message_dispatch.go` | **是** — 多消息合并去抖 |
| 4 | `extensions/discord/` | 目录不存在 | N/A | 否 |

### P5 关键发现

**message-handler.ts 多消息合并**（DY-030）：
- TS 使用 `createInboundDebouncer` 做 flush-based 合并：多条快速连发消息的文本用 `\n` 拼接为 synthetic message
- Go 使用简化的 seq-based debounce：仅保留最后一条消息，**丢弃**先到的消息
- 如果业务依赖多消息合并（用户快速连发短消息合并为一次 AI 请求），则 Go 实现存在行为差异

---

## 延迟项汇总

| 编号 | 文件 | 问题 | 风险 |
|------|------|------|------|
| DY-028 | monitor_message_dispatch.go | 缺少线程/forum/历史/ack/reply 等大量功能 | 高 |
| DY-029 | send_media.go | 缺少 chunk 分段发送（先附件后文本） | 中 |
| DY-030 | monitor_message_dispatch.go | 多消息合并去抖逻辑缺失 | 中 |
