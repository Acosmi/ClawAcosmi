# Phase 11 审计修复 — 任务清单

> 最后更新：2026-02-17
> 来源：6 个模块审计报告 (A-F) + `deferred-items.md` P11 段落
> 策略：按优先级 (P0→P1→P2) 分 Batch ，每 Batch ≤ 5 文件

---

## Batch A: P0 紧急修复 (阻塞运行时正确性)

### A1: `agent` 主处理器缺失 (模块 A) ✅

- [x] 新建 `server_methods_agent_rpc.go` — `agent` 方法
- [x] 接入 `dispatchInboundMessage` 管线
- [x] Session 自动创建 + key 解析
- [x] `chat.stream.start/update/final` 广播
- [x] 验证: `go build` + `go test ./internal/gateway/...`
- 参考: [phase11-gateway-methods-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-gateway-methods-audit.md) DIFF-1

### A2: SessionStore 磁盘持久化 (模块 B) ✅

- [x] `sessions.go` — 启动时 `loadFromDisk()`，变更时 `saveToDisk()`
- [x] 原子写入 (tmp+rename) + 0o600 权限
- [x] 基于文件的建议锁 (`storePath.lock`) + 30s 过期驱逐
- [x] `normalizeSessionStore()` 遗留字段迁移
- [x] 验证: `go test -race ./internal/gateway/...`
- 参考: [phase11-session-mgmt-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-session-mgmt-audit.md) 3.1

### A3: MaxPayloadBytes 修正 (模块 E) ✅

- [x] `broadcast.go` — `MaxPayloadBytes` 从 25MB 改为 `512 * 1024`
- [x] 验证: `go build`
- 参考: [phase11-ws-protocol-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-ws-protocol-audit.md) 3.2

---

## Batch B: P0 核心管线 (AutoReply + Agent Runner)

### B1: AutoReply `dispatch.go` 统一分发入口 (模块 C) ✅

- [x] 补全 `dispatch.go` — `DispatchInboundMessage()` 完整管线 (172L)
- [x] finalize context → dispatchReplyFromConfig → typing dispatcher (3 函数 + DI 接口)
- [x] 验证: `go build` + `go test` 通过
- 参考: [phase11-autoreply-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-autoreply-audit.md) C-P0-3

### B2: AutoReply session 管理 4 文件 (模块 C) ✅

- [x] `session.go` — InitSessionState + ResolveSessionKey + ForkSession (183L)
- [x] `session_updates.go` — 系统事件 + 技能快照 + compaction (155L)
- [x] `session_usage.go` — token/provider 追踪 (80L)
- [x] `session_reset_model.go` — 模型重置覆盖 (133L)
- [x] 依赖: Batch A2 (SessionStore 持久化) 已完成
- 参考: C-P0-2

### B3: AutoReply `model-selection.ts` 移植 (模块 C) ✅

- [x] `model_selection.go` (210L) — 模型选择状态 + 指令解析 + context token 解析
- [x] 依赖 `agents/models/selection.go` 的 DI (alias/allowed set)
- [x] 验证: `go build` + `go test` 通过
- 参考: C-P0-1

### B4: `tool_executor.go` 高级工具补全 (模块 D) ✅

- [x] kill-tree (进程组管理 + KillAllTrackedProcesses)
- [x] 权限守卫 (AllowExec/AllowWrite/AllowNetwork)
- [x] search/glob 工具 + notebook_edit/mcp stubs
- [x] 路径安全验证 (validateToolPath)
- [x] 验证: 40 tests 全通过
- 参考: P11-D-P0-1
- 备注: 完整 PTY 支持延迟至 P3

### B5: `tool-result-truncation` 移植 (模块 D) ✅

- [x] 新建 `tool_result_truncation.go` (185L)
- [x] 超大工具输出截断逻辑 (30% context share, 400K hard cap)
- [x] 验证: 15 单元测试全通过
- 参考: P11-D-P0-2

---

## Batch C: P1 Session 完整性 (模块 B)

### C1: `resolveSessionStoreKey` 实现

- [x] 去空格 + 特殊键检测 (`global`/`unknown`)
- [x] `agent:` 前缀解析 + `canonicalizeMainSessionAlias()`
- [x] 裸键自动添加 `agent:<defaultAgentId>:` 前缀
- 参考: P11-B2

### C2: `loadCombinedSessionStoreForGateway` 实现

- [x] 多 agent 域存储合并为统一视图
- [x] `sessions.list` 跨 agent 展示
- 参考: P11-B3

### C3: `sessions.patch` 缺失字段补全

- [x] 添加: `verboseLevel`, `reasoningLevel`, `elevatedLevel`, `ttsAuto`, `groupActivation`, `subject`, `queueMode`
- 参考: P11-B4

### C4: `internal/sessions/` 类型重复清理

- [ ] ⏳ deferred — 需要更广泛的 import 链分析
- 参考: P11-B5

---

## Batch D: P1 Gateway 方法 + Agent Runner (模块 A + D)

### D1: `usage` 实现 (模块 A) ✅

- [x] session discovery + cost aggregation
- [x] 模型/供应商/agent/频道/日期维度聚合
- [x] `costUsageCache` 30s TTL 缓存实现
- [x] **D-α 审计修复 (2026-02-17)**: byModel/byProvider/byAgent/tools 聚合从空数组改为真实数据；`uniqueTools` 从硬编码 0 改为 `len(globalToolNames)`；JSONL 解析器增加 model/provider/tool 名提取
- 文件: `server_methods_usage.go` (~727L) — sessions.usage + timeseries + logs
- 参考: [phase11-gateway-methods-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-gateway-methods-audit.md) DIFF-2

### D2: `agents` CRUD (模块 A) ✅

- [x] `agents.create` — 工作区目录创建 + IDENTITY.md 写入
- [x] `agents.update` — 工作区目录更新 + avatar 追加
- [x] `agents.delete` — 配置清除 + 可选文件删除 (workspace/agent/sessions)
- [x] **D-α 审计修复 (2026-02-17)**: 配置持久化 (`persistAgentToConfig`/`updateAgentInConfig`/`pruneAgentFromConfig` via `WriteConfigFile`)；新增 `agents.files.list/get/set` 3 个 handler；delete 返回 `removedBindings` 计数
- 文件: `server_methods_agents.go` (~580L)
- 参考: [phase11-gateway-methods-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-gateway-methods-audit.md) DIFF-3

### D3: `send` outbound 管线接入 (模块 A) ✅

- [x] `ChannelOutboundSender` DI 接口 + channel plugin 路由
- [x] `sync.Map` inflight 去重 (TS WeakMap 等价)
- [x] `poll` handler 从 stub 迁移 (空消息列表兜底)
- [x] **D-α 审计修复 (2026-02-17)**: send 参数对齐 TS (`message`/`to`/`idempotencyKey`/`mediaUrl`/`mediaUrls`/`sessionKey`)；poll 参数扩展 (`to`/`question`/`options`/`idempotencyKey`)；响应返回 `runId`/`messageId`/`channel`
- 文件: `server_methods_send.go` (~230L)
- 参考: [phase11-gateway-methods-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-gateway-methods-audit.md) DIFF-4

### D4: `agent.wait` 真实等待机制 (模块 A) ✅

- [x] `AgentCommandWaiter` DI 接口 + context.WithTimeout
- [x] 超时返回 `status: timeout` + runId
- [x] DI 未注入时兜底立即返回 `status: completed`
- [x] **D-α 审计修复 (2026-02-17)**: 默认超时从 5min(秒) 改为 30s(毫秒)；`AgentCommandSnapshot` 返回 `startedAt`/`endedAt`/`error` 字段
- 文件: `server_methods_agent.go` (~120L)
- 参考: [phase11-gateway-methods-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-gateway-methods-audit.md) P1-5

### D5: `system-prompt.ts` 缺失段落补全 (模块 D) ✅

- [x] 补全 17 个段落: Tooling/Safety/Memory/Messaging/Heartbeats 等
- [x] Go `prompt.go` 279L → ~380L + `prompt_sections.go` + `prompt_sections2.go`
- [x] 深度审计修补 9 处 TS 差异 (2026-02-17)：Memory low-confidence / Docs URL / ReplyTags whitespace / SilentReplies ❌ 案例 / Heartbeats 额外规则 / Reactions 详细 bullets / ReasoningFormat 完整示例 / Workspace Files 段落
- 参考: [phase11-agent-runner-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-agent-runner-audit.md) D-P1-1

### D6: `pi-embedded-subscribe` 流式订阅层 (模块 D) ✅

- [x] `subscribe.go` (220→314L) — 类型定义 + SubscribeState + StripBlockTags + usage/compaction
- [x] `subscribe_handlers.go` (280→448L) — 10 个事件 handler + SubscribeContext + dispatcher
- [x] `subscribe_test.go` 7 测试通过
- [x] **D-γ 深度审计修复 (2026-02-17)**: 覆盖率 40%→80%
  - Fix-5: `InlineCodeState` 码块感知 — `TagStripState` 增加反引号跟踪 (S5 P1)
  - Fix-6: `text_delta/start/end` 子事件处理 + delta 补偿逻辑 (S4 P1)
  - Fix-7: `subscribe_directives.go` (82L, **新建**) — `ParseReplyDirectives` + reasoning 流回调 (S2+S3 P1)
  - Fix-8: `OnAssistantStart` 回调 (S7) + block reply 去重 (S8) + assistant text 去重 (S9) (P2)
  - **延迟**: S6 rawStream 调试日志 / S10 promoteThinkingTagsToBlocks
- 参考: [phase11-agent-runner-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-agent-runner-audit.md) D-P1-2

### D7: `run/images.ts` 图片注入 (模块 D) ✅

- [x] `images.go` (280→430L) — 4 种正则检测 + base64 加载 + sandbox 校验 + 历史扫描
- [x] `images_test.go` 8 测试通过
- [x] **D-γ 深度审计修复 (2026-02-17)**: 覆盖率 65%→90%
  - Fix-3: "N files" 汇总跳过 + `file://localhost` 格式支持 + `mediaAttachedRE` 改进 (I1 P1)
  - Fix-4: URL ref 拒绝 + sandbox 优先解析 + array content block 支持 + `SanitizeLoadedImages` 后处理 (I2+I3+I4 P1/P2)
  - **额外**: `ExtractTextFromHistoryMessage` 支持 array content blocks
  - **额外**: `messageHasImageContent` 检查跳过已有图片的消息
  - **延迟**: `loadWebMedia` 图片优化 (EXIF旋转/JPEG压缩) — 非核心功能
- 参考: [phase11-agent-runner-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-agent-runner-audit.md) D-P1-3

### D8: `google.ts` Gemini 特殊处理 (模块 D) ✅

- [x] `google.go` (460→456L) — schema 清洗 + turn ordering + session 消毒 + thinking 校验 + model snapshot
- [x] `google_test.go` 10 测试通过
- [x] **D-γ 深度审计修复 (2026-02-17)**: 覆盖率 85%→95%
  - Fix-1: `ContentBlock.ThinkingSignature` 签名字段 + 验证逻辑 (`types.go` 修改) (G1 P2)
  - Fix-2: `transcript_repair.go` (210L, **新建**) — `SanitizeToolCallInputs` + `SanitizeToolUseResultPairing`，接入 `SanitizeSessionHistory` 管线 (G4 P2)
  - **额外**: `SanitizeSessionHistory` 管线 stub 全部填充（阶段3a+3b）
  - **延迟**: G3 session marker 持久化 / G5 compactionFailureEmitter
- 参考: [phase11-agent-runner-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-agent-runner-audit.md) D-P1-4

### D9: 全局 `activeRuns` 追踪 (模块 D) ✅

- [x] 实现全局 `activeRuns` Map 防止并发 run 冲突
- [x] `active_runs.go` (~135L) + `active_runs_test.go` (6 测试) + 接入 `run.go`
- [x] 深度审计补充 `IsStreaming()` 方法 (对应 TS `isEmbeddedPiRunStreaming`)
- 参考: [phase11-agent-runner-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-agent-runner-audit.md) D-P1-5

---

## Batch E: P1 AutoReply 管线补全 + WS 安全 (模块 C + E)

### E1: queue/* followup 系统 (模块 C) ✅

- [x] `queue/` 子包: types + state + enqueue + drain + directive + normalize + settings + cleanup
- [x] ~867L Go 等价（TS 8 文件 679L + helpers 152L = 831L）
- [x] 深度审计已完成 (2026-02-18)
- 参考: C-P1-1

**新增 Go 文件 (7)：**

| Go 文件 | TS 对照 | Go 行数 | 核心功能 |
|---------|---------|---------|---------|
| `queue_helpers.go` | `utils/queue-helpers.ts` (152L) | 179L | ElideQueueText, BuildQueueSummaryLine, ApplyQueueDropPolicy, WaitForDebounce, BuildQueueSummaryPrompt, BuildCollectPrompt, HasCrossChannelItems |
| `queue_normalize.go` | `queue/normalize.ts` (45L) | 50L | NormalizeQueueMode (9 别名→5 模式), NormalizeQueueDropPolicy (4 别名→3 策略) |
| `queue_state.go` | `queue/state.ts` (77L) | 137L | FollowupQueueState + 全局 map[string]*State + sync.RWMutex + GetFollowupQueue/ClearFollowupQueue/deleteFollowupQueue |
| `queue_enqueue.go` | `queue/enqueue.ts` (70L) | 101L | EnqueueFollowupRun (路由级去重 messageId+prompt + ApplyQueueDropPolicy) + GetFollowupQueueDepth |
| `queue_drain.go` | `queue/drain.ts` (136L) | 250L | ScheduleFollowupDrain (goroutine 异步消费) + drainCollectMode (跨频道检测→逐条/合并) + summarize overflow |
| `queue_settings.go` | `queue/settings.ts` (69L) | 170L | ResolveQueueSettings (inline→session→channel config→global config→default 5 级优先级链) |
| `queue_cleanup.go` | `queue/cleanup.ts` (30L) | 57L | ClearSessionQueues (批量清理 + 去重 + 统计) |

**修改 Go 文件 (2)：**

| 文件 | 变更内容 |
|------|---------|
| `followup_runner.go` | 新增 `MessageID`, `SummaryLine`, `EnqueuedAt` 字段；新增 `AgentDir`, `ElevatedLevel` 字段；`BashElevated` 从 `bool` 升级为 `*BashElevatedConfig` 结构体 (Enabled/Allowed/DefaultLevel) |
| `queue_directive.go` | 新增 `default`/`clear` 重置别名 + `=` 分隔符支持 (e.g. `cap=5`) |

**测试文件 (1)：**

| 文件 | 测试数 | 覆盖范围 |
|------|--------|---------|
| `queue_test.go` | 17 | NormalizeQueueMode (9), NormalizeQueueDropPolicy (7), ElideQueueText, GetFollowupQueue 创建+复用, ClearFollowupQueue, EnqueueFollowupRun 去重 (messageId+prompt), ApplyQueueDropPolicy (new/old/summarize), HasCrossChannelItems (同/跨频道), BuildQueueSummaryPrompt (无/有溢出), ResolveQueueSettings (默认/inline 覆盖), ClearSessionQueues, FollowupRun 字段完整性 |

**深度审计 TS↔Go 逐函数对照结果：**

| TS 函数 | Go 函数 | 对齐 | 备注 |
|---------|---------|------|------|
| `normalizeQueueMode` (26L) | `NormalizeQueueMode` | ✅ 完全对齐 | 9 别名映射一致 |
| `normalizeQueueDropPolicy` (15L) | `NormalizeQueueDropPolicy` | ✅ 完全对齐 | 4 别名映射一致 |
| `getFollowupQueue` (35L) | `GetFollowupQueue` | ✅ 完全对齐 | settings 更新 + 创建逻辑 + 默认值链一致 |
| `clearFollowupQueue` (17L) | `ClearFollowupQueue` | ✅ 完全对齐 | items+droppedCount 清零 + map 删除 |
| `isRunAlreadyQueued` (19L) | `isRunAlreadyQueued` | ✅ 完全对齐 | 路由上下文 (channel+to+accountId+threadId) + messageId/prompt 去重 |
| `enqueueFollowupRun` (31L) | `EnqueueFollowupRun` | ✅ 完全对齐 | 去重→drop policy→入队 流程一致 |
| `getFollowupQueueDepth` (10L) | `GetFollowupQueueDepth` | ✅ 完全对齐 | trim + nil 安全 |
| `scheduleFollowupDrain` (123L) | `ScheduleFollowupDrain` | ✅ 逻辑对齐 | TS async IIFE → Go goroutine；TS try/catch/finally → Go defer/recover |
| `resolveQueueSettings` (36L) | `ResolveQueueSettings` | ✅ 逻辑对齐 | 5 级优先级链一致，TS `resolvePluginDebounce` 延迟（Go 无 channel plugin 系统） |
| `clearSessionQueues` (18L) | `ClearSessionQueues` | ⚠️ 部分对齐 | TS `clearCommandLane(resolveEmbeddedSessionLane(key))` 延迟（Go 无 command-lane 系统），`LaneCleared` 始终 0 |
| `elideText` (6L) | `ElideQueueText` | ✅ 完全对齐 | rune 安全截断 |
| `buildSummaryLine` (4L) | `BuildQueueSummaryLine` | ✅ 完全对齐 | 清理+截断 |
| `shouldSkipQueueItem` (5L) | `ShouldSkipQueueItem` | ✅ 完全对齐 | 泛型去重回调 |
| `applyQueueDropPolicy` (29L) | `ApplyQueueDropPolicy` | ✅ 完全对齐 | old/new/summarize 3 策略 + 摘要行限制 |
| `waitForQueueDebounce` (15L) | `WaitForDebounce` | ✅ 完全对齐 | TS setTimeout 递归 → Go `time.After` + `select` 循环 |
| `buildQueueSummaryPrompt` (18L) | `BuildQueueSummaryPrompt` | ✅ 完全对齐 | 标题 + 摘要行 + 消耗 droppedCount |
| `buildCollectPrompt` (10L) | `BuildCollectPrompt` | ✅ 完全对齐 | title + summary + renderItem |
| `hasCrossChannelItems` (20L) | `HasCrossChannelItems` | ✅ 完全对齐 | key 归一化 + 多 key 检测 |
| `extractQueueDirective` (50L) | `ExtractQueueDirective` | ⚠️ 结构差异 | Go 用固定正则 vs TS 用 tokenizer + `consumed` 偏移量精确切割；Go 已添加 `default`/`clear`/`=` 支持但未使用 `NormalizeQueueMode` |
| `parseQueueDirectiveArgs` (91L) | *(内联到 ExtractQueueDirective)* | ⚠️ 未分离 | TS 为独立函数，Go 用 regex 内联解析 |

**隐藏依赖审计 (7 类)：**

| 类别 | 审计结果 |
|------|---------|
| ① npm 包黑盒行为 | `parseDurationMs` 用于 directive debounce 解析 — Go 用手动 `parseDebounceMs` 替代，支持 ms/s/m 单位，TS 还支持 h/d 等更多单位 |
| ② 全局状态/单例 | `FOLLOWUP_QUEUES` Map → Go `followupQueues` map + `followupQueuesMu` sync.RWMutex ✅ |
| ③ 事件总线/回调链 | drain.ts `runFollowup` 回调 → Go 同等函数参数 ✅ |
| ④ 环境变量 | 无直接环境变量依赖 ✅ |
| ⑤ 文件系统 | 无直接文件系统操作 ✅ |
| ⑥ 协议/消息格式 | `originatingThreadId` TS 类型为 `string \| number`，Go 统一为 `string`，drain.ts 用 `typeof threadId === "number"` 判断有效性 — Go 用 `threadId != ""` 替代，功能等价（因 Go 端调用者已预转为 string） |
| ⑦ 错误处理 | TS async void + console.error → Go goroutine + defer/recover + log.Printf ✅ |

**已知 Deferred 项 (2)：**

1. **command-lane 清理**: TS `clearCommandLane(resolveEmbeddedSessionLane(key))` 依赖 `process/command-queue.ts`，Go 端 `ClearSessionQueues.LaneCleared` 始终返回 0
2. **channel plugin debounce**: TS `resolvePluginDebounce` 依赖 `channels/plugins/index.ts` → `getChannelPlugin(channelKey).defaults.queue.debounceMs`，Go 端直接跳过（用 config-only 回退）

**审计额外发现 (2)：**

1. **directive.ts 结构差异**: Go `ExtractQueueDirective` 使用固定正则+硬编码 switch，TS 使用 tokenizer 逐 token 扫描 + `normalizeQueueMode`/`normalizeQueueDropPolicy` 函数。Go 实现在功能上覆盖了主要场景，但以下边界行为不同：
   - TS 在遇到第一个无法识别的 token 时立即停止解析 → Go 会匹配后续行的 regex（可能误匹配）
   - TS `parseDurationMs` 支持 `1h30m` 等复合时长 → Go 仅支持 `ms`/`s`/`m` 单位
   - **建议**：后续重构 `queue_directive.go` 为 tokenizer 架构（P2 优先级）
2. **FollowupRunParams 字段缺失**: 深度审计发现缺失 `AgentDir`、`ElevatedLevel`、`BashElevatedConfig` — 已修复

**验证结果：**

```
✅ go build ./...        — 编译通过
✅ go vet ./...          — 无警告
✅ go test -race         — 31/31 tests passed (1.073s)
```

### E2: commands-data 缺失命令 (模块 C) ✅

- [x] 28 个命令定义全量对齐 TS `buildChatCommands()`
- [x] 13 个命令补充 `ArgsParsing: ArgsParsingPositional`
- [x] `think` 命令 `level` 参数改用 `ChoicesProvider` 动态回调
- [x] `RegisterCommand` 新增 `InvalidateTextAliasCache()` 调用
- [x] 验证: `go build && go vet && go test -race` 通过
- 参考: C-P1-2

### E3: commands-registry 缺失函数 (模块 C) ✅

- [x] `ListChatCommandsForConfig()` 按配置过滤
- [x] `ResolveCommandArgChoices()` 支持静态 + 动态 ChoicesProvider
- [x] text alias 映射缓存 (`GetTextAliasMap` / `InvalidateTextAliasCache` / `MaybeResolveTextAlias`)
- [x] `ResolveNativeName()` 含 discord tts→voice 覆写
- [x] `BuildSkillCommandDefinitions()` 技能命令→命令定义转换
- [x] `ListNativeCommandSpecsForConfig()` 按配置过滤 native 命令
- [x] `ResolveCommandArgMenu()` 参数菜单解析
- [x] `IsNativeCommandSurface()` surface 判定
- [x] 验证: 10 个新单测 + 全量 `go test -race` 通过
- 参考: C-P1-3

### E4: block-streaming 管线 (模块 C) ✅

- [x] `block_reply_pipeline.go` (190L) — 管线核心（去重 + 超时 abort + coalescer 集成 + 媒体优先发送）
- [x] `block_streaming.go` (83L) — chunking/coalescing 配置解析（from `BlockStreamingCoalesceConfig`）
- [x] `block_reply_coalescer.go` (151L) — 文本合并缓冲器（min/maxChars 阈值 + idle timer + flushOnEnqueue）
- [x] `block_streaming_test.go` (263L) — 17 单测（coalescer 6 + config 3 + pipeline 8）
- [x] 验证: `go build && go vet && go test -race` 全通过
- **延迟**: channel dock registry 联动 — Go 端 channel dock 系统未完全实现，E4 使用 `BlockStreamingCoalesceConfig` + 硬编码默认值替代
- 参考: C-P1-4

### E5: WS connect.challenge nonce 握手 (模块 E) ✅

- [x] `ws_server.go` Phase 0 — 连接建立后发送 `connect.challenge` 事件（UUID nonce + 时间戳）
- [x] `ws_server.go` Phase 1.5 — connect 帧中验证 `device.nonce` 匹配
- [x] `origin_check.go` 新增 `isLocalAddr` 辅助函数
- [x] `ws_nonce_test.go` (67L) — 5 单测（UUID 生成 + 匹配/不匹配 + 空 nonce 兼容 + isLocalAddr）
- [x] `server_test.go` 更新 — 2 个集成测试适配新的 challenge 事件帧
- [x] 验证: `go build && go vet && go test -race` 全通过
- 参考: P11-E-P1-1

### E6: WS 设备认证 (模块 E) ✅

- [x] `device_auth.go` (230L) — 完整 Ed25519 设备认证套件：
  - `BuildDeviceAuthPayload` (v1/v2 签名载荷构建)
  - `VerifyDeviceSignature` (Ed25519 签名验证, 支持 PEM + base64url)
  - `DeriveDeviceIdFromPublicKey` (SHA256 hex)
  - `NormalizeDevicePublicKeyBase64Url`
  - `ValidateDeviceAuth` (完整流程 + v1 回退)
  - `IsSignedAtValid` (±10min 时间戳窗口)
- [x] `ws_server.go` Phase 2.5 — connect 流程集成设备认证检查
- [x] `device_auth_test.go` (236L) — 14 单测（payload v1/v2/auto + deviceId PEM/base64 + sig valid/invalid/wrongKey + timestamp + full flow + edge cases）
- [x] 验证: `go build && go vet && go test -race` 全通过
- **延迟**: 设备配对管理（~500L request/approve/getPaired/token 轮换/持久化）— E6 仅实现签名验证层
- 参考: P11-E-P1-2

### E7: WS Origin 检查 (模块 E) ✅

- [x] `origin_check.go` (102L) — `CheckBrowserOrigin` + 4 辅助函数（parseOriginURL/resolveHostName/isLoopbackHost/isLocalAddr）
- [x] `origin_check_test.go` (118L) — 11 单测覆盖各场景
- [x] 验证: `go build && go vet && go test -race` 全通过
- 参考: P11-E-P1-3

---

## Batch F: P2 体验优化 (全模块) ✅

### F1: `sessions.list` 模型解析与投递归一化 (模块 B) ✅

- [x] `resolveSessionModelRef` + `normalizeSessionDeliveryFields` + `buildGroupDisplayName` → `session_utils.go`（174 新行）
- [x] `handleSessionsList` 使用 3 个函数填充 row 字段 + displayName 4 级回退链
- [x] 14 个单元测试
- [x] 编译验证 + `-race` 通过
- **[EXTRA] F-EXTRA-1**: ✅ displayName 回退链对齐 TS 4 级（displayName → buildGroupDisplayName → label → origin.label），原实现仅 2 级
- **[EXTRA] F-EXTRA-2**: ✅ `LastChannel` 字段恢复（原实现意外删除）
- **延迟**: `lastThreadId` 合并、`normalizeMessageChannel`/`normalizeAccountId` 调用（P2 可接受简化）
- 参考: [phase11-session-mgmt-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-session-mgmt-audit.md) P2-1~P2-5

### F2: AutoReply 体验优化 (模块 C) ✅

- [x] `envelope.go` — 增加 timezone modes (utc/local/iana) + elapsed 时间 + `FormatInboundEnvelope`/`FormatInboundFromLabel`
- [x] `command_auth.go` — 增加 `ProviderID`/`OwnerList`/`SenderID`/`SenderIsOwner`/`From`/`To` 字段 + `ResolveFullCommandAuthorization`
- [x] 7+9 个单元测试
- [x] 编译验证 + `-race` 通过
- **[EXTRA] F-EXTRA-3**: ✅ `FormatInboundFromLabel` 签名对齐 TS（`isGroup/groupLabel/groupId/directLabel/directId/groupFallback`），原实现用 `@prefix` 而非 TS 的 `label id:xxx` 格式
- **延迟**: reply-elevated 提权管线、LINE 指令 + 线程回复（依赖 channel dock 系统）
- 参考: [phase11-autoreply-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-autoreply-audit.md) C-P2-1~5

### F3: WS 协议补全 (模块 E) ✅

- [x] `protocol.go` — `MinSupportedProtocol = 1` 常量
- [x] `ws_server.go` — 版本协商逻辑（`clientMax < ProtocolVersion || clientMin > ProtocolVersion`）+ close code 1002
- [x] `ws_server.go` — `WsServerConfig.HandshakeTimeout` 可配置（默认 30s）
- [x] 编译验证 + `-race` 通过
- **[EXTRA] F-EXTRA-4**: ✅ 协议协商条件修正 — 原实现检查 `MaxProtocol < MinSupportedProtocol`（错误），改为 TS 对齐 `maxProtocol < PROTOCOL_VERSION || minProtocol > PROTOCOL_VERSION`；close code 从 `4010` 改为 `1002`
- 参考: [phase11-ws-protocol-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-ws-protocol-audit.md) E-P2-1~2

### F4: Config UI Hints (模块 F) ✅

- [x] `types_openacosmi.go` — 33 个顶层字段新增 `label` struct tag
- [x] 编译验证通过
- **延迟**: 深层嵌套字段标签（200+ 字段）→ Phase 12+ UI 功能迭代
- 参考: [phase11-config-scope-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-config-scope-audit.md) F-P2-1~3

### F5: Gateway 方法行为对齐 (模块 A) ✅

- [x] `server_methods_channels.go` — channels.status 增加 `probeAt` 时间戳
- [x] `server_methods_chat.go` — chat.send ACK 增加 `ts` 时间戳
- [x] 编译验证 + `-race` 通过
- **延迟**: ErrorCodes 枚举补全（待全量方法完成后统一添加）
- 参考: [phase11-gateway-methods-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-gateway-methods-audit.md) DIFF-5~6

---

## Batch G: P3 延迟功能补全 (部分完成)

> 可独立实现的 3 项已完成，其余需基础设施支撑的保持 stub

### G1: WS 频繁断连修复 — `gateway.tick` 广播 ✅

- [x] 根因分析：Go 缺失 TS `server-maintenance.ts` 的 30s `gateway.tick` 应用层广播
- [x] 新建 `maintenance.go` — `MaintenanceTimers` + tick goroutine (30s 周期)
- [x] 修改 `server.go` — 集成到 `GatewayRuntime` 生命周期 (ready→start, shutdown→stop)
- [x] 新建 `maintenance_test.go` — 2 tests
- 参考: [phase11-ws-protocol-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-ws-protocol-audit.md) 已知问题 #6

### G2: SessionStore TTL 缓存 + 会话元数据 ✅

- [x] `sessions.go` — TTL 缓存 (45s) + mtime 检查 (`isCacheStale`/`reloadIfStale`/`loadFromDiskLocked`)
- [x] `sessions.go` — `UpdateLastRoute()` 更新 channel/to/accountId/threadId delivery context
- [x] `sessions.go` — `RecordSessionMeta()` 从入站消息合并 displayName/subject/channel/groupChannel
- [x] `sessions.go` — `loadFromDisk()` 初始化时设置 `loadedAt`/`mtimeMs` TTL 状态
- [x] 新建 `sessions_ttl_test.go` — 8 tests (TTL 5 + UpdateLastRoute 4 + RecordSessionMeta 3)
- 参考: P11-B1 (TTL), P11-B2 (UpdateLastRoute), P11-B3 (RecordSessionMeta)

### G3: WS 日志子系统 (3 模式) ✅

- [x] 新建 `ws_log.go` (337L) — 对齐 TS `ws-log.ts` (449L)
  - `auto`: 仅错误响应 + 慢请求 (>50ms)
  - `compact`: req/res 对合并，省略重复 connId
  - `full`: 每帧完整元数据
- [x] 工具函数: `ShortID()`, `FormatForLog()`, `SetWsLogStyle()`/`GetWsLogStyle()`
- [x] 新建 `ws_log_test.go` — 21 tests
- 参考: P11-E-P3-1

### 保持为 stub 的延迟项

- [ ] nodes.*(11 方法) / skills.* / devices.*/ cron.* / tts.*/ browser.request / wizard.* / web.login.*
- [ ] AutoReply: bash-command / stage-sandbox-media / reply-reference
- [ ] 模块 D P2: `model-scan.ts` 运行时扫描 + `session-write-lock` + `bash-tools.process.ts` 进程管理 + `pi-embedded-utils.ts` + `pi-embedded-block-chunker.ts`
- [ ] 模块 C 隐依赖: commands-registry 4 个模块级缓存失效机制

### 验证结果

```
✅ go build ./internal/gateway/...  — 编译通过
✅ go test -race -count=1 ./internal/gateway/... — 全通过 (9.1s, 31 个 Batch G 新增测试)
```

---

## 验证检查点

每个 Batch 完成后必须通过：

```bash
cd backend && go build ./... && go vet ./... && go test -race ./...
```

## 统计

| 优先级 | 项数 | 预估 Batch |
|--------|------|-----------|
| P0 | 8 项 | A + B (2 个 Batch) |
| P1 | 21 项 | C + D + E (3 个 Batch) |
| P2 | 16 项 | F (1 个 Batch) |
| P3 | ~25 项 | G (延迟) |
| **合计** | **~70 项** | **6 个活跃 Batch + 1 延迟** |
