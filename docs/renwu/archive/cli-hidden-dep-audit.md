# CLI 隐藏依赖深度审计报告

> 审计范围：`src/cli/` + `src/cli/program/` + 相关 infra 模块
> 审计日期：2026-02-14
> 对照实现：`cmd/openacosmi/` + `internal/cli/`

---

## 一、审计方法

逐文件读取 16+ TS 源文件，按 7 类隐藏依赖逐项检查，与 Go 实现对照。

---

## 二、7 类隐藏依赖检查

### H1: npm 包黑盒行为

| ID | TS 行为 | Go 状态 | 优先级 |
|----|---------|---------|--------|
| H1-1 | Commander.js `parseAsync` 内部行为：自动 `--help`/`--version` 输出、unknown option 处理、default action | ⚠️ Cobra 行为不同：unknown options 默认报错（TS 端某些子 CLI 设置了 `allowUnknownOption`） | P1 |
| H1-2 | Commander.js `hook("preAction")` 执行顺序保证（parent → child） | ⚠️ Cobra `PersistentPreRunE` 类似但只执行最近祖先的；需用 `cobra.OnInitialize` 或手动链 | P2 |

### H2: 全局状态/单例

| ID | TS 行为 | Go 状态 | 优先级 |
|----|---------|---------|--------|
| H2-1 | `globals.ts`: `globalVerbose` + `globalYes` 模块级变量，`setVerbose()`/`setYes()` 全局设置 | ❌ Go 端仅设置 env var `OPENACOSMI_VERBOSE=1`，未实现 `globalYes` 等价逻辑 | P1 |
| H2-2 | `plugin-registry.ts`: `pluginRegistryLoaded` 单例 guard — `ensurePluginRegistryLoaded()` 仅执行一次 | ❌ Go 端无等价实现（`cmd_misc.go` 中 plugins 命令仅为 stub） | P2 |
| H2-3 | `config-guard.ts`: `didRunDoctorConfigFlow` 模块级 flag — 首次命令执行时自动运行 `loadAndMaybeMigrateDoctorConfig` | ❌ Go 端 `PersistentPreRunE` 未实现 config 校验和自动迁移 | P1 |
| H2-4 | `runtime.ts`: `defaultRuntime` — `log`/`error`/`exit` 三个方法各有副作用：清除进度条行、恢复终端状态 | ⚠️ Go 端无 `clearActiveProgressLine` 联动 | P2 |

### H3: 事件总线/回调链

| ID | TS 行为 | Go 状态 | 优先级 |
|----|---------|---------|--------|
| H3-1 | Commander `preAction` hook → banner → verbose → config guard → plugin registry（链式调用） | ⚠️ Go 端 `PersistentPreRunE` 仅实现 profile + banner + verbose，缺少 config guard 和 plugin registry 步骤 | P1 |
| H3-2 | `run-main.ts`: `process.on("uncaughtException")` + `installUnhandledRejectionHandler()` 全局错误兜底 | ⚠️ Go 无等价需求（Go panic/recover 模式不同），但 `SilenceErrors: true` 已处理 | P3 |

### H4: 环境变量依赖

| ID | 环境变量 | TS 用途 | Go 状态 | 优先级 |
|----|----------|---------|---------|--------|
| H4-1 | `OPENACOSMI_HIDE_BANNER` | 隐藏 CLI banner | ✅ `banner.go` 已检查 | — |
| H4-2 | `OPENACOSMI_DISABLE_ROUTE_FIRST` | 禁用快速路由（route.ts） | ❌ Go 端无等价快速路由机制 | P2 |
| H4-3 | `OPENACOSMI_DISABLE_LAZY_SUBCOMMANDS` | 强制全量注册子命令 | ✅ Go 端本就全量注册（无需此变量） | — |
| H4-4 | `OPENACOSMI_EAGER_CHANNEL_OPTIONS` | 急切加载全部插件频道选项 | ❌ Go 端 `ChannelOptions` 为硬编码列表 | P2 |
| H4-5 | `OPENACOSMI_STATE_DIR` | 覆盖默认 state 目录 | ✅ `utils.go ResolveStateDir` 已读取 | — |
| H4-6 | `OPENACOSMI_PATH_BOOTSTRAPPED` | 防止重复 PATH 引导 | ❌ Go 端无 `ensureOpenAcosmiCliOnPath` 等价逻辑 | P3 |
| H4-7 | `NODE_NO_WARNINGS` | 非 verbose 模式抑制警告 | ✅ Go 无需此变量 | — |

### H5: 文件系统约定

| ID | TS 行为 | Go 状态 | 优先级 |
|----|---------|---------|--------|
| H5-1 | `path-env.ts ensureOpenAcosmiCliOnPath()`: 将 openacosmi 二进制路径注入 `$PATH`（支持 Homebrew/mise/pnpm/bun/yarn/XDG）| ❌ Go 二进制为独立编译产物，理论上不需要，但如果在 launchd 环境下可能需要 | P3 |
| H5-2 | `dotenv.ts loadDotEnv()`: 加载项目目录的 `.env` 文件 | ❌ Go 端启动时未加载 `.env` | P2 |

### H6: 协议/消息格式约定

| ID | TS 行为 | Go 状态 | 优先级 |
|----|---------|---------|--------|
| H6-1 | `route.ts` 快速路由: 5 个命令（health/status/sessions/agents-list/memory-status）绕过 Commander 直接执行 | ❌ Go 端无等价机制；Cobra 注册速度足够快，可能不需要 | P3 |

### H7: 错误处理约定

| ID | TS 行为 | Go 状态 | 优先级 |
|----|---------|---------|--------|
| H7-1 | `config-guard.ts`: 配置无效时 allowlist 命令可继续运行（doctor/logs/health/status + gateway 子命令），其余命令 `exit(1)` | ❌ Go 端无 config 校验逻辑 | P1 |
| H7-2 | `run-main.ts rewriteUpdateFlagArgv()`: `--update` flag 自动重写为 `update` 子命令 | ❌ Go 端无此兼容逻辑 | P2 |

---

## 三、优先级汇总

| 优先级 | 数量 | 关键项 |
|--------|------|--------|
| **P1** | 4 | H2-1 globalYes, H2-3 config guard, H3-1 preAction 缺步骤, H7-1 config allowlist |
| **P2** | 6 | H1-2 hook 链, H2-2 plugin registry, H2-4 runtime, H4-2/4 env vars, H5-2 dotenv, H7-2 --update rewrite |
| **P3** | 4 | H3-2 uncaught, H4-6 PATH, H5-1 PATH bootstrap, H6-1 fast route |

---

## 四、建议处理方案

### P1 项（本 Phase 可修）

1. **H2-1 + H3-1**: 在 `internal/cli/globals.go` 新增 `SetVerbose()`/`IsVerbose()`/`SetYes()`/`IsYes()` 全局状态，更新 `PersistentPreRunE` 使用
2. **H2-3 + H7-1**: 在 `internal/cli/config_guard.go` 新增 `EnsureConfigReady()` stub（标记 TODO，待 config 模块提供 snapshot 接口后完善）
3. **H3-1**: 更新 `PersistentPreRunE` 执行链为：profile → banner → verbose/globals → config guard → plugin registry（按需）

### P2 项（可延迟至命令业务逻辑阶段）

4. **H5-2**: `.env` 加载可在 `main()` 入口增加 `godotenv.Load()`
5. **H7-2**: `--update` 重写可在 `main()` 中 `os.Args` 预处理
6. 其余 P2 项在各命令 stub 填充业务逻辑时一并处理

### P3 项（延迟到 Phase 7+）

7. 快速路由和 PATH bootstrap 在 Go 端非必需，记入 deferred-items
