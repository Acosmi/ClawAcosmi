> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Discord 模块审计跟踪

## 审计统计
- 已映射文件对：40 对
- Go 独有文件：11 个
- TS 未映射（入口/聚合）：7 个
- TS 未映射（业务逻辑）：2 个
- TS 测试文件：17 个

## 审计优先级排序（按核心度从高到低）

### P0 - 核心消息处理链（优先审计）
- [x] accounts.ts ↔ accounts.go ✅ 已修复 NormalizeAccountID + 延迟项 DY-001
- [x] api.ts ↔ api.go ✅ 已修复 W-003 body读取 + 延迟项 DY-002/DY-003/DY-010/DY-011
- [x] token.ts ↔ token.go ✅ 100% 对齐，无需改动
- [x] send.messages.ts ↔ send_messages.go ✅ 已修复 W-004 forum content trim
- [x] send.channels.ts ↔ send_channels.go ✅ 100% 对齐，无需改动
- [x] send.shared.ts ↔ send_shared.go ⚠️ 延迟项 DY-004/DY-005（权限列表+retry wrapper）
- [x] send.types.ts ↔ send_types.go ✅ 18个类型完美映射
- [x] send.guild.ts ↔ send_guild.go ✅ 路由+逻辑对齐，I-008 编码差异无影响
- [x] send.permissions.ts ↔ send_permissions.go ✅ 权限位+3-pass算法完全对齐
- [x] send.reactions.ts ↔ send_reactions.go ✅ 延迟项 DY-006（并行vs串行）
- [x] send.emojis-stickers.ts ↔ send_emojis_stickers.go ✅ 100% 对齐
- [x] monitor/message-handler.preflight.ts ↔ monitor_message_preflight.go ⚠️ 延迟项 DY-007/DY-008
- [x] monitor/message-handler.process.ts ↔ monitor_message_process.go ✅ 已修复 W-028 调用点 + 延迟项 DY-009
- [x] monitor/message-utils.ts ↔ monitor_message_utils.go ✅ (随preflight一并审计)
- [x] monitor/message-handler.preflight.types.ts ↔ monitor_message_preflight_types.go ✅ (随preflight一并审计)

### P1 - Monitor 子系统（全量修复完成）
- [x] monitor/listeners.ts ↔ monitor_listeners.go ✅ 已修复 W-011/012/013
- [x] monitor/provider.ts ↔ monitor_provider.go ✅ 已修复 W-014/015/016/017/018/019/020/021/022（9项全部修复）
- [x] monitor/gateway-registry.ts ↔ monitor_gateway_registry.go ✅ 100% 对齐
- [x] monitor/reply-delivery.ts ↔ monitor_reply_delivery.go ✅ 已修复 W-023/024/025/026/027（5项全部修复）
- [x] monitor/reply-context.ts ↔ monitor_reply_context.go ✅ 已修复 W-028/029（2项全部修复）
- [x] monitor/sender-identity.ts ↔ monitor_sender_identity.go ✅ 已修复 W-030
- [x] monitor/threading.ts ↔ monitor_threading.go ✅ 已修复 W-031/032/033/034（4项全部修复）
- [x] monitor/typing.ts ↔ monitor_typing.go ✅ 已修复 W-035
- [x] monitor/system-events.ts ↔ monitor_system_events.go ✅ 已修复 W-036/037（2项全部修复）
- [x] monitor/exec-approvals.ts ↔ monitor_exec_approvals.go ✅ 已修复 W-038/039/040/041（4项全部修复）
- [x] monitor/allow-list.ts ↔ monitor_allow_list.go ✅ 已修复 W-042/043/044/045（4项全部修复）
- [x] monitor/native-command.ts ↔ monitor_native_command.go ✅ 已修复 W-046/047/048/049（4项全部修复）
- [x] monitor/format.ts ↔ monitor_format.go ✅ 已修复 W-050/051（2项全部修复）
- [x] monitor/presence-cache.ts ↔ monitor_presence_cache.go ✅ 已修复 W-052/053/054/055/056（5项全部修复）

### P2 - 辅助模块（全量审计修复完成）
- [x] audit.ts ↔ audit.go ✅ 已修复 W-057/059/068/069/074（4项）
- [x] chunk.ts ↔ chunk.go ✅ 已修复 W-057/075（CRITICAL 二次分块 + 空 slice）
- [x] probe.ts ↔ probe.go ✅ 已修复 W-060（HTTP Client DI 注入）
- [x] targets.ts ↔ targets.go ✅ 已修复 W-061/070（Normalized 字段 + 正则修正）
- [x] resolve-channels.ts ↔ resolve_channels.go ✅ 已修复 W-062/063/071（errors.Join + Archived + opts 注入）
- [x] resolve-users.ts ↔ resolve_users.go ✅ 已修复 W-064/072（errors.Join + opts 注入）
- [x] directory-live.ts ↔ directory_live.go ✅ 已修复 W-065（errors.Join 容错）
- [x] gateway-logging.ts ↔ gateway_logging.go ✅ 已修复 W-058/066/073（EventEmitter cleanup + IsVerbose 动态开关）
- [x] pluralkit.ts ↔ pluralkit.go ✅ 已修复 W-067（HTTP Client 注入）

### P3 - Action 桥接层（审计修复完成）
- [x] handle-action.ts ↔ discord_handle_action.go ✅ 已修复 W-077/078/079/082（2 CRITICAL + 2 HIGH）+ 延迟项 DY-025/026/027
- [x] handle-action.guild-admin.ts ↔ discord_guild_admin.go ✅ 已修复 W-088/090/091（3 HIGH）

### P4 - Go 独有文件验证（完成）
- [x] 验证 account_id.go 来源 ✅ 迁移自 session-key.ts，100% 对齐
- [x] 验证 monitor_deps.go 来源 ✅ Go 独有 DI 接口，签名一致
- [x] 验证 monitor_message_dispatch.go 来源 ⚠️ 简化版 message-handler.process.ts，缺少大量功能 → DY-028/030
- [x] 验证 send_media.go 来源 ✅ 多源合并（send.shared + send.emojis-stickers），缺 chunk 分段 → DY-029
- [x] 验证 webhook_verify.go 来源 ✅ Go 独有（HTTP Interactions Endpoint）
- [x] 验证 bridge/discord_actions*.go 来源 ✅ 一对一迁移 agents/tools/discord-actions*.ts

### P5 - TS 未映射文件确认（完成）
- [x] 确认 index.ts / send.ts / monitor.ts 等聚合文件 ✅ 纯 re-export，Go 包机制替代
- [x] 确认 send.outbound.ts 是否已合并 ✅ 已合并至 send_shared.go + send_media.go
- [x] 确认 monitor/message-handler.ts 是否已合并 ⚠️ 部分合并，缺多消息合并去抖 → DY-030
- [x] 确认 extensions/discord/ 逻辑 ✅ 目录不存在，bridge/ 已覆盖

## 修复报告索引
- shenji-004: listeners W-011/012/013
- shenji-005: P1 批次A 审计（provider/gateway-registry/reply-delivery/reply-context）
- shenji-006: P1 批次B 审计（sender-identity/threading/typing/system-events）
- shenji-007: P1 批次C 审计（exec-approvals/allow-list/native-command/format/presence-cache）
- shenji-008: P0 修复（W-031/032/033/045）
- shenji-009: P1 全量修复（W-014~W-056，37项）
- shenji-010: 延迟项全量修复（DY-001~DY-011，11项）
- shenji-011: P2 辅助模块审计修复（W-057~W-075，19项）
- shenji-012: P3 Action 桥接层审计修复（W-076~W-097，11项修复）
- shenji-013: P4/P5 验证报告（Go 独有 + TS 未映射确认）

## 延迟项状态
- DY-001~DY-011: ✅ 已全量修复并归档至 daibanyanchi_guidang.md
- DY-012~DY-024: Telegram 模块，由 Telegram 窗口处理
- DY-025: ✅ discord_handle_action.go context.Context 已添加
- DY-026: ✅ discord_handle_action.go DiscordActionConfig 配置链已添加
- DY-027: ✅ DiscordActionRequest.MarshalJSON 展平已实现
- DY-028: ✅ monitor_message_dispatch.go 管道功能全面补齐（ack/reply/thread/forum/batch）
- DY-029: ✅ send_media.go SendDiscordMediaChunked 分段发送已实现
- DY-030: ✅ monitor_message_dispatch.go 收集模式去抖已重写
