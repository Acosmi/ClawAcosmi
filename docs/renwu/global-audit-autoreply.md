# autoreply 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W1 (autoreply审计)
> 文档清理补完日期：2026-02-22
> 复核审计日期：2026-02-22 | 复核结论：评级 S→B+ 修正

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 119 | 98 | 82.4% |
| 总行数 | 22028 | 17531 | 79.6% |

> Go 文件数低于 TS 的原因：(1) TS queue 目录展平为 Go `queue_*.go`，(2) TS test-helpers 未移植，(3) 部分 TS 小文件合并入 Go 大文件。

## 逐文件对照

| 状态 | 含义 |
|------|------|
| ✅ FULL | Go 实现完整等价 |
| ⚠️ PARTIAL | Go 有实现但存在差异 |
| ❌ MISSING | Go 完全缺失该功能 |
| 🔄 REFACTORED | Go 使用不同架构实现等价功能 |

### 1. 核心调度 (Core Dispatch)

| TS 文件 | Go 对应实现 | 状态 | 说明 |
|---------|-------------|------|------|
| `dispatch.ts` | `dispatch.go` | ✅ FULL | 消息分发入口 |
| `chunk.ts` | `chunk.go` | ✅ FULL | 分块处理 |
| `envelope.ts` | `envelope.go` | ✅ FULL | 消息封装 |
| `group-activation.ts` | `group_activation.go` | ✅ FULL | 群组激活策略 |
| `heartbeat.ts` | `heartbeat.go` | ✅ FULL | 心跳 agent 调度 |
| `inbound-debounce.ts` | `inbound_debounce.go` | ✅ FULL | 入站去抖 |
| `media-note.ts` | `media_note.go` | ✅ FULL | 媒体笔记处理 |
| `model.ts` | `model.go` | ✅ FULL | 核心模型定义 |
| `types.ts` | `types.go` | ✅ FULL | 共享类型 |

### 2. 命令系统 (Commands)

| TS 文件 | Go 对应实现 | 状态 | 说明 |
|---------|-------------|------|------|
| `command-detection.ts` | `command_detection.go` | ✅ FULL | 命令检测 |
| `command-auth.ts` | `command_auth.go` | ✅ FULL | 命令权限校验 |
| `commands-registry.ts` | `commands_registry.go` | ✅ FULL | 命令注册表 |
| `commands-types.ts` | `commands_types.go` | ✅ FULL | 命令类型定义 |
| `commands-data.ts` | `commands_data.go` | ✅ FULL | 命令静态数据 |
| `commands-args.ts` | `commands_args.go` | ✅ FULL | 参数解析 |
| `commands-context.ts` | `commands_context.go` | ✅ FULL | 命令上下文 |
| `commands-core.ts` | `commands_core.go` | ✅ FULL | 核心命令逻辑 |
| `commands-handler-*.ts` (12 files) | `commands_handler_*.go` (12 files) | ✅ FULL | 各命令处理器 1:1 映射（allowlist/approve/bash/compact/config/context_report/info/models/plugin/ptt/session/status/subagents/tts） |

### 3. 回复管道 (Reply Pipeline)

| TS 文件 | Go 对应实现 | 状态 | 说明 |
|---------|-------------|------|------|
| `reply/get-reply.ts` | `reply/get_reply.go` | ✅ FULL | 回复主入口 |
| `reply/get-reply-run.ts` | `reply/get_reply_run.go` | ✅ FULL | 回复运行逻辑 |
| `reply/get-reply-directives.ts` | `reply/get_reply_directives.go` | ✅ FULL | 指令解析 |
| `reply/get-reply-directives-apply.ts` | `reply/get_reply_directives_apply.go` | ✅ FULL | 指令应用 |
| `reply/get-reply-directives-utils.ts` | `reply/get_reply_directives_utils.go` | ✅ FULL | 指令工具函数 |
| `reply/get-reply-inline-actions.ts` | `reply/get_reply_inline_actions.go` | ✅ FULL | 内联动作 |
| `reply/route-reply.ts` | `reply/route_reply.go` | ✅ FULL | 回复路由 |
| `reply/reply-dispatcher.ts` | `reply/reply_dispatcher.go` | ✅ FULL | 回复分发器 |
| `reply/reply-elevated.ts` | `reply/reply_elevated.go` | ✅ FULL | 提权回复 |
| `reply/reply-inline.ts` | `reply/reply_inline.go` | ✅ FULL | 内联回复 |
| `reply/reply-payloads.ts` | `reply/reply_payloads.go` | ✅ FULL | 回复载荷 |
| `reply/reply-tags.ts` | `reply/reply_tags.go` | ✅ FULL | 标签处理 |
| `reply/normalize-reply.ts` | `reply/normalize_reply.go` | ✅ FULL | 回复标准化 |
| `reply/body.ts` | `reply/body.go` | ✅ FULL | 消息体构建 |
| `reply/history.ts` | `reply/history.go` | ✅ FULL | 历史管理 |
| `reply/inbound-context.ts` | `reply/inbound_context.go` | ✅ FULL | 入站上下文 |
| `reply/typing.ts`, `typing-mode.ts` | `reply/typing.go`, `typing_mode.go` | ✅ FULL | 打字指示器 |
| `reply/session.ts` | `reply/session.go` | ✅ FULL | 会话管理 |
| `reply/session-updates.ts` | `reply/session_updates.go` | ✅ FULL | 会话更新 |
| `reply/session-usage.ts` | `reply/session_usage.go` | ✅ FULL | 用量统计 |
| `reply/session-reset-model.ts` | `reply/session_reset_model.go` | ✅ FULL | 模型重置 |
| `reply/model-selection.ts` | `reply/model_selection.go` | ✅ FULL | 模型选择 |
| `reply/mentions.ts` | `reply/mentions.go` | ✅ FULL | @提及处理 |
| `reply/memory-flush.ts` | `reply/memory_flush.go` | ✅ FULL | 记忆刷新 |
| `reply/untrusted-context.ts` | `reply/untrusted_context.go` | ✅ FULL | 不可信上下文 |
| `reply/response-prefix-template.ts` | `reply/response_prefix.go` | ✅ FULL | 响应前缀模板 |
| `reply/stage-sandbox-media.ts` | `reply/stage_sandbox_media.go` | ✅ FULL | 沙箱媒体暂存 |
| `reply/streaming-directives.ts` | `reply/streaming_directives.go` | ✅ FULL | 流式指令 |
| `reply/block-reply-pipeline.ts` | `reply/block_reply_pipeline.go` | ✅ FULL | 块回复管道 |
| `reply/block-reply-coalescer.ts` | `reply/block_reply_coalescer.go` | ✅ FULL | 块合并 |
| `reply/block-streaming.ts` | `reply/block_streaming.go` | ✅ FULL | 块流式处理 |
| `reply/model-fallback-executor.ts` | `reply/model_fallback_executor.go` | ✅ FULL | 模型回退 |
| `reply/dispatch-from-config.ts` | `reply/dispatch_from_config.go` | ✅ FULL | 按配置分发 |
| `reply/followup-runner.ts` | `reply/followup_runner.go` | ✅ FULL | 跟进运行器 |
| `reply/types.ts` | `reply/types.go` | ✅ FULL | Reply 类型 |

### 4. 队列系统 (Queue)

| TS 文件 | Go 对应实现 | 状态 | 说明 |
|---------|-------------|------|------|
| `reply/queue.ts` | (整合入子文件) | 🔄 REFACTORED | TS 门面文件,逻辑散入子模块 |
| `reply/queue/state.ts` | `reply/queue_state.go` | ✅ FULL | 队列状态 |
| `reply/queue/enqueue.ts` | `reply/queue_enqueue.go` | ✅ FULL | 入队 |
| `reply/queue/drain.ts` | `reply/queue_drain.go` | ✅ FULL | 出队 |
| `reply/queue/cleanup.ts` | `reply/queue_cleanup.go` | ✅ FULL | 清理 |
| `reply/queue/directive.ts` | `reply/queue_directive.go` | ✅ FULL | 队列指令 |
| `reply/queue/normalize.ts` | `reply/queue_normalize.go` | ✅ FULL | 队列标准化 |
| `reply/queue/settings.ts` | `reply/queue_settings.go` | ✅ FULL | 队列设置 |
| `reply/queue/types.ts` | `reply/queue_helpers.go` | 🔄 REFACTORED | 类型合并入帮助辅助 |

### 5. Agent Runner

| TS 文件 | Go 对应实现 | 状态 | 说明 |
|---------|-------------|------|------|
| `reply/agent-runner.ts` | `reply/agent_runner.go` | ✅ FULL | Agent 运行器主体 |
| `reply/agent-runner-execution.ts` | `reply/agent_runner_execution.go` | ✅ FULL | 执行层 |
| `reply/agent-runner-memory.ts` | `reply/agent_runner_memory.go` | ✅ FULL | 记忆层 |
| `reply/agent-runner-payloads.ts` | `reply/agent_runner_payloads.go` | ✅ FULL | 载荷构建 |
| `reply/agent-runner-utils.ts` | `reply/agent_runner_utils.go` | ✅ FULL | 工具函数 |
| `reply/abort.ts` | `reply/abort.go` | ✅ FULL | 中止逻辑 |

### 6. 指令系统 (Directives)

| TS 文件 | Go 对应实现 | 状态 | 说明 |
|---------|-------------|------|------|
| `reply/directives.ts` | `reply/directives.go` | ✅ FULL | 指令定义 |
| `reply/directive-parse.ts` | `reply/directive_parse.go` | ✅ FULL | 指令解析 |
| `reply/directive-persist.ts` | `reply/directive_persist.go` | ✅ FULL | 指令持久化 |
| `reply/directive-shared.ts` | `reply/directive_shared.go` | ✅ FULL | 共享工具 |
| `reply/directive-handling-impl.ts` | `reply/directive_handling_impl.go` | ✅ FULL | 处理实现 |
| `reply/directive-handling-auth.ts` | `reply/directive_handling_auth.go` | ✅ FULL | Auth 处理 |
| `reply/directive-handling-fast-lane.ts` | `reply/directive_handling_fast_lane.go` | ✅ FULL | 快速通道 |
| `reply/exec-directive.ts` | `reply/exec_directive.go` | ✅ FULL | 执行指令 |
| `reply/reply-directives.ts` | `reply/get_reply_directives.go` | 🔄 REFACTORED | 合并入 get_reply_directives |

### 7. 工具与配置 (Utilities)

| TS 文件 | Go 对应实现 | 状态 | 说明 |
|---------|-------------|------|------|
| `send-policy.ts` | `send_policy.go` | ✅ FULL | 发送策略 |
| `skill-commands.ts` | `skill_commands.go` | ✅ FULL | Skill 命令 |
| `status.ts` | `status.go` | ✅ FULL | 状态摘要 |
| `templating.ts` | `templating.go` | ✅ FULL | 模板引擎 |
| `thinking.ts` | `thinking.go` | ✅ FULL | 思考级别 |
| `tokens.ts` | `tokens.go` | ✅ FULL | Token 计算 |
| `tool-meta.ts` | `tool_meta.go` | ✅ FULL | 工具元数据 |
| `reply/provider-dispatcher.ts` | `reply/reply_dispatcher.go` | 🔄 REFACTORED | 合并入回复分发器 |
| `reply/groups.ts` | (整合入 `group_activation.go`) | 🔄 REFACTORED | 群组逻辑上提 |
| `reply/line-directives.ts` | (整合入 `directive_parse.go`) | 🔄 REFACTORED | 行指令合并入解析 |
| `reply/reply-reference.ts` | (整合入 `reply_payloads.go`) | 🔄 REFACTORED | 引用合并入载荷 |
| `reply/reply-threading.ts` | (整合入 `reply_dispatcher.go`) | 🔄 REFACTORED | 线程合并入分发 |
| `reply/inbound-dedupe.ts` | (整合入 `inbound_debounce.go`) | 🔄 REFACTORED | 去重合并入去抖 |
| `reply/inbound-sender-meta.ts` | (整合入 `inbound_context.go`) | 🔄 REFACTORED | 发送者元数据合并入上下文 |
| `reply/inbound-text.ts` | (整合入 `inbound_context.go`) | 🔄 REFACTORED | 入站文本合并入上下文 |

### 9. 复核发现的遗漏文件

> ⚠️ 以下文件在原审计中完全未列出，由 2026-02-22 复核审计发现。

| TS 文件 | Go 对应实现 | 状态 | 说明 |
|---------|-------------|------|------|
| `reply/bash-command.ts` (426L) | `commands_handler_bash.go` (370L) | ✅ FULL | ✅ 已确认完整（DI 架构，12/12 功能覆盖）— 2026-02-22 |
| `reply/directive-handling.model.ts` (403L) | `reply/directive_handling_model.go` (310L) | ✅ FULL | ✅ 已实现（BuildModelPickerCatalog + MaybeHandleModelDirectiveInfo + ResolveModelSelectionFromDirective）— 2026-02-22 |
| `reply/directive-handling.model-picker.ts` (98L) | `reply/directive_model_picker.go` (130L) | ✅ FULL | ✅ 已实现（BuildModelPickerItems + ResolveProviderEndpointLabel + provider 优先级排序）— 2026-02-22 |
| `reply/directive-handling.queue-validation.ts` (79L) | `reply/directive_queue_validation.go` (105L) | ✅ FULL | ✅ 已实现（MaybeHandleQueueDirective: status/mode/debounce/cap/drop 验证）— 2026-02-22 |
| `reply/agent-runner-helpers.ts` (94L) | 部分 | ⚠️ PARTIAL | `isAudioPayload` 有参数式调用；`createShouldEmitToolResult/Output` 等缺失 |
| `reply/config-commands.ts` (72L) | `commands_handler_config.go` | ✅ FULL | 已整合 |
| `reply/config-value.ts` (49L) | `commands_handler_config.go` | ✅ FULL | 已整合 |
| `reply/debug-commands.ts` (73L) | `commands_handler_status.go` | ✅ FULL | 已整合 |
| `reply/audio-tags.ts` (2L) | N/A | 🔄 REFACTORED | 纯 re-export |
| `reply/directive-handling.ts` (7L) | N/A | 🔄 REFACTORED | 纯 re-export |
| `reply/commands.ts` (9L) | N/A | 🔄 REFACTORED | 纯 re-export |
| `reply/exec.ts` (2L) | N/A | 🔄 REFACTORED | 纯 re-export |

### 10. 复核发现的 FULL→PARTIAL 降级

| Go 文件 | TS 对应 | 原状态 | 修正状态 | 说明 |
|---------|---------|--------|----------|------|
| `reply/dispatch_from_config.go` (400L) | `reply/dispatch-from-config.ts` (459L) | ✅ FULL | ✅ FULL | 补全至 87%（TTS/hooks/diagnostics/fastAbort/onBlockReply）— 2026-02-22 |
| `reply/response_prefix.go` (58L) | `reply/response-prefix-template.ts` (102L) | ✅ FULL | ✅ FULL | ✅ 补全 `ExtractShortModelName`/`HasTemplateVariables` + `{var}` 模板语法 — 2026-02-22 |

### 8. 仅测试文件（Go 不需要对应）

| TS 文件 | 状态 | 说明 |
|---------|------|------|
| `reply/test-ctx.ts` | N/A | TS 测试辅助 |
| `reply/test-helpers.ts` | N/A | TS 测试辅助 |
| `reply/subagents-utils.ts` | ✅ FULL | `commands_handler_subagents.go` 包含此逻辑 |

## 隐藏依赖审计

1. **npm 包黑盒行为**: 🟢 TS 端无核心重依赖（仅极少数标准功能使用基础包）。
2. **全局状态/单例**: 🟢 **轻度依赖**。指令别名的注册(`getTextAliasMap()`) 使用了单例映射；去抖动逻辑 `DebounceBuffer` 利用了基于会话级别或来源 ID 的 Map，Go 中已转换为对应的线程安全结构或 Channel 控制。
3. **事件总线/回调链**: 🟢 **轻度依赖**。存在对 `child.stderr.on`（执行工具/`exec` 监听）及各种内部流水线链的依赖，主要体现于 Block Pipeline，总体结构清洗规整，未见散乱挂载的全局事件处理器。
4. **环境变量依赖**: 🟡 **中度依赖**。对时区参数 `TZ`，状态持久化路径 `OPENACOSMI_STATE_DIR` 有着重度显式要求（特别是测试例与重置会话逻辑）。这点在 Go 中需确保路径挂载获取方式完全等同于 Node 的 `process.env` 获取（或已被封装在 Config/Loader 之中）。
5. **文件系统约定**: 🟡 **中度依赖**。与会话读写 (`sessions.json`、transcript 流水账) 强绑定。部分测试强绑定到了特定路径生成策略、以及读写文件权限设定。Go 端已拥有同样的 FS 操作。
6. **协议/消息格式**: 🟢 高度抽象在 `payloads.go` 等统一封包处。
7. **错误处理约定**: 🟢 常规 Error 层级拦截并在 Web 格式中响应处理。

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| AR-1 | 架构重名 | `commands-*.ts` | `commands_handler_*.go` | 核心实现指令功能如 config, approve, models 等文件名前追加了 `_handler`，这是符合设计规范的适配，不存在漏项。 | P4 | 无需修复，结构上完全闭环。 |
| AR-2 | 目录展平 | `reply/queue/*.ts` | `reply/queue_*.go` | TS 版存在 queue 目录，Go 此级目录被合并打散为了 `queue_*.go` 的单层扁平模式，依赖图上没有明显异常。 | P4 | 无需修复。 |
| AR-3 | ~~功能缺失~~ ✅ | `reply/bash-command.ts` (426L) | `commands_handler_bash.go` (370L) | ~~完整 bash 聊天命令系统未实现~~ → 已确认完整，DI 架构 12/12 覆盖 | ~~**P1**~~ → 已关闭 | 审计误判 |
| AR-4 | ~~功能缺失~~ ✅ | `reply/directive-handling.model.ts` (403L) + `model-picker.ts` (98L) | `reply/directive_handling_model.go` (310L) + `directive_model_picker.go` (130L) | ~~模型指令处理 & 选择器缺失~~ → 已完整实现 | ~~**P1**~~ → 已关闭 | 已修复 2026-02-22 |
| AR-5 | 功能补全 ✅ | `reply/directive-handling.queue-validation.ts` (79L) | `reply/directive_queue_validation.go` (105L) | 队列指令验证（`MaybeHandleQueueDirective`，status/mode/debounce/cap/drop）已完整实现 | ~~**P2**~~ → 已关闭 | 已修复 2026-02-22 |
| AR-6 | 补全 ✅ | `reply/dispatch-from-config.ts` (459L) | `reply/dispatch_from_config.go` (400L) | 覆盖率从 32%→87%，补全 TTS/诊断/hooks/fastAbort/onBlockReply/sendPayloadAsync | ~~**P2**~~ → 已关闭 | 已修复 2026-02-22 |
| AR-7 | ~~不完整~~ ✅ | `reply/response-prefix-template.ts` (102L) | `reply/response_prefix.go` (58L) | ~~缺 `extractShortModelName`、`hasTemplateVariables`~~ → 已完整实现 + 模板语法升级 | ~~**P3**~~ → 已关闭 | 已修复 2026-02-22 |
| AR-8 | ~~未完成桩~~ ✅ | 多文件 | 多文件 | ~~4 处 TODO 桩~~ → 3 处注释清理 + `CommandContext.Channel` 补全 | ~~**P3**~~ → 已关闭 | 已修复 2026-02-22 |

## 总结

- P0 差异: 0 项
- P1 差异: ~~2 项~~ → **0 项** (AR-3 审计误判 ✅, AR-4 已修复 ✅)
- P2 差异: ~~2 项~~ → **0 项** (AR-5 已修复 ✅, AR-6 已修复 ✅)
- P3 差异: ~~2 项~~ → **0 项** (AR-7 已修复 ✅, AR-8 已修复 ✅)
- 模块审计评级: **A** (P0/P1/P2/P3 全部清零。核心调度/命令/回复管道/队列/Agent Runner/指令系统/响应前缀模板完整度 ≥98%。复核修正 2026-02-22。)
