# Phase 8 任务清单 — P7D 延迟项（Ollama → Phase 11）

> 上下文：[phase7-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase7-bootstrap.md)
> 延迟项：[deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)
> 最后更新：2026-02-15 (W1-W4 全部完成 ✅，Phase 8.1 Ollama 延至 Phase 11)

---

## Window 1：基础层 + 指令处理 ✅

### W1-A: 指令处理（directive-handling 系列） ✅

- [x] `reply/directives.go` — 通用指令提取器（/think, /verbose, /elevated, /reasoning, /status）
- [x] `reply/directive_parse.go` — 内联指令链式解析 + IsDirectiveOnly
- [x] `reply/directive_shared.go` — 格式化工具（SystemMark、Ack、ElevatedEvent）
- [x] `reply/exec_directive.go` — /exec 指令提取（host/security/ask/node 选项）
- [x] `reply/queue_directive.go` — /queue 指令提取（mode/debounce/cap/drop 选项）

### W1-B: 分发/路由 ✅

- [x] `reply/dispatch_from_config.go` — 完善（音频检测 + 路由框架 + DI 接口）
- [x] `reply/route_reply.go` — DI 回复路由器 + stub 实现

### W1-C: 打字指示 + 后续运行 ✅

- [x] `reply/typing.go` — 打字控制器（TTL + 密封 + goroutine 循环）
- [x] `reply/typing_mode.go` — 打字模式解析器 + 信号器
- [x] `reply/followup_runner.go` — 后续运行器骨架（类型完整，逻辑延迟到 W2）

### W1-D: 提及 + 历史 ✅

- [x] `reply/mentions.go` — 提及匹配 + 去除 + 结构前缀去除
- [x] `reply/history.go` — 线程安全 HistoryMap + LRU 驱逐 + 上下文构建

### 验证 ✅

- [x] `go build ./...` 通过
- [x] `go vet ./internal/autoreply/reply/...` 通过
- [x] `go test -race ./internal/autoreply/reply/...` 通过（35+ 测试用例）

---

## Window 2：Agent Runner + get-reply ✅

### W2-A: agent-runner 核心 ✅

- [x] `reply/agent_runner.go` — `RunReplyAgent()` 主编排（内存冲刷→执行→载荷→typing）
- [x] `reply/agent_runner_execution.go` — `AgentExecutor` DI 接口 + `RunAgentTurnWithFallback()`
- [x] `reply/agent_runner_memory.go` — `MemoryFlusher` DI 接口 + `SessionEntry` 类型
- [x] `reply/agent_runner_payloads.go` — `BuildReplyPayloads()` 载荷清洗/过滤
- [x] `reply/agent_runner_utils.go` — 纯工具函数（Usage 格式化、Socket 错误、音频检测）

### W2-B: get-reply 全链路 ✅

- [x] `reply/get_reply.go` — `GetReplyFromConfig()` 顶层编排
- [x] `reply/get_reply_run.go` — `RunPreparedReply()` 消息体准备 + FollowupRun 构建
- [x] `reply/get_reply_directives.go` — `ResolveReplyDirectives()` 指令解析 + 级别解析
- [x] `reply/get_reply_directives_apply.go` — `ApplyInlineDirectiveOverrides()` 指令覆盖
- [x] `reply/get_reply_directives_utils.go` — `ClearInlineDirectives()`
- [x] `reply/get_reply_inline_actions.go` — `HandleInlineActions()` 内联动作

### W2-C: 骨架更新 ✅

- [x] `reply/followup_runner.go` — 填充 Window 2 TODO → 调用 `RunReplyAgent`

### 验证 ✅

- [x] `go vet` 0 错误
- [x] `go test` 47/47 通过（含 16 新测试）

---

## Window 3：命令处理器（~3,965L TS → ~2,200L Go）✅

### W3-A: 核心 + 小文件（5 文件）✅

- [x] `commands_handler_types.go` — 处理器类型 + 4 DI 接口（GatewayCaller, SessionStoreUpdater, BashExecutor, PluginCommandMatcher）
- [x] `commands_context.go` — BuildCommandContext + StripMentionsFromBody
- [x] `commands_core.go` — HandleCommands 核心分发器 + 处理器链
- [x] `commands_handler_bash.go` — /bash, ! 前缀处理器
- [x] `commands_handler_plugin.go` — 插件命令匹配

### W3-B: 中型处理器（4 文件）✅

- [x] `commands_handler_approve.go` — /approve [allow|always|deny] + 别名解析
- [x] `commands_handler_compact.go` — /compact [full|summary|hard|soft]
- [x] `commands_handler_info.go` — /help, /commands, /status, /whoami, /context
- [x] `commands_handler_ptt.go` — /ptt [start|stop|once|toggle|status|mute|unmute]

### W3-C: 配置/状态/媒体（4 文件）✅

- [x] `commands_handler_status.go` — /debug, /usage, /queue
- [x] `commands_handler_config.go` — /config [get|set|unset|list|reset|validate] + /config temp
- [x] `commands_handler_tts.go` — /tts [on|off|say|provider|maxlen|summarize|status|providers]
- [x] `commands_handler_models.go` — /model [list|set|info|providers|search]

### W3-D: 会话/子代理/白名单（4 文件）✅

- [x] `commands_handler_session.go` — /session [list|switch|new|delete|rename|info|export|usage|stop|restart] + /activation + /send-policy
- [x] `commands_handler_context_report.go` — /cr [full|summary|tokens|history|agent] [-v]
- [x] `commands_handler_subagents.go` — /subagent [list|create|delete|send|info|switch]
- [x] `commands_handler_allowlist.go` — /allowlist [add|remove|check|clear|pair|unpair|pairs|export|import|batch]

### W3 DI 接口（15 个）

`GatewayCaller`, `SessionStoreUpdater`, `BashExecutor`, `PluginCommandMatcher`, `SessionCompactor`, `NodeResolver`, `StatusReplyBuilder`, `ConfigManager`, `ConfigOverrideManager`, `TTSService`, `ModelCatalogProvider`, `SessionManager`, `ContextReportResolver`, `SubagentManager`, `AllowlistManager`

### 验证 ✅

- [x] `go build ./internal/autoreply/...` 通过
- [x] `go vet ./internal/autoreply/...` 通过
- [x] `go test ./internal/autoreply/...` 100 tests 通过（88 新 + 12 已有）

---

## Window 4：集成层 + 文档（~820L TS）✅

### W4-A: status.go ✅

- [x] `status.go` (300L) — StatusDeps DI, FormatTokenCount, FormatContextUsageShort, BuildStatusMessage, BuildHelpMessage, FormatCommandsGrouped
- [x] `status_test.go` — 9 tests PASS

### W4-B: skill_commands.go ✅

- [x] `skill_commands.go` (175L) — SkillCommandDeps DI, NormalizeSkillCommandLookup, FindSkillCommand, ResolveSkillCommandInvocation
- [x] `skill_commands_test.go` — 3 tests PASS

### W4-C: 修补项 ✅

- [x] P7D-9: SanitizeUserFacingText 已在 `helpers/errors.go` L550-600 实现 — 无需额外工作
- [x] P7D-10: abort.go 扩展 59L→196L（ABORT_MEMORY, AbortDeps, TryFastAbortFromMessage, StopSubagentsForRequester, FormatAbortReplyText）

### W4-D: 文档更新 ✅

- [x] `docs/gouji/autoreply.md` — 新增 W4 文件行 + 测试覆盖
- [x] `docs/renwu/phase8-task.md` — W4 checklist 完善
- [x] `docs/renwu/refactor-plan-full.md` — Phase 8 状态更新
- [x] `docs/renwu/deferred-items.md` — P7D-1/2/9/10 标记完成

### 验证 ✅

- [x] `go build ./internal/autoreply/...` 通过
- [x] `go vet ./internal/autoreply/...` 通过
- [x] `go test -race ./internal/autoreply/...` 全部通过（含 12 新测试）
