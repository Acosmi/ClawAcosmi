> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Telegram 延迟项修复完成报告

## 修复范围

DY-012 ~ DY-024（共 13 项），全部已修复或确认。

## 修复日期

2026-02-24

## 修复摘要

### DY-012: bot_message.go 配置解析函数

| 修复项 | 内容 |
|--------|------|
| ResolveGroupActivationFunc | 从 stub 实现为完整版：通过 `LoadSessionEntry` DI 读取 session store 的 `groupActivation` 设置 |
| requireMention 优先级 | 修正 last-wins 顺序对齐 TS `firstDefined`: activation > topic > group > callback > base |
| ResolveGroupRequireMentionFunc | 修复引用不存在的 `GroupChatConfig.RequireMention` 字段 |
| LoadSessionEntry | 新增到 `TelegramMonitorDeps`，用于 session store 访问 |
| ParentPeer | 审计修复：从 dead code 变量提升到 context 字段赋值 |

### DY-013: bot_updates.go 去重 + 媒体组

| 修复项 | 内容 |
|--------|------|
| MediaGroupEntry | 移除未使用的死代码结构体 |
| flushMediaGroup 排序 | 添加 `sort.Slice` 按 message_id 排序 |
| flushMediaGroup 主消息 | 审计修复：优先选有 caption/text 的消息（对齐 TS `captionMsg ?? messages[0]`） |
| TTL/LRU | 已在之前实现，本次确认通过 |

### DY-014: monitor.go Webhook + 重试

| 修复项 | 内容 |
|--------|------|
| computeBackoff | 从双极性 `rand*2-1` 改为单极性 `rand` 抖动 |
| Webhook/maxRetryTime/panic | 已在之前实现，本次确认通过 |

### DY-015~019: bot_delivery.go 批量修复（前次已完成）

- DY-015: ChunkMode 参数 + chunkText 辅助函数
- DY-016: TableMode 穿透
- DY-017: FormatLocationText 完整重写
- DY-018: DisableLinkPreview 反转语义
- DY-019: 媒体按钮 reply_markup 附加

### DY-020: draft_chunking.go config 级联

| 修复项 | 内容 |
|--------|------|
| ResolveTextChunkLimit | 使用 `autoreply.ResolveTextChunkLimit` 完整级联 |
| buildTelegramProviderChunkConfig | 新增辅助函数，映射通道级 + 账号级 TextChunkLimit/ChunkMode |
| 审计修复 | 补充通道级 `tg.TextChunkLimit` 映射（原遗漏） |

### DY-021: draft_stream.go 架构差异

| 修复项 | 内容 |
|--------|------|
| 确认 | TS 使用非公开 sendMessageDraft API，Go 使用 sendMessage+editMessageText 降级方案 |
| 审计修复 | Stop() 新增草稿消息删除（deleteMessage）避免聊天残留 |

### DY-022: bot_delivery.go isParseError

| 修复项 | 内容 |
|--------|------|
| 第三模式 | 添加 `"find end of the entity"` 检查 |

### DY-023: bot_delivery.go ResolveMedia

| 修复项 | 内容 |
|--------|------|
| 贴纸过滤 | 添加 `!IsAnimated && !IsVideo` 条件 |
| TelegramSticker | 新增 `IsAnimated`/`IsVideo` 字段 |
| video_note | 新增 `msg.VideoNote` 媒体处理路径 |
| resolveMediaPlaceholder | 审计修复：添加 VideoNote → `<media:video>` |

### DY-024: send.go GIF 扩展名

| 修复项 | 内容 |
|--------|------|
| resolveMediaAPIMethodWithFileName | 新增函数，MIME + `.gif` 扩展名双重检测 |
| 调用方更新 | bot_delivery.go + send.go 共 2 处 |

## 复核审计结果

5 组并行交叉颗粒度审计已完成:
- draft_chunking + draft_stream: 2 项修复（通道级 config + 草稿清理）
- bot_delivery + send: PASS
- bot_message + context: 3 项修复（ParentPeer + 确认事项）
- bot_updates + handlers + monitor: 1 项修复（caption 优先选择）
- bot_types: PASS

## 残余已知差异（LOW，已确认可接受）

1. DM `storeAllowFrom` 合并依赖上游调用方预处理
2. `isGetUpdatesConflict` Go 匹配条件略宽松
3. `resolveGroupActivation` 忽略 chatID/threadID（by design，使用 sessionKey）
4. Go `TelegramSticker` 为简化版，省略 width/height/thumbnail 等未使用字段

## 修改文件清单

| 文件 | 修改类型 |
|------|----------|
| `bot_delivery.go` | DY-022/023/024 修复 |
| `bot_handlers.go` | DY-013 审计修复 |
| `bot_message.go` | DY-012 修复 |
| `bot_message_context.go` | DY-012 + 审计修复 |
| `bot_types.go` | DY-023 字段新增 |
| `bot_updates.go` | DY-013 清理 |
| `draft_chunking.go` | DY-020 修复 + 审计修复 |
| `draft_stream.go` | DY-021 审计修复 |
| `monitor.go` | DY-014 修复 |
| `monitor_deps.go` | DY-012 LoadSessionEntry |
| `send.go` | DY-024 新函数 |
