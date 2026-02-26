# commands 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W1 (命令审计)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 134 | 31 | 23.1% |
| 总行数 | 28567 | 4811 | 16.8% |

*(注：由于 Go 端采用集中式 Cobra 注册，文件数和行数大幅精简。Go 代码集中在 `backend/cmd/openacosmi/cmd_*.go` 及 `backend/internal/cli/`)*

## 逐文件对照

| 状态 | 含义 |
|------|------|
| ✅ FULL | Go 实现完整等价 |
| ⚠️ PARTIAL | Go 有实现但存在差异 |
| ❌ MISSING | Go 完全缺失该功能 |
| 🔄 REFACTORED | Go 使用不同架构实现等价功能 |

### 1. 代理与守护进程 (Agent & Daemon)

- `agent*.ts`: Go `cmd_agent.go` ⚠️ PARTIAL (仅实现基础代理控制，某些如 delivery 暂无)
- `daemon*.ts`, `node-daemon-*.ts`: Go `cmd_daemon.go`/`cmd_nodes.go` 🔄 REFACTORED
- `systemd-linger.ts`: Go `cmd_setup.go` 🔄 REFACTORED

### 2. 用户流与配置 (Auth, Onboard, Doctor)

- `auth-choice*.ts`, `onboard*.ts`: Go `cmd_setup.go` ⚠️ PARTIAL (交互式流大幅简化)
- `doctor*.ts`, `health.ts`: Go `cmd_doctor.go`/`cmd_status.go` ✅ FULL
- `status*.ts`, `gateway-status.ts`: Go `cmd_status.go`/`cmd_gateway.go` ✅ FULL
- `configure*.ts`: Go `cmd_setup.go`/`config_guard.go` 🔄 REFACTORED

### 3. 模型与 Oauth (Models, Oauth)

- `models*.ts`: Go `cmd_models.go` ✅ FULL
- `oauth-flow.ts`, `chutes-oauth.ts`: 未找到 Go 对应 ❌ MISSING

### 4. 插件技能与其他 (Skills, Channels, Misc)

- `channels*.ts`: Go `cmd_channels.go` ✅ FULL
- `dashboard.ts`, `message.ts`: 缺失端点命令 ❌ MISSING
- `sandbox*.ts`, `sessions.ts`: 缺失端点命令 ❌ MISSING
- `reset.ts`, `uninstall.ts`, `setup.ts`: Go `cmd_misc.go`/`cmd_setup.go` ⚠️ PARTIAL

## 隐藏依赖审计

1. **npm 包黑盒行为**: 🟢 未发现依赖外部未知黑盒行为，无复杂的第三方包导入。
2. **全局状态/单例**: 🟢 主要为本地 `Map` 缓存和短生命周期单例。
3. **事件总线/回调链**: 🟢 极少，仅用于捕获网络或进程 `on('error')` 事件。
4. **环境变量依赖**: 🔴 **重度依赖**。大量读取 `process.env.OPENACOSMI_STATE_DIR`, `OPENACOSMI_GATEWAY_TOKEN`, `CHUTES_CLIENT_ID` 等。Go 重构需确保环境变量加载优先级一致。
5. **文件系统约定**: 🔴 **重度依赖**。涉及读写 `credentials/`, `sessions/`，状态目录权限校验 (`chmod`) 等，必须在 Go `config_guard.go` 和各命令仔细对齐 FS semantics。
6. **协议/消息格式**: 🟢 CLI 直接交互，无 WebSocket 或复杂序列化强耦合。
7. **错误处理约定**: 🟡 普遍使用抛出 `Error()` 并在主循环中捕捉，Go 中表现为 `cobra` 的 `RunE` 结构和 `fmt.Errorf`。

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| CMD-1 | 功能缺失 | `sandbox*.ts` | 未知 | Go 端暂缺沙盒状态查看与分析 (`explore`, `explain`) 的独立子命令 | P0 | 需在 `cmd_agent.go` 增补或补充 `cmd_sandbox.go` |
| CMD-2 | 功能缺失 | `oauth-flow.ts` | 缺失 | Go 的 Oauth Flow 登录流在本地客户端尚未完整迁移，特别是针对 Chutes 等第三方验证 | P1 | 在 `cmd_setup.go` 或独立文件中实现 OAuth Web Server 并在浏览器中回调 |
| CMD-3 | 体验差异 | `auth-choice*.ts` | `cmd_setup.go` | 原交互式选择提供商和填 KEY 的巨型向导流程，在 Go 中被大幅简化，缺少部分自动化推断逻辑 | P2 | 增强 `setup` 命令的交互行为，提供更灵活的 auth 引导 |
| CMD-4 | 架构差异 | 全部 TS 文件 | `cmd_*.go` | TS 按职责拆分出 130 多个分散文件，Go 则整合收敛至 20 个左右的 `cmd_xxx.go` 大文件内，集中于 `backend/cmd/openacosmi` 和 `cli` 核心路由。 | P3 | 维持现状，Go 的 Cobra 模式及 `cli` 快速路由模块 `TryRouteCli` 性能显著更优。 |
| CMD-5 | 隐性环境依赖 | 多文件 | `cli/dotenv.go` | TS 的 `OPENACOSMI_STATE_DIR` 多级推断在 Go 端需完全对齐，否则可能会覆盖或建立新的不一致存储树。 | P1 | 通过单元测试验证 `backend/internal/cli/dotenv.go` 和 `config.go` 中的路径解析及 fallback 行为。 |

## 总结

- P0 差异: 1 项 (Sandbox CLI 命令群缺失)
- P1 差异: 2 项 (OAuth CLI Web Flow 缺失、FS 路径推断潜在不对齐)
- P2 差异: 1 项 (交互式引导体验差异)
- P3 差异: 1 项 (结构收敛)
- 模块审计评级: **B** (核心状态与守护进程等功能大部分完整，但遗漏了若干进阶管理的 CLI 指令，且复杂引导流程被削减)

- 模块审计评级: 待定
