> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 待办延迟项 — 归档

本文档记录已解决并通过复核审计的延迟项。

---

## Discord 模块（DY-001 ~ DY-011）— 2026-02-24 全量修复

修复报告：`goujia/shenji-010-discord-deferred-fixes.md`

| 编号 | 摘要 | 修复方式 |
|------|------|----------|
| DY-001 | accounts.go 合并策略 | 6 个 string → *string 指针 |
| DY-002 | retry Jitter bool → float64 | JitterFactor 0.1 对齐 TS |
| DY-003 | RetryAfterHint 应用 jitter | calculateDelay 统一路径 |
| DY-004 | send_shared 权限探测列表 | 动态列表对齐 TS |
| DY-005 | send_shared retry wrapper | discordRESTWithRetry 包装 |
| DY-006 | reactions 并行移除 | WaitGroup + semaphore(3) |
| DY-007 | preflight mention 检测 | role/everyone/reply-to 补全 |
| DY-008 | preflight 命令门控 | ResolveControlCommandGate 补全 |
| DY-009 | process auto-thread 路由 | CreateThread + 路由重定向 |
| DY-010 | api.go retry config 合并 | mergeRetryConfig 逐字段 |
| DY-011 | api.go HTTP Client 注入 | DiscordFetchOptions.Client |

---

## Discord 模块（DY-025 ~ DY-030）— 2026-02-24 全量修复

修复报告：`goujia/shenji-012-discord-p3-action-bridge.md` + `goujia/shenji-013-discord-p4-p5-verification.md`

| 编号 | 摘要 | 修复方式 |
|------|------|----------|
| DY-025 | discord_handle_action.go 缺少 context.Context | 两个 Build 函数均添加 ctx context.Context 首参 |
| DY-026 | discord_handle_action.go 缺少 cfg 配置传递链 | DiscordActionConfig 结构体 + cfg 参数 + Config 字段 |
| DY-027 | DiscordActionRequest.Extra 序列化展平 | 自定义 MarshalJSON 展平 Extra 到顶层 |
| DY-028 | monitor_message_dispatch.go 缺少大量功能 | 全面重写管道：ack reaction/reply context/thread/forum/batch IDs |
| DY-029 | send_media.go 缺少 chunk 分段发送 | SendDiscordMediaChunked 首块+媒体 multipart, 后续纯文本 |
| DY-030 | monitor_message_dispatch.go 多消息合并去抖 | 收集模式重写：messages 列表 + mergeDebounceMessages `\n` 拼接 |

---

## Telegram 模块（DY-012 ~ DY-024）— 2026-02-24 全量修复

修复报告：`goujia/shenji-011-telegram-deferred-fixes.md`

| 编号 | 摘要 | 修复方式 |
|------|------|----------|
| DY-012 | bot_message.go 配置解析函数缺失 | ResolveGroupActivationFunc + requireMention 优先级对齐 |
| DY-013 | bot_updates.go 去重 TTL 与媒体组 | TTL + LRU + NestedUpdateID + sort.Slice |
| DY-014 | monitor.go Webhook 模式与重试 | computeBackoff 单极性抖动对齐 TS |
| DY-015 | bot_delivery.go 缺少 chunkMode | ChunkMode 字段 + chunkText 辅助函数 |
| DY-016 | bot_delivery.go 缺少 tableMode 穿透 | TableMode 字段穿透 chunkText/caption |
| DY-017 | bot_helpers.go FormatLocationText | 完整重写：emoji/精度/caption/live/place/pin |
| DY-018 | bot_delivery.go linkPreview 反转 | LinkPreview → DisableLinkPreview 反转语义 |
| DY-019 | bot_delivery.go 按钮未附加到媒体 | shouldAttachButtonsToMedia 逻辑 |
| DY-020 | draft_chunking.go textLimit 未级联 | ResolveTextChunkLimit config 级联 |
| DY-021 | draft_stream.go 草稿生命周期 | 已知架构差异，sendMessage+edit 降级 |
| DY-022 | bot_delivery.go isParseError 第三种 | "find end of the entity" 检查 |
| DY-023 | ResolveMedia 动画贴纸+video_note | IsAnimated/IsVideo 过滤 + video_note |
| DY-024 | GIF 检测文件扩展名回退 | resolveMediaAPIMethodWithFileName |

---

## Slack + Providers 模块（DY-031 ~ DY-035）— 2026-02-24 全量修复

修复/审计报告：`goujia/shenji-014-slack-media-slash-providers.md`（含 P0 安全修复 + 功能补全 + DY-031~035 全量审计）

| 编号 | 摘要 | 修复方式 |
|------|------|----------|
| DY-031 | monitor_slash.go Native 命令注册 | ResolveSlackNativeCommands + HandleSlackNativeSlashCommand + toNativeCommandsSetting |
| DY-032 | monitor_slash.go Reply Delivery 高级选项 | deliverSlackSlashReplies + ResolveSlackMarkdownTableMode + dispatcher 集成 |
| DY-033 | monitor_slash.go 交互式参数菜单 | ResolveCommandArgMenu 集成 + Block Kit 菜单 + action 回调串联 |
| DY-034 | Providers CLI 交互层 | GithubCopilotLoginCommand TTY+设备码+auth profile |
| DY-035 | Slack + Providers 测试覆盖 | 6 个测试文件 71 个用例：slash/media/auth/token/models/qwen |

测试验证：`go test` Slack 42 PASS + Providers 29 PASS = 71 用例全绿

---

## Pairing 模块（DY-P01 ~ DY-P04）— 2026-02-25 全量修复

修复/审计报告：`goujia/shenji-014-pairing-audit.md`

| 编号 | 摘要 | 修复方式 |
|------|------|----------|
| DY-P01 | node_pairing_ops.go 多处函数缺少 nodeId 规范化 | RequestNodePairing/VerifyNodeToken/UpdatePairedNodeMetadata 入口添加 strings.TrimSpace |
| DY-P02 | node_pairing_ops.go 缺少 pending TTL 过期清理 | node_pairing.go 添加 pendingTTLMs 常量 + pruneExpiredPending() 在 loadPairingState 中调用 |
| DY-P03 | ListNodePairingStatus 返回未排序列表 | slices.SortFunc 按 Ts/ApprovedAtMs 降序排序 |
| DY-P04 | VerifyNodeToken 缺少返回匹配 node 对象 | 签名改为 (*NodePairingPairedNode, bool) + 调用方 server_methods_nodes.go 同步更新 |

测试验证：`go build ./...` 通过 + `go test ./internal/pairing/...` 12/12 PASS + `go test ./internal/infra/...` 无回归

---

## Signal 模块（DY-S01 ~ DY-S03）— 2026-02-25 全量修复

修复/审计报告：`goujia/shenji-signal-full-audit.md`

| 编号 | 摘要 | 修复方式 |
|------|------|----------|
| DY-S01 | event_handler.go 群组历史上下文 `buildPendingHistoryContextFromMap` 未实现 | 集成 `reply.HistoryMap` + `BuildHistoryContextFromMap`，dispatch 后 `ClearEntries` 清理 |
| DY-S02 | Signal 单元测试：TS 有 10 个测试文件，Go 当前 0 个 | 新增 7 个 _test.go 文件共 62 个测试用例，覆盖 accounts/daemon/format/identity/reaction_level/send/send_reactions |
| DY-S03 | readReceiptsViaDaemon: TS 在 autoStart+sendReadReceipts 时跳过手动已读回执 | 计算 `readReceiptsViaDaemon = autoStart && sendReadReceipts`，传入 handler 条件跳过 |

测试验证：`go build` + `go vet` 通过 + `go test ./internal/channels/signal/` 62/62 PASS
