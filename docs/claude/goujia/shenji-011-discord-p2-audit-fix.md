> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 审计修复报告 #11：Discord P2 辅助模块（9 对）

审计日期：2026-02-24

---

## 审计范围

| 文件对 | TS 行数 | Go 行数 | WARNING 数 | 修复数 |
|--------|---------|---------|-----------|--------|
| audit.ts ↔ audit.go | 140 | 179 | 4 | 4 |
| chunk.ts ↔ chunk.go | 278 | 303 | 2 | 2 |
| probe.ts ↔ probe.go | 194 | 211 | 1 | 1 |
| targets.ts ↔ targets.go | 163 | 201 | 2 | 2 |
| resolve-channels.ts ↔ resolve_channels.go | 324 | 349 | 3 | 3 |
| resolve-users.ts ↔ resolve_users.go | 181 | 207 | 2 | 2 |
| directory-live.ts ↔ directory_live.go | 107 | 182 | 1 | 1 |
| gateway-logging.ts ↔ gateway_logging.go | 68 | 81 | 3 | 3 |
| pluralkit.ts ↔ pluralkit.go | 59 | 83 | 1 | 1 |
| **合计** | | | **19** | **19** |

---

## 修复详情

### CRITICAL (1)

| 编号 | 文件 | 问题 | 修复 |
|------|------|------|------|
| W-057 | chunk.go | `ChunkDiscordTextWithMode` newline 模式缺少二次分块，超 2000 字符 | 添加嵌套 `ChunkDiscordText` 二次分块 |

### HIGH (9)

| 编号 | 文件 | 问题 | 修复 |
|------|------|------|------|
| W-058 | gateway_logging.go | EventEmitter 订阅/cleanup 完全丢失 | 添加 `GatewayEmitter` 接口 + `AttachDiscordGatewayLogging` 返回 cleanup func |
| W-059 | audit.go | 缺少 `accountID` 参数 | 添加参数，穿透至 fetchPermsFn |
| W-060 | probe.go | 缺少 HTTP Client DI 注入 | 添加 `ProbeOptions` struct + `HTTPClient` 字段 |
| W-061 | targets.go | `DiscordTarget` 缺少 `Normalized` 字段 | 添加字段 + `newDiscordTarget` 构造函数 |
| W-062 | resolve_channels.go | `getChannels` 错误被 `_` 丢弃 | `errors.Join` 收集错误 + 返回部分结果 |
| W-063 | resolve_channels.go | `fetchSingleChannel` 丢失 `Archived` 字段 | 从 `ThreadMetadata` 提取 |
| W-064 | resolve_users.go | member search API 错误 continue | `errors.Join` 收集错误 + 返回部分结果 |
| W-065 | directory_live.go | channels/members 获取失败 continue | `errors.Join` 收集错误 + 返回部分结果 |
| W-066 | gateway_logging.go | `Verbose` 静态字段丢失动态开关 | 改为 `IsVerbose func() bool` |
| W-067 | pluralkit.go | 缺少 HTTP Client 注入 | 添加 `opts *DiscordFetchOptions` 参数 |

### MEDIUM (8)

| 编号 | 文件 | 问题 | 修复 |
|------|------|------|------|
| W-068 | audit.go | 缺少 `context.Context` | 添加 ctx 首参数 |
| W-069 | audit.go | `fetchPermsFn==nil` 标记 OK=true 假阳性 | 改为 OK=false + error msg |
| W-070 | targets.go | `isLikelyUsername` 正则 `^\d+$` 不等价 | 改为 `\d+$`（去掉行首锚点） |
| W-071 | resolve_channels.go | 缺少 fetcher/opts 注入 | 添加 `opts *DiscordFetchOptions` |
| W-072 | resolve_users.go | 缺少 fetcher/opts 注入 | 添加 `opts *DiscordFetchOptions` |
| W-073 | gateway_logging.go | nil → `"<nil>"` vs `"null"` | 改为 `"null"` |
| W-074 | audit.go | `Channels: nil` vs `[]` | 初始化为空 slice |
| W-075 | chunk.go | 空输入返回 nil vs `[]` | 改为 `[]string{}` |

---

## 设计决策说明

### 错误处理：`errors.Join` 收集错误模式

联网调研确认：Go 1.20+ 标准库 `errors.Join` 是批量操作部分失败的推荐模式。对 Discord 多 guild 操作场景（guild 相互独立、瞬态失败常见），返回部分结果 + 聚合错误比 fail-fast 更合理。

涉及文件：resolve_channels.go, resolve_users.go, directory_live.go

### gateway_logging：EventEmitter 对齐

联网调研确认 discordgo 的 `AddHandler` 返回 cleanup `func()`。添加 `GatewayEmitter` 接口 + `AttachDiscordGatewayLogging` 对齐 TS 的订阅/取消模式。`IsVerbose func() bool` 支持运行时动态切换。

---

## 调用方更新

| 文件 | 更新 |
|------|------|
| monitor_provider.go | `FetchDiscordApplicationId` 添加第 4 参数 nil |
| monitor_provider.go | `ResolveDiscordChannelAllowlist` 添加 opts nil |
| monitor_provider.go | `ResolveDiscordUserAllowlist` 添加 opts nil |

---

## 编译验证

`go build ./internal/channels/discord/` ✅ 通过

## 修改文件清单

1. `audit.go` — W-059/068/069/074
2. `chunk.go` — W-057/075
3. `probe.go` — W-060
4. `targets.go` — W-061/070
5. `resolve_channels.go` — W-062/063/071
6. `resolve_users.go` — W-064/072
7. `directory_live.go` — W-065
8. `gateway_logging.go` — W-058/066/073
9. `pluralkit.go` — W-067
10. `monitor_provider.go` — 调用方更新
