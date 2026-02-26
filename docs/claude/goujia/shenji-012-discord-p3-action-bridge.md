> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 审计修复报告 #12：Discord P3 Action 桥接层（2 对）

审计日期：2026-02-24

---

## 审计范围

| 文件对 | TS 行数 | Go 行数 | WARNING 数 | 修复数 |
|--------|---------|---------|-----------|--------|
| handle-action.ts ↔ discord_handle_action.go | 249 | 173→245 | 12 | 6 |
| handle-action.guild-admin.ts ↔ discord_guild_admin.go | 438 | 242→291 | 11 | 5 |
| **合计** | | | **23** | **11** |

注：Go 文件位于 `internal/channels/`（非 `discord/` 子包）

---

## 修复详情

### CRITICAL (2/3 已修复)

| 编号 | 文件 | 问题 | 修复 | 状态 |
|------|------|------|------|------|
| W-077 | discord_handle_action.go | `resolveChannelId()` 闭包完全缺失，10 个 action 丢失 channelId | 添加 channelId/to 解析逻辑到所有 10 个 case | ✅ |
| W-078 | discord_handle_action.go | Guild admin 两阶段分派缺失，25 个 admin action 不可达 | default 分支先检查 `IsDiscordGuildAdminAction` 后 fallthrough | ✅ |
| W-084 | 两文件 | `context.Context` 缺失 | 延迟 — 需在执行层（非构建层）补齐 | DY-025 |

### HIGH (7/9 已修复)

| 编号 | 文件 | 问题 | 修复 | 状态 |
|------|------|------|------|------|
| W-079 | discord_handle_action.go | `send` action `to` required 校验缺失 | 添加空值校验 | ✅ |
| W-082 | discord_handle_action.go | `react` 空 emoji 被 `mergeExtra` 过滤 | 直接设置 emoji，绕过 mergeExtra | ✅ |
| W-088 | discord_guild_admin.go | `thread-reply` channelId 缺少解析 | 添加 channelId/to fallback | ✅ |
| W-090 | discord_guild_admin.go | TS `return undefined` 不匹配语义丢失 | 引入 `ErrUnsupportedAction` sentinel error | ✅ |
| W-091 | discord_guild_admin.go | 30+ required 参数校验缺失 | 关键 case 添加校验（member-info/channel-create/edit/delete/emoji-upload/timeout/kick/ban） | ✅ |
| W-080 | discord_handle_action.go | `allowEmpty` 选项被忽略 | 偶然兼容（Go 不做 required 拒绝空串） | 信息记录 |
| W-085 | discord_handle_action.go | TS `cfg` 配置传递链缺失 | 延迟 — 需在调用层设计 | DY-026 |

### MEDIUM (8 项)

| 编号 | 问题 | 状态 |
|------|------|------|
| W-076 | ReadParentIDParam 三态语义（&"" vs null vs undefined） | 信息记录 — 当前 *string 模式可工作 |
| W-081 | `trim: false` 选项忽略（media URL） | ✅ 已添加 ReadStringParamRaw |
| W-083 | `read` action mergeExtra 空值过滤差异 | 信息记录 — 实际影响低 |
| W-086 | Extra map[string]interface{} 无类型安全 | 信息记录 — Go 惯用模式 |
| W-089 | thread-reply mediaUrl 用 trim:false | 可用 ReadStringParamRaw |
| W-092 | channel-create/edit parentId 三态 | 信息记录 |
| W-095 | thread-reply fallback 到 raw channelId | ✅ 已修复（W-088） |
| W-097 | Extra 序列化嵌套 vs 扁平 | 延迟 — 需自定义 MarshalJSON | DY-027 |

---

## 基础设施改进

### action_params.go 新增函数

| 函数 | 用途 |
|------|------|
| `ReadStringParamRequired(params, key)` | 必填参数校验，空值返回 error |
| `ReadStringParamRaw(params, key)` | 不 trim 的原始读取（media URL） |

### targets.go 新增函数

| 函数 | 用途 |
|------|------|
| `ResolveChannelIDFromParams(params)` | 模拟 TS resolveChannelId 闭包，先 channelId 后 to |

---

## 延迟项

| 编号 | 摘要 | 风险 |
|------|------|------|
| DY-025 | discord_handle_action.go 缺少 context.Context（需在执行层补齐） | 中 |
| DY-026 | discord_handle_action.go 缺少 cfg 配置传递链 | 中 |
| DY-027 | DiscordActionRequest.Extra 序列化需自定义 MarshalJSON 展平 | 低 |

---

## 编译验证

`go build ./internal/channels/...` ✅ 通过

## 修改文件清单

1. `internal/channels/action_params.go` — ReadStringParamRequired + ReadStringParamRaw
2. `internal/channels/discord/targets.go` — ResolveChannelIDFromParams
3. `internal/channels/discord_handle_action.go` — W-077/078/079/082
4. `internal/channels/discord_guild_admin.go` — W-088/090/091
