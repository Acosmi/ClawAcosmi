# CLI 模块全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W2 (CLI审计)

## 概览

| 维度 | TS (`src/cli`) | Go (`backend/internal/cli` + `cmd`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 106 | 12 (cli) + 19 (cmd) | 高度复合 |
| 总行数 | 21105 | 1390 (cli) + 3421 (cmd) | 架构级对齐 |

*说明：此前的 `commands` 模块审计（详见[global-audit-commands.md](./global-audit-commands.md)）因为 TS 的职责分离，将具体执行逻辑置于 `src/commands`，使得 Go 端的 `backend/cmd/openacosmi` 被部分划入了 `commands` 的范畴进行审计。本篇 `cli` 审计聚焦于 TS `src/cli/` 中定义的 CLI 参数绑定、交互行为与顶层入口配置，主要映射到 Go 的 `backend/internal/cli/` 框架层与各 Cobra 初始化方法。*

## 逐文件对照

| 状态 | 含义 |
|------|------|
| ✅ FULL | Go 实现完整等价 |
| ⚠️ PARTIAL | Go 有实现但存在差异 |
| ❌ MISSING | Go 完全缺失该功能 |
| 🔄 REFACTORED | Go 使用不同架构实现等价功能 |

### 1. CLI 核心执行框架 (Framework & Context)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `program*.ts`, `program/*.ts` | `backend/internal/cli/` (`route.go`, `argv.go`, `config_guard.go`) | 🔄 REFACTORED | TS 使用自定义庞大的构建器（底层基于 `commander` 和 `clack`）；Go 完全切换至 Cobra 主线应用注册挂载体系。 |
| `run-main.ts` | `backend/cmd/openacosmi/main.go` | ✅ FULL | 前端入口 |

### 2. 公共 CLI 工具组件 (CLI Utilities)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `progress.ts`, `banner.ts`, `tagline.ts` | `backend/internal/cli/` (`progress.go`, `banner.go`), `version.go` | ✅ FULL | UI 构建与标志等效实现（TS 含较多节日彩蛋等冗余，Go 做了一定精简）。 |
| `command-options.ts`, `prompt.ts` | `backend/internal/cli/globals.go` | 🔄 REFACTORED | 全局参数静默和提问覆盖逻辑被集中管控。 |

### 3. CLI 端点注册层 (Endpoints Registration)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `daemon-cli*.ts`, `gateway-cli*.ts` | `cmd_daemon.go`, `cmd_gateway.go` | 🔄 REFACTORED | 注册命令、拉出帮助文档并转发请求至逻辑层。 |
| `browser-cli*.ts`, `channels-cli.ts` | `cmd_browser.go`, `cmd_channels.go` | 🔄 REFACTORED | CLI 终端交互包装层。 |
| `nodes-cli*.ts`, `config-cli.ts` | `cmd_nodes.go`, `cmd_setup.go` | 🔄 REFACTORED | 对于 Node 指令的高级交互封装在 Go 端做了扁平化重整。 |

## 隐藏依赖审计

1. **npm 包黑盒行为**: 🟢 TS 大量依赖类似 `commander`。Go 的等价物为 `cobra` 与 `viper`，未见隐藏黑盒。
2. **全局状态/单例**: 🟡 **中度依赖**。TS 端有一些直接覆盖的全局行为（如 `isYes()`, `isVerbose()`），Go 在 `backend/internal/cli/globals.go` 内使用了强类型的 `atomic` 或结构管理维护单例配置。
3. **事件总线/回调链**: 🟡 **中度依赖**。TS 会监听 `process.on('SIGTERM')` 和 `SIGUSR1` 处理 Gateway/Daemon 的软重启或退出。Go 则利用 context 与标准 `os/signal` 通道实现了等价系统级退出和优雅关闭 (详见 `gateway_rpc.go`、`cmd_gateway.go`)。
4. **环境变量依赖**: 🟡 **中度依赖**。`OPENACOSMI_PROFILE`, `OPENACOSMI_CLAUDE_CLI_LOG_OUTPUT`, `OPENACOSMI_RAW_STREAM` 等。Go 由 `dotenv.go` 搭配 `config_guard` 实现了对齐装载。
5. **文件系统约定**: 🟡 对临时扩展解压工作区（browser extensions）、写入配置文件强依赖，Go 的 CLI 体系能够同等履行文件系统管理，不存在不可逆的差异。
6. **协议/消息格式**: 🟢 针对远程网关时，通过 `gateway_rpc.go` 实现 CLI 端对核心引擎的 WebSocket/HTTP 交互。
7. **错误处理约定**: 🟢 统一的日志阻断捕捉。

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| CLI-1 | 架构差异 | `src/cli/program/` | `internal/cli` | TS 的自定义路由器异常繁冗，Go 端借助标准 cobra 极大降低了维护成本与圈复杂度。 | P4 | 无需修复，重构架构更优。 |
| CLI-2 | 细节丢失 | `tagline.ts` | `banner.go` | TS 版带有的多国/节日特色 Banner 输出并未全部平移，属于体验降级。 | P4 | 无需修复，非核心。 |
| CLI-3 | 控制流 | `daemon-cli.ts` | `cmd_daemon.go` | TS 中含有多进程控制及复杂的 coverage 保底钩子，Go 因为是单进程高并发天生优势，省去此类繁杂钩子。 | P4 | 无需修复，天然优势抵消。 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 0 项
- P3/P4 差异: 3 项 (架构级提升引发的代码精简，控制流变化)
- 模块审计评级: **S** (TS 的面条式 CLI 定义网被规整收敛到了清晰的标准中，核心路由逻辑 `backend/internal/cli` 为未来提供了强力护航)
