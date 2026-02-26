> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Telegram 模块审计跟踪

## 审计统计
- 已映射文件对：33 对
- Go 独有文件：5 个
- TS 未映射（入口/配置）：5 个
- TS 未映射（业务逻辑）：5 个
- TS 测试文件：49 个

## 审计优先级排序（按核心度从高到低）

### P0 - 核心消息处理链（优先审计）
- [x] accounts.ts ↔ accounts.go — Phase 1 修复 (binding 合并)
- [x] token.ts ↔ token.go — 基本对齐，无需修复
- [x] bot.ts ↔ bot.go — Phase 1 修复 (默认值)
- [x] bot-message.ts ↔ bot_message.go — 已审计 + DY-012 已修复（配置解析 + requireMention 优先级 + session store）
- [x] bot-message-context.ts ↔ bot_message_context.go — Phase 2 修复 (agent 路由 + 线程 session key + 命令门控)
- [x] bot-message-dispatch.ts ↔ bot_message_dispatch.go — Phase 2 修复 (空回复回退 + sticker 视觉 + ACK 移除)
- [x] bot-handlers.ts ↔ bot_handlers.go — Phase 2 修复 (debouncing + 文本分片聚合)
- [x] bot-updates.ts ↔ bot_updates.go — 已审计 + DY-013 已修复（TTL/LRU + 媒体组排序 + caption 优先）
- [x] bot-access.ts ↔ bot_access.go — 基本对齐，无需修复
- [x] bot-native-commands.ts ↔ bot_native_commands.go — Phase 1 修复 (重写)
- [x] send.ts ↔ send.go — Phase 2 修复 (voice/videoNote + caption splitting)
- [x] monitor.ts ↔ monitor.go — 已审计 + DY-014 已修复（webhook + maxRetryTime + 单极性抖动）

### P1 - Bot 子系统
- [x] bot/delivery.ts ↔ bot_delivery.go — Phase 3-1 + Phase 4-3 + DY-015~019 已修复 + DY-022~024 已修复（isParseError + sticker过滤 + video_note + GIF扩展名）
- [x] bot/helpers.ts ↔ bot_helpers.go — Phase 4-1 修复 + DY-017 已修复（FormatLocationText 完整重写）
- [x] bot/types.ts ↔ bot_types.go — 复核通过，无需修复
- [x] draft-chunking.ts ↔ draft_chunking.go — Phase 3-2 修复 + DY-020 已修复（config 级联 + ProviderChunkConfig）
- [x] draft-stream.ts ↔ draft_stream.go — Phase 4-2 修复 + DY-021 已确认（已知架构差异 + 草稿清理）

### P2 - 消息格式与媒体
- [x] format.ts ↔ format.go — P2 修复（headingStyle→none, blockquotePrefix→"", TableMode 常量对齐, 分块度量统一）
- [x] caption.ts ↔ caption.go — 基本对齐，无需修复
- [x] download.ts ↔ download.go — P2 修复（maxBytes 溢出检测, media.DetectMime 三级 MIME 检测, 死代码清理）
- [x] voice.ts ↔ voice.go — P2 修复（getVoiceFileExtension URL 路径解析）
- [x] inline-buttons.ts ↔ inline_buttons.go — P2 修复（capabilities 联合类型 UnmarshalJSON + 数组/对象优先级 + 空数组处理 + 空账户回退）
- [x] model-buttons.ts ↔ model_buttons.go — P2 修复（(?i) 正则标志 + truncateModelID rune 长度 + pageSize 参数）
- [x] reaction-level.ts ↔ reaction_level.go — 基本对齐，无需修复
- [x] sticker-cache.ts ↔ sticker_cache.go — P2 修复（sync.Mutex 并发锁 + StickerDescription 常量）
- [x] sent-message-cache.ts ↔ sent_message_cache.go — 基本对齐，无需修复

### P3 - 基础设施
- [x] audit.ts ↔ audit.go — P3 修复（HTTP 2xx 范围检查）
- [x] probe.ts ↔ probe.go — P3 修复（Status/Error 指针类型 null 语义 + HTTP 2xx 范围检查）
- [x] targets.ts ↔ targets.go — 高质量迁移，无需修复
- [x] api-logging.ts ↔ api_logging.go — P3 修复（shouldLog 回调参数）
- [x] webhook.ts ↔ webhook.go — P3 修复（ctx.Done() 优雅关闭 + net.Listen 启动等待 + panic recovery 500）
- [x] group-migration.ts ↔ group_migration.go — P3 修复（大小写回退查找）
- [x] update-offset-store.ts ↔ update_offset_store.go — P3 修复（resolveStateDir 路径修复 + sanitize 保留大小写）

### P4 - Go 独有文件验证
- [x] 验证 http_client.go 来源 — PASS（合并 fetch.ts + proxy.ts，架构映射合理）
- [x] 验证 monitor_deps.go 来源 — PASS（Go 独有 DI 文件，无直接 TS 对应）
- [x] 验证 network.go 来源 — P4 修复（合并 allowed-updates.ts + network-config.ts + network-errors.ts，清理死代码 recoverableErrorCodes）
- [x] 验证 bridge/telegram_actions.go 来源 — P4 修复（ReadTelegramButtons 严格验证 + InlineButtonsScopeChecker + ReactionLevelChecker）
- [x] 验证 onboarding_telegram.go 来源 — P4 修复（quickstartScore 对齐 + selectionHint + API username 解析 + allowFrom merge）

### P5 - TS 未映射文件确认
- [x] 确认 index.ts — 纯 re-export 桶文件，Go package 内直接可见，无需映射
- [x] 确认 allowed-updates.ts — 已合并至 network.go AllowedUpdates()
- [x] 确认 fetch.ts — 已合并至 http_client.go NewTelegramHTTPClient()
- [x] 确认 network-config.ts + network-errors.ts — 已合并至 network.go
- [x] 确认 proxy.ts — 已合并至 http_client.go（SOCKS5 + HTTP/HTTPS 代理）
- [x] 确认 webhook-set.ts — 已合并至 webhook.go SetTelegramWebhook/DeleteTelegramWebhook
- [x] 确认 extensions/telegram/ — TS 源中不存在此目录，误列项
