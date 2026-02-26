# Phase 6 深度审计报告 — CLI + 插件 + 钩子 + 守护进程

> 最后更新：2026-02-14
> 审计方法：逐目录文件清单 + import 链分析 + 隐藏依赖交叉验证
> 基于 `phase4-9-deep-audit.md` Phase 6 项的深度展开

---

## 一、规模总览

### 1.1 Phase 6 TS 代码量真实统计

| 模块 | TS 非测试文件数 | 代码量 | 全局审计预估 | 差异 |
|------|----------------|--------|-------------|------|
| `src/plugins/` | 29 | **176KB** | 160KB | +10% |
| `src/hooks/` | 22 | **113KB** | "28文件" | 首次量化 |
| `src/cron/` | 22 | **116KB** | "33文件" | 首次量化 |
| `src/daemon/` | 19 | **105KB** | "30文件" | 首次量化 |
| `src/acp/` | 10 | **36KB** | "13文件" | 首次量化 |
| `src/cli/` | 80 | **715KB** | "27文件" | ⚠️ **26x** |
| `src/commands/` | 80+ | **900KB** | "核心命令" | ⚠️ **严重低估** |
| `src/plugin-sdk/` | 1 | **13KB** | 13KB | ✅ 准确 |
| `src/macos/` | 3 | **10KB** | 7KB | +43% |
| `src/terminal/` | 9 | **21KB** | "12文件" | 首次量化 |
| `src/wizard/` | 7 | **55KB** | "10文件" | 首次量化 |
| **Phase 6 合计** | **282** | **~2.26MB** | — | — |

> [!CAUTION]
> Phase 6 TS 代码量为 **2.26MB**，占全项目 9.2MB 的 **24.5%**。
> 全局审计中 `cli/` 仅标注 "27 文件"，实际为 **80 文件 715KB**；`commands/` 实际为 **80+ 文件 900KB**。

### 1.2 Go 端现状（2026-02-14 更新）

| 目录 | 现有文件 | 状态 |
|------|---------|------|
| `internal/daemon/` | 21 Go 文件 + 21 测试 | ✅ A1 完成 |
| `internal/plugins/` | 10 source + 3 test (30 PASS) | ✅ A2 完成 |
| `internal/hooks/` | 10 source + 4 test (35 PASS) | ✅ A3 完成（Phase 3 基础 + 内部事件钩子系统） |
| `internal/cron/` | 空 | 待创建 |
| `cmd/acosmi/` | 入口骨架 | 待创建完整 CLI |
| `internal/acp/` | 不存在 | 待创建 |

---

## 二、各子模块文件级清单

### 2.1 plugins/ — 29 文件，176KB

| 文件 | 大小 | 职责 |
|------|------|------|
| `runtime/types.ts` | **19KB** | 插件运行时完整类型系统 |
| `install.ts` | **17KB** | 插件安装（npm/本地/git） |
| `types.ts` | **15KB** | 核心类型定义 |
| `registry.ts` | **14KB** | 插件注册表管理 |
| `loader.ts` | **14KB** | 插件加载器（jiti 动态加载） |
| `hooks.ts` | **14KB** | 插件 ⇔ 钩子桥接 |
| `runtime/index.ts` | **13KB** | 运行时核心（频道适配器绑定） |
| `update.ts` | **12KB** | 插件更新检查 |
| `discovery.ts` | **10KB** | 插件发现（本地+远程） |
| `commands.ts` | **8KB** | 插件 CLI 命令注册 |
| `manifest-registry.ts` | **6KB** | Manifest 注册表 |
| `config-state.ts` | **6KB** | 插件配置状态管理 |
| `manifest.ts` | **5KB** | Manifest 解析 |
| `tools.ts` | **4KB** | 插件工具注册 |
| `slots.ts` | **3KB** | 插件插槽系统 |
| 其余 14 文件 | **~25KB** | cli, services, schema, enable, status 等 |

### 2.2 hooks/ — 22 文件，113KB

| 文件 | 大小 | 职责 |
|------|------|------|
| `install.ts` | **15KB** | 钩子安装/卸载流程 |
| `gmail-setup-utils.ts` | **11KB** | Gmail OAuth 设置 + Tailscale 端点 |
| `gmail-ops.ts` | **11KB** | Gmail API 操作（读/发/标签） |
| `gmail.ts` | **8KB** | Gmail 集成主入口 |
| `workspace.ts` | **8KB** | 工作区钩子加载 |
| `soul-evil.ts` | **8KB** | Soul-evil 安全检测钩子 |
| `gmail-watcher.ts` | **7KB** | Gmail 推送监听 |
| `hooks-status.ts` | **6KB** | 钩子健康状态 |
| `internal-hooks.ts` | **5KB** | 内部钩子注册表 |
| `loader.ts` | **5KB** | 钩子文件加载器 |
| `frontmatter.ts` | **5KB** | Frontmatter 解析 |
| `config.ts` | **4KB** | 钩子配置解析 |
| `plugin-hooks.ts` | **3KB** | 插件 → 钩子桥接 |
| `bundled/` 4 handler | **11KB** | 内置钩子: session-memory, command-logger, boot-md, soul-evil |
| 其余 | **6KB** | types, installs, bundled-dir, hooks.ts |

### 2.3 cron/ — 22 文件，116KB

| 文件 | 大小 | 职责 |
|------|------|------|
| `isolated-agent/run.ts` | **22KB** | 独立 Agent 运行器（最核心） |
| `service/timer.ts` | **16KB** | 定时器管理 + cron 表达式调度 |
| `service/store.ts` | **15KB** | 持久化存储 |
| `normalize.ts` | **13KB** | Cron 表达式标准化 |
| `service/jobs.ts` | **12KB** | Job 生命周期管理 |
| `service/ops.ts` | **6KB** | CRUD 操作 |
| `run-log.ts` | **4KB** | 运行日志 |
| `delivery.ts` | **2KB** | 投递计划 |
| 其余 14 文件 | **26KB** | types, schedule, parse, state, helpers 等 |

### 2.4 daemon/ — 19 文件，105KB

| 文件 | 大小 | 职责 |
|------|------|------|
| `launchd.ts` | **14KB** | macOS launchd 服务管理 |
| `systemd.ts` | **14KB** | Linux systemd 管理 |
| `schtasks.ts` | **12KB** | Windows 计划任务 |
| `inspect.ts` | **12KB** | 服务状态检查 |
| `service-audit.ts` | **11KB** | 服务配置审计 |
| `program-args.ts` | **8KB** | 程序参数解析 |
| `service-env.ts` | **6KB** | 服务环境变量 |
| `runtime-paths.ts` | **5KB** | 运行时路径解析 |
| `service.ts` | **4KB** | 服务主入口 |
| `launchd-plist.ts` | **4KB** | plist 生成 |
| `systemd-unit.ts` | **3KB** | systemd unit 生成 |
| 其余 8 文件 | **12KB** | constants, paths, hints, linger 等 |

### 2.5 acp/ — 10 文件，36KB

| 文件 | 大小 | 职责 |
|------|------|------|
| `translator.ts` | **14KB** | ACP ⇔ Gateway 协议翻译 |
| `client.ts` | **5KB** | ACP 客户端 |
| `server.ts` | **4KB** | ACP 服务端 |
| `session-mapper.ts` | **3KB** | 会话映射 |
| `session.ts` | **3KB** | ACP 会话管理 |
| `event-mapper.ts` | **3KB** | 事件映射 |
| `commands.ts` | **2KB** | ACP 命令 |
| `meta.ts` | **1KB** | 元信息解析 |
| `types.ts` | **0.7KB** | 类型定义 |
| `index.ts` | **0.2KB** | 入口 |

---

## 三、隐藏依赖链分析（全局审计未覆盖）

### 3.1 plugins/ → 全系统扇出依赖⭐最关键

`plugins/runtime/index.ts` 是**最复杂的单文件**（13KB），它导入了：

```
plugins/runtime/index.ts
  → channels/ (discord/slack/telegram/imessage/line/signal/whatsapp)
    ├── monitorXxxProvider (7 个频道监控)
    ├── probeXxx (7 个频道探测)
    └── 频道专属操作 (discordMessageActions 等)
  → agents/tools/ (memory-tool, slack-actions, whatsapp-actions)
  → auto-reply/ (reply-dispatcher, dispatch-from-config, inbound-context)
  → media/ (fetch, mime, image-ops, audio, constants)
  → infra/ (system-events, channel-activity)
  → pairing/ (pairing-messages)
  → markdown/ (tables)
  → logging/ (subsystem, levels)
  → config/ (config, types)
```

> [!CAUTION]
> `plugins/runtime/index.ts` 是**整个系统的集成枢纽**。它直接引用了 7 个频道的 monitor 和 probe、agent 工具、auto-reply 管线、媒体处理。Go 端必须通过 `contracts` 接口解耦，否则 Phase 6 将依赖几乎所有已完成 Phase。

### 3.2 hooks/ → plugins/ 双向依赖

```
hooks/plugin-hooks.ts → plugins/hooks.ts
plugins/hooks.ts → hooks/loader.ts + hooks/internal-hooks.ts
hooks/internal-hooks.ts → agents/ (triggerAgent)
```

**影响**：全局审计已标注此循环。Go 端需在 `pkg/contracts/` 中定义 `HookRunner` 和 `PluginHookBridge` 接口。

### 3.3 cron/ → agents/ 深度耦合链

```
cron/isolated-agent/run.ts (22KB)
  → agents/pi-embedded.ts (runEmbeddedPiAgent)
  → agents/model-selection.ts (resolveConfiguredModelRef)
  → agents/model-fallback.ts (runWithModelFallback)
  → agents/subagent-announce.ts (runSubagentAnnounceFlow)
  → agents/agent-scope.ts (resolveAgentDir 等)
  → agents/workspace.ts (ensureAgentWorkspace)
  → agents/skills.ts + skills/refresh.ts
  → agents/defaults.ts (DEFAULT_MODEL/PROVIDER/CONTEXT_TOKENS)
  → agents/usage.ts (deriveSessionTotalTokens)
  → agents/timeout.ts (resolveAgentTimeoutMs)
  → agents/cli-session.ts + cli-runner.ts
  → infra/outbound/deliver.ts (deliverOutboundPayloads)
  → infra/agent-events.ts (registerAgentRunContext)
  → infra/skills-remote.ts (getRemoteSkillEligibility)
  → routing/session-key.ts (buildAgentMainSessionKey)
  → config/sessions.ts
  → security/external-content.ts
  → auto-reply/thinking.ts
```

> [!WARNING]
> `cron/isolated-agent/run.ts` 有 **20+ 对外依赖**，其中大部分指向 Phase 4 agents 模块。
> 这意味着 cron 系统的核心功能——运行独立 Agent——不能在 Phase 4 完成前实现。

### 3.4 ACP → gateway 协议层依赖

```
acp/translator.ts (14KB)
  → gateway/client.ts (GatewayClient)
  → gateway/protocol/index.ts (EventFrame)
  → gateway/session-utils.ts (SessionsListResult)
  → gateway/call.ts (buildGatewayConnectionDetails)
  → gateway/auth.ts (resolveGatewayAuth)
  → @agentclientprotocol/sdk (外部 npm 包)
```

**影响**：ACP 需要 Gateway 客户端已完成。Go 端需实现 `@agentclientprotocol/sdk` 的等价协议。

### 3.5 daemon/ 自包含（低风险）

```
daemon/* → 仅依赖:
  ├── version.ts (VERSION 常量)
  ├── terminal/theme.ts (CLI 着色)
  └── daemon/ 内部互引
```

**评估**：daemon 模块**几乎没有对外依赖**，是 Phase 6 中最独立的子模块。

### 3.6 cli/ + commands/ → 全系统依赖⭐第二关键

CLI 命令直接调用所有其他系统：

- `commands/` 中的 `doctor*.ts` → config, gateway, daemon, channels, security
- `commands/agent*.ts` → agents/runner, auto-reply
- `commands/onboard*.ts` → wizard, config, channels

> [!IMPORTANT]
> `cli/` 和 `commands/` 合计 **1.6MB**，是 Phase 6 中最庞大的部分。但它们属于**顶层消费者**，理论上是最后重构。

---

## 四、七类隐式行为审计

| # | 类别 | plugins | hooks | cron | daemon | acp |
|---|------|---------|-------|------|--------|-----|
| 1 | **npm 包黑盒** | ⚠️ `jiti`动态加载 | ✅ | ⚠️ `cron`表达式 | ⚠️ `plist`解析 | ⚠️ `@agentclientprotocol/sdk` |
| 2 | **全局状态/单例** | ⚠️ PluginRegistry 单例 | ⚠️ InternalHookHandlers Map | ⚠️ CronServiceState | ✅ | ⚠️ AcpSessionStore |
| 3 | **事件总线/回调链** | ⚠️ onPluginEvent | ⚠️ hook→agent触发链 | ⚠️ timer→job→agent→deliver | ✅ | ⚠️ ndJsonStream |
| 4 | **环境变量** | ✅ | ⚠️ GMAIL_*凭证 | ✅ | ⚠️ SERVICE_* 环境 | ✅ |
| 5 | **文件系统** | ⚠️ 插件安装目录 | ⚠️ hooks/目录扫描 | ⚠️ cron.json持久化 | ⚠️ plist/unit文件 | ✅ |
| 6 | **协议/格式** | ⚠️ manifest.json 格式 | ⚠️ frontmatter 格式 | ✅ | ⚠️ launchd/systemd格式 | ⚠️ ACP JSON-lines 协议 |
| 7 | **错误处理** | ⚠️ 插件加载降级 | ⚠️ hook失败不阻塞 | ⚠️ job失败重试 | ⚠️ 服务安装回退 | ✅ |

### 关键隐式行为详解

#### plugins/loader.ts — `jiti` 动态加载

TS 使用 `createJiti()` 实现 ESM/CJS 混合动态加载。Go 无等价机制。

**Go 方案**：插件系统改为 `plugin.Open()` (Go plugin) 或 gRPC 子进程模式。

#### hooks/gmail*.ts — Google API OAuth

Gmail 集成需要完整的 OAuth2 流程 + Gmail API 调用（googleapis npm 包）。

**Go 方案**：`google.golang.org/api/gmail/v1` + `golang.org/x/oauth2`，API 完全对等。

#### cron/service/timer.ts — Cron 表达式解析

TS 使用自定义 cron 解析器（normalize.ts 13KB）。

**Go 方案**：`robfig/cron/v3` 但需验证表达式语法差异（6位 vs 5位）。

#### daemon/ — 跨平台服务管理

macOS(`launchd`)、Linux(`systemd`)、Windows(`schtasks`) 三套独立实现。

**Go 方案**：使用 `//go:build` 标签隔离平台代码。

---

## 五、修订后的 Phase 6 执行计划

> [!IMPORTANT]
> 基于深度审计结果，将原 8 个子任务重新分解为 **12 个子任务**，按依赖拓扑排序。

### 5.1 推荐执行顺序（拓扑排序）

| 批次 | 子任务 | TS 代码量 | Go 目标包 | 前置依赖 | 风险 |
|------|--------|----------|----------|---------|------|
| **A1** | 6.7 daemon/ 核心 | 105KB | `internal/daemon/` | 无（自包含） | 🟢 低 |
| **A2** | 6.4a plugins/ 类型+注册表 | ~45KB | `internal/plugins/` | pkg/contracts | 🟡 中 |
| **A3** | 6.5a hooks/ 核心+加载 | ~35KB | `internal/hooks/` | 6.4a(接口) | 🟡 中 |
| **B1** | 6.4b plugins/ 发现+安装+更新 | ~40KB | `internal/plugins/` | 6.4a | 🟡 中 |
| **B2** | 6.5b hooks/ Gmail集成 | ~38KB | `internal/hooks/gmail/` | 6.5a | 🟡 中 |
| **B3** | 6.4c plugins/ 运行时+桥接 | ~55KB | `internal/plugins/runtime/` | 6.4b+6.5a | 🔴 高 |
| **C1** | 6.6 cron/ 服务层 | ~70KB | `internal/cron/` | 6.4+6.5 | 🟡 中 |
| **C2** | 6.6b cron/ 独立Agent运行 | ~46KB | `internal/cron/agent/` | C1+Phase4 | 🔴 高 |
| **D1** | 6.8 ACP 协议 | 36KB | `internal/acp/` | gateway | 🟡 中 |
| **E1** | 6.1 CLI 框架(Cobra) | ~50KB | `cmd/openacosmi/` | 无 | 🟢 低 |
| **E2** | 6.2 核心命令 | ~200KB | `cmd/openacosmi/commands/` | E1+多系统 | 🔴 高 |
| **E3** | 6.3 辅助命令+完整CLI | ~450KB | `cmd/openacosmi/commands/` | E2+全系统 | 🔴 高 |

### 5.2 与原计划对比

| 原子任务 | 原预估 | 修订后 | 变化 |
|---------|--------|--------|------|
| 6.1-6.3 CLI | 3 个子任务 | **5 个子任务** (E1-E3 + 2 batch) | ⬆️ 代码量 1.6MB |
| 6.4 Plugins | 1 个子任务 | **3 个子任务** (A2,B1,B3) | ⬆️ 运行时集成复杂度 |
| 6.5 Hooks | 1 个子任务 | **2 个子任务** (A3,B2) | ⬆️ Gmail 独立拆出 |
| 6.6 Cron | 1 个子任务 | **2 个子任务** (C1,C2) | ⬆️ Agent 耦合 |
| 6.7 Daemon | 1 个子任务 | **1 个子任务** (A1) | = 不变 |
| 6.8 ACP | 1 个子任务 | **1 个子任务** (D1) | = 不变 |

---

## 六、风险评级与建议

### 6.1 🔴 高风险项（3 个）

1. **B3: plugins/runtime 集成**
   - 原因：引用 7 个频道适配器 + agents + auto-reply
   - 建议：全部用 `pkg/contracts/` 接口替代直接依赖
   - 如 Phase 5 频道未完成 → 用 stub 占位

2. **C2: cron/isolated-agent 运行**
   - 原因：20+ 对 Phase 4 agents 的硬依赖
   - 建议：Phase 4 未完成前仅实现 service 层（C1），Agent 运行延后

3. **E2-E3: CLI commands 体量**
   - 原因：1.6MB 代码，调用全系统 API
   - 建议：分 batch 实现，doctor/status 命令最后做

### 6.2 🟢 低风险优先项

1. **A1: daemon/** — 自包含，无外部依赖，可**立即开工**
2. **E1: CLI Cobra 框架** — 纯框架搭建，无业务逻辑
3. **A2: plugins/types** — 仅类型定义 + 注册表接口

### 6.3 设计决策（已确认）

#### 决策 1：Go 插件加载机制 → **Go 接口 + 编译时注册**

**TS 原实现分析**：

原 TS 使用 `jiti`（ESM/CJS 混合动态加载器）在**同一进程**内加载 `.ts/.js` 文件。
插件通过导出 `register(api)` 函数与宿主交互，api 对象提供 10+ 注册方法
（registerTool, registerHook, registerChannel, registerProvider 等）。
关键特征：**同步调用、同进程、共享内存**。

**Go 方案对比**（基于 2025 业界研究）：

| 方案 | 优势 | 劣势 | 适用场景 |
|------|------|------|---------|
| `plugin.Open()` | 性能高 | ❌ Windows 不支持、ABI 脆弱、插件无法卸载 | 不推荐 |
| HashiCorp `go-plugin` | 进程隔离、跨语言、Terraform 验证 | 🟡 RPC 延迟、部署复杂 | 第三方插件 |
| WASM | 沙箱安全、跨平台 | 🟡 生态不成熟、单线程 | 远程/沙箱场景 |
| **Go 接口注册模式** | ✅ 零额外依赖、与 TS 行为等价 | 需编译时链接 | **内部插件** |

**最终决策**：采用 **Go 接口 + 编译时注册模式**（类似 `database/sql` 驱动注册）。

原因：TS 插件系统本质是**同进程接口调用**，Go 的 `interface{}` + `init()` 注册模式
可完全等价实现。后续如需第三方/安全插件可叠加 HashiCorp `go-plugin`。

```go
// pkg/contracts/plugin.go — 插件 API 接口
type PluginAPI interface {
    RegisterTool(tool AgentTool)
    RegisterHook(events []string, handler HookHandler)
    RegisterChannel(ch ChannelPlugin)
    RegisterProvider(p ProviderPlugin)
    RegisterService(svc PluginService)
    RegisterCommand(cmd PluginCommand)
}

// internal/plugins/registry.go — 编译时注册
var pluginRegistry = make(map[string]PluginFactory)

func Register(id string, factory PluginFactory) {
    pluginRegistry[id] = factory
}
```

#### 决策 2：Cron Agent 运行 → **Phase 6 实现 Service 层 + Phase 4 完成后集成 Agent 运行**

**TS 原实现分析**：

`cron/isolated-agent/run.ts`（597 行，22KB）是**完整的 Agent 运行器**：

- 导入 20+ agents/ 模块（model-selection, pi-embedded, model-fallback 等）
- 解析模型级联、thinking level、超时策略
- 调用 `runEmbeddedPiAgent()` 或 `runCliAgent()` 执行完整 Agent 对话
- 处理消息投递（outbound/subagent-announce）
- 包含 Gmail hook 安全检测（external-content.ts）

这不是简单的 cron 调度 — 它是**以 cron 触发的完整 Agent 会话**。

**最终决策**：

- **Phase 6 实现**：cron service 层（service/, timer, store, jobs, normalize）
  - 类型定义、Cron 表达式解析、持久化、CRUD、定时器触发
  - 可独立编译、测试
- **Phase 4 完成后集成**：`cron/isolated-agent/run.ts`
  - 此文件直接依赖 `agents/` 所有核心模块
  - Go 端在 `internal/cron/agent/` 中实现，但需 Phase 4 的 agent runner

#### 决策 3：CLI 命令 → **全量 80+ 命令**

用户确认：先实现全量命令，后续升级安全模式。

#### 决策 4：跨平台 daemon → **macOS + Linux 优先，Windows 条件纳入**

用户确认：优先 macOS(`launchd`) + Linux(`systemd`)。
Windows(`schtasks`) 仅增加 ~12KB 代码（`schtasks.ts` 12KB），
工作量有限，可视最终时间余量决定是否纳入。

Go 实现使用 `//go:build` 标签隔离：

- `daemon_darwin.go` — launchd plist 管理
- `daemon_linux.go` — systemd unit 管理
- `daemon_windows.go` — schtasks 管理（可选）
