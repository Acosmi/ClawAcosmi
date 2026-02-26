> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 修复报告 #8：Discord P0 修复 — threading.go + allow_list.go

修复日期：2026-02-24

---

## 修复文件

### 1. monitor_threading.go — 4 项修复

| 编号 | 修复内容 | 严重度 | 状态 |
|------|----------|--------|------|
| W-031 | `ResolveDiscordReplyTarget` 添加 `"off"`/`"all"` 枚举值别名，兼容 TS 和 Go 两种枚举 | HIGH | ✅ 已修复 |
| W-032 | `SanitizeDiscordThreadName` 添加 mention 正则清理（user/role/channel）、两阶段截断（80→100）、fallback 添加 "Thread " 前缀 | HIGH | ✅ 已修复 |
| W-033 | `ResolveDiscordAutoThreadContext` From/To/SessionKey 格式对齐 TS。使用 `routing.BuildAgentPeerSessionKey` 构建标准 session key | CRITICAL | ✅ 已修复 |
| W-031(扩展) | `ResolveDiscordReplyDeliveryPlan` 中 replyToMode switch 也添加 `"all"` 别名 | HIGH | ✅ 已修复 |

**关键改动细节**：

**W-033 格式对齐**（最关键）：
- 旧 From: `"discord:" + agentID` → 新 From: `"discord:channel:{threadId}"` (对齐 TS `${channel}:channel:${threadId}`)
- 旧 To: `"discord:" + threadID` → 新 To: `"channel:{threadId}"` (对齐 TS `channel:${threadId}`)
- 旧 SessionKey: `agentID + ":discord:" + threadID` → 新: `routing.BuildAgentPeerSessionKey(...)` 生成 `"agent:{normalizedAgentId}:discord:channel:{threadId}"`
- 新增 `messageChannelID` 空值检查（对齐 TS 的 trim+empty check）
- 新增 `routing` 包导入，复用标准 session key 构建器

**W-032 清理规则对齐**：
- 新增 4 个正则（命名为 `thread*Re` 避免与 `targets.go` 中 `discordUserMentionRe` 冲突）
- 截断从 100 改为先 80 后 100（对齐 TS 两阶段截断）
- fallback 从 `fallbackID` 改为 `"Thread " + fallbackID`

### 2. monitor_allow_list.go — 1 项修复

| 编号 | 修复内容 | 严重度 | 状态 |
|------|----------|--------|------|
| W-045 | `IsDiscordGroupAllowedByPolicy` allowlist 分支添加 `!guildAllowlisted` 提前返回 false | HIGH (Security) | ✅ 已修复 |

**安全漏洞说明**：
- 旧逻辑：`if guildAllowlisted && !channelAllowlistConfigured { return true }; return channelAllowed`
- 漏洞：当 `!guildAllowlisted` 时，代码跳过 if 直接执行 `return channelAllowed`，若碰巧 channel 配置为 allowed，非白名单 guild 也会被放行
- 新逻辑：先检查 `!guildAllowlisted → false`，再检查 `!channelAllowlistConfigured → true`，最后 `return channelAllowed`（对齐 TS 三段逻辑）

---

## 编译验证

- `go build ./internal/channels/discord/` ✅ 通过
- 变量名冲突已解决（`threadUserMentionRe` vs `discordUserMentionRe`）

---

## 遗留项

以下 P1 优先级 WARNING 未在本次修复范围内，记录为延迟项：

- W-034 (HIGH): `ResolveDiscordReplyDeliveryPlan` 缺少 replyTarget 更新和 ReplyReferencePlanner 有状态逻辑
- W-015 (HIGH): provider intents 硬编码
- W-016 (HIGH): provider allowlist name→ID resolve 缺失
- W-024 (HIGH): reply-delivery media 发送缺失
- W-028 (HIGH): reply-context resolveReplyContext 核心逻辑缺失
- W-037 (HIGH): system-events location 参数丢失
- W-047 (HIGH): native-command slash command 授权检查缺失
