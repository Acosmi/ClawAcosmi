> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 修复报告 #9：Discord Monitor P1 全量修复

修复日期：2026-02-24

---

## 修复总览

| 批次 | 严重度 | 数量 | 编号 |
|------|--------|------|------|
| HIGH 批次1 | HIGH | 3 | W-015, W-034, W-037 |
| HIGH 批次2 | HIGH | 4 | W-016, W-024, W-028(+W-029), W-047 |
| MEDIUM | MEDIUM | 18 | W-014/017/018/021/023/025/029/036/038/039/040/042/043/044/048/049/052/053 |
| LOW | LOW | 12 | W-019/020/022/026/027/030/035/041/046/050/051/054/055/056 |

**总计修复：37 项 WARNING（含 7 HIGH + 18 MEDIUM + 12 LOW）**

---

## HIGH 批次1 详情

### W-015: monitor_provider.go — intents 条件启用 ✅
- 新增 `resolveDiscordGatewayIntents` 函数
- GuildPresences/GuildMembers intent 从硬编码改为根据 `intentsCfg.Presence/GuildMembers` 条件启用
- 事件处理器注册也条件化

### W-034: monitor_threading.go — ReplyDeliveryPlan 补全 ✅
- 新增 `ReplyReferencePlanner` 结构体（有状态 planner，对齐 TS createReplyReferencePlanner）
- replyTarget 在创建线程时跟随更新
- allowReference 机制：新线程中不引用父频道消息

### W-037: monitor_system_events.go — location 参数 + 用户标签 ✅
- `FormatDiscordSystemMessageEvent` 新增 `location` 参数
- 用 `FormatDiscordUserTag` 替代 raw `Username`
- 所有 28 个 `buildSystemEventText` 调用点已更新

---

## HIGH 批次2 详情

### W-016: monitor_provider.go — allowlist name→ID resolve ✅
- 新增 `resolveAllowlistNames` 方法（~200 行）
- 三阶段解析：Guild/Channel name→ID、DM allowFrom username→ID、per-guild/channel user name→ID
- 辅助函数：mergeAllowlist、summarizeResolveMapping、mergeGuildEntry

### W-024: monitor_reply_delivery.go — media 发送 ✅
- 新增 `DeliverDiscordReplies` 支持 `[]ReplyPayload` 数组迭代
- 新增 `deliverMediaMessages`/`sendMediaMessage` 处理附件发送
- 复用 `loadDiscordMedia`（send_media.go）+ `ChannelMessageSendComplex`

### W-028 + W-029: monitor_reply_context.go — resolveReplyContext + labels ✅
- 新增 `ResolveReplyContext` 核心函数（回复链解析 + sender 身份 + envelope 格式化）
- 新增 `BuildDirectLabel`/`BuildGuildLabel` 标签构造
- 新增 `FormatReplyEnvelope` 信封格式化
- `monitor_message_process.go` 调用点已更新

### W-047: monitor_native_command.go — 15 项授权检查 ✅
- 完整移植 TS dispatchDiscordCommandInteraction 授权链
- 新增 `resolveCommandAuthorizedFromAuthorizers` 授权判定
- 覆盖：channel config/enabled/allowed、group policy、DM enabled、user allowlist、access groups

---

## MEDIUM 详情

### provider 4 项 (W-014/017/018/021)
- W-014: MonitorDiscordOpts 新增 MediaMaxMb/HistoryLimit/ReplyToMode 运行时覆盖
- W-017: provider 中添加 RegisterGateway/UnregisterGateway 调用
- W-018: 补全 slash command deploy/clear 逻辑 + ClearDiscordSlashCommands
- W-021: execApprovalsHandler 初始化 + 生命周期管理

### reply-delivery 3 项 (W-023/025/029)
- W-023: 已由 W-024 覆盖（DeliverDiscordReplies 支持数组迭代）
- W-025: 新增 sendDiscordMessageWithRetry 包装（retry + rate limit）
- W-029: 已由 W-028 覆盖（BuildDirectLabel/BuildGuildLabel）

### exec-approvals / allow-list 6 项 (W-038/039/040/042/043/044)
- W-038: SessionFilter 添加正则 fallback
- W-039: 移除 break，发送给所有 approvers
- W-040: 新增 ExecApprovalGatewayClient 接口 + Start/ResolveApproval 方法
- W-042: NormalizeDiscordAllowList 返回 *DiscordAllowList（nil = 未配置）
- W-043: allowlist 条目剥离 Discord mention 格式
- W-044: 新增 ResolveDiscordChannelConfigWithFallback（线程 parent fallback）

### native-command / events / cache 5 项 (W-036/048/049/052/053)
- W-036: 事件文案对齐（"joined the server" → "user joined"，ThreadCreated 简化）
- W-048: 新增 safeDiscordInteractionCall 错误包装
- W-049: deliverDiscordInteractionReply 支持 media 附件
- W-052: 淘汰策略改为 LRU 单条淘汰
- W-053: cache 改为 per-account + Update/Get 签名扩展

---

## LOW 详情

| 编号 | 修复内容 |
|------|----------|
| W-019 | 30s HELLO timeout 僵尸连接检测 |
| W-020 | 完整关闭语义（Disconnect handler + gatewayDone channel）|
| W-022 | GuildMember/Channel 事件处理器标记为 [Go 扩展] |
| W-026 | textLimit/chunkLimit 可配置对齐 |
| W-027 | reaction 状态标记标记为 [Go 扩展] |
| W-030 | resolveDiscordWebhookId variadic fallback |
| W-035 | typing 添加 channel 存在性检查 |
| W-041 | OnResolve nil 时添加日志 + 用户反馈 |
| W-046 | isDiscordUnknownInteraction 结构化错误检测 |
| W-050 | ResolveTimestampMs 返回 *int64（区分 nil/0）|
| W-051 | 新增 9 种时间格式 + Unix timestamp 解析 |
| W-054 | presence cache 扩展存储 Activities/ClientStatus |
| W-055 | Clear 方法已在 W-053 中实现 |
| W-056 | maxEntries 已在 W-052 中对齐为 5000/account |

---

## 编译验证

`go build ./internal/channels/discord/` ✅ 通过

## 修改文件清单

1. `monitor_provider.go` — W-014/015/016/017/018/019/020/021/022
2. `monitor_threading.go` — W-034
3. `monitor_system_events.go` — W-036/037
4. `monitor_reply_delivery.go` — W-023/024/025/026/027
5. `monitor_reply_context.go` — W-028/029
6. `monitor_native_command.go` — W-046/047/048/049
7. `monitor_exec_approvals.go` — W-038/039/040/041
8. `monitor_allow_list.go` — W-042/043/044
9. `monitor_presence_cache.go` — W-052/053/054/055/056
10. `monitor_sender_identity.go` — W-030
11. `monitor_typing.go` — W-035
12. `monitor_format.go` — W-050/051
13. `monitor_message_process.go` — W-028 调用点更新
14. `monitor_listeners.go` — W-053/054 调用点更新
