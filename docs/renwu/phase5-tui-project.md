# 5.1 TUI 终端聊天客户端 — 独立项目实施方案

> 创建日期：2026-02-19 | 更新日期：2026-02-19 | 状态：方案已审批，审计补全后待执行
> 本项目从 `phase5-research-plan.md` 独立立项，工作量等同于小型项目
> 审计报告：`global-audit-tui.md`（22 项差异） + `global-audit-tui-deps.md`（14 个依赖颗粒度审计），本方案已据此补全

---

## 项目概览

| 维度 | 详情 |
|------|------|
| 目标 | 用 bubbletea 完整重写终端聊天客户端 |
| TS 对标 | 24 个文件 / `@mariozechner/pi-tui` |
| Go 产出 | **15 个文件** / ~3,740L |
| 预估工时 | **~38h（7 个窗口）** |
| 框架 | `charmbracelet/bubbletea` + `bubbles` + `lipgloss` + `alecthomas/chroma` |

## 现状

Go 端 `backend/internal/tui/` 已有 6 个文件（spinner/progress/wizard/table/prompter/tui），仅为 setup wizard 的 UI 原语，无聊天功能。

## 架构设计

```
Model (状态) ──→ View (渲染)
    ↑                │
    └── Update (事件) ←┘
         ↑
    Cmd (异步副作用: WS连接/API调用)
```

**核心状态结构**：

- 连接状态（gateway WS 连接/断开/重连）
- 会话状态（当前 session key/agent/model）
- 消息列表（用户/助手/工具执行，含流式组装中间状态）
- UI 状态（输入框/overlay/滚动位置/主题）

## 文件规划

> [!NOTE]
> 🔴 = 审计新增文件 | 🟡 = 审计扩充功能。对应审计报告差异 ID 标注在功能列。

| 文件 | 功能 | 行数 | 窗口 |
|------|------|------|------|
| `model.go` | 🟡 顶层 Model + Init/Update/View + 三路分发(T-01) + localRunIDs 追踪(T-02) + 补全 provider(T-03) | ~350L | W1 |
| `gateway_ws.go` | 🟡 WebSocket 客户端 + 12 字段协议参数(G-01) + 6 层 fallback 连接解析(G-02) + **⚡ token/password 6 层 fallback 组装**(DEP-audit: ~30L) | ~330L | W1 |
| `stream_assembler.go` | 流式文本组装器 | ~200L | W2 |
| `formatters.go` | 🔴 **审计新增(F-01)** 消息格式化 9 函数：extractThinking/extractContent/composeThinkingAndContent/resolveFinalAssistantText/extractTextFromMessage/isCommandMessage/formatTokens/formatContextUsageLine/asString | ~120L | W2 |
| `view_chat_log.go` | 🟡 聊天消息列表渲染 + 工具追踪 Map(CL-01): toolById/streamingRuns + startTool/updateToolArgs/updateToolResult | ~330L | W2 |
| `view_message.go` | 单条消息渲染（用户/助手/工具） | ~250L | W3 |
| `view_tool.go` | 🔴 **审计新增(TE-01)** 工具执行渲染组件：pending→running→success/error 状态机、参数格式化、输出预览裁剪 12 行。**⚡ 需同步修复 `agents/tools/display.go`**(DEP-audit P1): 补 meta fallback + detailKeys 多字段 + read/write/edit/attach 特殊 detail 解析 + `redactToolDetail()` + `shortenHomeInString()` | ~180L | W3 |
| `view_input.go` | 🟡 输入框 + slash 命令自动补全(IN-01): bubbles/textarea + `/` 触发补全列表 | ~230L | W3 |
| `view_status.go` | 状态栏 + 连接指示器 | ~100L | W3 |
| `commands.go` | 🟡 完整命令注册表(CMD-01): 20+ 内置命令 + 动态 gateway 自定义命令 + getArgumentCompletions 参数补全 + **⚡ skillCommands 技能命令动态注入**(DEP-audit P1) | ~300L | W4 |
| `event_handlers.go` | 🟡 键盘/鼠标/resize 事件分发 + pruneRunMap(E-01): >200 条裁剪至 150 + 10 分钟过期 + tool 流三阶段(E-02) + verboseLevel 过滤 | ~300L | W4 |
| `local_shell.go` | 🔴 **审计新增(LS-01)** `!command` 本地 shell 执行：os/exec 子进程 + 安全确认弹窗 + 输出截断 40KB | ~100L | W4 |
| `session_actions.go` | 🟡 会话切换/创建/历史加载 + applyAgentsResult(S-01) + model override 3 层优先级(S-02) + 历史消息重建(S-03) | ~250L | W5 |
| `overlays.go` | 🟡 模态弹窗 + bubbles/list fuzzy filtering 替代 TS 自研组件(SS-01)，需定制 FilterFunc 支持 4 层优先级搜索。**⚡ 需注意 visibleWidth 替代**(DEP-audit P3): lipgloss.Width() 更准确，需额外处理 OSC-8 超链接剥离 | ~280L | W5 |
| `theme.go` | 🟡 完整 lipgloss 主题(TH-01): 6 个主题对象 + alecthomas/chroma 代码高亮(TH-02) 替代 cli-highlight | ~200L | W5 |

## 窗口分配

| 窗口 | 内容 | 依赖 | 验证 |
|------|------|------|------|
| W1 | model + gateway_ws（核心骨架） | 无 | 可连接 gateway 并收发消息 |
| W2 | stream_assembler + **formatters**(🔴) + view_chat_log | W1 | 流式消息 + 格式化 + 工具追踪正确渲染 |
| W3 | view_message + **view_tool**(🔴) + view_input + view_status | W2 | 完整聊天 UI + 工具执行渲染可用 |
| W4 | commands + event_handlers + **local_shell**(🔴) | W3 | slash 命令 + `!command` 本地执行工作 |
| W5 | session_actions + overlays + theme | W4 | 会话管理 + 主题切换 + 代码高亮 |
| W6 | 集成测试 + 修复 | W5 | 全部功能通过 |
| W7 | 性能调优 + 边缘场景修复 | W6 | 压力测试通过 |

## 联网验证的参考项目

- **ZUSE**（2025-07）：Go + bubbletea + lipgloss 完整 IRC 客户端
- **whisper**：WebSocket + bubbletea 终端聊天
- **go-chat**：bubbletea + WebSocket IRC 风格聊天
- **Charm 官方**（2025-09）：`tea.Listen()` + channel 实时推送

## 依赖

```
github.com/charmbracelet/bubbletea  # 已在 go.mod
github.com/charmbracelet/bubbles    # textarea/viewport/list/spinner
github.com/charmbracelet/lipgloss   # 样式引擎 + Width()（替代 TS visibleWidth）
github.com/charmbracelet/glamour    # 🔴 审计新增 — Markdown 渲染（替代 pi-tui Markdown 组件）
github.com/alecthomas/chroma/v2     # 🔴 审计新增 — 代码高亮（替代 cli-highlight）
```

## 进度跟踪

- [x] W1：核心骨架（model + gateway_ws）✅ 审计通过 2026-02-20
- [ ] W2：流式组装 + **格式化器(🔴)** + 消息列表渲染
- [ ] W3：完整聊天 UI（消息 + **工具渲染(🔴)** + 输入 + 状态栏）
- [ ] W4：命令 + 事件处理 + **本地 shell(🔴)**
- [ ] W5：会话管理 + overlay + 主题 + 代码高亮
- [ ] W6：集成测试 + 修复
- [ ] W7：性能调优 + 边缘场景修复

---

## 审计补全：隐藏依赖清单

> 来源：`global-audit-tui.md` 隐藏依赖审计章节

| # | 类别 | 检查结果 | Go 替代方案 |
|---|------|---------|------------|
| 1 | npm 包 `@mariozechner/pi-tui` | 提供 12 个 UI 原语（TUI/Container/Text/Box/Markdown/Input/SelectList 等） | bubbletea + bubbles + lipgloss **全部替代** + glamour（Markdown 渲染） |
| 2 | npm 包 `chalk` | ANSI 颜色 | lipgloss 替代 ✅ 低风险 |
| 3 | npm 包 `cli-highlight` | 代码高亮 + `supportsLanguage()` | `alecthomas/chroma` ⚠️ 需新增依赖 |
| 4 | 外部模块依赖（14 个） | config/config、gateway/client/protocol/call、agents/tool-display/pi-embedded-helpers/agent-scope、routing/session-key、infra/format-time、utils/usage-format/message-channel、auto-reply/commands-registry/thinking、terminal/ansi | ✅ **已验证 12/14**，2 个 P2 级薄封装（见依赖审计报告 `global-audit-tui-deps.md`） |
| 5 | 环境变量 | `OPENACOSMI_GATEWAY_TOKEN` / `OPENACOSMI_GATEWAY_PASSWORD` | ✅ Go config 模块已有 |
| 6 | 进程/平台依赖 | `process.platform`/`process.cwd`/`child_process.spawn` | `runtime.GOOS`/`os.Getwd`/`os/exec` ⚠️ 中风险 |
| 7 | 全局状态 | 无全局单例，闭包封装 | ✅ bubbletea Model 天然解决 |

## 审计补全：P2 行为差异速查

> 以下差异不影响核心功能，但需在实现时逐一对齐

| ID | 文件 | 要点 |
|----|------|------|
| T-01 | model.go | submit handler 三路分发：`!`(shell) / `/`(命令) / 普通消息 |
| T-02 | model.go | `localRunIDs map[string]struct{}` 区分本地/外部 run |
| T-03 | model.go | 动态更新 autocomplete provider（slash + 文件路径） |
| T-04 | model.go | `formatSessionKey` 截断长 key |
| G-01 | gateway_ws.go | 12 个协议字段完整传递 |
| G-02 | gateway_ws.go | 连接 URL 6 层 fallback：CLI→env→config→本地默认 |
| E-01 | event_handlers.go | pruneRunMap: >200→150 + 10min 过期 |
| E-02 | event_handlers.go | tool 流三阶段 + verboseLevel 过滤 |
| S-01 | session_actions.go | applyAgentsResult 更新 agent 映射 |
| S-02 | session_actions.go | model override 3 层优先级 |
| S-03 | session_actions.go | loadHistory 历史消息重建 |
| C-01 | commands.go | 20+ 命令分支完整实现 |

---

## 依赖审计发现：待修复事项

> 来源：`global-audit-tui-deps.md` 补充审计章节，☄️ = 需在 TUI 窗口中同步处理

| # | 所属文件 | 事项 | 优先级 | 窗口 |
|---|---------|------|--------|------|
| 1 | `agents/tools/display.go` | 补 `meta` 参数 fallback | P1 | W3 |
| 2 | `agents/tools/display.go` | 补 `detailKeys[]` 多字段解析（替代单一 DetailField） | P1 | W3 |
| 3 | `agents/tools/display.go` | 补 read/write/edit/attach 特殊 detail 解析（路径+offset） | P1 | W3 |
| 4 | `agents/tools/display.go` 或新建 | 实现 `redactToolDetail()` 工具详情脱敏 | P2 | W3 |
| 5 | `agents/tools/display.go` 或新建 | 实现 `shortenHomeInString()` 路径 `~` 缩写 | P2 | W3 |
| 6 | `autoreply/commands_registry.go` | 补 `skillCommands` 技能命令动态注入参数 | P1 | W4 |
| 7 | `gateway_ws.go` (TUI 层) | 实现 token/password 6 层 fallback 组装逻辑 (~30L) | P2 | W1 |
| 8 | `overlays.go` (TUI 层) | lipgloss.Width() 替代 visibleWidth，需确认/补 OSC-8 超链接剥离 | P3 | W5 |

---

*独立自 `phase5-research-plan.md`，2026-02-19 | 审计补全：2026-02-19 | 依赖审计更新：2026-02-19*
