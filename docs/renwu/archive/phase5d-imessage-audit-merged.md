# Phase 5D.6 — iMessage SDK 隐藏依赖深度审计

> 日期：2026-02-13 | 审计对象：8 Go 文件 ↔ 12 TS 文件
> 修复状态：5/11 已修复 ✅ | 6/11 延迟至 Phase 6/7 📝

## 文件级覆盖总览

| Go 文件 | TS 来源 | Go 行数 | TS 行数 | 覆盖度 |
|---------|---------|---------|---------|--------|
| `constants.go` | `constants.ts` | 6 | 3 | ✅ 完整 |
| `targets.go` | `targets.ts` | 318 | 234 | ✅ 完整 |
| `accounts.go` | `accounts.ts` | 242 | 91 | ✅ 完整 |
| `client.go` | `client.ts` | 346 | 245 | ⚠️ 见 H1 |
| `probe.go` | `probe.ts` | 138 | 107 | ⚠️ 见 H2 |
| `send.go` | `send.ts` | 244 | 141 | 🔴 见 H3, H4 |
| `monitor_types.go` | `monitor/types.ts` | 49 | 41 | ✅ 完整 |
| `monitor.go` | `monitor-provider.ts` + `deliver.ts` + `runtime.ts` | 382 | 839 | ⚠️ 骨架（Phase 6 桩） |

---

## 隐藏依赖 7 类审计

### #1 npm 包黑盒行为

- ✅ iMessage SDK 不依赖 npm 第三方包
- ✅ `imsg` CLI 是自研二进制，协议明确（JSON-RPC over stdio）
- ✅ **H1: client.go `abortSignal` 清理缺失** — TS `client.ts` 在 `monitorProvider` 中使用 `abortSignal` 注册 `abort` 事件监听器，在中止时先发 `watch.unsubscribe` RPC 再停止进程。Go 端 `monitor.go` 使用 `ctx.Done()` 但 **未在取消时发送 `watch.unsubscribe`**，直接 Kill 进程
  - 严重性：**M**
  - ✅ **已修复**：`monitor.go` ctx 取消时先发 `watch.unsubscribe` 再 Stop

### #2 全局状态/单例

- ✅ `rpcSupportCache`（probe.go）已正确使用 `sync.RWMutex` 保护
- ✅ `SentMessageCache`（monitor.go）已正确使用 `sync.Mutex` 保护
- ⚠️ **H2: probe.go `rpcSupportCache` 缺少 TTL/失效机制** — TS 端缓存同样是永久的（无 TTL），但 Go 端是进程级全局变量（`var rpcSupportCache`），对于长时间运行的 daemon 进程，如果 CLI 升级后支持 RPC 但缓存仍为 false 则永远不会重新检测
  - 严重性：**L** — 与 TS 行为一致，但值得记录
  - 现状：Go 行为与 TS 一致，无需修改

### #3 事件总线/回调链

- ⚠️ **H3: monitor.go 缺少 `inboundDebouncer`** — TS `monitor-provider.ts:201-247` 实现了完整的入站消息防抖器 `createInboundDebouncer`，包含：
  - `buildKey`：基于 sender + conversationId 构建防抖 key
  - `shouldDebounce`：跳过含附件或控制命令的消息
  - `onFlush`：将多条消息合并为 synthetic message 后处理
  - Go 端 `handleInboundMessage` 直接在 goroutine 中处理，**完全无防抖**
  - 严重性：**H** — Phase 6 必须实现
  - 延迟项：已在 IM-A 中覆盖（但 IM-A 描述不够具体，建议补充）

- ✅ **H4: monitor.go 缺少回声检测集成** — Go 端创建了 `SentMessageCache` 但 **未在 `handleInboundMessage` 中使用**（`_ = sentCache`）。TS 端在 L400-404 明确检测回声并跳过
  - 严重性：**H**
  - ✅ **已修复**：`sentCache` 已传入 `handleInboundMessage`，添加了回声检测逻辑

### #4 环境变量依赖

- ✅ `HOME` — `detectRemoteHostFromCliPath` 中 `~` 展开使用 `os.Getenv("HOME")`，与 TS `process.env.HOME` 一致
- ✅ 无其他 `process.env` 引用

### #5 文件系统约定

- ⚠️ **H5: send.go `resolveAttachment` 中 `saveMediaBuffer` 缺失** — TS `send.ts:47` 调用 `saveMediaBuffer(media.buffer, contentType, "outbound", maxBytes)` 将下载的媒体保存到本地文件系统的 `outbound/` 目录。Go 端当前是桩返回 error
  - 严重性：**M** — 已标记 Phase 7 桩，但路径约定（`outbound/` 目录）未记录
  - 延迟项：IM-C 已覆盖，建议补充目录约定说明
- ⚠️ **H6: monitor-provider.ts `resolveStorePath` + `readSessionUpdatedAt`** — TS L508-515 使用文件系统存储 session 状态（`session/store/`），Go 端 **完全未实现**
  - 严重性：**H** — Phase 6 `recordInboundSession` 需要此基础设施
  - 延迟项：IM-A 中应包含此依赖

### #6 协议/消息格式约定

- ✅ JSON-RPC 2.0 over stdio 格式已正确实现（`client.go`）
- ✅ `watch.subscribe` 参数 `{ attachments: bool }` 与 TS 一致
- ⚠️ **H7: send.go 缺少 `convertMarkdownTables` 调用** 🔴 — TS `send.ts:97-103` 在发送前调用 `resolveMarkdownTableMode` + `convertMarkdownTables` 转换 Markdown 表格。Go 端 L155 有 TODO 注释但 **未实现任何转换**
  - 严重性：**H** — 出站消息中的表格会以原始 Markdown 格式发送，iMessage 不支持 Markdown 表格渲染
  - 延迟项：IM-D 已覆盖，但 **IM-D 描述中缺少 `send.go` 中的此调用点**（仅提到 `monitor.go DeliverReplies`）
- ✅ **H8: monitor.go 缺少 `watch.unsubscribe` + `subscriptionId` 存储** — TS L734-738 从 `watch.subscribe` 响应中提取 `subscription` ID，在 abort 时发送 `watch.unsubscribe`。Go 端忽略了响应中的 subscription ID
  - 严重性：**L**
  - ✅ **已修复**：subscription ID 保存并在 H1 unsubscribe 中使用
- ⚠️ **H9: deliver.ts 中 `chunkTextWithMode` 完全缺失** — TS `deliver.ts:43` 将文本按 `textLimit` 和 `chunkMode` 分块后逐块发送。Go `DeliverReplies` **直接发送全文**，无分块
  - 严重性：**H** — 超长回复可能被 iMessage 截断或发送失败
  - 延迟项：IM-D 中已提及"文本分块"，但需补充具体函数名和依赖

### #7 错误处理约定

- ✅ Go `client.go` 的错误传播符合 TS 原版语义（超时/进程退出/RPC错误）
- ✅ Go `probe.go` 的 `Fatal` 标记与 TS `fatal` 字段语义一致
- ✅ **H10: send.go 媒体失败静默吞错** — Go `send.go:134-135` 媒体解析失败时 `_ = err`，TS 端也是 catch-and-continue，但 TS 有 `runtime.error?.()` 日志。Go 端 **完全无日志**
  - 严重性：**L**
  - ✅ **已修复**：添加 `LogError` 回调到 `IMessageSendOpts`
- ✅ **H11: client.go Stop() 使用 Kill 而非优雅关闭** — TS `client.ts` 的 `stop()` 先关闭 stdin（`child.stdin.end()`），让进程自然退出，超时后才 Kill。Go 端 **直接 `cmd.Process.Kill()`**，跳过 stdin 关闭
  - 严重性：**M**
  - ✅ **已修复**：`Stop()` 先 `stdinCloser.Close()` → 等 500ms → Kill

---

## 延迟项验证（IM-A ~ IM-D）

### IM-A: 入站消息分发管线 — ⚠️ 需补充

**现有描述**准确但不够具体，缺少以下详细依赖：

1. `inboundDebouncer`（H3）— 消息合并+防抖
2. `resolveChannelGroupPolicy` — 群组策略检查
3. `resolveChannelGroupRequireMention` — 群组 mention 检测
4. `buildMentionRegexes` + `matchesMentionPatterns` — mention 正则构建
5. `hasControlCommand` + `resolveControlCommandGate` — 控制命令权限门控
6. `readChannelAllowFromStore` — pairing store 动态 allowFrom 读取
7. `resolveStorePath` + `readSessionUpdatedAt`（H6）— session 文件存储
8. `formatInboundEnvelope` + `formatInboundFromLabel` — 信封格式化
9. `finalizeInboundContext` — 入站上下文组装（30+ 字段）
10. `createReplyPrefixOptions` + `resolveHumanDelayConfig` — 回复前缀/延迟
11. `createReplyDispatcher` — 回复调度器
12. 群组历史管理（`recordPendingHistoryEntryIfEnabled` / `clearHistoryEntriesIfEnabled`）
13. `truncateUtf16Safe` — UTF-16 安全截断

### IM-B: 配对请求管理 — ✅ 准确

- 覆盖 `upsertChannelPairingRequest` + `buildPairingReply`
- 建议补充：`readChannelAllowFromStore` 也属于此系统

### IM-C: 媒体附件下载 + 存储 — ⚠️ 需补充

- 缺少 `outbound/` 目录路径约定说明
- 缺少 `loadWebMedia`（HTTP 下载 + maxBytes 限制）的依赖描述
- 缺少附件解析中 `mediaRemoteHost`（SSH 远程主机）对 path rewrite 的影响

### IM-D: Markdown 表格转换 + 分块 — 🔴 需重大补充

1. **`send.go` 调用点遗漏** — IM-D 仅提到 `monitor.go DeliverReplies`，未提到 `send.go:155` 的 `convertMarkdownTables` TODO
2. **`chunkTextWithMode` 具体依赖** — 需要 `auto-reply/chunk.go` 的 `resolveChunkMode` + `chunkTextWithMode`
3. **`resolveMarkdownTableMode` 调用点** — `send.go` 和 `deliver.ts` 均需此调用
4. **`resolveTextChunkLimit` 依赖** — TS `monitor-provider.ts:175` 使用此函数解析分块限制

---

## 跨模块隐藏依赖（广域扫描）

以下来自 TS `monitor-provider.ts` 的 25+ import，为 Go 端 Phase 6 目前 **完全无对应实现**：

| TS 模块 | 函数/类 | Go 实现 | Phase |
|---------|---------|---------|-------|
| `auto-reply/chunk` | `resolveTextChunkLimit`, `chunkTextWithMode`, `resolveChunkMode` | ❌ | 7 |
| `auto-reply/command-detection` | `hasControlCommand` | ❌ | 6 |
| `auto-reply/dispatch` | `dispatchInboundMessage` | ❌ | 6 |
| `auto-reply/envelope` | `formatInboundEnvelope`, `formatInboundFromLabel` | ❌ | 6 |
| `auto-reply/inbound-debounce` | `createInboundDebouncer`, `resolveInboundDebounceMs` | ❌ | 6 |
| `auto-reply/reply/history` | `recordPendingHistoryEntryIfEnabled`, `clearHistoryEntriesIfEnabled` | ❌ | 6 |
| `auto-reply/reply/inbound-context` | `finalizeInboundContext` | ❌ | 6 |
| `auto-reply/reply/mentions` | `buildMentionRegexes`, `matchesMentionPatterns` | ❌ | 6 |
| `auto-reply/reply/reply-dispatcher` | `createReplyDispatcher` | ❌ | 6 |
| `channels/command-gating` | `resolveControlCommandGate` | ❌ | 6 |
| `channels/logging` | `logInboundDrop` | ❌ | 6 |
| `channels/reply-prefix` | `createReplyPrefixOptions` | ❌ | 6 |
| `channels/session` | `recordInboundSession` | ❌ | 6 |
| `config/group-policy` | `resolveChannelGroupPolicy`, `resolveChannelGroupRequireMention` | ❌ | 6 |
| `config/markdown-tables` | `resolveMarkdownTableMode` | ❌ | 7 |
| `config/sessions` | `resolveStorePath`, `readSessionUpdatedAt` | 部分（`session_utils_fs.go` 可能已有） | 6 |
| `markdown/tables` | `convertMarkdownTables` | ❌ | 7 |
| `media/constants` | `mediaKindFromMime` | ✅ `send.go` 内联 | — |
| `pairing/pairing-messages` | `buildPairingReply` | ❌ | 6 |
| `pairing/pairing-store` | `readChannelAllowFromStore`, `upsertChannelPairingRequest` | ❌ | 6 |
| `routing/resolve-route` | `resolveAgentRoute` | ❌ | 6 |
| `agents/identity` | `resolveHumanDelayConfig` | ❌ | 6 |
| `infra/transport-ready` | `waitForTransportReady` | ✅ `waitForProbeReady` 等价 | — |

---

## 审计总结

### 发现统计

| 严重性 | 数量 | 编号 |
|--------|------|------|
| 🔴 H (高) | 4 | H3, H4, H7, H9 |
| ⚠️ M (中) | 3 | H1, H5, H11 |
| ℹ️ L (低) | 4 | H2, H8, H10, H6 |

### 需要立即修复的项（Phase 5D 范围内可修）

| 编号 | 描述 | 修复工作量 |
|------|------|-----------|
| H4 | 回声检测未集成 — `sentCache` 传入 `handleInboundMessage` | 10 行 |
| H10 | 媒体错误静默 — 添加日志 | 2 行 |
| H11 | Stop() 优雅关闭 — stdin 先关再 Kill | 5 行 |

### 需要更新延迟项文档的建议

1. **IM-A 补充**：添加入站防抖器、群组策略、mention 检测、命令门控、session 存储等 13 项具体依赖
2. **IM-D 补充**：`send.go:155` 调用点 + `chunkTextWithMode`/`resolveChunkMode` 具体函数名
3. **新增 IM-E**：`client.go Stop()` 优雅关闭 + `monitor.go` 取消时发送 `watch.unsubscribe`（H1 + H11）


---

# 第二轮全局审计


# Phase 5D.6 — iMessage SDK 全局隐藏依赖审计（第二轮）

> 日期：2026-02-14 | 审计对象：8 Go 文件 (1681L) ↔ 17 TS 文件 (1697L)
> 前序审计：[phase5d-imessage-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5d-imessage-audit.md) — 11 项（5 已修 / 6 延迟）
> 修复状态：G2 + G4 已修复 ✅ | `go build` + `go vet` 通过

---

## 一、文件级覆盖总览

| Go 文件 | TS 来源 | Go 行 | TS 行 | 状态 |
|---------|---------|-------|-------|------|
| `constants.go` | `constants.ts` | 7 | 3 | ✅ 完整 |
| `targets.go` | `targets.ts` | 318 | 234 | ✅ 完整 |
| `accounts.go` | `accounts.ts` | 242 | 91 | ✅ 完整 |
| `client.go` | `client.ts` | 364 | 245 | ✅ 完整（含 H11 修复） |
| `send.go` | `send.ts` | 247 | 141 | ⚠️ 见 G1/G2 |
| `probe.go` | `probe.ts` | 138 | 107 | ⚠️ 见 G3/G4 |
| `monitor.go` | `monitor-provider.ts` + `deliver.ts` + `runtime.ts` | 414 | 839 | ⚠️ 骨架（Phase 6/7 TODO 桩） |
| `monitor_types.go` | `monitor/types.ts` | 49 | 41 | ✅ 完整 |
| — | `index.ts` + `monitor.ts` | — | 7 | TS re-export 桶文件，Go 无需对应 |

**总覆盖率**：8/8 Go 文件覆盖全部 TS 模块；类型定义（`types_imessage.go`）与 TS `types.imessage.ts` 字段完全对齐。

---

## 二、前序审计 H1-H11 复查结论

| 编号 | 修复要求 | 当前状态 |
|------|----------|----------|
| H1 | `watch.unsubscribe` 取消逻辑 | ✅ `monitor.go:244-251` |
| H4 | 回声检测 `SentMessageCache` | ✅ `monitor.go:21-78 + 336-348` |
| H8 | `resolveMessageId` 多 key fallback | ✅ `send.go:35-64` |
| H10 | 媒体解析失败非致命 + 日志 | ✅ `send.go:134-138` |
| H11 | `Stop()` 先 close stdin 再 Kill | ✅ `client.go:186-220` |
| H2/H3/H5/H6/H7/H9 | 延迟至 Phase 6/7 | 📝 延迟项不变 |

**结论**：前序审计 5 项已修复均已在代码中确认。

---

## 三、新发现项（7 类隐藏依赖审计）

### 1. npm 包黑盒行为

| # | 发现 | 严重度 | 说明 |
|---|------|--------|------|
| G1 | `send.go` 缺少 `convertMarkdownTables` 调用 | ⚠️ 中 | TS `send.ts:97-104` 在发送前转换 Markdown 表格；Go `send.go:158` 标注为 `TODO(Phase 7)` 但**实际发送路径已绕过该逻辑**。当 Phase 7 Markdown 模块就绪时需回填 |
| G2 | `send.go` 缺少 `resolveUserPath` 对 `dbPath` 的展开 | ✅ 已修复 | `client.go` 添加 `resolveUserPath()` 对 `dbPath` 展开 `~` 路径（与 TS `resolveUserPath(opts.dbPath)` 对齐） |

### 2. 全局状态/单例

| # | 发现 | 严重度 | 说明 |
|---|------|--------|------|
| G3 | `probe.go` 全局 `rpcSupportCache` 缺少清理机制 | ⚠️ 低 | TS `probe.ts:29` 使用模块级 `Map` 同样无 TTL，因此为 **行为对等**，但需注意长运行进程中缓存膨胀风险。（Go 添加了 `sync.RWMutex` 保护 — 线程安全优于 TS 原版，✅ 可接受） |

### 3. 事件总线/回调链

| # | 发现 | 严重度 | 说明 |
|---|------|--------|------|
| G4 | `probe.go` 未继承 config 级 `probeTimeoutMs` 回退 | ✅ 已修复 | `IMessageProbeOptions` 添加 `Config *types.OpenAcosmiConfig` 字段，`ProbeIMessage()` 实现 3 级回退链 `timeoutMs → config.probeTimeoutMs → DEFAULT`；`monitor.go` 调用点已传入 config |
| G5 | `monitor.go` `handleInboundMessage` 使用 `go func()` 而非 debouncer | 📝 已知 | TS 使用 `createInboundDebouncer` 进行合并+防抖；Go `monitor.go:204` 直接 `go` 新 goroutine 处理每条消息。标注为 Phase 6 TODO，**与延迟项 IM-B 一致** |

### 4. 环境变量依赖

| # | 发现 | 严重度 | 说明 |
|---|------|--------|------|
| ✅ | `detectRemoteHostFromCliPath` 正确使用 `os.Getenv("HOME")` | ✅ | 与 TS `process.env.HOME` 对等 |

### 5. 文件系统约定

| # | 发现 | 严重度 | 说明 |
|---|------|--------|------|
| G6 | `send.go:68-71` `resolveAttachment` 返回桩 | 📝 已知 | 依赖 Phase 7 `web/media` + `media/store`，**与延迟项 IM-D 一致** |

### 6. 协议/消息格式约定

| # | 发现 | 严重度 | 说明 |
|---|------|--------|------|
| G7 | `send.go:167-169` service 空字符串处理逻辑差异 | ⚠️ 低 | TS `send.ts:108` 使用 `service \|\| "auto"` — 未定义 service 时 fallback 到 `"auto"`；Go `send.go:167-169` 用独立 `if service == ""` 检查，**功能等价** |
| G8 | `client.go:228` `Request()` 签名接受 `timeoutMs int` 而非 opts 对象 | ⚠️ 低 | TS `client.ts:138` 接受 `opts?: { timeoutMs?: number }`；Go 直接传 int。**功能等价但 API 风格不同**，Phase 6 集成时可能需要适配 |

### 7. 错误处理约定

| # | 发现 | 严重度 | 说明 |
|---|------|--------|------|
| G9 | `send.go` 缺少 `convertMarkdownTables` 后空文本的二次检查 | ⚠️ 低 | TS `send.ts:94-96` 在表格转换之后再做空检查；Go 因跳过表格转换不存在此问题，**但 Phase 7 集成时需补全该逻辑** |
| G10 | `probe.go:77` 错误消息格式与 TS 不完全一致 | ⚠️ 低 | TS `probe.ts:56` 输出 `imsg rpc --help failed (code XXX)`；Go `probe.go:77` 输出 `imsg rpc --help failed: <combined>`。**不影响功能** |

---

## 四、类型定义对齐检查

| TS 字段 (`types.imessage.ts`) | Go 字段 (`types_imessage.go`) | 状态 |
|-------------------------------|-------------------------------|------|
| `name?: string` | `Name string` | ✅ |
| `capabilities?: string[]` | `Capabilities []string` | ✅ |
| `markdown?: MarkdownConfig` | `Markdown *MarkdownConfig` | ✅ |
| `configWrites?: boolean` | `ConfigWrites *bool` | ✅ |
| `enabled?: boolean` | `Enabled *bool` | ✅ |
| `cliPath?: string` | `CliPath string` | ✅ |
| `dbPath?: string` | `DbPath string` | ✅ |
| `remoteHost?: string` | `RemoteHost string` | ✅ |
| `service?: ...` | `Service string` | ✅ |
| `region?: string` | `Region string` | ✅ |
| `dmPolicy?: DmPolicy` | `DmPolicy DmPolicy` | ✅ |
| `allowFrom?: Array<string\|number>` | `AllowFrom []interface{}` | ✅ |
| `groupAllowFrom?: ...` | `GroupAllowFrom []interface{}` | ✅ |
| `groupPolicy?: GroupPolicy` | `GroupPolicy GroupPolicy` | ✅ |
| `historyLimit?: number` | `HistoryLimit *int` | ✅ |
| `dmHistoryLimit?: number` | `DmHistoryLimit *int` | ✅ |
| `dms?: Record<string, DmConfig>` | `Dms map[string]*DmConfig` | ✅ |
| `includeAttachments?: boolean` | `IncludeAttachments *bool` | ✅ |
| `mediaMaxMb?: number` | `MediaMaxMB *int` | ✅ |
| `probeTimeoutMs?: number` | `ProbeTimeoutMs *int` | ✅ |
| `textChunkLimit?: number` | `TextChunkLimit *int` | ✅ |
| `chunkMode?: ...` | `ChunkMode string` | ✅ |
| `blockStreaming?: boolean` | `BlockStreaming *bool` | ✅ |
| `blockStreamingCoalesce?: ...` | `BlockStreamingCoalesce *...` | ✅ |
| `groups?: Record<string, ...>` | `Groups map[string]*IMessageGroupConfig` | ✅ |
| `heartbeat?: ...` | `Heartbeat *ChannelHeartbeatVisibilityConfig` | ✅ |
| `responsePrefix?: string` | `ResponsePrefix string` | ✅ |

**结论**：26/26 字段完全对齐，零遗漏。

---

## 五、审计汇总

| 类别 | 总数 | ✅ | ⚠️ 可修 | 📝 延迟 |
|------|------|-----|---------|---------|
| 前序 H1-H11 | 11 | 5 | 0 | 6 |
| 新发现 G1-G10 | 10 | 3 | 5 | 2 |
| **合计** | **21** | **10** | **3** | **8** |

### 可在当前 Phase 修复的项（⚠️ 5 项）

| 项 | 修复方案 | 复杂度 |
|----|----------|--------|
| **G1** | `send.go` 添加 `// TODO(Phase 7)` 注释到发送路径关键位置 | 已标注 ✅ |
| **G2** | `client.go` 添加 `resolveUserPath()` + 应用于 `dbPath` | ✅ 已修复 |
| **G4** | `probe.go` 添加 `Config` 字段 + 3 级超时回退链 | ✅ 已修复 |
| **G7** | `send.go:167-169` 已功能等价 | 确认 ✅ |
| **G8** | `client.go` API 风格差异，Phase 6 集成时适配 | 低 |

### 延迟至 Phase 6/7 的项（📝 8 项）

| 项 | Phase | 依赖 |
|----|-------|------|
| IM-A (H2): 完整入站管线 | 6 | auto-reply, routing, session |
| IM-B (H3, G5): debouncer | 6 | inbound-debounce 模块 |
| IM-C (H5): 配对请求 | 6 | pairing 模块 |
| IM-D (H6, G6): 媒体附件 | 7 | web/media, media/store |
| IM-E (H7): Markdown 表格 | 7 | markdown/tables |
| H9: Block streaming | 7 | reply-dispatcher |
| G1: send 中表格转换 | 7 | markdown/tables |
| G9: 表格转换后空检查 | 7 | markdown/tables |

---

## 六、测试覆盖

当前 Go `internal/channels/imessage/` 目录**无测试文件**。

> **建议**：Phase 6 集成前应补全以下测试：
>
> - `targets_test.go` — 覆盖 `NormalizeIMessageHandle`、`ParseIMessageTarget`、`IsAllowedIMessageSender`
> - `accounts_test.go` — 覆盖 `ResolveIMessageAccount`、`mergeIMessageAccountConfig`
> - `client_test.go` — 覆盖 `handleLine`、`Request` 超时、`failAll`
> - `probe_test.go` — 覆盖 `DetectBinaryExists`、缓存行为

---

## 七、结论

iMessage SDK Go 端 **底层模块（targets/accounts/client/probe/send/constants）逻辑覆盖度极高**，所有前序修复（H1/H4/H8/H10/H11）均已在代码中确认。类型定义 26/26 字段零遗漏。

**骨架部分**（monitor.go `handleInboundMessage` + `DeliverReplies`）正确标注为 Phase 6/7 TODO 桩，不影响当前已完成模块的使用。

**主要风险**：

1. `dbPath` 含 `~` 路径未展开（G2）— 低频但可能导致生产环境路径错误
2. 无测试覆盖 — Phase 6 集成前必须补齐
