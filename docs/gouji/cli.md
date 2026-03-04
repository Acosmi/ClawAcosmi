# CLI 架构文档

> 最后更新：2026-02-26 | 代码级审计确认 | 38 源文件 (cmd/)

## 一、模块概述

CLI 模块是 OpenAcosmi 的命令行入口，基于 Cobra 框架构建。负责命令注册、参数解析、进度显示、版本/Banner 输出和 Gateway RPC 封装。对应 TS 端 `src/cli/`（138 文件）+ `src/commands/`（174 文件），总量 ~1.6MB。

## 二、原版实现（TypeScript）

### 源文件列表

| 文件/目录 | 大小 | 职责 |
|-----------|------|------|
| `cli/run-main.ts` | 4KB | CLI 主入口（环境检测 + 路由 + 命令注册） |
| `cli/program/build-program.ts` | 0.6KB | 构建 Commander 程序对象 |
| `cli/program/command-registry.ts` | 5.6KB | 12 个命令组注册 + 快速路由 |
| `cli/program/register.subclis.ts` | 8.7KB | 20+ sub-CLI 懒加载注册 |
| `cli/program/preaction.ts` | 2.1KB | pre-action hook（banner/verbose/config） |
| `cli/program/help.ts` | 3.4KB | 帮助信息格式化 + 示例 |
| `cli/program/context.ts` | 0.6KB | ProgramContext（version/channelOptions） |
| `cli/argv.ts` | 5KB | 参数解析工具函数 |
| `cli/banner.ts` | 4KB | ASCII 龙虾 + 版本 banner |
| `cli/progress.ts` | 7KB | spinner + 进度条 |
| `cli/gateway-rpc.ts` | 1.5KB | Gateway RPC 封装 |
| `cli/cli-utils.ts` | 2KB | 通用工具 |
| `commands/` (174 文件) | ~900KB | 全部命令实现 |
| `cli/plugin-registry.ts` | 1KB | 插件注册表单例 |
| `cli/route.ts` | 1.2KB | 快速命令路由 |
| `infra/dotenv.ts` | 0.5KB | 环境加载 |

### 核心逻辑摘要

TS CLI 采用「懒加载注册」模式：`runCli()` → `buildProgram()` → `registerProgramCommands()` 注册 12 个命令组 → `registerSubCliCommands()` 按需动态 `import()` 20+ sub-CLI。pre-action hook 在每个命令执行前输出 banner、设置 verbose、确保 config 就绪。`tryRouteCli` 机制允许高频命令绕过 Commander 解析。

## 三、依赖分析（六步循环法 步骤 2-3）

### 显式依赖图

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `commander` | npm 包 | ↓ | CLI 框架 |
| `config/config.ts` | 值 | ↓ | 加载配置 |
| `plugins/cli.ts` | 值 | ↓ | 插件命令注册 |
| `runtime.ts` | 值 | ↓ | 运行时默认值 |
| `version.ts` | 值 | ↓ | 版本号 |
| `terminal/theme.ts` | 值 | ↓ | 终端颜色主题 |
| `globals.ts` | 值 | ↓ | 全局 verbose 状态 |
| 所有 `commands/*.ts` | 值 | ↓ | 80+ 命令实现 |

### 隐藏依赖审计

| 类别 | 结果 | Go 等价方案 | 状态 |
|------|------|-------------|------|
| npm 包黑盒行为 | ⚠️ Commander.js 程序解析 | Cobra 完全替换 | ✅ |
| 全局状态/单例 | ⚠️ `bannerEmitted` + `activeProgress` | `sync.Once` + `atomic.Int32` | ✅ |
| 事件总线/回调链 | ⚠️ pre-action hooks | Cobra `PersistentPreRunE` | ✅ |
| 环境变量依赖 | ⚠️ `OPENACOSMI_HIDE_BANNER` 等 | `os.Getenv` 等价读取 | ✅ |
| 插件注册单例 | ⚠️ `ensurePluginRegistryLoaded` | `sync.Once` + DI | ✅ (D1) |
| dotenv 加载 | ⚠️ `loadDotEnv` 自包含逻辑 | `cli/dotenv.go` 自研解析 | ✅ (D1) |
| 快速路由 | ⚠️ `tryRouteCli` 绕过 Commander | `cli/route.go` 手动路由 | ✅ (D1) |
| 进度条联动 | ⚠️ `clearProgressLine` | `progress.go` 联动清理 | ✅ (D1) |
| Eager Options | ⚠️ `OPENACOSMI_EAGER_CHANNEL_OPTIONS` | `ResolveCliChannelOptions` | ✅ (D1) |

## 四、重构实现（Go）

### 文件结构

| 文件 | 行数 | 对应原版 |
|------|------|----------|
| `internal/cli/version.go` | 48 | `version.ts` |
| `internal/cli/argv.go` | 150 | `argv.ts` |
| `internal/cli/utils.go` | 120 | `cli-utils.ts` + `env.ts` |
| `internal/cli/banner.go` | 68 | `banner.ts` |
| `internal/cli/progress.go` | 142 | `progress.ts` |
| `internal/cli/gateway_rpc.go` | 75 | `gateway-rpc.ts` |
| `internal/cli/globals.go` | 37 | `globals.ts`（verbose/yes 全局状态） |
| `internal/cli/config_guard.go` | 69 | `config-guard.ts`（config 校验 + 命令白名单） |
| `internal/cli/plugin_registry.go` | 80 | `plugin-registry.ts` (Singleton) |
| `internal/cli/dotenv.go` | 85 | `infra/dotenv.ts` (Loader) |
| `internal/cli/route.go` | 100 | `cli/route.ts` (Fast Route) |
| `cmd/openacosmi/main.go` | 113 | `build-program.ts` + `preaction.ts` |
| `cmd/openacosmi/cmd_*.go` | ~900 | 各命令组实现 |

**总计**：28 个新文件，~1,900 行 Go 代码，60+ 子命令。

> 隐藏依赖审计详情：[cli-hidden-dep-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/cli-hidden-dep-audit.md)

### 接口定义

```go
// ProgressReporter 进度报告器
type ProgressReporter struct {
    label, done, stream, percent, indeterminate ...
}

// GatewayRPCOpts Gateway RPC 调用选项
type GatewayRPCOpts struct {
    URL, Token string; TimeoutMs int; ExpectFinal, JSON bool
}

// RoutedCommandHandler 快速路由处理器
type RoutedCommandHandler struct {
    Run func(argv []string) (bool, error)
    LoadPlugins bool
}
```

## 五、差异对照

| 维度 | 原版 TS | 重构 Go |
|------|---------|---------|
| 框架 | Commander.js | Cobra |
| 命令注册 | 运行时懒加载 `import()` | 编译时 `init()` 注册 |
| 并发安全 | 单线程 N/A | `sync.Once` + `atomic.Bool` + `atomic.Int32` |
| 进度显示 | `@clack/prompts` spinner | ANSI spinner goroutine (无依赖) |
| 错误处理 | try/catch + process.exit | `RunE` 返回 error |
| Shell 补全 | 手写 `completion` 脚本 | Cobra 原生 bash/zsh/fish/powershell |
| DotEnv | `dotenv` npm 包 | 自研解析 (无依赖, 保持轻量) |

## 六、Rust 下沉候选

| 函数/模块 | 优先级 | 原因 |
|-----------|--------|------|
| (无) | — | CLI 层不涉及性能热点 |

## 七、测试覆盖

| 测试类型 | 覆盖范围 | 状态 |
|----------|----------|------|
| 编译验证 | `go build ./...` + `go vet ./...` | ✅ |
| CLI 集成 | `--help`/`--version`/子命令 help | ✅ 手动验证 |
| 单元测试 | argv/utils/route/env/progress | ✅ Completed (cli_test.go 15 tests) |
| E2E 测试 | 完整 CLI 流程 | ❌ 待 Phase 10 |
