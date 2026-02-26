> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 深度审计报告 #7：Discord Monitor P1 批次C

审计范围：exec-approvals, allow-list, native-command, format, presence-cache（共 5 对）
审计日期：2026-02-24

---

## 1. exec-approvals.ts (579L) ↔ monitor_exec_approvals.go (644L)

**对齐项**：ExecApprovalRequest/Resolved 类型、customId 编解码、embed 格式化（truncation/colors/fields）、timeout 机制

**WARNING (4项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-038 | SessionFilter 缺少正则 fallback（Go 只做 strings.Contains） | MEDIUM |
| W-039 | Go 只发送给 1 个 approver（break），TS 发送给所有 approvers | MEDIUM |
| W-040 | GatewayClient 未实现，`start()` 和 `resolveApproval` 缺失（架构差异） | MEDIUM |
| W-041 | `OnResolve` 为 nil 时无错误反馈 | LOW |

---

## 2. allow-list.ts (454L) ↔ monitor_allow_list.go (303L)

**WARNING (4项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-042 | 空 raw list 返回 AllowAll:true 而非 nil（语义反转） | MEDIUM |
| W-043 | Discord mention 格式 `<@!?ID>` 未在 Go 中剥离 | MEDIUM |
| W-044 | `resolveDiscordChannelConfigWithFallback`（线程 parent fallback）未移植 | MEDIUM |
| **W-045** | **`IsDiscordGroupAllowedByPolicy` allowlist 分支安全漏洞：非白名单 guild 可能被允许** | **HIGH (Security)** |

**INFO**：Go 缺少 `resolveDiscordOwnerAllowFrom`、`shouldEmitDiscordReactionNotification` 等 6 个函数；PluralKit "pk:" 前缀缺失

---

## 3. native-command.ts (935L) ↔ monitor_native_command.go (755L)

**WARNING (4项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-046 | `isDiscordUnknownInteraction` 简化：string matching 可能误判 | LOW |
| W-047 | `dispatchDiscordSlashCommand` 缺少 ~15 项授权/配置检查（channel config、user allowlist、group policy 等） | **HIGH** |
| W-048 | `safeDiscordInteractionCall` 错误包装未移植 | MEDIUM |
| W-049 | `deliverDiscordInteractionReply` 缺少 media/file attachment 支持 | MEDIUM |

---

## 4. format.ts (41L) ↔ monitor_format.go (60L)

**WARNING (2项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-050 | `ResolveTimestampMs` 返回 0 而非 nil（无法区分"无时间戳"和"epoch"） | LOW |
| W-051 | Go 只解析 RFC3339/RFC3339Nano，TS Date.parse 接受更多格式 | LOW |

---

## 5. presence-cache.ts (61L) ↔ monitor_presence_cache.go (97L)

**WARNING (5项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-052 | 淘汰策略：TS 淘汰 1 条最旧的，Go 淘汰 ~50% 随机条目 | MEDIUM |
| W-053 | Go cache 是全局的（非 per-account），TS 是 per-account | MEDIUM |
| W-054 | Go 只存 status string，TS 存完整 GatewayPresenceUpdate | MEDIUM |
| W-055 | `clearPresences` 函数未移植 | LOW |
| W-056 | Max entries 差异：5000/account(TS) vs 10000 global(Go) | LOW |

---

## 跨文件汇总

| 严重度 | 数量 | 编号 |
|--------|------|------|
| **HIGH (Security)** | 1 | W-045 (allowlist 安全漏洞) |
| **HIGH** | 1 | W-047 (slash command 授权缺失) |
| MEDIUM | 9 | W-038/039/040/042/043/044/048/049/052/053/054 |
| LOW | 5 | W-041/046/050/051/055/056 |
