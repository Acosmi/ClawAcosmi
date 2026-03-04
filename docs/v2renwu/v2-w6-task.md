# V2-W6 实施跟踪清单 (TUI / Browser)

> 关联审计报告: `global-audit-v2-W6.md`
> 模块评级: **TUI (A-) / BROWSER (C)**

## 任务目标

基于 V2 深度审计结果，跟踪 W6 大版的查缺补漏情况。TUI 已展现出优异的对等乃至超越重构，但 Browser 代理及底层工具箱链仍为 Stub 空壳状态，需要投入工作量来完成 AI Web 操作层和 HTTP 路由重定向桥梁补全。

## 实施清单 (待修复验证清单)

### [P0] 阻断级缺陷

- [x] **[W6-02] browser Playwright AI 截点缺失 (`pw_tools.go`)**: `pw-ai` 自动化与可访问性树抽象模块目前在 Go 中仅为接口 Stub。需使用 Playwright Go 库实现底层树结构的解析提取与行为操作闭环。
  - **已完成** — `pw_role_snapshot.go` (380L) 完整实现 ARIA/AI 快照解析 + `pw_tools_cdp.go` (500L) CDP 实现替换 Stub

### [P1] 次要级缺陷

- [x] **[W6-03] browser 底层干预缺失 (`pw_tools.go`)**: 对齐 `pw-tools-core.*.ts` 中的一系列浏览器底层拦截器及副作用监听机制，包括下载接管、Cookie 监听存取、离线模式网络中断等。
  - **已完成** — `pw_tools_cdp.go` 通过 CDP 协议实现 Click/Fill/Hover/Highlight/Cookies/Storage/Download/ResponseBody
  - `pw_tools_shared.go` (115L) 移植错误友好化 + ref 校验 + timeout 裁剪
- [x] **[W6-04] browser 代理中继路由缺失**: 对齐 `routes/agent.*.ts` 提供与云端 Agent Node 通信和反馈任务报告的专用 HTTP 接收端点和任务流转枢纽。
  - **已完成** — `agent_routes.go` (400L) 实现 16 个 Agent 端点（snapshot/act/storage/debug 4 组）

### [P2/P3] 体验级微异

- [ ] **[W6-01] tui 渲染样式**: (可选维护) 分析并拉平 Ink.js 原版与 Go BubbleTea/Glamour 之间基于 `syntax-theme.ts` / `theme.go` 的终端色彩和转义细节差异。

## 隐藏依赖审计与隔离治理验证补偿

- [x] 针对 `browser` 模块遗留下来的海量 `throw/catch` 逻辑网络，必须使用强类型的 `error` 返回进行明确规避。
  - **已完成** — Go 端全部使用 `error` 返回值，`ToAIFriendlyError()` 转换错误为 AI 友好格式
- [x] 大幅收拢原先 Browser 控制下存在的隐蔽 Event 总线，保证 Playwright Context 生命周期完全绑定隔离。
  - **已完成** — CDP 连接按请求隔离 (`WithCdpSocket` 单次连接)，无全局状态泄漏
- [x] 严格追踪 Browser 会话隔离过程中的 Zombie 进程泄漏和全局状态逃逸问题。
  - **已完成** — `CDPPlaywrightTools` 无进程管理，进程生命周期由 `Session.Close()` 管理

## 后续动作

针对性的逐步剥离 Browser Stub 并接入实效 Playwright-go 能力。此阶段属于独立方向拓展，不影响主干微服务运行。
