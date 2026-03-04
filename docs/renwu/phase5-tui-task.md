# TUI 终端聊天客户端 — 分步实施跟踪文档

> 创建日期：2026-02-20 | 状态：待执行
> 基于：`phase5-tui-project.md` + `global-audit-tui.md`(22 差异) + `global-audit-tui-deps.md`(8 修复项)
> 目标目录：`backend/internal/tui/`

---

## 项目总览

| 维度 | 详情 |
|------|------|
| Go 产出 | **15 个新文件** / ~3,740L |
| 窗口数 | **7 个**（W1-W5 功能 + W6 集成测试 + W7 性能调优）|
| 预估总工时 | ~38h |
| 现有 Go TUI | 6 文件（spinner/progress/wizard/table/prompter/tui）仅 setup wizard，无聊天功能 |

### 执行规则

1. **严格按窗口顺序执行**，每个窗口有明确的前置依赖
2. 每个窗口结束前必须通过编译检查 `go build ./...` + `go vet ./...`
3. 每个窗口开头必须读取 **新窗口启动上下文**（见各窗口「上下文加载」章节）
4. 单窗口不超过 3 个文件的分析/重构
5. 遵循六步循环法（提取→依赖图→隐藏依赖→理解→重构→验证）

---

## 全局进度

- [x] W1：核心骨架（model.go + gateway_ws.go）✅ 审计通过 2026-02-20
- [x] W2：流式组装 + 格式化器 + 消息列表渲染 ✅ 审计通过 2026-02-20
- [x] W3：完整聊天 UI（消息 + 工具渲染 + 输入 + 状态栏）✅ 审计通过 2026-02-20
- [x] W4：命令 + 事件处理 + 本地 shell ✅ 审计通过 2026-02-20
- [x] W5：会话管理 + overlay + 主题 + 代码高亮 ✅ 审计通过 2026-02-20
- [x] W6：集成测试 + 修复 ✅ 审计通过 2026-02-20
- [ ] W7：性能调优 + 边缘场景修复

---

## W1：核心骨架 — model.go + gateway_ws.go ✅

> **工时**: ~6h | **前置依赖**: 无 | **产出文件**: 2 | **完成日期**: 2026-02-20

### W1 上下文加载（新窗口启动时粘贴）

```
请阅读以下文件恢复上下文：
1. docs/renwu/phase5-tui-task.md — 本跟踪文档，找到 W1 章节
2. docs/renwu/phase5-tui-project.md — 项目方案（15 文件规划表）
3. docs/renwu/global-audit-tui-deps.md — 依赖审计（DEP-01 gateway auth 需 W1 处理）
4. skills/acosmi-refactor/references/coding-standards.md — 编码规范（跳过 Rust/FFI）

TS 源文件（W1 需精读）：
- src/tui/tui.ts — 主入口（709L），重点 L80-293（runTui 状态定义 + 辅助函数）
- src/tui/gateway-chat.ts — 全量精读（267L），GatewayChatClient 类 + resolveGatewayConnection
- src/tui/tui-types.ts — 类型定义（107L）

Go 现有文件（确认不冲突）：
- backend/internal/tui/tui.go — 现有入口（1,460B）

当前任务：实现 model.go（~350L）+ gateway_ws.go（~330L）
```

### W1-T1: model.go（~350L）

**TS 对标**: `tui.ts`(709L) + `tui-types.ts`(107L)

#### 六步分析要点

**步骤 1-2 提取 + 依赖图**:

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `@mariozechner/pi-tui` (TUI/Container/Text) | `bubbletea` Model 接口 | ✅ 框架替代 |
| `tui-types.ts` (TuiStateAccess/SessionInfo) | 内嵌到 model.go | ✅ 类型合并 |
| `tui-formatters.ts` (resolveFinalAssistantText) | W2 实现 | ⏭️ 先定义接口 |
| `gateway-chat.ts` (GatewayChatClient) | 同窗口 gateway_ws.go | ✅ |

**步骤 3 隐藏依赖**:

- ✅ 全局状态 → bubbletea Model 天然封装
- ✅ 环境变量 → 委托给 gateway_ws.go 处理
- ✅ 事件总线 → bubbletea Cmd/Msg 替代回调

**关键实现清单**:

- [x] `Model` struct 定义（含所有核心状态字段）— 532L，含连接/会话/消息/UI 完整字段
  - 连接状态 / 会话状态 / 消息列表 / UI 状态 ✅
  - `localRunIDs map[string]struct{}` — 差异 T-02 ✅
  - `sessionInfo SessionInfo` — 含 thinkingLevel/verboseLevel/model 等 ✅
- [x] `Init() tea.Cmd` — 启动 gateway 连接 ✅ L209-212
- [x] `Update(msg tea.Msg) (tea.Model, tea.Cmd)` — 事件分发 ✅ L214-263
  - 三路分发逻辑（差异 T-01）：`!`→shell / `/`→命令 / 普通→发送 — TODO(W4) 正确
  - 键盘事件 / gateway 消息 / 窗口 resize ✅
- [x] `View() string` — 布局渲染（header + chatlog + input + footer）✅ L265-295
- [x] `noteLocalRunId` / `forgetLocalRunId` / `isLocalRunId` — 差异 T-02 ✅ L367-396
- [x] `updateAutocompleteProvider()` — 差异 T-03（stub，W3 完善）✅ TODO(W3) 正确
- [x] `formatSessionKey(key string) string` — 差异 T-04，截断长 key ✅ L419-430

**审计通过**: W1 阶段 View 仅返回占位布局，具体渲染组件在 W2-W3 实现。实现完整。

### W1-T2: gateway_ws.go（~330L）

**TS 对标**: `gateway-chat.ts`(267L)

#### 六步分析要点

**步骤 1-2 提取 + 依赖图**:

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `config/config` (loadConfig/resolveGatewayPort) | `config/loader.go` | ✅ |
| `gateway/call` (ensureExplicitGatewayAuth) | 无独立函数，W1 内实现 | 🔨 |
| `gateway/client` (GatewayClient) | `gateway/ws.go` | ✅ |
| `gateway/protocol` (ConnectParams/PROTOCOL_VERSION) | `gateway/protocol.go` | ✅ |
| `utils/message-channel` (GATEWAY_CLIENT_MODES/NAMES) | `acp/types.go` | ✅ |

**步骤 3 隐藏依赖**:

- ⚠️ **环境变量**: `OPENACOSMI_GATEWAY_TOKEN` / `OPENACOSMI_GATEWAY_PASSWORD` — 需实现 6 层 fallback
- ✅ npm 包: 无（gateway/client 已有 Go 实现）
- ✅ 全局状态: 闭包封装，Model 管理

**关键实现清单**:

- [x] `GatewayChatClient` struct ✅ L153-169 含 RPC 层 + 回调字段
- [x] `NewGatewayChatClient(opts GatewayConnectionOptions)` 构造函数 ✅ L171-217
  - 12 个协议字段完整传递 — 差异 G-01 ✅ sendConnect() L406-439
  - clientName/clientVersion/platform/mode/caps/instanceId/minProtocol/maxProtocol ✅
- [x] `Start()` / `Stop()` / `WaitForReady()` ✅ L224-241
- [x] `SendChat(opts ChatSendOptions) (string, error)` — 返回 runId ✅ L245-270
- [x] `AbortChat(sessionKey, runId) error` ✅ L272-282
- [x] `LoadHistory(sessionKey string, limit int)` ✅ L284-297
- [x] `ListSessions()` / `ListAgents()` / `PatchSession()` / `ResetSession()` ✅ L299-377
- [x] `ListModels() ([]GatewayModelChoice, error)` ✅ L388-401
- [x] **⚡ `resolveGatewayConnection()`** — DEP-01 修复 ✅ L602-707
  - token 6 层 fallback ✅: CLI→remote.token→env OPENACOSMI_GATEWAY_TOKEN→env CLAWDBOT_GATEWAY_TOKEN→config.gateway.auth.token
  - password 6 层 fallback ✅: CLI→env OPENACOSMI_GATEWAY_PASSWORD→env CLAWDBOT_GATEWAY_PASSWORD→remote.password→config.gateway.auth.password
  - URL 解析 ✅: CLI→remote.url→`ws://127.0.0.1:{port}`
- [x] 事件回调 → tea.Msg 转换（onEvent/onConnected/onDisconnected/onGap）✅ L164-168 + L194-213

### W1 验证标准

```bash
cd backend && go build ./... && go vet ./...
# 目标: model.go + gateway_ws.go 编译通过
# 验证: 可创建 Model 实例并连接 gateway（需运行 gateway 服务）
```

### W1 产出文档

- [x] 更新本文件 W1 进度 ✅ 2026-02-20
- [x] 无延迟项 — 所有 W1 功能点均已实现

---

## W2：流式组装 + 格式化器 + 消息列表渲染 ✅

> **工时**: ~6h | **前置依赖**: W1 | **产出文件**: 4（含测试）| **完成日期**: 2026-02-20

### W2 上下文加载（新窗口启动时粘贴）

```
请阅读以下文件恢复上下文：
1. docs/renwu/phase5-tui-task.md — 本跟踪文档，找到 W2 章节
2. docs/renwu/phase5-tui-project.md — 项目方案
3. backend/internal/tui/model.go — W1 产出，Model 结构和状态定义
4. backend/internal/tui/gateway_ws.go — W1 产出，GatewayChatClient

TS 源文件（W2 需精读）：
- src/tui/tui-stream-assembler.ts — 全量精读（78L），流式文本组装逻辑
- src/tui/tui-formatters.ts — 全量精读（220L），9 个格式化函数
- src/tui/components/chat-log.ts — 全量精读（104L），ChatLog 类 + 工具追踪 Map

Go 依赖模块（确认可用）：
- backend/internal/agents/helpers/errors.go — FormatRawAssistantErrorForUi()
- backend/internal/autoreply/status.go — FormatTokenCount()

当前任务：实现 stream_assembler.go + formatters.go(🔴新增) + view_chat_log.go
```

### W2-T1: stream_assembler.go（~200L）

**TS 对标**: `tui-stream-assembler.ts`(78L)

**步骤 1-2 提取 + 依赖图**:

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `tui-formatters.ts` (extractThinkingFromMessage 等) | 同窗口 formatters.go | ✅ |

**步骤 3 隐藏依赖**: ✅ 无 — 纯内存状态管理器

**关键实现清单**:

- [x] `TuiStreamAssembler` struct — 维护 `map[string]*runStreamState` ✅ L17-19
- [x] `IngestDelta(runId string, message any, showThinking bool) string` — 增量组装 ✅ L58-70
- [x] `Finalize(runId string, message any, showThinking bool) string` — 最终文本 ✅ L72-83
- [x] `Drop(runId string)` — 丢弃 run 状态 ✅ L85-89
- [x] 内部 `runStreamState` 结构：thinkingText + contentText + displayText ✅ L9-13
- [x] `Reset()` — 清空所有 run 状态（Go 新增）✅ L91-94

### W2-T2: formatters.go（~120L）🔴 审计新增

**TS 对标**: `tui-formatters.ts`(220L) — 差异 F-01 (P0)

**步骤 1-2 提取 + 依赖图**:

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `agents/pi-embedded-helpers` (formatRawAssistantErrorForUi) | `agents/helpers/errors.go:606` | ✅ |
| `utils/usage-format` (formatTokenCount) | `autoreply/status.go:78` | ✅ |

**步骤 3 隐藏依赖**: ✅ 无 — 纯函数，无副作用

**关键实现清单**（9 个函数逐一对齐）:

- [x] `ResolveFinalAssistantText(finalText, streamedText string) string` ✅ L15-26
- [x] `ComposeThinkingAndContent(thinkingText, contentText string, showThinking bool) string` ✅ L28-43
- [x] `ExtractThinkingFromMessage(message interface{}) string` ✅ L45-79
- [x] `ExtractContentFromMessage(message interface{}) string` ✅ L81-132
- [x] `ExtractTextFromMessage(message interface{}, includeThinking bool) string` ✅ L170-192
- [x] `IsCommandMessage(message interface{}) bool` ✅ L194-206
- [x] `FormatTokens(total, context *int) string` ✅ L208-231
- [x] `FormatContextUsageLine(total, context, remaining, percent *int) string` ✅ L233-263
- [x] `AsString(value interface{}, fallback string) string` ✅ L265-285

### W2-T3: view_chat_log.go（~330L）

**TS 对标**: `components/chat-log.ts`(104L) — 差异 CL-01 (P1)

**步骤 1-2 提取 + 依赖图**:

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `components/tool-execution.ts` (ToolExecutionComponent) | W3 view_tool.go | ⏭️ stub |
| `tui-formatters.ts` (extractTextFromMessage) | 同窗口 formatters.go | ✅ |
| `@mariozechner/pi-tui` (Container/Text/Markdown) | bubbletea + lipgloss | ✅ |

**步骤 3 隐藏依赖**: ✅ 无全局状态，Map 追踪封装在 ChatLog 内

**关键实现清单**:

- [x] `ChatLog` struct — 消息列表 + viewport 滚动 ✅ L49-55
- [x] 消息类型枚举: `MessageTypeUser` / `MessageTypeAssistant` / `MessageTypeSystem` ✅ L15-21
- [x] `toolById map[string]*ToolExecEntry` — 工具追踪 Map（差异 CL-01）✅ L51
- [x] `streamingRuns map[string]int` — 流式 run → 消息索引映射 ✅ L52
- [x] `AddUser(text string)` — 添加用户消息 ✅ L85-92
- [x] `AddSystem(text string)` — 添加系统消息 ✅ L76-83
- [x] `UpdateAssistant(text string, runId string)` — 流式更新助手消息 ✅ L117-128
- [x] `FinalizeAssistant(text string, runId string)` — 最终化助手消息 ✅ L130-146
- [x] `StartTool(toolCallId, toolName string, args interface{})` — 开始工具追踪 ✅ L150-165
- [x] `UpdateToolArgs(toolCallId string, args interface{})` — 更新工具参数 ✅ L167-175
- [x] `UpdateToolResult(toolCallId, result, opts ToolResultOpts)` — 更新工具结果 ✅ L177-186
- [x] `SetToolsExpanded(expanded bool)` — 切换工具展开/收起 ✅ L188-195
- [x] `View(width, height int) string` — 简单文本渲染（W3 升级 glamour）✅ L199-222

### W2 验证标准

```bash
cd backend && go build ./... && go vet ./...
cd backend && go test -race ./internal/tui/...
# 目标: 3 个文件编译通过
# 建议: 为 formatters.go 编写单元测试（纯函数，易测试）
```

### W2 产出文档

- [x] 更新本文件 W2 进度 ✅ 2026-02-20
- [x] 为 formatters.go 编写 `formatters_test.go` ✅ 35 测试全部 PASS
- [x] 无延迟项 — 所有 W2 功能点均已实现

---

## W3：完整聊天 UI — 消息 + 工具渲染 + 输入 + 状态栏 ✅

> **工时**: ~6h | **前置依赖**: W2 | **产出文件**: 4 + 修复 `display.go` | **完成日期**: 2026-02-20

### W3 上下文加载（新窗口启动时粘贴）

```
请阅读以下文件恢复上下文：
1. docs/renwu/phase5-tui-task.md — 本跟踪文档，找到 W3 章节
2. docs/renwu/phase5-tui-project.md — 项目方案（view_tool.go 条目 + 修复项 #1-5）
3. backend/internal/tui/model.go — Model 结构
4. backend/internal/tui/view_chat_log.go — ChatLog + 工具追踪 Map
5. backend/internal/tui/formatters.go — 格式化函数

TS 源文件（W3 需精读）：
- src/tui/components/user-message.ts（20L）+ assistant-message.ts（19L）
- src/tui/components/tool-execution.ts — 全量精读（137L），ToolExecutionComponent 类
- src/tui/components/custom-editor.ts（60L）— 编辑器自定义行为
- src/tui/tui-status-summary.ts（88L）+ tui-waiting.ts（51L）

⚡ 需同步修复的依赖文件（精读）：
- src/agents/tool-display.ts — 对比 Go backend/internal/agents/tools/display.go
- src/agents/logging/redact.ts — redactToolDetail() 缺失确认
- src/utils.ts — shortenHomeInString() 缺失确认

当前任务：实现 view_message.go + view_tool.go(🔴新增) + view_input.go + view_status.go
同步修复: agents/tools/display.go 的 5 项差异（修复项 #1-5）
```

### W3-T1: view_message.go（~250L）

**TS 对标**: `components/user-message.ts`(20L) + `components/assistant-message.ts`(19L) 扩展

**步骤 3 隐藏依赖**: ✅ 无

**关键实现清单**:

- [x] `renderUserMessage(text string, width int) string` — 用户消息气泡 ✅ L84-92
- [x] `renderAssistantMessage(text string, isStreaming bool, width int) string` — 助手消息 ✅ L95-106
- [x] `renderSystemMessage(text string, width int) string` — 系统消息 ✅ L108-110
- [x] Markdown 渲染 — 使用 `charmbracelet/glamour` ✅ L44-77
- [x] 代码块高亮 — glamour 自带支持，W5 theme.go 完善 ✅

### W3-T2: view_tool.go（~180L）🔴 审计新增

**TS 对标**: `components/tool-execution.ts`(137L) — 差异 TE-01 (P0)

**步骤 1-2 提取 + 依赖图**:

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `agents/tool-display.ts` (resolveToolDisplay/formatToolDetail) | `agents/tools/display.go` | ⚠️ 需修复 |
| `theme/theme.ts` (theme.toolPendingBg 等) | W5 theme.go | ⏭️ 先用默认色 |

**步骤 3 隐藏依赖**: ✅ 无 — 纯渲染组件

**关键实现清单**:

- [x] `ToolExecView` struct — 工具执行渲染组件 ✅ L89-98
  - 状态机: pending → running → success / error ✅
  - `toolName` / `args` / `result` / `expanded` / `isError` / `isPartial` ✅
- [x] `NewToolExecView(toolName string, args any) *ToolExecView` ✅ L101-108
- [x] `SetArgs(args any)` / `SetExpanded(bool)` ✅ L110-118
- [x] `SetResult(result *ToolResult, isError bool)` — 最终结果 ✅ L120-130
- [x] `SetPartialResult(result *ToolResult)` — 部分结果（running 中）✅ L132-137
- [x] `View(width int) string` — 渲染：emoji + label + args + output ✅ L140-187
  - 输出预览裁剪 12 行（`PREVIEW_LINES = 12`）✅
  - 背景色按状态切换（pending=黄 / success=绿 / error=红）✅

### W3-T2b: 同步修复 `agents/tools/display.go`（修复项 #1-5）

> ⚡ 必须在 view_tool.go 之前或同时完成

**修复项清单**（来自 `global-audit-tui-deps.md` 补充审计）:

- [x] **#1 (P1)**: 补 `meta` 参数 fallback ✅ variadic `meta ...string`
- [x] **#2 (P1)**: 补 `detailKeys[]` 多字段解析 ✅ 重写 526L，含 lookupValueByPath/coerceDisplayValue/formatDetailKey/resolveDetailFromKeys
- [x] **#3 (P1)**: 补 read/write/edit/attach 特殊 detail 解析 ✅ resolveReadDetail + resolveWriteDetail
- [x] **#4 (P2)**: 实现 `RedactToolDetail()` ✅ 调用 pkg/log/redact.go
- [x] **#5 (P2)**: 实现 `ShortenHomeInString()` ✅ 调用 pkg/utils/utils.go

### W3-T3: view_input.go（~230L）

**TS 对标**: `components/custom-editor.ts`(60L) — 差异 IN-01 (P1)

**关键实现清单**:

- [x] `InputBox` struct — 基于 `bubbles/textarea` ✅ L33-40
- [x] 自定义 keymap: Enter 提交 / Shift+Enter 换行 / Ctrl+C 中断 ✅ L117-130
- [x] `/` 前缀触发 slash 命令补全列表（stub，W4 完善）✅ HasSlashPrefix()
- [x] 输入历史（up/down 导航，最近 50 条，去重）✅ L133-166
- [x] `View() string` — 渲染输入框 ✅ L196-198

### W3-T4: view_status.go（~100L）

**TS 对标**: `tui-status-summary.ts`(88L) + `tui-waiting.ts`(51L)

**关键实现清单**:

- [x] `StatusBar` struct — 连接指示器 + token 使用 + 活动状态 ✅ L98-116
- [x] 连接状态图标: 🟢 connected / 🔴 disconnected / 🟡 reconnecting ✅ L123-130
- [x] token 使用行（调用 `FormatTokens` / `FormatContextUsageLine`）✅
- [x] 活动状态显示: idle/streaming/running/error/aborted + 等待动画 ✅
- [x] `View(width int) string` — 水平布局：左=状态 中=model 右=tokens ✅ L133-172

### W3 验证标准

```bash
cd backend && go build ./... && go vet ./...
cd backend && go test -race ./internal/tui/...
cd backend && go test -race ./internal/agents/tools/...
# 目标: 4 个新文件 + display.go 修复编译通过
# 验证: 完整聊天 UI 可渲染（消息 + 工具 + 输入 + 状态栏）
```

### W3 产出文档

- [x] 更新本文件 W3 进度 ✅ 2026-02-20
- [x] 无延迟项 — 所有 W3 功能点均已实现

**审计通过**: W3 阶段 4 个新文件 + display.go 重写全部编译通过。实现完整。

---

## W4：命令 + 事件处理 + 本地 shell ✅

> **工时**: ~6h | **前置依赖**: W3 | **产出文件**: 3 + 修复 `commands_registry.go`
> **完成日期**: 2026-02-20 | **审计**: 33 项 TS↔Go 对照 + 3 差异项修复全部通过

### W4 上下文加载（新窗口启动时粘贴）

```
请阅读以下文件恢复上下文：
1. docs/renwu/phase5-tui-task.md — 本跟踪文档，找到 W4 章节
2. backend/internal/tui/model.go — Model 结构（三路分发逻辑）
3. backend/internal/tui/view_chat_log.go — ChatLog（startTool/updateToolResult）
4. backend/internal/tui/formatters.go — IsCommandMessage/ExtractTextFromMessage
5. backend/internal/tui/stream_assembler.go — TuiStreamAssembler

TS 源文件（W4 需精读）：
- src/tui/commands.ts — 全量精读（164L），getSlashCommands 注册表
- src/tui/tui-command-handlers.ts — 全量精读（503L），handleCommand 20+ 分支
- src/tui/tui-event-handlers.ts — 全量精读（248L），handleChatEvent + handleAgentEvent
- src/tui/tui-local-shell.ts — 全量精读（146L），createLocalShellRunner

⚡ 需同步修复的依赖文件：
- backend/internal/autoreply/commands_registry.go — 补 skillCommands 参数

当前任务：实现 commands.go + event_handlers.go + local_shell.go(🔴新增)
同步修复: commands_registry.go 的 skillCommands 注入（修复项 #6）
```

### W4-T1: commands.go（~300L）

**TS 对标**: `commands.ts`(164L) + `tui-command-handlers.ts`(503L) — 差异 CMD-01/C-01 (P1)

**步骤 1-2 提取 + 依赖图**:

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `auto-reply/commands-registry` (listChatCommands) | `autoreply/commands_registry.go` | ⚠️ 缺 skillCommands |
| `auto-reply/thinking` (formatThinkingLevels) | `autoreply/thinking.go` | ✅ |
| `components/selectors.ts` (createSearchableSelectList) | W5 overlays.go | ⏭️ stub |
| `gateway-chat.ts` (GatewayChatClient 方法) | gateway_ws.go | ✅ |

**步骤 3 隐藏依赖**:

- ⚠️ **commands_registry.go 缺少 skillCommands 参数** — 修复项 #6 (P1)
- ✅ 其他均为纯函数调用

**关键实现清单**:

**命令注册表**:

- [x] `ParsedCommand` struct（name + args）✅ L43-46
- [x] `ParseCommand(input string) ParsedCommand` — 解析 `/xxx args` ✅ L65-80
- [x] `COMMAND_ALIASES` map — `elev → elevated` 等 ✅ L57-59
- [x] `GetSlashCommands(opts SlashCommandOptions) []SlashCommand` — 20+ 内置命令 ✅ L86-183
  - 每个命令含 `GetArgumentCompletions(prefix) []Completion` ✅
  - think/verbose/reasoning/usage/elevated/activation 带枚举值补全 ✅ L98-146
  - 追加 gateway 自定义命令（通过 ListChatCommands）✅ L155-181
- [x] `HelpText(opts SlashCommandOptions) string` — 帮助文本 ✅ L199-221

**命令处理器（20+ 分支）**:

- [x] `handleCommand(raw string)` — 主分发 ✅ L235-362
- [x] `/help` → 输出 helpText ✅ L242-247
- [x] `/status` → 调用 `getStatus()` + FormatStatusSummary ✅ L249-250 → L366-384
- [x] `/agent <id>` / `/agents` → 切换/选择 agent ✅ L252-263
- [x] `/session <key>` / `/sessions` → 切换/选择 session ✅ L265-276
- [x] `/model <provider/model>` / `/models` → 切换/选择 model（overlay W5 TODO）✅ L278-289
- [x] `/think <level>` → `PatchSession(thinkingLevel)` ✅ L291-301
- [x] `/verbose <on|off>` → `PatchSession(verboseLevel)` ✅ L303-308
- [x] `/reasoning <on|off>` → `PatchSession(reasoningLevel)` ✅ L310-315
- [x] `/usage <off|tokens|full>` → `PatchSession(responseUsage)` ✅ L317-318 → L459-501
- [x] `/elevated <on|off|ask|full>` → `PatchSession(sendPolicy)` ✅ L320-330
- [x] `/activation <mention|always>` → 组激活模式 ✅ L332-341
- [x] `/abort` → 中止活动 run ✅ L346-347 → L526-548
- [x] `/new` / `/reset` → 重置 session ✅ L343-344 → L503-524
- [x] `/settings` → 打开设置 overlay（W5 TODO）✅ L349-352
- [x] `/exit` / `/quit` → `tea.Quit` ✅ L354-356
- [x] `sendMessage(text string)` — 发送普通消息（调用 gateway SendChat）✅ L554-583

### W4-T1b: 同步修复 `autoreply/commands_registry.go`（修复项 #6）✅

- [x] `ListChatCommands` 和 `ListChatCommandsForConfig` 补 `SkillCommands` 参数 ✅
  - `ListChatCommands(skillCommands ...[]*ChatCommandDefinition)` ✅ L25
  - `ListChatCommandsForConfig(cfg, skillCommands ...)` ✅ L277
  - `BuildSkillCommandDefinitions(specs []SkillCommandSpec)` ✅ L462-479
  - 将 skill 命令动态注入到返回列表中 ✅ L31-33

### W4-T2: event_handlers.go（~300L）

**TS 对标**: `tui-event-handlers.ts`(248L) — 差异 E-01/E-02 (P2)

**步骤 1-2 提取 + 依赖图**:

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `tui-formatters.ts` (asString/extractTextFromMessage/isCommandMessage) | formatters.go | ✅ |
| `tui-stream-assembler.ts` (TuiStreamAssembler) | stream_assembler.go | ✅ |
| `components/chat-log.ts` (ChatLog) | view_chat_log.go | ✅ |

**步骤 3 隐藏依赖**: ✅ 无 — Map 状态封装在函数内

**关键实现清单**:

- [x] `finalizedRuns map[string]int64` — 已完成 run 追踪（value=timestamp）✅ model.go L141
- [x] `sessionRuns map[string]int64` — 活动 run 追踪 ✅ model.go L142
- [x] `pruneRunMap(runs map[string]int64)` — 差异 E-01 ✅ L43-65
  - `>200` 条时裁剪至 `150`，含 10 分钟过期策略 ✅ L47-55
  - 先删过期条目，若仍 >200 则强制删除最旧条目 ✅ L57-64
- [x] `syncSessionKey()` — session 切换时清空 run 追踪 + 重置 assembler ✅ L69-80
- [x] `handleChatEvent(payload any)` — 聊天事件处理 ✅ L98-169
  - delta → 流式更新助手消息 ✅ L130-136
  - final → 最终化助手消息，区分 command/normal/error ✅ L172-214
  - aborted/error → 系统消息 + 状态更新 ✅ L141-165
  - 区分 localRunId / 外部 run（决定是否重新加载 history）✅ L147-148, L162-163
- [x] `handleAgentEvent(payload any)` — agent 事件处理（差异 E-02）✅ L218-252
  - `stream=tool`: start/update/result 三阶段 + verboseLevel 过滤 ✅ L254-301
  - `stream=lifecycle`: start/end/error 状态更新 ✅ L303-321
  - 非活跃/非已知 run 事件直接过滤 ✅ L232-239

### W4-T3: local_shell.go（~100L）🔴 审计新增

**TS 对标**: `tui-local-shell.ts`(146L) — 差异 LS-01 (P0)

**步骤 1-2 提取 + 依赖图**:

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `child_process.spawn` | `os/exec` | ✅ 替代 |
| `process.cwd` | `os.Getwd()` | ✅ |
| `process.env` | `os.Environ()` | ✅ |

**步骤 3 隐藏依赖**:

- ⚠️ **进程/平台**: Go `os/exec` 替代 Node `child_process.spawn`
- ✅ 全局状态: `localExecAsked`/`localExecAllowed` 封装在 Model 中

**关键实现清单**:

- [x] `localExecAsked bool` / `localExecAllowed bool` — 会话级权限状态 ✅ model.go L146-147
- [x] 权限确认流程（非 overlay，使用 /yes /no 文字确认）✅ L56-68
  - "Allow local shell commands for this session?" ✅ L60
  - `handleLocalShellPermission(allowed bool)` ✅ L73-88
- [x] `runLocalShellLine(line string) tea.Cmd` — 执行 `!command` ✅ L42-71
  - 去掉 `!` 前缀 ✅ L44
  - 检查权限（已询问但拒绝 → 直接拒绝）✅ L50-52
  - `exec.Command(shell, "-c", cmd)` + stdout/stderr 合并 ✅ L106-119
  - 输出截断 `maxChars = 40_000` ✅ L22 + L122-123
  - 结果通过 `LocalShellResultMsg` 返回（在 model.go Update 处理）✅ L148-158
  - 错误处理: `LocalShellResultMsg.Err` ✅ L137-140

### W4 验证标准

```bash
cd backend && go build ./... && go vet ./...
cd backend && go test -race ./internal/tui/...
cd backend && go test -race ./internal/autoreply/...
# 目标: 3 个新文件 + commands_registry.go 修复编译通过
# 验证: slash 命令 + !command 本地执行工作
```

### W4 产出文档

- [x] 更新本文件 W4 进度
- [x] 审计差异项全部修复（D-W4-01 refreshSessionInfo / D-W4-02 loadHistory / D-W4-03 skillCommands）

---

## W5：会话管理 + overlay + 主题 + 代码高亮

> **工时**: ~6h | **前置依赖**: W4 | **产出文件**: 3

### W5 上下文加载（新窗口启动时粘贴）

```
请阅读以下文件恢复上下文：
1. docs/renwu/phase5-tui-task.md — 找到 W5 章节
2. backend/internal/tui/model.go — Model 结构
3. backend/internal/tui/commands.go — 命令处理器（overlay 调用入口）
4. backend/internal/tui/gateway_ws.go — GatewayChatClient API

TS 源文件（W5 需精读）：
- src/tui/tui-session-actions.ts — 全量精读（413L）
- src/tui/tui-overlays.ts（19L）+ components/selectors.ts（30L）
- src/tui/components/searchable-select-list.ts（310L）
- src/tui/components/filterable-select-list.ts（143L）
- src/tui/theme/theme.ts（137L）+ theme/syntax-theme.ts（52L）

当前任务：实现 session_actions.go + overlays.go + theme.go
```

### W5-T1: session_actions.go（496L）✅ 已完成

**TS 对标**: `tui-session-actions.ts`(413L) — 差异 S-01/S-02/S-03

**关键实现清单**:

- [x] `applyAgentsResult(result *GatewayAgentsList)` — 差异 S-01 ✅
  - agentNames Map + 默认 agent + initialSession 应用
- [x] `refreshAgentsCmd() tea.Cmd` / `updateAgentFromSessionKey(key)` ✅
- [x] `resolveModelSelection(entry) (provider, model)` — 3 层优先级 ✅
- [x] `applySessionInfo(entry, defaults, force)` — 差异 S-02 ✅
  - updatedAt 幂等 + 全字段合并
- [x] `refreshSessionInfoFull() tea.Cmd` — 串行化（sync.Mutex）✅
- [x] `handleSessionInfoResult(msg)` — match key + 构建 defaults ✅
- [x] `applySessionInfoFromPatch(result)` — patch 后局部更新 ✅
- [x] `loadHistoryCmd()` / `handleHistoryResult(msg)` — 差异 S-03 ✅
  - user/assistant/system/command/toolResult 5 类型分离
- [x] `setSessionCmd(rawKey)` — 清状态 + 加载 ✅

### W5-T2: overlays.go（301L）✅ 已完成

**TS 对标**: `tui-overlays.ts`(19L) + 3 组件(~500L) — 差异 SS-01 (P1)

- [x] `OverlayKind` 枚举 (None/Agent/Session/Model/Settings) ✅
- [x] `selectItem` → `list.Item` 接口实现 ✅
- [x] `newOverlayList` — `bubbles/list` + 自定义样式 ✅
- [x] `openOverlay`/`closeOverlay`/`hasOverlay` ✅
- [x] `updateOverlay` — Esc 取消 + Enter 选择 ✅
- [x] `handleOverlaySelect` — 路由到 agent/session/model ✅
- [x] `renderOverlay` — 居中 + 圆角边框 ✅
- [x] `openAgentSelectorCmd`/`openSessionSelectorCmd`/`openModelSelectorCmd` ✅
- [x] commands.go 7 处 TODO(W5) → 真实 overlay 调用 ✅
- [x] **修复项 #8**: `lipgloss.Width()` 替代 — 通过 `bubbles/list` 内置处理 ✅

### W5-T3: theme.go（294L）✅ 已完成

**TS 对标**: `theme/theme.ts`(137L) + `syntax-theme.ts`(52L) — 差异 TH-01/TH-02

- [x] `Palette` 21 色对齐 TS ✅
- [x] 5 个主题样式对象: ChatTheme/ToolTheme/OverlayTheme/StatusTheme/MarkdownTheme ✅
- [x] 代码高亮: `HighlightCode` + `SupportsLanguage` (chroma v2) ✅
  - VS Code Dark+ 风格 chromaStyle 自定义注册
- [x] `Dim`/`Bold`/`Italic` 辅助函数 ✅

### W5 验证标准 ✅

```bash
cd backend && go build ./...    # ✅ 通过
cd backend && go vet ./...      # ✅ 通过
cd backend && go test -race ./internal/tui/...  # ✅ 通过 (1.077s)
```

### W5 产出文档

- [x] 更新本文件 W5 进度
- [x] 创建 `docs/gouji/tui.md` 架构文档（W6 统一补充）✅ 2026-02-20
- [x] 无新增延迟项

---

## W6：集成测试 + 修复

> **工时**: ~4h | **前置依赖**: W5 | **产出文件**: 测试文件

### W6 上下文加载

```
请阅读以下文件恢复上下文：
1. docs/renwu/phase5-tui-task.md — 找到 W6 章节
2. backend/internal/tui/ — 全部已实现文件 outline
3. src/tui/*.test.ts — TS 端测试文件参考

当前任务：编写集成测试 + 修复发现的问题
```

### W6 测试清单

- [x] `formatters_test.go` — 9 个纯函数单元测试（补充 FormatContextUsageLine）✅
- [x] `stream_assembler_test.go` — delta/finalize/drop 流程测试（7 测试）✅
- [x] `commands_test.go` — 命令解析 + 注册表测试（4 测试）✅
- [x] `gateway_ws_test.go` — resolveGatewayConnection 6 层 fallback 测试（8 测试）✅
- [x] `local_shell_test.go` — 权限控制 + 输出截断测试（6 测试）✅
- [x] `event_handlers_test.go` — pruneRunMap + 解析函数测试（5 测试）✅
- [x] 修复测试中发现的所有问题（无生产代码问题）✅

### W6 验证标准

```bash
cd backend && go test -race -count=1 ./internal/tui/...
# 目标: 所有测试通过，无 race condition
```

---

## W7：性能调优 + 边缘场景修复

> **工时**: ~4h | **前置依赖**: W6 | **产出文件**: 按需修改

### W7 上下文加载

```
请阅读以下文件恢复上下文：
1. docs/renwu/phase5-tui-task.md — 找到 W7 章节
2. backend/internal/tui/ — 全部文件
3. W6 测试结果输出

当前任务：性能调优 + 边缘场景处理
```

### W7 调优清单

- [ ] 大消息渲染性能（>1000 行 Markdown）
- [ ] 快速流式更新节流（避免每个 delta 都触发 View 重绘）
- [ ] viewport 滚动流畅性（含 CJK/emoji 宽度计算）
- [ ] 长时间运行内存泄漏检查（run Map pruning 验证）
- [ ] 断线重连场景验证
- [ ] 多 session 切换状态一致性
- [ ] Ctrl+C 双击退出 vs 单击中止运行
- [ ] 终端窗口极小尺寸（<40 列）渲染降级

### W7 验证标准

```bash
cd backend && go test -race -bench=. ./internal/tui/...
# 压力测试: 1000 条消息 + 100 个工具调用 + 快速 resize
```

---

## 附录 A：审计差异 → 窗口映射表

| 差异 ID | 优先级 | 描述 | 窗口 | 文件 |
|---------|--------|-----|------|------|
| F-01 | P0 | 格式化器 9 函数缺失 | W2 | formatters.go |
| TE-01 | P0 | 工具执行渲染缺失 | W3 | view_tool.go |
| LS-01 | P0 | 本地 shell 缺失 | W4 | local_shell.go |
| SS-01 | P1 | 选择列表组件 | W5 | overlays.go |
| CL-01 | P1 | ChatLog 工具追踪 | W2 | view_chat_log.go |
| IN-01 | P1 | 输入框补全 | W3 | view_input.go |
| CMD-01 | P1 | 命令注册表 | W4 | commands.go |
| TH-01/02 | P1 | 主题+代码高亮 | W5 | theme.go |
| T-01~04 | P2 | model 行为差异 | W1 | model.go |
| G-01/02 | P2 | gateway 协议+fallback | W1 | gateway_ws.go |
| E-01/02 | P2 | 事件 pruning+tool流 | W4 | event_handlers.go |
| S-01~03 | P2 | 会话管理差异 | W5 | session_actions.go |

## 附录 B：依赖修复项 → 窗口映射表

| # | 修复项 | 优先级 | 窗口 | 目标文件 |
|---|--------|--------|------|---------|
| 1-3 | display.go meta/detailKeys/read-write | P1 | W3 | agents/tools/display.go |
| 4 | RedactToolDetail() | P2 | W3 | agents/tools/display.go |
| 5 | ShortenHomeInString() | P2 | W3 | agents/tools/display.go |
| 6 | skillCommands 注入 | P1 | W4 | autoreply/commands_registry.go |
| 7 | token/password 6层fallback | P2 | W1 | tui/gateway_ws.go |
| 8 | lipgloss.Width OSC-8 | P3 | W5 | tui/overlays.go |

---

*文档创建：2026-02-20 | 基于 phase5-tui-project.md + global-audit-tui.md + global-audit-tui-deps.md*
