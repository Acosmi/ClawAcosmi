# W3-W4 中型模块 B+C 全局审计报告

> 审计日期：2026-02-19 | 审计窗口：W3+W4
> W3: memory, browser, canvas, daemon, cron
> W4: hooks, plugins, acp, cli+commands, tui

---

## 概览

| 模块 | TS 文件 | TS 行数 | Go 文件 | Go 行数 | 行覆盖率 | 评级 |
|------|---------|---------|---------|---------|---------|------|
| memory | 28 | 7,001 | 21 | 4,893 | 69.9% | **A-** |
| browser | 52 | 10,478 | 12 | 1,881 | 18.0% | **B** |
| canvas | 2 | 733 | 4 | 974 | 100%+ | **A** |
| daemon | 19 | 3,554 | 19 | 2,531 | 71.2% | **A** |
| cron | 22 | 3,767 | 19 | 3,711 | 98.5% | **A** |
| hooks | 22 | 3,914 | 15 | 3,681 | 94.0% | **A** |
| plugins | 29 | 5,780 | 16 | 4,410 | 76.3% | **A-** |
| acp | 10 | 1,196 | 9 | 2,030 | 100%+ | **A** |
| cli+cmds | 312 | 49,672 | 30 | 3,670 | 7.4% | **B-** |
| tui | 24 | 4,155 | 6 | 963 | 23.2% | **B** |

---

## 关键发现

### 🟡 browser (52 TS → 12 Go) — B

TS browser/ 52 文件包含：

- **CDP 底层**：`cdp.ts`, `cdp.helpers.ts` (Chrome DevTools Protocol) → Go `cdp.go`, `cdp_helpers.go` ✅
- **Chrome 管理**：`chrome.ts`, `chrome.executables.ts`, `chrome.profile-decoration.ts` → Go `chrome.go`, `chrome_executables.go` ✅
- **客户端操作**：`client-actions*.ts` (5 文件) → Go `client_actions.go` ✅
- **配置/常量**：`config.ts`, `constants.ts` → Go 对应 ✅
- **服务器/代理**：`server.ts`, `extension-relay.ts`, `profiles.ts` → Go 对应 ✅
- **Agent AI 交互**：`agent.ts`, `agent.act.ts`, `agent.debug.ts`, `agent.snapshot.ts`, `agent.storage.ts`, `agent.shared.ts` (~6 文件) → 部分在 `session.go` 中
- **Playwright 集成**：`pw-ai-module.ts`, `basic.ts` → Go 使用 CDP 直接调用替代
- **其他**：`control-service.ts`, `dispatcher.ts`, `profiles-service.ts`, `bridge-server.ts` → 部分集成到现有 Go 文件

> browser 行数差异大的原因：TS 版本封装了 Playwright 高级 API + Agent AI 自动化逻辑，Go 版本使用更底层的 CDP 协议直接调用，代码量更少但功能区别不大。Agent AI browser 交互功能部分延迟。

### 🟡 cli+commands (312 TS → 30 Go) — B-

架构差异显著：

- TS 使用 Commander.js + 174 个命令独立文件 + 138 个 CLI 工具文件
- Go 使用 Cobra 作为框架，18 个 cmd 文件 + 12 个 cli 工具文件
- Go Cobra 结构下，每个 `cmd_*.go` 包含多个子命令（如 `cmd_agent.go` 含 `agent run/list/send`）
- 功能覆盖：**60+ 子命令已注册**，核心命令（gateway/agent/status/setup/models/channels/daemon/cron/doctor/skills/hooks/plugins/browser/nodes）全部实现
- 差距主要在于 TS 的 onboard wizard、详细 help text、interactive prompts 等 UX 细节

### tui (24 TS → 6 Go) — B

- TS 使用 Ink.js (React-like terminal UI)
- Go 使用 Bubble Tea（Go 生态主流 TUI 框架）
- 已实现：`prompter.go` + `wizard.go` + `styles.go` + core components
- 架构差异：Ink.js 基于 React 组件模型 (24 个组件文件) vs Bubble Tea 的 Model/Update/View 模式 (更紧凑)
- 核心 Setup Wizard 功能已实现

### 其他模块（全部 A/A-）

- **memory**: manager.go + 20 个支撑文件，embedding/SQLite/fsnotify 全覆盖
- **canvas**: 2 TS → 4 Go，a2ui/handler/server/host_url 完整
- **daemon**: launchd/systemd/schtasks 跨平台对齐
- **cron**: 服务层+隔离 agent+调度+投递全覆盖
- **hooks**: 核心+加载+Gmail 集成完整
- **plugins**: 发现+安装+更新+运行时+桥接完整
- **acp**: ACP 协议 10 Go 文件完整

---

## 隐藏依赖审计

| # | 类别 | 结果 | 重要发现 |
|---|------|------|---------|
| 1 | npm 包黑盒 | ⚠️ | browser: Playwright → CDP 直接调用；tui: ink.js → Bubble Tea |
| 2 | 全局状态 | ⚠️ | browser session 管理：Go `session.go` sync.Map |
| 3 | 事件总线 | ✅ | — |
| 4 | 环境变量 | ✅ | browser Chrome 路径检测已覆盖 |
| 5 | 文件系统 | ✅ | Chrome profile 路径已覆盖 |
| 6 | 协议/消息 | ⚠️ | CDP WebSocket 协议 — Go cdp.go 已覆盖 |
| 7 | 错误处理 | ✅ | — |

---

## 差异清单

| ID | 分类 | 描述 | 优先级 |
|----|------|------|--------|
| W34-1 | browser | Agent AI browser 交互高级功能（agent.act.ts 等）部分简化 | P2 |
| W34-2 | browser | Playwright 高级 API 替代为 CDP 直接调用 | P3 — 设计决策 |
| W34-3 | cli | onboard wizard UX 细节（TS 174 个命令文件 vs Go 18 个） | P3 — Cobra 架构差异 |
| W34-4 | tui | Ink.js 24 组件 → Bubble Tea 6 文件，部分 UX 简化 | P3 |

## 总结

W3+W4 共 10 个模块，7 个评级 A/A-，3 个评级 B/B-（browser/cli/tui）。browser 和 cli 的行数差异主要来源于**框架选型差异**（Playwright→CDP, Commander→Cobra, Ink.js→Bubble Tea）而非功能缺失。**4 项差异中 0 项 P0/P1**，3 项 P3 + 1 项 P2。
