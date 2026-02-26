> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Telegram P1 审计完成报告 — Bot 子系统

## 审计范围

P1 共 5 个文件对（Bot 子系统）：

| # | TS 文件 | Go 文件 | 状态 |
|---|---------|---------|------|
| 1 | `bot/delivery.ts` (563L) | `bot_delivery.go` (498L) | 已修复 + 复核通过 |
| 2 | `bot/helpers.ts` (444L) | `bot_helpers.go` (554L) | 已修复 + 复核通过 |
| 3 | `bot/types.ts` (29L) | `bot_types.go` (125L) | 复核通过（无需修复） |
| 4 | `draft-chunking.ts` (42L) | `draft_chunking.go` (86L) | 已修复 + 复核通过 |
| 5 | `draft-stream.ts` (140L) | `draft_stream.go` (245L) | 已修复 + 复核通过 |

## 修复清单

### Phase 3（CRITICAL）

| 任务 | 文件 | 修复内容 |
|------|------|----------|
| Task #9 | bot_delivery.go | 重写 `DeliverReplies`：添加 `shouldReplyTo()`（off/first/all 模式）、`applyReplyTo()`、`isVoiceMessagesForbidden()`、`deliverTextChunks()` 共享辅助函数；补全 voice 消息路径 + VOICE_MESSAGES_FORBIDDEN 文本回退；补全 caption splitting + follow-up 文本；补全 inline keyboard 附加 |
| Task #10 | draft_chunking.go | 将硬编码 `textLimit = 4096` 替换为 `channels.GetTextChunkLimit(channels.ChannelTelegram)` 动态解析 |

### Phase 4（HIGH）

| 任务 | 文件 | 修复内容 |
|------|------|----------|
| Task #11 | bot_helpers.go | 添加 `utf16SliceToString()` 和 `utf16SpliceRunes()` UTF-16 偏移量转换函数；更新 `HasBotMention()` 和 `ExpandTextLinks()` 使用 UTF-16 感知切片 |
| Task #12 | draft_stream.go | 修复 `flush()` 在 `inFlight` 状态下的重调度逻辑，通过 `schedule()` 确保 pending 更新不丢失 |
| Task #13 | bot_delivery.go | 对齐 `deliverTextChunks` 错误处理策略：非解析错误返回 error 快速失败，所有 3 个调用方传播错误 |

## 复核审计结果

### 通过项（核心对齐）

- **reply-to 模式语义**（off/first/all）完全对齐
- **voice 消息回退**（VOICE_MESSAGES_FORBIDDEN → 文本）完全对齐
- **caption splitting**（1024 字符限制）完全对齐，复用已有 `caption.go`
- **UTF-16 偏移量处理** — Go 补全 `utf16SliceToString`/`utf16SpliceRunes`
- **draft stream 节流/调度/flush 管线** — 并发模型正确适配（mutex vs 单线程事件循环）
- **类型定义** — `TelegramStreamMode`、`StickerMetadata`、`TelegramMessage` 全部对齐
- **draft chunking 配置解析** — 账户级 → 通道级 → 默认值级联正确

### 残余缺口（延迟项）

| 编号 | 严重度 | 文件 | 描述 |
|------|--------|------|------|
| DY-015 | HIGH | bot_delivery.go | `chunkMode` 参数未复制（TS 支持 "newline" 段落预分割） |
| DY-016 | HIGH | bot_delivery.go | `tableMode` 未穿透至投递管线（始终使用 TableModeDefault） |
| DY-017 | HIGH | bot_helpers.go | `FormatLocationText` 过于简化（缺少 emoji、精度显示、直播前缀、来源判断） |
| DY-018 | MEDIUM | bot_delivery.go | `linkPreview` 默认值反转（Go 零值 = false，TS 默认 = true） |
| DY-019 | MEDIUM | bot_delivery.go | 按钮未附加到媒体消息（仅文本消息附加 reply_markup） |
| DY-020 | MEDIUM | draft_chunking.go | `textLimit` 未使用 config 级 `resolveTextChunkLimit` 级联（仅用 dock 值） |
| DY-021 | MEDIUM | draft_stream.go | 草稿消息生命周期管理（pencil emoji 后缀 + draftMsgID 清理） |
| DY-022 | LOW | bot_delivery.go | `isParseError` 缺少 `"find end of the entity"` 第三种解析错误模式 |
| DY-023 | LOW | bot_delivery.go | `ResolveMedia` 缺少 animated/video sticker 过滤 + video_note 支持 |
| DY-024 | LOW | bot_delivery.go | GIF 检测缺少文件扩展名回退（仅检查 MIME） |

### 已知架构差异（不视为缺口）

- **draft_stream.go**：使用 `sendMessage` + `editMessageText` 替代 TS 未公开的 `sendMessageDraft` API — 有意设计
- **bot_types.go**：`TelegramContext` 分解为独立组件（Go 无 Grammy 依赖）— 正确架构适配
- **bot_types.go**：媒体字段使用 `interface{}` 而非强类型 — 功能正确，仅影响编译时安全

## 构建验证

```
go build github.com/anthropic/open-acosmi/internal/channels/telegram
# Telegram 包编译通过（无 telegram 相关错误）
# 仅 internal/agents/scope/identity.go 存在预存 discord 类型错误（无关）
```

## 下一步

1. P2 审计（消息格式与媒体，9 个文件对）
2. 或优先修复 DY-015~DY-019（HIGH/MEDIUM 残余缺口）
