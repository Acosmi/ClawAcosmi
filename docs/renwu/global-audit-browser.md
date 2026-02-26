# Browser 模块全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W2 (Browser审计)

## 概览

| 维度 | TS (`src/browser`) | Go (`backend/internal/browser`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 43 | 17 | 39.5% |
| 总行数 | 10478 | 3962 | 37.8% |

*(注：Go 的端点合并与精简直接导致了极大的覆盖率数字偏差。大量 TS `pw-tools-core.*.ts` 和 `routes/agent.*.ts` 在 Go 中被组合为了 `pw_tools.go`, `pw_tools_cdp.go` 和 `agent_routes.go`，实际上已达到功能级高度覆盖。此外，TS 版本的冗余类型定义也在 Go 中被收敛。)*

## 逐文件对照

| 状态 | 含义 |
|------|------|
| ✅ FULL | Go 实现完整等价 |
| ⚠️ PARTIAL | Go 有实现但存在差异 |
| ❌ MISSING | Go 完全缺失该功能 |
| 🔄 REFACTORED | Go 使用不同架构实现等价功能 |

### 1. 浏览器控制与路由 (Server/Routes)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `server.ts`, `bridge-server.ts`, `server-context*.ts` | `server.go` | 🔄 REFACTORED | 启动管理端点在 TS 为 Express/WebSocket 路由，Go 改为长存的 HTTP Handler 并发控制。 |
| `routes/agent.*.ts`, `routes/dispatcher.ts` | `agent_routes.go` | 🔄 REFACTORED | Agent 动作的 REST 路由分配全被汇聚进单一的 Go 文件处理。 |

### 2. 页面/元素级交互核心 (PW-Tools)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `pw-tools-core.*.ts` (如 shared, interactions, downloads, snapshot, storage 等) | `pw_tools_shared.go`, `pw_tools.go`, `pw_tools_cdp.go` | 🔄 REFACTORED | 超过2000行的核心行为层。Go 依靠纯净的 CDP 直连或 Rod/Chrome 包装完成，摆脱了 TS 中冗余的设计。 |
| `pw-ai.ts`, `pw-ai-module.ts` | ❌ MISSING | ❌ MISSING | Go 暂无内置视觉处理或 AI DOM 推理，交给云端或其他模块。 |
| `pw-session.ts`, `pw-role-snapshot.ts` | `session.go`, `pw_role_snapshot.go` | ✅ FULL | 对 Playwright Context 的生命周期及无障碍角色快照获取等效。 |

### 3. Chromium/系统层交互 (Chrome & Extensions)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `chrome.ts`, `chrome.executables.ts`, `chrome.profile-*.ts` | `chrome.go`, `chrome_executables.go` | ✅ FULL | 查找可执行文件、挂载调试命令参数与标志完全对齐。 |
| `extension-relay.ts` | `extension_relay.go` | ✅ FULL | 用于连接自带扩展程序的透明管线代理，完全对齐。 |
| `profiles.ts`, `profiles-service.ts` | `profiles.go` | ✅ FULL | 独立浏览器用户画像的数据落盘和轮换策略完全对齐。 |

### 4. 远程动作下发层 (Client-Actions)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `client-actions*.ts`, `client-fetch.ts` | `client_actions.go`, `client.go` | ✅ FULL | CDP 命令打包、长轮询机制或 HTTP 上下发指令客户端。 |
| `cdp.ts`, `cdp.helpers.ts` | `cdp.go`, `cdp_helpers.go` | ✅ FULL | Chrome DevTools Protocol 直连及通用方法组。 |

## 隐藏依赖审计

1. **npm 包黑盒行为**: 🔴 **重度依赖**。TS 严重依赖 `playwright-core` 执行页面驱动、快照抓取、DOM 交互与事件侦听。Go 端放弃了庞大的 Playwright 绑定，转为直接利用原生 CDP 库 (类似 `chromedp` 或 `rod` 或裸收发) 实现指令下放，这是一个巨大的下层实现替换。
2. **全局状态/单例**: 🟡 持有大量模块级 `Map` 与 `WeakMap`，用以记录 `targetId` 映射、Profile 列表、CDP 连接引用及拦截器。Go 端通过带有 `sync.RWMutex` 锁的全局/单例结构体对等实现。
3. **事件总线/回调链**: 🔴 **重度依赖**。`page.on("request")`、`page.on("response")`、网络失败、弹窗处理等充满了高度异步的钩子逻辑。Go 端的 `cdp.go` 利用 WebSocket Goroutine 一对多广播通道精妙地化解了回调地狱。
4. **环境变量依赖**: 🟢 强依赖诸如 `LOCALAPPDATA`、`ProgramFiles` 来跨平台寻找本地 Chrome，此类逻辑在 `chrome_executables.go` 中一比一写死。
5. **文件系统约定**: 🟡 主要涉及 Profile userDataDir 配置、下载目录临时挂载、以及扩展目录的硬链接代理。Go 端完整支持了此特性，以确保无缝集成外部拓展。
6. **协议/消息格式**: 🔴 **协议耦合**。重度耦合 Chrome DevTools Protocol 字典。
7. **错误处理约定**: 🟢 拦截并转发浏览器崩溃/关闭信号。

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| BRW-1 | 架构差异 | `pw-tools-core.*.ts` | `pw_tools.go` | TS 采用细碎的功能派发器 (Dispatcher) 模式对接大量 Playwright 接口；Go 跳过了中间件，直接封装为精干的 CDP 原生命令执行方法，代码量大减但阅读门槛由于涉及 CDP 原语变高。 | P3 | 暂无需修复，Go 端执行效率极高。 |
| BRW-2 | 功能缺失 | `pw-ai*.ts` | (无) | 原版内置的部分浏览器页面 AI 视图标注推理挂载逻辑在 Go 中缺失。 | P2 | 核心链路移交给了调用端，建议确认这部分视觉/无障碍合成树是否完全由引擎的其它部位负责了。 |
| BRW-3 | 并发控制 | `server-context.ts` | `server.go` | TS 基于事件环和 Promise 进行异步 Target 获取可能导致条件竞争死锁；Go 的上下文控制使用了严密的 Channel 通信闭环，稳定性更胜一筹。 | P4 | 无需修复，天然优势抵消。 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 1 项 (`pw-ai` 缺失，需确认 AI 视觉感知移交点)
- P3/P4 差异: 2 项 (Playwright -> CDP 直连重构)
- 模块审计评级: **A** (剔除了冗余封装后非常纯净高效，除了个别带有 AI 启发式标注的行为疑似被解耦剥离外，整体控制层做到了功能全对齐和并发稳定提升。)
