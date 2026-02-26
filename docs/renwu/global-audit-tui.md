# tui 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W8 (中型模块)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 24 | 21 | 87.5% |
| 总行数 | 4155 | 6105 | 146.9% |

*(注：Go 因为 `bubbletea` 框架特性，行数较多属正常现象)*

## 逐文件对照

| 状态 | TS 文件 | Go 文件 |
|------|---------|---------|
| ✅ FULL | `tui.ts` | `tui.go`, `model.go` |
| ✅ FULL | `tui-event-handlers.ts` | `event_handlers.go` |
| ✅ FULL | `tui-stream-assembler.ts` | `stream_assembler.go` |
| ✅ FULL | `tui-session-actions.ts` | `session_actions.go` |
| ✅ FULL | `tui-command-handlers.ts` | `commands.go` |
| ✅ FULL | `commands.ts` | `commands.go` |
| ✅ FULL | `gateway-chat.ts` | `gateway_ws.go` |
| ✅ FULL | `tui-local-shell.ts` | `local_shell.go` |
| ✅ FULL | `tui-formatters.ts` | `formatters.go` |
| ✅ FULL | `tui-types.ts` | `model.go`, 其他 |
| ✅ FULL | `theme/theme.ts`, `theme/syntax-theme.ts` | `theme.go` |
| ✅ FULL | `tui-waiting.ts` | `spinner.go`, `progress.go` |
| ✅ FULL | `tui-overlays.ts` | `overlays.go` |
| ✅ FULL | Components (`assistant-message.ts`, `user-message.ts`, `chat-log.ts`, `custom-editor.ts`, `tool-execution.ts`, `selectors.ts`) | View 层 (`view_message.go`, `view_input.go`, `view_tool.go`, `view_chat_log.go`, `view_status.go`, `prompter.go`, `table.go`) |

> 评价：Go 端采用 `bubbletea` 进行重构，拆分了 `Model` 和多个 View 子组件，结构更加清晰，覆盖完整。

## 隐藏依赖审计

| # | 类别 | 检查结果 | Go端实现方案 |
|---|------|----------|-------------|
| 1 | npm 包黑盒行为 | ⚠️ 使用了 `@mariozechner/pi-tui` | Go 端使用成熟的 `charmbracelet/bubbletea` 和 `lipgloss` 完全替代 |
| 2 | 全局状态/单例 | ✅ 无持久化隐式单例 | Go 端全部集成在主 `Model` 的生命周期中 |
| 3 | 事件总线/回调链 | ⚠️ TS 存在大量基于事件的回调 | Go 端利用 `bubbletea` 的 `tea.Msg` 和 Update 循环实现了等价的消息传递（参考 `event_handlers.go`） |
| 4 | 环境变量依赖 | ✅ 无直接读取 | 通过 Config / Base Configuration 注入 |
| 5 | 文件系统约定 | ✅ 无 | 无文件操作 |
| 6 | 协议/消息格式 | ⚠️ 与 Gateway 通信 (WebSocket) | `gateway_ws.go` 已完整实现 JSON 帧协议对称编解码 |
| 7 | 错误处理约定 | ✅ 标准 Error 冒泡 | Bubble Tea 通过 `tea.Cmd` 返回 Error Msg 处理 |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| TUI-A1 | 架构重构 | `components/*.ts` | `view_*.go` | TS 使用基于 DOM 的类 DOM 终端组件，Go 采用典型的 Elm 架构 (BubbleTea) | P3 | 架构演进，功能等价，无需修复 |
| TUI-A2 | API | `gateway-chat.ts` | `gateway_ws.go` | 消息收发机制由于框架原因从回调变更为通道消息驱动 | P3 | 状态机驱动，更健壮，无需修复 |

> **ID 说明**: TUI-A1/A2 为审计报告架构差异 ID（A=Audit），与 `deferred-items.md` 中的 TUI-1/TUI-2（功能补全，已 ✅ W-FIX-8）区分。

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 0 项
- **模块审计评级: A** (架构优秀，重构完全等价，极高质量)
