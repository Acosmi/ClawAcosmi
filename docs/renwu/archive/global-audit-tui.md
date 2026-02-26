# TUI 终端聊天客户端 全局审计报告

> 审计日期：2026-02-19 | 对标文档：`phase5-tui-project.md`
> 审计范围：TS 端 `src/tui/` 全部非测试文件 vs Go 实施方案

## 概览

| 维度 | TS 端 | Go 方案 | 覆盖率 |
|------|-------|---------|--------|
| 非测试文件数 | 24 | 12（规划） | 50% |
| 总行数 | 4,155L | ~3,000L（规划） | 72% |
| 现有 Go 文件 | — | 6（仅 setup wizard） | 0%（聊天功能） |

> [!IMPORTANT]
> Go 端现有 6 个文件（963L）全部为 setup wizard UI 原语，**无任何聊天功能**，需完全重写。

---

## 逐文件对照表

### 核心文件（4 个）

| # | TS 文件 | 行数 | Go 对应 | 状态 | 备注 |
|---|---------|------|---------|------|------|
| 1 | `tui.ts` | 709 | `model.go` (532L) | ✅ W1 DONE | 骨架完成：Model struct 25+ 状态 + Init/Update/View + T-01 stub + T-02 ✅ + T-04 ✅ |
| 2 | `gateway-chat.ts` | 266 | `gateway_ws.go` (713L) | ✅ W1 DONE | 骨架完成：RPC 层 + 10 API 方法 + G-01 ✅ + DEP-01/G-02 ✅ |
| 3 | `tui-command-handlers.ts` | 502 | `commands.go` | ⚠️ PARTIAL | 见差异 C-01~C-04（W4 实现） |
| 4 | `tui-session-actions.ts` | 412 | `session_actions.go` | ⚠️ PARTIAL | 见差异 S-01~S-03（W4 实现） |

### 事件与流式处理（3 个）

| # | TS 文件 | 行数 | Go 对应 | 状态 | 备注 |
|---|---------|------|---------|------|------|
| 5 | `tui-event-handlers.ts` | 247 | `event_handlers.go` | ⚠️ PARTIAL | 见差异 E-01~E-02 |
| 6 | `tui-stream-assembler.ts` | 78 | `stream_assembler.go` | ✅ FULL | 方案已覆盖 |
| 7 | `tui-formatters.ts` | 219 | （未规划独立文件） | ❌ MISSING | 见差异 F-01 |

### 组件文件（7 个）

| # | TS 文件 | 行数 | Go 对应 | 状态 | 备注 |
|---|---------|------|---------|------|------|
| 8 | `components/chat-log.ts` | 104 | `view_chat_log.go` | ⚠️ PARTIAL | 见差异 CL-01 |
| 9 | `components/tool-execution.ts` | 136 | （未规划） | ❌ MISSING | 见差异 TE-01 |
| 10 | `components/searchable-select-list.ts` | 310 | （未规划） | ❌ MISSING | 见差异 SS-01 |
| 11 | `components/filterable-select-list.ts` | 143 | （未规划） | ❌ MISSING | 见差异 SS-01 |
| 12 | `components/fuzzy-filter.ts` | 138 | （未规划） | ❌ MISSING | 见差异 SS-01 |
| 13 | `components/selectors.ts` | 30 | （未规划） | 🔄 REFACTORED | bubbles/list 替代 |
| 14 | `components/user-message.ts` | 20 | `view_message.go` | ✅ FULL | 方案已覆盖 |

### 辅助文件（6 个）

| # | TS 文件 | 行数 | Go 对应 | 状态 | 备注 |
|---|---------|------|---------|------|------|
| 15 | `commands.ts` | 163 | `commands.go` | ⚠️ PARTIAL | 见差异 CMD-01 |
| 16 | `tui-types.ts` | 107 | `model.go` 内嵌 | ✅ W1 DONE | 类型合并到 model（SessionInfo/AgentSummary/TuiOptions/SessionScope） |
| 17 | `tui-overlays.ts` | 19 | `overlays.go` | ✅ FULL | 方案已覆盖 |
| 18 | `tui-waiting.ts` | 51 | `view_status.go` | ✅ FULL | 合并到状态栏 |
| 19 | `tui-status-summary.ts` | 88 | `view_status.go` | ✅ FULL | 合并到状态栏 |
| 20 | `tui-local-shell.ts` | 145 | （未规划） | ❌ MISSING | 见差异 LS-01 |

### 主题文件（2 个）

| # | TS 文件 | 行数 | Go 对应 | 状态 | 备注 |
|---|---------|------|---------|------|------|
| 21 | `theme/theme.ts` | 137 | `theme.go` | ⚠️ PARTIAL | 见差异 TH-01 |
| 22 | `theme/syntax-theme.ts` | 52 | `theme.go` | ⚠️ PARTIAL | 见差异 TH-02 |

### 外部桥接文件（2 个）

| # | TS 文件 | 行数 | Go 对应 | 状态 | 备注 |
|---|---------|------|---------|------|------|
| 23 | `components/assistant-message.ts` | 19 | `view_message.go` | ✅ FULL | 方案已覆盖 |
| 24 | `components/custom-editor.ts` | 60 | `view_input.go` | ⚠️ PARTIAL | 见差异 IN-01 |

### 覆盖统计

| 状态 | 数量 | 占比 |
|------|------|------|
| ✅ FULL | 8 | 33% |
| ⚠️ PARTIAL | 10 | 42% |
| ❌ MISSING | 5 | 21% |
| 🔄 REFACTORED | 1 | 4% |

---

## 差异清单

### P0 差异（功能完全缺失，影响核心流程）

| ID | TS 文件 | Go 文件 | 描述 | 修复方案 |
|----|---------|---------|------|---------|
| F-01 | `tui-formatters.ts` (219L) | 无独立文件 | **格式化工具完全缺失**：`extractThinkingFromMessage`、`extractContentFromMessage`、`composeThinkingAndContent`、`resolveFinalAssistantText`、`extractTextFromMessage`、`isCommandMessage`、`formatTokens`、`formatContextUsageLine`、`asString` 共 9 个函数。stream_assembler 和 event_handlers 均强依赖此模块 | 新建 `formatters.go`（~120L），实现所有 9 个函数 |
| TE-01 | `components/tool-execution.ts` (136L) | 无 | **工具执行渲染完全缺失**：`ToolExecutionComponent` 负责工具调用的实时展示（pending→running→success/error 状态机、参数格式化、输出预览裁剪 12 行），ChatLog 的 `startTool`/`updateToolResult` 依赖此组件。缺失则 agent 工具调用过程不可见 | 在 `view_chat_log.go` 或新建 `view_tool.go` 中实现工具执行渲染 |
| LS-01 | `tui-local-shell.ts` (145L) | 无 | **本地 shell 执行完全缺失**：`!command` 语法允许用户在 TUI 中直接执行本地命令（spawn 子进程、安全确认弹窗、输出截断 40KB）。此功能在 TS 端被 `tui.ts` 的 submit handler 调用 | 新建 `local_shell.go`（~100L），使用 `os/exec` 替代 `child_process.spawn` |

### P1 差异（功能部分缺失，影响重要特性）

| ID | TS 文件 | Go 文件 | 描述 | 修复方案 |
|----|---------|---------|------|---------|
| SS-01 | `searchable-select-list.ts` (310L) + `filterable-select-list.ts` (143L) + `fuzzy-filter.ts` (138L) | 无 | **选择列表组件遗漏**：TS 端自研了 fuzzy filtering + searchable select list + filterable select list 三个组件（共 591L），是模型/agent/session 选择器的基础。Go 方案未说明替代方案 | 使用 `bubbles/list` 内置 fuzzy filtering 替代（联网验证：bubbles/list 已含 fuzzy 过滤，可替代）。需确认 `bubbles/list` 是否支持分层搜索（TS 有 4 层优先级：精确→词界→描述→模糊） |
| CL-01 | `components/chat-log.ts` (104L) | `view_chat_log.go` | **ChatLog 工具追踪缺失**：TS 的 ChatLog 类维护 `toolById` Map 和 `streamingRuns` Map，支持 `startTool`/`updateToolArgs`/`updateToolResult`/`setToolsExpanded` 方法。Go 方案仅提及"聊天消息列表渲染"，未体现工具事件追踪 | `view_chat_log.go` 需增加工具执行追踪状态（Map[toolCallId]ToolExec） |
| IN-01 | `components/custom-editor.ts` (60L) | `view_input.go` | **编辑器自定义行为缺失**：TS 的 CustomEditor 扩展了 pi-tui 的 Editor，添加了自动补全 provider 集成 + 提交回调。Go 方案仅写"输入框 + 自动补全"，未说明如何实现 slash 命令自动补全 | 使用 `bubbles/textarea` + 自定义 keymap，在输入以 `/` 开头时触发补全列表 |
| CMD-01 | `commands.ts` (163L) | `commands.go` | **Slash 命令注册表遗漏**：TS 端 `getSlashCommands()` 返回 20+ 个内置命令 + 动态注册 gateway 自定义命令（`listChatCommandsForConfig`），每个命令支持 `getArgumentCompletions` 参数补全。Go 方案仅写"slash 命令处理"200L，未涵盖动态命令注册和参数补全 | `commands.go` 需实现完整命令注册表 + 参数补全接口 |
| TH-01 | `theme/theme.ts` (137L) | `theme.go` | **主题定义不完整**：TS 端定义了 6 个主题对象（`theme`/`markdownTheme`/`selectListTheme`/`filterableSelectListTheme`/`settingsListTheme`/`editorTheme`/`searchableSelectListTheme`）+ 代码高亮（`cli-highlight`）。Go 方案仅写"lipgloss 主题定义（暗色/亮色）"150L。**代码高亮**是核心 UX 功能 | `theme.go` 需定义完整 lipgloss 样式，代码高亮用 `alecthomas/chroma` |
| TH-02 | `theme/syntax-theme.ts` (52L) | `theme.go` | **语法高亮主题缺失**：TS 端用 `cli-highlight` 的自定义主题映射。Go 端需用 `chroma` 的 style 机制对等 | 合并到 `theme.go`，用 chroma Style builder |

### P2 差异（行为差异，不影响核心功能）

| ID | TS 文件 | Go 文件 | 描述 | 修复方案 |
|----|---------|---------|------|---------|
| T-01 | `tui.ts` L40-78 | `model.go` | `createEditorSubmitHandler` 三路分发（shell/command/message） | ✅ W1 stub 就位（L335-339），W4 完善 |
| T-02 | `tui.ts` L229-250 | `model.go` | `noteLocalRunId`/`forgetLocalRunId`/`isLocalRunId` 本地 run ID 追踪 | ✅ W1 完整实现（L365-396） |
| T-03 | `tui.ts` L271-282 | `model.go` | `updateAutocompleteProvider` 动态更新补全 provider | ⏳ W4 实现（bubbles/textinput suggestion） |
| T-04 | `tui.ts` L287-293 | `model.go` | `formatSessionKey` 截断长 session key 显示 | ✅ W1 完整实现（L419-430） |
| G-01 | `gateway-chat.ts` L106-146 | `gateway_ws.go` | 12 字段 ConnectParams + sendConnect 帧 | ✅ W1 完整实现（L405-438） |
| G-02 | `gateway-chat.ts` L221-266 | `gateway_ws.go` | `resolveGatewayConnection` 6 层 fallback | ✅ W1 完整实现（L600-707，DEP-01 修复） |
| E-01 | `tui-event-handlers.ts` L36-57 | `event_handlers.go` | `pruneRunMap` 实现 run Map 自动修剪（>200 条时裁剪到 150，含 10 分钟过期策略）。Go 方案未提及内存管理 | 实现 goroutine-safe pruning |
| E-02 | `tui-event-handlers.ts` L179-244 | `event_handlers.go` | `handleAgentEvent` 处理 tool 流事件（start/update/result 三阶段）+ lifecycle 事件。需根据 `verboseLevel` 过滤输出 | 关键功能，需完整实现 |
| S-01 | `tui-session-actions.ts` L73-107 | `session_actions.go` | `applyAgentsResult` 从 gateway 返回的 agents 列表中更新 agent 名称映射和默认 agent | 需完整实现 |
| S-02 | `tui-session-actions.ts` L147-226 | `session_actions.go` | `applySessionInfo` 包含 model override 优先级逻辑（覆盖→会话→默认，共 3 层）| 关键状态管理 |
| S-03 | `tui-session-actions.ts` L296-369 | `session_actions.go` | `loadHistory` 含历史消息重建（遍历 history items，分离 user/assistant/system/command/tool 消息）| 需完整实现 |
| C-01 | `tui-command-handlers.ts` L240-463 | `commands.go` | `handleCommand` 包含 20+ 个 slash 命令处理分支（help/status/agent/session/model/think/verbose/reasoning/usage/elevated/activation/abort/new/reset/settings/exit 等）| 逐个实现 |

---

## 隐藏依赖审计

| # | 类别 | 检查结果 | 风险 |
|---|------|---------|------|
| 1 | **npm 包黑盒行为** | `@mariozechner/pi-tui` 提供 TUI/Container/Text/Box/Markdown/Input/SelectList/SettingsList 等 12 个 UI 原语。Go 端需用 bubbletea+bubbles+lipgloss **全部替代** | ⚠️ 高 — pi-tui 的 Markdown 渲染器含代码高亮，Go 端需 glamour 或自研 |
| 2 | **npm 包：chalk** | `theme.ts` 和 `filterable-select-list.ts` 使用 chalk 做 ANSI 颜色。Go 端用 lipgloss 完全替代 | ✅ 低 |
| 3 | **npm 包：cli-highlight** | `theme.ts` 用 `highlight()` + `supportsLanguage()` 做代码高亮。Go 端用 `alecthomas/chroma` 替代 | ⚠️ 中 — 需新增依赖 |
| 4 | **外部模块依赖** | TS 端引用 14 个外部模块：`config/config`、`gateway/client`、`gateway/protocol`、`gateway/call`、`agents/tool-display`、`agents/pi-embedded-helpers`、`agents/agent-scope`、`routing/session-key`、`infra/format-time`、`utils/usage-format`、`utils/message-channel`、`auto-reply/commands-registry`、`auto-reply/thinking`、`terminal/ansi`。Go 端需确认这些模块已有对应实现 | ⚠️ 高 — 需逐一验证 Go 端是否已实现 |
| 5 | **环境变量依赖** | `OPENACOSMI_GATEWAY_TOKEN`、`OPENACOSMI_GATEWAY_PASSWORD`（`gateway-chat.ts` L250-261）。Go 端 config 模块需对齐 | ✅ 已有 |
| 6 | **进程/平台依赖** | `process.platform`（gateway-chat L121）、`process.cwd`（tui.ts L279、local-shell L32）、`child_process.spawn`（local-shell L2）| ⚠️ 中 — Go 用 `runtime.GOOS`/`os.Getwd`/`os/exec` 替代 |
| 7 | **全局状态** | 无全局单例。状态封装在 `runTui` 闭包内（TuiStateAccess）+ Map 追踪（finalizedRuns/sessionRuns/localRunIds/toolById/streamingRuns）| ✅ bubbletea Model 模式天然解决 |

---

## 联网验证结论

| 验证项 | 结论 | 来源 |
|--------|------|------|
| bubbletea + WebSocket | ✅ **可行**。通过 `tea.Cmd` 执行异步 WS 操作，channel 注入实时事件。官方文档和社区示例已验证 | charm.land, dev.to, github.com |
| bubbletea overlay/modal | ✅ **可行**。Lip Gloss v2 Beta 支持 modal 绘制，社区有 dedicated overlay 组件。nested model 模式管理多视图 | reddit.com (Jan 2025), github.com |
| bubbles/list fuzzy filter | ✅ **可替代 TS 自研组件**。bubbles/list 内置 fuzzy filtering。但 TS 有 4 层优先级搜索，bubbles 仅含基础 fuzzy，**可能需定制 FilterFunc** | github.com/charmbracelet/bubbles |
| 代码高亮（chroma） | ✅ **可替代 cli-highlight**。chroma 是 Go 生态主流语法高亮库（Gitea/Hugo 均使用），支持 200+ 语言 | github.com/alecthomas/chroma |
| 流式文本渲染 | ✅ **可行**。bubbletea 天然支持频繁 Model Update + View 刷新，适合流式 LLM 输出 | charm.land |

---

## 总结

- **P0 差异**: 3 项（formatters 遗漏、tool execution 遗漏、local shell 遗漏）
- **P1 差异**: 7 项（选择列表组件、ChatLog 工具追踪、编辑器补全、命令注册表、主题不完整、语法高亮）
- **P2 差异**: 12 项（submit handler 三路分发、run ID 追踪、连接参数、pruning 等行为细节）
- **总差异**: 22 项

### 模块审计评级: **B**

> 方案架构正确（bubbletea MVU 模式），12 文件覆盖了主要功能区域。
> 但存在 3 个 P0 级遗漏和 7 个 P1 级遗漏，需在执行前将方案从 12 文件扩充到 **~15 文件**（新增 `formatters.go`、`view_tool.go`、`local_shell.go`），并补充各 PARTIAL 文件的缺失功能细节。
> 预估工时从 30h 应上调至 **~38h**（+8h 补缺），窗口从 5-6 个增至 **7 个**。

### 建议更新方案的文件规划

| 文件 | 功能 | 新增行数 | 窗口 |
|------|------|---------|------|
| `formatters.go`（**新增**） | 消息格式化 9 个函数 | ~120L | W2 |
| `view_tool.go`（**新增**） | 工具执行渲染组件 | ~150L | W3 |
| `local_shell.go`（**新增**） | `!command` 本地执行 | ~100L | W4 |
| `commands.go`（扩充） | 完整命令注册 + 参数补全 | +80L | W4 |
| `theme.go`（扩充） | 代码高亮 + 完整主题 | +50L | W5 |
| `view_chat_log.go`（扩充） | 工具追踪 Map | +30L | W2 |

---

*审计完成 — 2026-02-19*
