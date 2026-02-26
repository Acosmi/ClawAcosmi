> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Discord 模块 P0+P1 审计完成总结

日期：2026-02-24

---

## 一、审计范围

| 优先级 | 范围 | 文件对数 | 状态 |
|--------|------|----------|------|
| P0 | 核心消息处理链 | 15 对 | ✅ 全部完成 |
| P1 | Monitor 子系统 | 14 对 | ✅ 全部完成 |
| **合计** | | **29 对** | **✅** |

---

## 二、P0 核心消息处理链（15 对）

### 审计结果

| 文件对 | 结果 | 关键发现 |
|--------|------|----------|
| accounts.ts ↔ accounts.go | ✅ 已修复 | NormalizeAccountID + DY-001 指针迁移 |
| api.ts ↔ api.go | ✅ 已修复 | W-003 body读取 + DY-010 config合并 + DY-011 Client注入 |
| token.ts ↔ token.go | ✅ 100% 对齐 | 无需改动 |
| send.messages.ts ↔ send_messages.go | ✅ 已修复 | W-004 forum content trim |
| send.channels.ts ↔ send_channels.go | ✅ 100% 对齐 | 无需改动 |
| send.shared.ts ↔ send_shared.go | ✅ 已修复 | DY-004 权限列表 + DY-005 retry wrapper |
| send.types.ts ↔ send_types.go | ✅ 100% 对齐 | 18个类型完美映射 |
| send.guild.ts ↔ send_guild.go | ✅ 对齐 | I-008 编码差异无影响 |
| send.permissions.ts ↔ send_permissions.go | ✅ 100% 对齐 | 权限位+3-pass算法 |
| send.reactions.ts ↔ send_reactions.go | ✅ 已修复 | DY-006 并行移除 |
| send.emojis-stickers.ts ↔ send_emojis_stickers.go | ✅ 100% 对齐 | 无需改动 |
| monitor/message-handler.preflight.ts ↔ _preflight.go | ✅ 已修复 | DY-007 mention + DY-008 门控 |
| monitor/message-handler.process.ts ↔ _process.go | ✅ 已修复 | DY-009 auto-thread + W-028 调用点 |
| monitor/message-utils.ts ↔ _utils.go | ✅ 对齐 | 随 preflight 一并审计 |
| monitor/message-handler.preflight.types.ts ↔ _types.go | ✅ 对齐 | 随 preflight 一并审计 |

### P0 修复统计
- 代码修复：W-003, W-004 + NormalizeAccountID
- 延迟项修复：DY-001~DY-011（全部）

---

## 三、P1 Monitor 子系统（14 对）

### 审计结果

| 文件对 | WARNING 数 | 修复数 | 状态 |
|--------|-----------|--------|------|
| listeners.ts ↔ monitor_listeners.go | 3 | 3 | ✅ W-011/012/013 |
| provider.ts ↔ monitor_provider.go | 9 | 9 | ✅ W-014~022 |
| gateway-registry.ts ↔ monitor_gateway_registry.go | 0 | 0 | ✅ 100% 对齐 |
| reply-delivery.ts ↔ monitor_reply_delivery.go | 5 | 5 | ✅ W-023~027 |
| reply-context.ts ↔ monitor_reply_context.go | 2 | 2 | ✅ W-028/029 |
| sender-identity.ts ↔ monitor_sender_identity.go | 1 | 1 | ✅ W-030 |
| threading.ts ↔ monitor_threading.go | 4 | 4 | ✅ W-031~034 |
| typing.ts ↔ monitor_typing.go | 1 | 1 | ✅ W-035 |
| system-events.ts ↔ monitor_system_events.go | 2 | 2 | ✅ W-036/037 |
| exec-approvals.ts ↔ monitor_exec_approvals.go | 4 | 4 | ✅ W-038~041 |
| allow-list.ts ↔ monitor_allow_list.go | 4 | 4 | ✅ W-042~045 |
| native-command.ts ↔ monitor_native_command.go | 4 | 4 | ✅ W-046~049 |
| format.ts ↔ monitor_format.go | 2 | 2 | ✅ W-050/051 |
| presence-cache.ts ↔ monitor_presence_cache.go | 5 | 5 | ✅ W-052~056 |

### P1 修复统计
- 发现 WARNING 总计：**46 项**（1 CRITICAL + 8 HIGH + 21 MEDIUM + 16 LOW）
- 全部修复：**46/46 = 100%**

---

## 四、修复分类汇总

### 按严重度

| 严重度 | 数量 | 代表性修复 |
|--------|------|-----------|
| CRITICAL | 1 | W-033 SessionKey/From/To 格式完全不对齐 |
| HIGH (Security) | 1 | W-045 allowlist 安全漏洞 |
| HIGH | 7 | W-015 intents, W-016 allowlist resolve, W-024 media, W-028 reply-context, W-034 deliveryPlan, W-037 location, W-047 授权检查 |
| MEDIUM | 21 | provider 生命周期, retry wrapper, 多 payload, per-account cache 等 |
| LOW | 16 | 时间格式, webhook ID, typing 检查, 扩展标记等 |

### 按修复类型

| 类型 | 数量 | 说明 |
|------|------|------|
| 逻辑补全 | 18 | 缺失的核心逻辑（resolve, auth, media, reply chain） |
| 枚举/格式对齐 | 6 | ReplyToMode, SessionKey, 事件文案 |
| 安全漏洞修复 | 1 | allowlist 分支逻辑反转 |
| 结构重构 | 8 | *string 指针, per-account cache, retry config |
| 功能补全 | 7 | HELLO timeout, webhook ID, gateway 注册, command deploy |
| 扩展标记 | 2 | Go 独有功能添加 [Go 扩展] 注释 |
| 性能优化 | 2 | 并行 reaction, LRU 淘汰 |
| 接口改进 | 2 | HTTP Client 注入, JitterFactor |

---

## 五、延迟项处理

| 编号 | 摘要 | 状态 |
|------|------|------|
| DY-001 | accounts 合并策略 *string | ✅ 已修复 |
| DY-002 | retry Jitter → JitterFactor | ✅ 已修复 |
| DY-003 | RetryAfterHint 加 jitter | ✅ 已修复 |
| DY-004 | send_shared 权限列表 | ✅ 已修复 |
| DY-005 | send_shared retry wrapper | ✅ 已修复 |
| DY-006 | reactions 并行移除 | ✅ 已修复 |
| DY-007 | preflight mention 补全 | ✅ 已修复 |
| DY-008 | preflight 命令门控 | ✅ 已修复 |
| DY-009 | process auto-thread | ✅ 已修复 |
| DY-010 | api retry config 逐字段合并 | ✅ 已修复 |
| DY-011 | api HTTP Client 注入 | ✅ 已修复 |

**11/11 延迟项全部修复，已归档至 `daibanyanchi_guidang.md`**

---

## 六、修改文件清单

共修改 **22 个 Go 文件**：

### discord 包（18 个）
1. `accounts.go` — DY-001
2. `api.go` — W-003, DY-010, DY-011
3. `send_messages.go` — W-004
4. `send_shared.go` — DY-004, DY-005, DY-010
5. `send_guild.go` — DY-004
6. `send_reactions.go` — DY-006
7. `monitor_listeners.go` — W-011/012/013 + W-053/054 调用点
8. `monitor_provider.go` — W-014~022 + DY-001 调用点
9. `monitor_reply_delivery.go` — W-023~027
10. `monitor_reply_context.go` — W-028/029
11. `monitor_sender_identity.go` — W-030
12. `monitor_threading.go` — W-031~034
13. `monitor_typing.go` — W-035
14. `monitor_system_events.go` — W-036/037
15. `monitor_exec_approvals.go` — W-038~041
16. `monitor_allow_list.go` — W-042~045
17. `monitor_native_command.go` — W-046~049
18. `monitor_format.go` — W-050/051
19. `monitor_presence_cache.go` — W-052~056
20. `monitor_message_preflight.go` — DY-007/008
21. `monitor_message_process.go` — DY-009 + W-028 调用点

### 跨包文件（4 个）
22. `pkg/retry/retry.go` — DY-002/003
23. `pkg/types/types_discord.go` — DY-001
24. `internal/agents/scope/identity.go` — DY-001
25. `internal/channels/onboarding_discord.go` — DY-001
26. `internal/gateway/server_methods_channels.go` — DY-001
27. `internal/config/plugin_auto_enable.go` — DY-001
28. `internal/memory/batch_openai.go` — DY-002
29. `internal/memory/batch_voyage.go` — DY-002

---

## 七、文档产出

| 文档 | 路径 | 内容 |
|------|------|------|
| 审计报告 #4 | `goujia/shenji-004-discord-monitor-listeners.md` | listeners 审计 |
| 审计报告 #5 | `goujia/shenji-005-discord-monitor-batch-a.md` | P1 批次A 审计 |
| 审计报告 #6 | `goujia/shenji-006-discord-monitor-batch-b.md` | P1 批次B 审计 |
| 审计报告 #7 | `goujia/shenji-007-discord-monitor-batch-c.md` | P1 批次C 审计 |
| 修复报告 #8 | `goujia/shenji-008-discord-p0-fixes-threading-allowlist.md` | P0 修复 |
| 修复报告 #9 | `goujia/shenji-009-discord-full-fix-report.md` | P1 全量修复 |
| 修复报告 #10 | `goujia/shenji-010-discord-deferred-fixes.md` | 延迟项修复 |
| 跟踪表 | `renwu/discord_audit_tracker.md` | 全局跟踪 |
| 延迟项归档 | `daibanyanchi_guidang.md` | DY-001~011 归档 |

---

## 八、待办（P2~P5）

| 优先级 | 内容 | 文件对数 |
|--------|------|----------|
| P2 | 辅助模块（audit/chunk/probe/targets/resolve/directory/gateway-logging/pluralkit） | 9 对 |
| P3 | Action 桥接层（handle-action/guild-admin） | 2 对 |
| P4 | Go 独有文件验证（account_id/monitor_deps/dispatch/media/webhook/bridge） | 6 项 |
| P5 | TS 未映射文件确认（index/send/monitor 聚合 + outbound/handler/extensions） | 4 项 |
| **合计** | | **21 项** |
