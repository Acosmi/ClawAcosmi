> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 深度审计报告 #3：Discord 核心模块 Batch 1

审计范围：audit、chunk、directory-live、gateway-logging、token、targets、probe（共 7 对）

---

## 1. audit.ts ↔ audit.go

**对齐**: shouldAuditChannelConfig、listConfiguredGuildChannelKeys、isNumeric、CollectDiscordAuditChannelIds、AuditDiscordChannelPermissions 主逻辑完全对齐。

**问题**:
- **[MEDIUM]** `accountId` 参数在 Go 的 `AuditDiscordChannelPermissions` 中丢失。TS 将 `params.accountId` 传递给 `fetchChannelPermissionsDiscord`；Go 的 `fetchPermsFn` 签名为 `func(channelID, token string)` 无 accountId。
- **[LOW]** 缺少 `context.Context` 参数，无法取消长时间运行的审计。
- **[LOW]** 当 `fetchPermsFn` 为 nil 时，Go 标记 `OK: true` 并继续（静默通过）。TS 无此路径。
- **[LOW]** TS 返回 `error: null`；Go 通过 `omitempty` 省略字段。JSON 输出形状不同。

---

## 2. chunk.ts ↔ chunk.go

**对齐**: 核心常量(maxChars=2000, maxLines=17)、countLines、parseFenceLine、closeFenceIfNeeded、splitLongLine、主 ChunkDiscordText 状态机、rebalanceReasoningItalics 全部正确移植。

**问题**:
- **[MEDIUM] ⚠️ 关键**: `ChunkDiscordTextWithMode` newline 模式不完整。TS 版本调用 `chunkMarkdownTextWithMode` 后**再次**对每行通过 `chunkDiscordText` 分块。Go 版本调用 `autoreply.ChunkMarkdownTextWithMode` 后直接返回——跳过了嵌套的 `ChunkDiscordText` 再分块步骤。**可能导致超长消息发送到 Discord**。
- **[LOW]** `splitLongLine` 中 `rune(window[i])` 对多字节空白字符可能行为不同。

---

## 3. directory-live.ts ↔ directory_live.go

**对齐**: normalizeQuery、buildUserRank、ListDiscordDirectoryGroupsLive/PeersLive 完全对齐，limit=25 默认值保留，searchLimit≤100 守卫正确。

**问题**:
- **[MEDIUM]** 错误处理分歧：TS 在每个 guild API 调用失败时抛出异常终止整个操作；Go 静默 `continue` 跳过失败的 guild。行为更健壮但不同。
- **[LOW]** Go 定义了独立的 `ChannelDirectoryEntry` 结构体而非从共享包导入。

---

## 4. gateway-logging.ts ↔ gateway_logging.go

**对齐**: infoDebugMarkers 列表一致，shouldPromoteGatewayDebug、formatGatewayMetrics 逻辑对齐。

**问题**:
- **[MEDIUM]** 架构分歧：TS 使用 EventEmitter attach/detach 模式（返回 cleanup 函数）；Go 使用 struct 方法需手动调用。TS 的自动监听器清理丢失。
- **[LOW]** `formatGatewayMetrics` nil 输出：TS `"null"/"undefined"` vs Go `"<nil>"`。

---

## 5. token.ts ↔ token.go

**对齐**: NormalizeDiscordToken（trim、空检查、strip "Bot " 前缀）完全一致。ResolveDiscordToken 优先级链正确。Go 使用函数选项模式 idiomatic。

**问题**:
- **[LOW]** TS 返回 `undefined`，Go 返回 `""`。所有 Go 调用方已正确检查 `== ""`。
- **[INFO]** `DiscordTokenSource` 为 `string` 类型（非枚举），编译时安全性较低。

---

## 6. targets.ts ↔ targets.go

**对齐**: ParseDiscordTarget 全部格式覆盖完整，ResolveDiscordChannelID、ResolveDiscordTarget 流程匹配。

**问题**:
- **[MEDIUM]** `isLikelyUsername` 正则不等价。TS 用 `[\d]+$`（匹配末尾数字），Go 用 `^\d+$`（完整数字匹配）。对 `"hello123"` 这类输入，TS 返回 false（有尾部数字），Go 返回 true（非已知前缀且非纯数字）。可能导致不必要的用户名查找。
- **[MEDIUM]** `ResolveDiscordTarget` 签名差异：TS 接收 `DirectoryConfigParams`；Go 注入 `DirectoryLookupFunc`。`DirectoryEntry` 类型不同。

---

## 7. probe.ts ↔ probe.go

**对齐**: Flag 位常量一致，ResolveDiscordPrivilegedIntentsFromFlags、fetchWithTimeout(AbortController→context.WithTimeout)、ProbeDiscord、FetchDiscordApplicationSummary/Id 全部正确。

**问题**:
- **[MEDIUM]** 网络错误处理差异：TS 检查 `err instanceof Response` 提取 status；Go 只返回 error.Error() 无 status。
- **[MEDIUM]** `fetchWithTimeout` 使用 `http.DefaultClient`，无 DI 注入能力。
- **[LOW]** JSON 形状差异：TS 始终包含 `status`/`error` 字段；Go 通过 omitempty 省略。

---

## 跨模块总结

| 主题 | 严重度 | 数量 | 涉及文件 |
|------|--------|------|----------|
| `ChunkDiscordTextWithMode` newline 模式不完整 | **HIGH** | 1 | chunk.go |
| 缺少 `context.Context` | MEDIUM | 1 | audit.go |
| 错误处理分歧（静默继续 vs 抛出） | MEDIUM | 2 | directory_live.go, audit.go |
| DI 架构差异 | MEDIUM | 2 | audit.go, targets.go |
| `isLikelyUsername` 正则不等价 | MEDIUM | 1 | targets.go |
| EventEmitter 模式未移植 | MEDIUM | 1 | gateway_logging.go |
| HTTP Client 不可注入 | MEDIUM | 1 | probe.go |
| JSON null vs omitted 差异 | LOW | 4 | 多个文件 |

**最高优先级修复**:
1. `chunk.go:249-258` — ChunkDiscordTextWithMode newline 模式必须再分块
2. `audit.go:108` — 添加 context.Context 参数
3. `targets.go:124-130` — 修复 isLikelyUsername 正则
4. `probe.go:82-94` — 支持 HTTP Client 注入
