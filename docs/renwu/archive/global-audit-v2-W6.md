# tui & browser 全局审计报告

> 审计日期：2026-02-20 | 审计窗口：W6
> 版本：V2（反映 TUI Phase 5 独立重构后的最新状态）

## 概览

| 模块 | TS 文件数 | Go 文件数 | 文件覆盖率 | TS 行数 | Go 行数 | 行覆盖率 |
|------|-----------|-----------|-----------|---------|---------|--------|
| **tui** | 24 | 21 | 87.5% | ~4155 | ~6087 | ~146% |
| **browser** | 52 | 13 | 25.0% | ~10478 | ~2113 | ~20% |

## 逐文件对照

### TUI 模块

主要基于 BubbleTea 框架重构，替代了原有的 Ink 框架，实现了全面对等覆盖。

| 状态 | 文件/功能组 | TS 源文件示例 | Go 目标文件示例 | 备注 |
|------|------------|--------------|---------------|------|
| 🔄 REFACTORED | 事件驱动状态机 | `tui.ts`, `tui-event-handlers.ts` | `tui.go`, `event_handlers.go`, `model.go` | TS版本为React-like, Go采用Elm架构 |
| ✅ FULL | 流组装器 | `tui-stream-assembler.ts` | `stream_assembler.go` | |
| ✅ FULL | 会话/向导动作 | `tui-session-actions.ts` | `session_actions.go`, `wizard.go` | |
| 🔄 REFACTORED | 终端组件集合 | `components/*` | `view_*.go`, `prompter.go` | UI 渲染对等 |
| ✅ FULL | 命令及热键 | `commands.ts` | `commands.go` | |
| ✅ FULL | 本地 Shell | `tui-local-shell.ts` | `local_shell.go` | |
| ✅ FULL | 网关通信桥 | `gateway-chat.ts` | `gateway_ws.go` | |

### Browser 模块

目前 Go 端仅实现了基于 CDP 和 HTTP 的基础桥连接，核心的 Playwright AI 操作层及工具链均处于未实现或 Stub 状态。

| 状态 | 文件/功能组 | TS 源文件示例 | Go 目标文件示例 | 备注 |
|------|------------|--------------|---------------|------|
| ✅ FULL | 浏览器启停/状态基座 | `server.ts`, `cdp.ts`, `chrome.ts` | `server.go`, `cdp.go`, `chrome.go` | 基础 CDP 机制已对接 |
| ✅ FULL | 多配置档案管理 | `profiles.ts`, `config.ts` | `profiles.go`, `config.go` | |
| ❌ MISSING | AI 快照及解析节点 | `pw-ai.ts`, `pw-role-snapshot.ts` | - / Stub (`pw_tools.go`) | Go 中 `StubPlaywrightTools` 仅为空壳 |
| ❌ MISSING | pw-tools 核心操作 | `pw-tools-core.*.ts` | `pw_tools.go` (Stub) | 下载、离线、网络拦截全部缺失 |
| ❌ MISSING | Agent 路由级交互 | `routes/agent.*.ts` | - | 缺失了各种向后端 Agent 返回分析结果的路由 |
| ❌ MISSING | TUI 代理请求 | `client-fetch.ts` | - | 对应端到端中间网关转发层未完善 |

## 隐藏依赖审计

| 模块 | npm 黑盒 | 全局状态/单例 | 事件总线/回调链 | 环境变量依赖 | 文件系统 | 错误约定(throw/Error) |
|------|---------|-------------|---------------|------------|---------|-------------------|
| **tui** | 0 | 17 | 4 | 11 | 0 | 21 |
| **browser** | 0 | 35 | 30 | 51 | 43 | 473 |

**审计分析**：

- **TUI**：架构清晰。作为一个客户端直接运行的模块，无网络/磁盘的重度 IO，全程由 BubbleTea 原生状态机（Event 总线）处理更迭。
- **BROWSER**：在原版 TS 中具有极大的历史包袱，包含了 473 处基于 throw/catch 的流程控制，并广泛读写文件系统（43处）与全局事件。在向 Go 迁移时，重构尚未触及深水区，若实现全量映射需留意隔离 Playwright 的 Zombie 进程泄漏及复杂异常边界。

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| W6-01 | 渲染样式 | `syntax-theme.ts` | `theme.go` | 终端下的 Markdown 转义与 Ink.js 的色彩边界细节不同 | P3 | 不影响核心流程，无需强制修复 → 推迟 W6-D1 |
| W6-02 | 核心缺失 | `pw-ai.ts` | `pw_tools.go` | pw-ai 浏览自动化与可访问性树抽象全量丢失 (现为 Stub Interface) | P0 | ✅ 已修复 — `pw_role_snapshot.go` + `pw_tools_cdp.go` CDP 实现 |
| W6-03 | 功能缺失 | `pw-tools-core.*.ts` | `pw_tools.go` | 核心底层拦截器、cookie获取、下载监听空白 | P1 | ✅ 已修复 — `pw_tools_cdp.go` + `pw_tools_shared.go` |
| W6-04 | 路由断层 | `routes/agent.*.ts` | - | 缺失了与云端 Agent Node 之间进行信息汇流的中继路由机制 | P1 | ✅ 已修复 — `agent_routes.go` 16 个 HTTP 端点 |

## 总结

- **P0 差异**: ~~1 项 (Playwright AI 操作层缺失)~~ ✅ 已修复
- **P1 差异**: ~~2 项 (工具链及代理中继路由缺失)~~ ✅ 已修复
- **P2/P3 差异**: 1 项 (局部 UI 样式微异) → 推迟 W6-D1

**模块审计评级**:

- **TUI**: **A-** (重构表现优异卓越，代码结构比原版更健壮并已达到独立可用标准)
- **Browser**: **B** (CDP 底座 + AI 操作层 + Agent 路由已补全，全量通过原生 CDP 实现而非 playwright-go)
