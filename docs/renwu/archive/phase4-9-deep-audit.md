# Phase 4-9 深度审计报告 — 接口依赖清单与隐藏依赖目录（合并三轮审计）

---

## 一、审计概览

### 1.1 审计范围

对原版 TS 后端 `src/` 目录下与 Phase 4-9 相关的所有模块进行逐目录、逐文件的依赖审计。

### 1.2 关键发现总结（含四轮审计修订）

| 发现类别 | 第一轮 | 第二轮 | 第三轮 | **第四轮** | 影响 |
|----------|--------|--------|--------|-----------|------|
| 遗漏的模块/目录 | 15 个 | 19 个 | 28 个 | **28+32 目录** | 🔴 高 |
| 隐藏依赖链 | 12 条 | 15 条 | 20 条 | **20+ 条** | 🔴 高 |
| 文件大小低估 | 6 个 | 10 个 | 18 个 | **18 个** | 🟡 中 |
| 未覆盖代码量 | — | — | — | **~3.5MB (38%)** | � 极高 |
| 总项目规模 | — | — | — | **9.2MB** | — |

---

## 二、原计划遗漏的模块清单

> [!CAUTION]
> 以下模块在 `refactor-plan-full.md` 的 Phase 4-9 中**完全没有提及**，但它们被多个核心模块依赖，缺失将导致功能不完整。
> 总计 **28 个**（第一轮 15 + 第二轮 +4 + 第三轮 +9）。

| # | 模块路径 | 文件数 | 核心职责 | 被谁依赖 | 建议归入 Phase |
|---|----------|--------|----------|----------|---------------|
| 1 | `src/process/` | 9 | 命令队列、进程桥接、spawn工具 | agents/PI Runner, bash-tools | **4** |
| 2 | `src/routing/` | 5 | 会话路由解析、session-key 生成 | gateway/chat, auto-reply | **4** |
| 3 | `src/sessions/` | 7 | 模型覆盖、级别覆盖、发送策略 | agents/runner, gateway | **4** |
| 4 | `src/providers/` | 8 | GitHub Copilot 认证/token、Google 共享逻辑 | agents/model-auth, PI Runner | **4** |
| 5 | `src/plugins/` | 35 | 完整插件系统(发现/加载/注册/钩子/运行时) | gateway, hooks, channels | **6** |
| 6 | `src/hooks/` | 28 | 钩子加载/执行、Gmail 集成、soul-evil 检测 | gateway/hooks, cron | **6** |
| 7 | `src/pairing/` | 5 | 设备配对存储、配对消息 | channels, gateway | **5** |
| 8 | `src/media/` | 19 | 媒体处理工具集 | agents/tools, channels | **7** |
| 9 | `src/link-understanding/` | 7 | URL 内容提取与分析 | agents/tools/web-fetch | **7** |
| 10 | `src/terminal/` | 12 | 终端 UI 组件 | CLI 交互 | **6** |
| 11 | `src/tui/` | 39 | TUI 界面框架 | CLI 命令 | **6** |
| 12 | `src/wizard/` | 10 | 配置向导 | commands/configure | **6** |
| 13 | `src/markdown/` | 8 | Markdown 解析/渲染 | agents, auto-reply | **7** |
| 14 | `src/shared/` | 2 | 共享常量/类型 | 多模块 | **4** |
| 15 | `src/acp/` | 13 | ACP 协议实现 | gateway | **6** |
| 20 | `src/infra/outbound/` | 31 | 出站消息管线：threading、重试、分片、投递 | agents/PI Runner, channels | **4** |
| 21 | `src/infra/exec-approvals` | 3 | 执行审批：策略匹配、滑动窗口、持久化 | bash-tools, sandbox | **4** |
| 22 | `src/infra/heartbeat-*` | 5 | 心跳系统：定时检测、可见性控制 | gateway, cron | **4** |
| 23 | `src/infra/session-cost-usage` | 1 | 会话成本/用量追踪 | gateway/usage, PI Runner | **4** |
| 24 | `src/infra/state-migrations` | 1 | 跨版本状态文件迁移 | 全局 | **4** |
| 25 | `src/infra/provider-usage.*` | 10+ | Provider 用量获取(7 个 provider 适配器) | gateway/usage | **4** |
| 26 | `src/infra/bonjour+tailscale+dns` | 5 | 网络发现：mDNS/Bonjour + Tailscale VPN | 多节点 | **6** |
| 27 | `src/config/zod-schema.*` | 12 | Zod Schema 验证子系统(106KB) | config/io | **4** |
| 28 | `src/gateway/server-methods/` | 41 | 网关 RPC 方法完整实现层(270KB+) | gateway | **4** |

---

## 三、文件大小严重低估清单

| 文件 | 实际大小 | 原计划预估 | 差距 |
|------|----------|-----------|------|
| `src/tts/tts.ts` | **47KB** (单文件) | 归入 Phase 7 "TTS" ⭐⭐⭐ | 复杂度应为 ⭐⭐⭐⭐⭐ |
| `src/memory/manager.ts` | **78KB** (单文件最大) | 归入 Phase 7 "记忆" ⭐⭐⭐⭐ | 需拆 10+ Go 文件 |
| `src/media-understanding/runner.ts` | **38KB** | 归入 Phase 7 "媒体" ⭐⭐⭐ | 复杂度应为 ⭐⭐⭐⭐⭐ |
| `src/security/audit.ts` + `audit-extra.ts` | **82KB** 合计 | 归入 Phase 7 "安全" ⭐⭐⭐ | 需拆 8+ Go 文件 |
| `src/agents/pi-embedded-runner/run.ts` | **34KB** | Phase 4 "PI核心" | 需拆 5+ Go 文件 |
| `src/plugins/loader.ts` + `registry.ts` | **29KB** 合计 | 未被计划 | 需新增 Phase |
| `src/config/schema.ts` | **55KB** | 第三轮发现 | 单文件最大之一 |
| `src/infra/exec-approvals.ts` | **41KB** | 第三轮发现 | 需拆分 |
| `src/infra/outbound/message-action-runner.ts` | **33KB** | 第三轮发现 | 需拆分 |
| `src/infra/heartbeat-runner.ts` | **33KB** | 第三轮发现 | 需拆分 |
| `src/infra/session-cost-usage.ts` | **33KB** | 第三轮发现 | 按 provider 拆分 |
| `src/config/zod-schema.providers-core.ts` | **30KB** | 第三轮发现 | 验证规则复杂 |
| `src/infra/outbound/outbound-session.ts` | **29KB** | 第三轮发现 | 投递会话管理 |
| `src/infra/state-migrations.ts` | **29KB** | 第三轮发现 | 多步迁移链 |

---

## 四、隐藏跨模块依赖链（12 条关键链路）

> [!IMPORTANT]
> 以下依赖链在原版 TS 中通过 `import` 隐式连接，但在原计划中未被图示。
> 缺失任何一环都将导致该链路上的功能无法编译或运行。

### 4.1 PI Runner → process/command-queue 链

```
agents/pi-embedded-runner/run.ts
  → process/command-queue.ts (enqueueCommandInLane)
  → process/lanes.ts (全局/会话级命令排队)
```

**影响**: PI Runner 的所有 LLM 调用都通过 command-queue 串行化，防止并发冲突。Go 端需要等价的 goroutine 排队机制。

### 4.2 PI Runner → providers/ 链

```
agents/pi-embedded-runner/run.ts
  → providers/github-copilot-token.ts (动态 import)
  → providers/github-copilot-auth.ts
```

**影响**: GitHub Copilot provider 需要 OAuth token 交换，这是一个**动态 import**（`await import(...)`），不会在静态分析中出现！

### 4.3 PI Runner → routing/ 链

```
agents/pi-embedded-runner/run.ts
  → agents/workspace-run.ts
  → routing/session-key.ts (session key 解析)
  → routing/resolve-route.ts (路由解析)
  → routing/bindings.ts (绑定)
```

**影响**: 会话路由(session routing) 决定消息发送到哪个 agent。

### 4.4 agents/tools/ → channels/ 双向依赖

```
agents/tools/discord-actions*.ts → discord/ (SDK 调用)
agents/tools/slack-actions.ts → slack/ (SDK 调用)
agents/tools/telegram-actions.ts → telegram/ (SDK 调用)
agents/tools/whatsapp-actions.ts → web/ (SDK 调用)
agents/tools/message-tool.ts → channels/registry.ts
```

**影响**: Agent 工具系统直接调用各频道 SDK，Phase 4(tools) 和 Phase 5(channels) 存在**循环依赖**。建议先用 interface 抽象解耦。

### 4.5 auto-reply/reply/ → agents/ 深度耦合

```
auto-reply/reply/ (139 子文件!)
  → agents/model-selection.ts (模型选择)
  → agents/auth-profiles/ (密钥轮换)
  → agents/pi-embedded-runner/ (启动 Agent)
  → agents/skills/ (技能解析)
```

**影响**: auto-reply 不是独立模块，它**深度依赖** Phase 4 的 agents/。Phase 7 必须在 Phase 4 之后。

### 4.6 hooks/ → plugins/ 双向依赖

```
hooks/plugin-hooks.ts → plugins/hooks.ts
plugins/hooks.ts → hooks/loader.ts
hooks/internal-hooks.ts → agents/ (triggerAgent)
hooks/gmail*.ts → 外部 Gmail API
```

**影响**: hooks 和 plugins 互相引用，必须在同一 Phase 中实现。

### 4.7 memory/ → 外部 embedding 提供者链

```
memory/manager.ts (78KB!)
  → memory/embeddings.ts → embeddings-openai.ts
  → memory/embeddings.ts → embeddings-gemini.ts
  → memory/embeddings.ts → embeddings-voyage.ts
  → memory/batch-*.ts (3 个批处理器)
  → memory/qmd-manager.ts (28KB, QMD 索引)
```

**影响**: 记忆系统对接 3 个外部 embedding API + SQLite 向量搜索。Go 端需要等价的 HTTP 客户端。

### 4.8 cron/service/ → agents/ 链

```
cron/service/ (7 文件)
  → cron/isolated-agent/ (独立 agent 运行)
  → agents/pi-embedded-runner/ (启动内嵌引擎)
  → infra/system_events.ts (系统事件队列)
```

**影响**: 定时任务直接启动 Agent 引擎，Phase 6(cron) 依赖 Phase 4(agents)。

### 4.9 browser/ → playwright-core 深度集成

```
browser/pw-session.ts (18KB) → playwright-core (CDP协议)
browser/pw-tools-core.*.ts (7 文件) → playwright Page/Frame API
browser/extension-relay.ts (23KB) → Chrome DevTools Protocol
browser/server-context.ts (22KB) → 标签页管理/上下文隔离
```

**影响**: 浏览器自动化深度绑定 Playwright。Go 替代方案 `chromedp`/`rod` 的 API 完全不同。

### 4.10 security/audit.ts → 全局扫描

```
security/audit.ts (37KB) + audit-extra.ts (45KB)
  → config/ (配置回溯)
  → agents/ (agent 配置审计)
  → channels/ (频道配置审计)
  → hooks/ (钩子安全检查)
  → plugins/ (插件安全扫描)
  → security/skill-scanner.ts (技能文件扫描)
```

**影响**: 安全审计模块扫描**几乎所有其他模块**的配置，必须最后实现。

### 4.11 daemon/ → 跨平台服务管理

```
daemon/launchd.ts (14KB) → macOS launchd plist 生成
daemon/systemd.ts (14KB) → Linux systemd unit 生成
daemon/schtasks.ts (12KB) → Windows 计划任务
daemon/service-env.ts → 环境变量注入
daemon/service-audit.ts (11KB) → 服务健康审计
```

**影响**: 守护进程管理是**纯平台相关**代码，Go 端需要 build tags 隔离。

### 4.12 agents/bash-tools.exec.ts → 安全沙箱链

```
bash-tools.exec.ts (54KB)
  → bash-tools.shared.ts (Docker 沙箱参数)
  → bash-tools.process.ts (21KB, PTY 进程管理)
  → bash-process-registry.ts (7KB, 进程注册表)
  → sandbox/docker.ts (11KB, Docker容器管理)
  → sandbox/config.ts (沙箱配置解析)
  → sandbox/workspace.ts (工作区挂载)
  → process/exec.ts (底层进程执行)
  → process/spawn-utils.ts (spawn 工具)
```

**影响**: Bash 工具执行是最复杂的单一功能链路(54KB+21KB+11KB = 86KB总量)，涉及 PTY、Docker、安全策略审批。

### 4.16 出站消息管线 → channels + agents 三方耦合（第三轮发现）

```
infra/outbound/message-action-runner.ts (33KB)
  → infra/outbound/outbound-session.ts (29KB, 投递会话)
  → infra/outbound/target-resolver.ts (14KB, 目标解析)
  → infra/outbound/deliver.ts (12KB, 投递执行)
  → channels/registry.ts (频道查找)
  → agents/pi-embedded-runner/ (Agent 回调)
```

**影响**: 出站管线是 Agent 输出到用户的**唯一桥梁**。它同时依赖 channels 和 agents，形成 agents→outbound→channels 的**单向链**（非循环）。Go 端需在 Phase 4 中实现此管线，置于 `internal/outbound/`。

### 4.17 执行审批 → gateway + node-host 双通道（第三轮发现）

```
infra/exec-approvals.ts (41KB)
  → infra/exec-approval-forwarder.ts (10KB)
    → gateway/server-methods/exec-approval.ts (4KB)
    → node-host/runner.ts (审批快照)
  → agents/bash-tools.exec.ts (审批检查)
  → agents/sandbox/*.ts (沙箱审批)
```

**影响**: 审批系统同时服务于 **本地执行** 和 **远程节点执行**。第二轮审计的 bash-tools 链路（4.12）遗漏了 `exec-approvals.ts` 本身。

### 4.18 心跳 → outbound + session 联动（第三轮发现）

```
infra/heartbeat-runner.ts (33KB)
  → infra/outbound/deliver.ts (投递心跳消息)
  → infra/heartbeat-visibility.ts (可见性判断)
  → infra/heartbeat-events.ts (事件总线)
  → config/sessions/ (会话状态查询)
```

**影响**: 心跳不只是 ping/pong，而是通过 outbound 管线**投递格式化消息**到指定频道。

### 4.19 会话成本 → provider-usage → 外部 API（第三轮发现）

```
infra/session-cost-usage.ts (33KB)
  → infra/provider-usage.fetch.*.ts (7 个 provider 适配器)
  → infra/provider-usage.auth.ts (API 认证)
  → infra/provider-usage.format.ts (格式化)
  → gateway/server-methods/usage.ts (26KB, 查询接口)
```

**影响**: 成本追踪系统依赖 7 个独立的 provider API 适配器。Go 端需为每个 provider 编写 HTTP 客户端。

### 4.20 配置验证 → zod-schema → 运行时类型守卫（第三轮发现）

```
config/io.ts (18KB, 配置 I/O)
  → config/schema.ts (55KB, 类型定义)
  → config/zod-schema.ts (19KB, 主 schema)
  → config/zod-schema.providers-core.ts (30KB)
  → config/zod-schema.agent-runtime.ts (17KB)
  → config/zod-schema.core.ts (16KB)
  → config/validation.ts (10KB, 验证)
  → config/defaults.ts (12KB, 默认值)
  → config/env-substitution.ts (3KB, 环境变量替换)
```

**影响**: 配置加载链条 200KB+。Go 端 `internal/config/` 已有 35 文件，但需验证是否覆盖了 Zod schema 的**所有验证规则**（字段约束、默认值、迁移）。

---

## 六、修订后的 Phase 4-9 任务计划

> [!IMPORTANT]
> 基于三轮审计结果，原 Phase 4-9 需大幅修订。子任务总数从 29 增加到 **56**。

### Phase 4（修订）: Agent 引擎 — 从 10→20 子任务

| 编号 | 子任务 | 原版文件(KB) | 新增依赖 | 复杂度 |
|------|--------|-------------|----------|--------|
| 4.0 | **进程队列系统** | `process/` 9文件 16KB | 无(新增) | ⭐⭐⭐ |
| 4.1 | 模型配置与发现 | `models-config*.ts` 24KB | +synthetic/venice/zen/bedrock 30KB | ⭐⭐⭐⭐ |
| 4.2 | 模型选择 & 失败切换 | `model-selection.ts` 13KB | +model-catalog/scan/compat 20KB | ⭐⭐⭐⭐ |
| 4.3 | 认证配置 | `auth-profiles/` 15文件 68KB | +chutes-oauth/cli-credentials 23KB | ⭐⭐⭐⭐⭐ |
| 4.4 | 系统提示词 | `system-prompt.ts` 27KB | +system-prompt-params/report 8KB | ⭐⭐⭐⭐ |
| 4.5 | PI 引擎核心(运行循环) | `run.ts` 34KB | +compact/model/google 43KB | ⭐⭐⭐⭐⭐ |
| 4.6 | PI 订阅 & 流式处理 | `subscribe*.ts` 6文件 50KB | +block-chunker 10KB | ⭐⭐⭐⭐⭐ |
| 4.7 | PI 工具注册 & 调度 | `pi-tools*.ts` 6文件 47KB | +policy/schema 16KB | ⭐⭐⭐⭐⭐ |
| 4.8 | PI 辅助函数 | `pi-embedded-helpers/` 9文件 | +utils 12KB | ⭐⭐⭐ |
| 4.9 | Bash 工具执行 | `bash-tools*.ts` 4文件 90KB | +pty 7KB | ⭐⭐⭐⭐⭐ |
| 4.10 | 沙箱系统 | `sandbox/` 17文件 59KB | — | ⭐⭐⭐⭐ |
| 4.11 | 技能系统 | `skills/` 13文件 + 外部3文件 | — | ⭐⭐⭐ |
| 4.12 | **会话路由** | `routing/` 5文件 32KB | 无(新增) | ⭐⭐⭐ |
| 4.13 | **会话覆盖** | `sessions/` 7文件 10KB | 无(新增) | ⭐⭐ |
| 4.14 | **Provider 适配层** | `providers/` 8文件 32KB | 无(新增) | ⭐⭐⭐ |
| 4.15 | **Agent 工具集(不含频道)** | `tools/` 中非频道工具 ~20文件 | 无(新增) | ⭐⭐⭐⭐ |
| 4.16 | **出站消息管线** | `infra/outbound/` 31文件 ~160KB | 无(第三轮新增) | ⭐⭐⭐⭐⭐ |
| 4.17 | **执行审批系统** | `infra/exec-approvals` + forwarder ~52KB | 无(第三轮新增) | ⭐⭐⭐⭐ |
| 4.18 | **心跳系统** | `infra/heartbeat-*` 5文件 ~40KB | 无(第三轮新增) | ⭐⭐⭐ |
| 4.19 | **Gateway 协议层验证** | `gateway/protocol/` 21文件 90KB | 无(第三轮新增) | ⭐⭐⭐ |

### Phase 5（修订）: 通信频道 — 从 4→6 子任务

| 编号 | 子任务 | 原版文件 | 新增依赖 | 复杂度 |
|------|--------|---------|----------|--------|
| 5.1 | 频道抽象层 | `channels/` 31文件 | +channels/plugins 71文件 | ⭐⭐⭐⭐ |
| 5.2 | **频道工具适配** | `tools/discord-actions.ts` 等 | Agent tool→channel桥接 | ⭐⭐⭐ |
| 5.3 | Discord 适配器 | `discord/` 67文件 | — | ⭐⭐⭐ |
| 5.4 | Slack 适配器 | `slack/` 65文件 | — | ⭐⭐⭐ |
| 5.5 | Telegram 适配器 | `telegram/` 89文件 | — | ⭐⭐⭐ |
| 5.6 | 其余频道 | signal/line/imessage/web/pairing | +pairing/ 5文件 | ⭐⭐⭐ |

### Phase 6（修订）: CLI + 插件 + 钩子 — 从 4→8 子任务

| 编号 | 子任务 | 原版文件 | 新增依赖 | 复杂度 |
|------|--------|---------|----------|--------|
| 6.1 | CLI 框架搭建 | `cli/program/` 27文件 | +terminal/tui | ⭐⭐⭐ |
| 6.2 | 核心命令 | `commands/` 核心命令 | — | ⭐⭐⭐⭐ |
| 6.3 | 辅助命令 | `commands/` 辅助命令 | — | ⭐⭐⭐ |
| 6.4 | **插件系统** | `plugins/` 35文件 160KB | 无(新增)| ⭐⭐⭐⭐⭐ |
| 6.5 | **钩子系统(完整)** | `hooks/` 28文件 | +Gmail 集成 | ⭐⭐⭐⭐ |
| 6.6 | 定时任务 | `cron/` 33文件 | — | ⭐⭐⭐⭐ |
| 6.7 | 守护进程 | `daemon/` 30文件 | build tags | ⭐⭐⭐⭐ |
| 6.8 | **ACP 协议** | `acp/` 13文件 | 无(新增) | ⭐⭐⭐ |

### Phase 7（修订）: 辅助模块 — 从 5→7 子任务

| 编号 | 子任务 | 原版文件(KB) | 新发现 | 复杂度 |
|------|--------|-------------|--------|--------|
| 7.1 | 自动回复引擎 | `auto-reply/` 71文件 + `reply/` 139文件 | 210文件！非121 | ⭐⭐⭐⭐⭐ |
| 7.2 | 记忆系统 | `memory/` 43文件 | manager.ts=78KB | ⭐⭐⭐⭐⭐ |
| 7.3 | 安全模块 | `security/` 13文件 | audit=82KB | ⭐⭐⭐⭐ |
| 7.4 | 浏览器自动化 | `browser/` 68文件 | 比预估多16文件 | ⭐⭐⭐⭐⭐ |
| 7.5 | 媒体理解 | `media-understanding/` 21文件 | runner=38KB | ⭐⭐⭐⭐⭐ |
| 7.6 | **TTS** | `tts/tts.ts` 47KB单文件 | 远超预估 | ⭐⭐⭐⭐ |
| 7.7 | **链接理解 + Markdown** | `link-understanding/`+`markdown/` 15文件 | 新增 | ⭐⭐⭐ |

### Phase 8-9 无重大变更

Phase 8 (Ollama 集成 + 补充模块) 和 Phase 9 (集成验证) 保持原计划不变。Phase 10+ 新增 Rust 性能下沉。

---

## 七、执行顺序约束总结

```
Phase 4.0 (process/) → Phase 4.1-4.3 → Phase 4.4-4.8 → Phase 4.9-4.15
                                                              ↓
Phase 5.1 (channels抽象) → Phase 5.2 (工具桥接) → Phase 5.3-5.6
                                                              ↓
Phase 6.4-6.5 (plugins+hooks 同步) → Phase 6.1-6.3 → Phase 6.6-6.8
                                                              ↓
Phase 7.1 (auto-reply, 依赖Phase 4) → Phase 7.2-7.7
                                                              ↓
Phase 8 → Phase 9
```

---

## 八、风险评级修订

| 风险 | 原评级 | 修订评级 | 原因 |
|------|--------|---------|------|
| agents/ 模块复杂度 | 🔴 高 | 🔴🔴 极高 | 发现 27+ 隐藏导入，子任务从10→20 |
| auto-reply/ 独立性 | 🟡 中 | 🔴 高 | 139 子文件深度依赖 agents/ |
| plugins/ 未计划 | — | 🔴 高 | 35 文件完整插件系统遗漏 |
| memory/ 单文件 | 🟡 中 | 🔴 高 | 78KB 单文件需拆 10+ Go 文件 |
| agents↔channels 循环 | — | ✅ 已解决 | 采用契约包模式(见第九节) |
| tts.ts 47KB | — | 🟡 中 | 单文件比整个 Phase 7.5 预估都大 |
| **Phase 4 代码量** | 🔴 极高 | 🔴🔴🔴 | 实际 ~1,260KB vs 预估 350KB (3.6 倍) |
| **出站管线遗漏** | — | 🔴 高 | Agent 回复无法投递 |
| **执行审批遗漏** | — | 🔴 高 | Bash 工具无安全审批 |
| **协议兼容性** | — | 🟡 中 | 90KB 协议定义需对齐 |
| **配置验证遗漏** | — | 🟡 中 | 160KB 验证规则 |

> [!CAUTION]
> Phase 4 的实际代码量是第二轮预估的 **3.6 倍**。建议将 Phase 4 拆分为 Phase 4A（核心引擎）和 Phase 4B（基础设施）两个子阶段。

### Phase 4 修订后总代码量预估

| 类别 | 第二轮预估 | 第三轮修正 |
|------|-----------|-----------|| PI Runner 核心 | ~200KB | ~200KB（确认准确） |
| 工具系统 | ~90KB | **~500KB**（agents/tools/ 60 文件） |
| 基础设施 | ~60KB | **~400KB**（outbound+审批+心跳+成本+迁移） |
| 配置验证 | 未计入 | **~160KB**（schema+zod+legacy） |
| **合计** | **~350KB** | **~1,260KB** |

---

## 九、契约包模式 — 解决 agents↔channels 循环依赖

> [!NOTE]
> 基于 Go 社区最佳实践和联网调研结果，采用用户建议的"契约包"模式。

### 9.1 问题分析

```
internal/agents/tools/discord_actions.go → internal/channels/discord/ (发送消息)
internal/channels/discord/handler.go     → internal/agents/ (触发 Agent)
```

TS 中这种双向依赖通过 JS 动态导入绕过，Go 编译器**严格禁止**循环导入。

### 9.2 解决方案：三层依赖架构

```
pkg/types/       ← 纯数据结构(已有 31 文件)       ← 最底层
pkg/contracts/   ← 行为接口(新建)                 ← 中间层
internal/agents/ ← 具体实现(依赖 contracts)       ← 上层
internal/channels/ ← 具体实现(依赖 contracts)     ← 上层
cmd/openacosmi/main.go ← 组装注入点                 ← 顶层
```

### 9.3 现有 `pkg/types/` 的角色

已有 31 个文件提供**纯数据结构**:

- `types_agents.go` — AgentModelConfig, AgentListItemConfig 等
- `types_channels.go` — ChannelsConfig, ChannelDefaultsConfig 等
- `types_tools.go` — 工具 schema 定义
- `types_messages.go` — 消息结构体

这些**保持不变**，继续作为共享数据层。

### 9.4 新增 `pkg/contracts/` 接口定义

```go
// pkg/contracts/channel_sender.go
package contracts

import "github.com/openacosmi/pkg/types"

// ChannelSender — agents 模块需要的频道发送能力
type ChannelSender interface {
    SendMessage(ctx context.Context, req *types.OutboundMessage) error
    SendReaction(ctx context.Context, channel, messageID, emoji string) error
}

// ChannelRegistry — agents 查询可用频道
type ChannelRegistry interface {
    GetChannel(channelType string) (ChannelSender, bool)
    ListActiveChannels() []string
}
```

```go
// pkg/contracts/agent_trigger.go
package contracts

// AgentTrigger — channels 模块需要的 Agent 触发能力
type AgentTrigger interface {
    TriggerAgent(ctx context.Context, req *AgentTriggerRequest) (*AgentTriggerResult, error)
}

type AgentTriggerRequest struct {
    AgentID    string
    SessionKey string
    Prompt     string
    Channel    string
    SenderID   string
}

type AgentTriggerResult struct {
    Payloads []AgentPayload
    Meta     AgentMeta
}
```

### 9.5 依赖流向（单向 DAG）

```
pkg/types       ← 无依赖
pkg/contracts   ← 仅依赖 pkg/types
internal/agents ← 依赖 pkg/types + pkg/contracts
internal/channels ← 依赖 pkg/types + pkg/contracts
cmd/main.go     ← 依赖所有包，执行 DI 注入
```

**关键**: `internal/agents` 和 `internal/channels` **互不引用**，都只依赖 `pkg/contracts` 中的 interface。

### 9.6 组装注入 (Composition Root)

```go
// cmd/openacosmi/main.go (简化)
func main() {
    // 1. 创建 channels 实例 (实现 ChannelSender)
    discordCh := discord.New(cfg)
    slackCh := slack.New(cfg)
    
    // 2. 创建 channel registry
    registry := channels.NewRegistry()
    registry.Register("discord", discordCh)
    registry.Register("slack", slackCh)
    
    // 3. 创建 agents 引擎，注入 registry
    engine := agents.NewEngine(cfg, registry) // registry 类型为 contracts.ChannelRegistry
    
    // 4. 将 engine 注入回 channels (实现 AgentTrigger)
    discordCh.SetAgentTrigger(engine) // engine 类型为 contracts.AgentTrigger
    slackCh.SetAgentTrigger(engine)
}
```

### 9.7 Go 惯用法要点

| 原则 | 说明 |
|------|------|
| **接口定义在消费方** | `ChannelSender` 定义在 agents 需要的位置 |
| **隐式实现** | Go 的 interface 无需 `implements` 关键字 |
| **小接口优于大接口** | 每个 interface 仅 2-3 个方法 |
| **组装在 main** | 具体类型绑定只在 composition root |

### 9.8 实施时间安排

- **Phase 4 开始前**: 创建 `pkg/contracts/` 包，定义 `ChannelSender`, `ChannelRegistry` 接口
- **Phase 4.15 (Agent工具集)**: 工具实现使用 `contracts.ChannelSender` 接口
- **Phase 5.1 (频道抽象)**: 频道实现 `ChannelSender`，定义 `AgentTrigger` 接口
- **Phase 5.2 (频道工具桥接)**: 在 main.go 完成双向注入

---

## 五、Phase 4 接口契约规范

> [!WARNING]
> 每个子任务必须明确 **输入/输出/错误** 三个契约维度，否则会重复 Phase 1-3 的审计回溯问题。

### 5.1 子任务 4.1 — 模型配置与发现

**输入契约**:

- `OpenAcosmiConfig.agents.defaults.model` (provider, model, fallbacks)
- `OpenAcosmiConfig.agents.defaults.providers` → `models-config.providers.ts` 中定义的 19KB provider 映射
- 环境变量: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GITHUB_TOKEN` 等
- 文件系统: `${agentDir}/models.json` (运行时模型注册表)

**输出契约**:

- `ModelRegistry` — 所有发现的模型索引 (provider→model→config)
- `ModelCatalogEntry[]` — 模型目录条目
- `ensureOpenAcosmiModelsJson()` — 确保 models.json 文件存在

**错误契约**:

- provider 不支持 → 返回 `nil` + error message
- API key 缺失 → 尝试环境变量回退

**隐藏依赖**（原计划未列）:

- `synthetic-models.ts` (4KB) — 合成/虚拟模型定义
- `opencode-zen-models.ts` (9KB) — Opencode Zen 模型特殊处理
- `venice-models.ts` (10KB) — Venice 模型映射
- `bedrock-discovery.ts` (7KB) — AWS Bedrock 模型发现

### 5.2 子任务 4.2 — 模型选择 & 失败切换

**输入契约**:

- `ModelRef{provider, model}` — 模型引用
- `ModelAliasIndex` — 别名映射 (用户输入"sonnet"→"claude-sonnet-4-5")
- `OpenAcosmiConfig.agents.defaults.model.allowlist` — 可用模型白名单
- `ModelCatalogEntry[]` — 模型目录

**输出契约**:

- `resolveConfiguredModelRef()` → `ModelRef`
- `buildAllowedModelSet()` → `{allowAny, allowedCatalog, allowedKeys}`
- `resolveThinkingDefault()` → `ThinkLevel` ("off"|"low"|"medium"|"high")

**错误契约**:

- 模型不在白名单 → `{error: "model not allowed"}`
- 别名无匹配 → `null`

**隐藏依赖**:

- `model-catalog.ts` (5KB) — 模型目录构建
- `model-compat.ts` — 模型兼容性检查
- `model-scan.ts` (14KB) — 模型扫描/发现
- `live-model-filter.ts` — 实时过滤器

### 5.3 子任务 4.3 — 认证配置

**输入契约**:

- `AuthProfileStore` — JSON 持久化的 profile 存储 (`store.ts` 12KB)
- `OpenAcosmiConfig.agents.defaults.auth` — 认证配置
- 环境变量/Keychain 中的 API keys

**输出契约**:

- `ensureAuthProfileStore()` → `AuthProfileStore`
- `getApiKeyForModel()` → `ResolvedProviderAuth{apiKey, mode, profileId}`
- `resolveAuthProfileOrder()` → `string[]` (profile ID 排序)
- `markAuthProfileFailure()` — 标记失败 + cooldown
- `markAuthProfileGood()` — 标记恢复

**错误契约**:

- 所有 profile 都在 cooldown → `FailoverError{reason: "rate_limit"}`
- keychain 不可用 → 降级到环境变量

**隐藏依赖**:

- `auth-profiles/oauth.ts` (9KB) — OAuth 流程（Chutes, 各 provider）
- `auth-profiles/order.ts` (7KB) — profile 排序算法
- `auth-profiles/repair.ts` (5KB) — 损坏 profile 修复
- `auth-profiles/session-override.ts` (5KB) — 按会话覆盖认证
- `auth-profiles/external-cli-sync.ts` (4KB) — 外部 CLI 同步
- `agents/chutes-oauth.ts` (6KB) — Chutes OAuth 特殊处理
- `agents/cli-credentials.ts` (17KB!) — CLI 凭证管理

### 5.4 子任务 4.5 — PI 嵌入式引擎核心

**输入契约** (65+ 参数的 `RunEmbeddedPiAgentParams`):

- 会话标识: `sessionId`, `sessionKey`, `sessionFile`
- 模型: `provider`, `model`, `thinkLevel`, `reasoningLevel`
- 消息: `prompt`, `images`, `messageChannel`
- 回调: `onPartialReply`, `onBlockReply`, `onToolResult`, `onAgentEvent`
- 控制: `abortSignal`, `timeoutMs`, `runId`

**输出契约**:

- `EmbeddedPiRunResult{payloads, meta}`
  - `payloads[]`: `{text, isError, images?}`
  - `meta`: `{durationMs, agentMeta, usage, error?}`

**错误契约**:

- 上下文溢出 → 自动压缩(最多3次) → 工具结果截断 → 返回错误
- 认证失败 → profile 轮换 → `FailoverError`
- thinking 级别不支持 → 自动降级
- prompt 角色顺序错误 → 用户友好消息

**隐藏依赖** (严重低估 — 实际 27+ import):

- `process/command-queue.ts` — 命令排队(Phase 4 未列)
- `utils/message-channel.ts` — 消息频道判断
- `providers/github-copilot-token.ts` — 动态 import
- `pi-embedded-runner/compact.ts` (19KB) — 上下文压缩
- `pi-embedded-runner/model.ts` (8KB) — 模型解析
- `pi-embedded-runner/run/attempt.ts` — 单次尝试
- `pi-embedded-runner/run/payloads.ts` — 载荷构建
- `pi-embedded-runner/tool-result-truncation.ts` (12KB) — 工具结果截断
- `pi-embedded-runner/google.ts` (13KB) — Google 模型特殊处理
- `pi-embedded-runner/extra-params.ts` (5KB) — 额外参数
- `pi-embedded-runner/extensions.ts` — 扩展点
- `pi-embedded-runner/cache-ttl.ts` — 缓存 TTL

### 5.5 子任务 4.8 — Bash 工具执行

**输入契约**:

- `ExecToolDefaults` — 19 个配置字段
- 命令: `command`, `workdir`, `timeout`, `background`
- 安全: `host`(local/sandbox/node), `security`(off/standard), `ask`(off/on-miss/always)
- 沙箱: `BashSandboxConfig` → Docker 容器参数

**输出契约**:

- `ExecToolDetails` — 三种状态联合类型:
  - `{status:"running", sessionId, pid, tail}`
  - `{status:"completed"|"failed", exitCode, durationMs, aggregated}`
  - `{status:"approval-pending", approvalId, approvalSlug, expiresAtMs}`

**错误契约**:

- 危险环境变量 → `validateHostEnv()` 抛出
- 超时 → 进程 kill + `timedOut:true`
- 沙箱容器创建失败 → 错误返回

**隐藏依赖** (86KB 总调用链):

- `bash-tools.shared.ts` (7KB) — Docker 参数、截断、数值钳位
- `bash-tools.process.ts` (21KB) — PTY 管理、send-keys
- `bash-process-registry.ts` (7KB) — 进程注册表
- `sandbox/docker.ts` (11KB) — Docker 容器管理
- `sandbox/config.ts` (6KB) — 沙箱配置解析
- `sandbox/tool-policy.ts` (4KB) — 工具策略
- `process/exec.ts` (4KB) — 底层 exec
- `process/spawn-utils.ts` (4KB) — spawn 工具
- `pty-dsr.ts`, `pty-keys.ts` (7KB) — PTY DSR/键映射

---

## 十、第二轮补充审计 — 新增遗漏模块（+4 个）

> [!CAUTION]
> 以下 4 个模块在第一轮审计中也被遗漏，它们包含**完整的运行时系统**和**公共 API 契约**。

| # | 模块路径 | 文件数 | 大小 | 核心职责 | 被谁依赖 | 建议归入 |
|---|----------|--------|------|----------|----------|---------|
| 16 | `src/node-host/` | 3 | **38KB** (runner.ts) | 远程节点执行运行时：系统命令执行、浏览器代理、审批流、环境变量清洗、技能缓存 | gateway (WebSocket调度) | **Phase 4** |
| 17 | `src/canvas-host/` | 5 | **16KB** (server.ts) | Canvas渲染服务器 + A2UI渲染系统(7KB) | agents/tools, gateway | **Phase 7** |
| 18 | `src/plugin-sdk/` | 2 | **13KB** (index.ts) | 第三方插件 SDK 公共 API：注册hooks、定义工具、频道适配器 | 外部插件消费者 | **Phase 6** |
| 19 | `src/macos/` | 4 | **7KB** (gateway-daemon) | macOS专用 gateway daemon + relay socket | daemon/ | **Phase 6** |

### 关键发现详细说明

#### 16. `node-host/runner.ts` (38KB, 1309行, 53+函数)

> [!WARNING]
> 这是一个**完整的远程执行运行时**，与 PI Runner 平级的核心组件，但未出现在任何 Phase 计划中。

**功能清单**:

- `SystemRunParams` — 远程命令执行（带超时/审批/沙箱）
- `BrowserProxyParams/Result` — 远程浏览器代理调用
- `SkillBinsCache` — 技能二进制缓存（90s TTL）
- `sanitizeEnv()` — 环境变量安全清洗（阻止 `NODE_OPTIONS`, `DYLD_*`, `LD_*` 等）
- `ExecApprovalsSnapshot` — 执行审批快照管理
- `resolveExecSecurity/resolveExecAsk` — 安全策略解析
- `NodeInvokeRequestPayload` — 节点调用请求协议
- `isProfileAllowed` — profile 白名单过滤

**import链**（27+ 依赖）:

```
node-host/runner.ts
  → agents/agent-scope.ts (agent配置解析)
  → config/config.ts (全局配置)
  → infra/exec-host.ts (执行宿主)
  → infra/machine-name.ts (机器名)
  → infra/path-env.ts (PATH管理)
  → media/mime.ts (MIME检测)
  → utils/message-channel.ts (消息频道)
  → version.ts (版本号)
```

#### 17. `canvas-host/server.ts` (16KB) + `a2ui.ts` (7KB)

- Canvas服务器提供画布渲染能力，被 agent tools 调用
- A2UI (Agent-to-UI) 是从 agent 输出到界面元素的渲染管线

#### 18. `plugin-sdk/index.ts` (13KB)

- 暴露给第三方插件的 **公共 API 契约**
- 定义插件可以访问的所有接口：tools注册、hooks注册、channel适配器注册
- 此文件变更将**破坏所有外部插件**

#### 19. `src/macos/gateway-daemon.ts` (7KB)

- macOS专用的 `launchd` 服务管理
- 与 `daemon/launchd.ts` 不同：这是**gateway进程**的 macOS 服务封装

---

## 十一、第二轮补充审计 — 新增隐藏依赖链（+3 条）

### 4.13 消息频道归一化 → 插件运行时链

```
utils/message-channel.ts
  → channels/registry.ts (CHANNEL_IDS, normalizeChatChannelId)
  → channels/plugins/types.ts (ChannelId 类型)
  → plugins/runtime.ts (getActivePluginRegistry)  ← 运行时依赖！
  → gateway/protocol/client-info.ts (GatewayClientMode)
```

**影响**: 消息频道归一化（`normalizeMessageChannel`、`isMarkdownCapableMessageChannel`）依赖 **运行时已加载的插件注册表**。这意味着：

- 🔴 在 Phase 4 中使用 `message-channel.ts` 的代码，需要 Phase 6 的 plugins 已初始化
- 🔴 `channels/registry.ts` 的 `normalizeAnyChannelId` 调用 `requireActivePluginRegistry()`

**Go端解决方案**: 使用契约包模式，在 `pkg/contracts/` 中定义 `PluginRegistry` 接口，避免硬依赖。

### 4.14 根级入口文件 → 全模块引导链

```
entry.ts (启动入口)
  → cli/profile.ts (CLI profile 解析)
  → infra/env.ts (环境变量归一化)
  → infra/warning-filter.ts (Node 警告过滤)
  → process/child-process-bridge.ts (子进程桥接)  ← 隐藏的进程管理！

index.ts (库入口 / npm 公共 API)
  → auto-reply/reply.ts + templating.ts
  → channel-web.ts → web/ (6 个子模块)
  → config/sessions.ts
  → infra/ (7 个子模块)
  → process/exec.ts
  → cli/ (3 个子模块)
```

**影响**: `index.ts` 的 `export` 列表定义了**npm 包的公共 API 契约**。Go 端的对外 API 必须提供等价的函数集。

### 4.15 日志子系统 → channels + runtime 链

```
logging/subsystem.ts (318行, 10KB)
  → channels/registry.ts (CHAT_CHANNEL_ORDER)  ← 日志着色依赖频道列表！
  → globals.ts → logging/logger.ts (循环引用规避)
  → runtime.ts → terminal/progress-line.ts
  → logging/console.ts (8KB, 控制台捕获)
  → logging/diagnostic.ts (10KB, 诊断事件)
    → infra/diagnostic-events.ts (事件总线)
```

**影响**: 日志子系统 `createSubsystemLogger()` 硬编码了频道名称列表用于着色。Go 端需要解耦日志与频道模块。

---

## 十二、第二轮补充审计 — 根级隐式行为契约（11 项）

> [!IMPORTANT]
> 以下行为在 TS 代码中通过根级文件隐式实现，在 Go 重构中必须显式处理。

| # | 行为 | 来源文件 | Go 端等价 |
|---|------|----------|----------|
| 1 | 进程标题设为 `openacosmi` | `entry.ts:10` | `os.Args[0]` 或 `prctl` |
| 2 | Node 实验性警告抑制 + 进程重启 | `entry.ts:34-77` | Go 无需 (无实验性特性) |
| 3 | Windows argv 归一化（控制字符/引号清理） | `entry.ts:79-143` | Go 需要 Windows build tag |
| 4 | CLI profile 环境变量注入 | `entry.ts:149-160` | Go CLI 需等价 profile 系统 |
| 5 | 全局 verbose/yes 标志 | `globals.ts` | Go 全局 flag 或 context |
| 6 | 终端主题颜色导出 (`success/warn/info/danger`) | `globals.ts:49-52` | Go `lipgloss` 或 `color` |
| 7 | 控制台输出捕获到结构化日志 | `logging.ts` + `index.ts:41` | Go `slog` handler |
| 8 | VERSION 四级回退解析 | `version.ts:67-71` | Go `ldflags` 注入 |
| 9 | `CONFIG_DIR` 全局常量 | `utils.ts:347` | Go `pkg/paths` |
| 10 | JID↔E164 转换 (WhatsApp LID反向映射) | `utils.ts:80-190` | Go 独立工具函数 |
| 11 | `extensionAPI.ts` 公共 API 重导出 | `extensionAPI.ts` | Go `pkg/api` 包 |

### 关键行为详解

#### 版本号解析四级回退链 (`version.ts`)

```
VERSION = 
  1. __OPENACOSMI_VERSION__ (编译时注入 define)
  2. process.env.OPENACOSMI_BUNDLED_VERSION (环境变量)
  3. package.json 的 version 字段 (多级目录搜索)
  4. build-info.json (CI 生成)
  5. 兜底 "0.0.0"
```

Go 等价方案: `go build -ldflags "-X main.Version=x.y.z"`

#### `extensionAPI.ts` 公共 API 契约

此文件从 7 个子模块重导出关键函数：

- `resolveAgentDir`, `resolveAgentWorkspaceDir` ← `agents/agent-scope.ts`
- `DEFAULT_MODEL`, `DEFAULT_PROVIDER` ← `agents/defaults.ts`
- `resolveAgentIdentity` ← `agents/identity.ts`
- `resolveThinkingDefault` ← `agents/model-selection.ts`
- `runEmbeddedPiAgent` ← `agents/pi-embedded.ts`
- `resolveAgentTimeoutMs` ← `agents/timeout.ts`
- `ensureAgentWorkspace` ← `agents/workspace.ts`
- `resolveStorePath`, `loadSessionStore`, `saveSessionStore` ← `config/sessions.ts`

Go 端必须在 `pkg/api` 中提供等价的**稳定公共 API**。

#### Agent配置四级级联 (`identity.ts`)

```
resolveResponsePrefix() 的配置优先级：
  L1: channels.<channel>.accounts.<accountId>.responsePrefix
  L2: channels.<channel>.responsePrefix
  L3: (跳过 — 无 L3)
  L4: messages.responsePrefix
  特殊值: "auto" → 自动使用 identity.name
```

Go 端必须实现相同的四级级联逻辑。

---

## 十三、第二轮补充审计 — 新增文件大小低估（+4 个）

| 文件 | 实际大小 | 原始发现 | 影响 |
|------|----------|---------|------|
| `src/node-host/runner.ts` | **38KB** (1309行) | 第二轮发现 | 需拆 6+ Go 文件 |
| `src/canvas-host/server.ts` | **16KB** | 第二轮发现 | 独立服务 |
| `src/plugin-sdk/index.ts` | **13KB** | 第二轮发现 | 公共 API |
| `src/logging/subsystem.ts` | **10KB** + `diagnostic.ts` **10KB** + `console.ts` **8KB** = **28KB** | 第二轮发现 | 日志系统远超预估 |
| `src/channels/plugins/types.adapters.ts` | **10KB** | 第二轮发现 | 频道适配器类型 |
| `src/channels/plugins/types.core.ts` | **9KB** | 第二轮发现 | 频道核心类型 |
| `src/channels/plugins/catalog.ts` | **10KB** | 第二轮发现 | 频道目录 |
| `src/channels/plugins/group-mentions.ts` | **12KB** | 第二轮发现 | 群组提及逻辑 |

---

## 十四、第二轮审计 — 修订后的外部类型声明清单

`src/types/` 包含 9 个 `.d.ts` 文件，为以下**非标准 npm 包**提供 TypeScript 类型声明：

| 类型文件 | 对应的 npm 包 | Go 替代方案 |
|----------|-------------|------------|
| `lydell-node-pty.d.ts` | `node-pty` (PTY 终端) | `github.com/creack/pty` |
| `node-llama-cpp.d.ts` | `node-llama-cpp` (本地LLM) | FFI + llama-cpp-rs 或 Ollama API |
| `pdfjs-dist-legacy.d.ts` | `pdfjs-dist` (PDF 解析) | `github.com/ledongthuc/pdf` |
| `cli-highlight.d.ts` | `cli-highlight` (代码高亮) | `github.com/alecthomas/chroma` |
| `napi-rs-canvas.d.ts` | `@napi-rs/canvas` (Canvas) | FFI + Skia 或 `github.com/fogleman/gg` |
| `node-edge-tts.d.ts` | `node-edge-tts` (微软TTS) | HTTP API 直连 |
| `osc-progress.d.ts` | `osc-progress` (终端进度条) | `github.com/schollz/progressbar` |
| `proper-lockfile.d.ts` | `proper-lockfile` (文件锁) | `syscall.Flock` |
| `qrcode-terminal.d.ts` | `qrcode-terminal` (QR码) | `github.com/skip2/go-qrcode` |

> [!TIP]
> Go 替代方案已标注，Phase 实施时选用对应的 Go 原生库即可。

---

## 十五、`src/utils/` 工具库详细清单（15 文件）

> [!NOTE]
> `src/utils/` 被多个核心模块依赖，但从未在任何 Phase 计划中出现。

| 文件 | 大小 | 被谁使用 | 关键功能 |
|------|------|---------|---------|
| `message-channel.ts` | 4.6KB | agents, gateway, PI Runner | 消息频道归一化、Markdown能力判断 |
| `delivery-context.ts` | 4.3KB | auto-reply, cron, gateway | 投递上下文归一化、会话级投递字段合并 |
| `queue-helpers.ts` | 3.8KB | gateway, cron | 队列辅助操作 |
| `usage-format.ts` | 2.3KB | agents, PI Runner | Token 用量格式化 |
| `directive-tags.ts` | 2.0KB | agents, auto-reply | 指令标签解析 (`[thinking]`, `[tool]` 等) |
| `transcript-tools.ts` | 1.9KB | agents, auto-reply | 对话记录工具 |
| `shell-argv.ts` | 1.1KB | agents/bash-tools | Shell 参数解析 |
| `boolean.ts` | 1.0KB | config, infra | 布尔值归一化 |
| `provider-utils.ts` | 1.0KB | providers, agents | Provider 工具函数 |
| `account-id.ts` | 0.2KB | utils/delivery-context | AccountID 归一化 |

**Go 端处理**: 在 `pkg/utils/` 中创建等价的工具函数包。

---

## 十六、第二轮审计 — `channels/plugins/` 子系统详细结构（38 文件）

> [!WARNING]
> `channels/plugins/` 不是简单的插件注册，而是一个**完整的频道抽象层**，包含类型系统、配置管理、消息处理、群组特性等。

| 子目录/文件 | 大小 | 职责 |
|------------|------|------|
| `types.core.ts` | 9KB | 频道核心类型定义 |
| `types.adapters.ts` | 10KB | 频道适配器接口 |
| `types.plugin.ts` | 2.5KB | 插件类型 |
| `types.ts` | 1.7KB | 类型重导出 |
| `catalog.ts` | 10KB | 频道目录管理 |
| `directory-config.ts` | 9KB | 目录配置解析 |
| `group-mentions.ts` | 12KB | 群组@提及逻辑 |
| `config-helpers.ts` | 3.3KB | 配置辅助函数 |
| `slack.actions.ts` | 7KB | Slack 特定操作 |
| `bluebubbles-actions.ts` | 1.3KB | BlueBubbles (iMessage) 操作 |
| `pairing.ts` | 2KB | 设备配对集成 |
| `setup-helpers.ts` | 3.5KB | 频道设置辅助 |
| `status-issues/` | 5文件 | 频道健康状态检测 |
| `normalize/` | 8文件 | 消息归一化管线 |
| `outbound/` | 8文件 | 出站消息管线 |
| `onboarding/` | 8文件 | 频道引导流程 |
| `actions/` | 9文件 | 频道操作集 |

**总计**: 38 文件，约 **85KB** 代码。这是 Phase 5.1 复杂度被严重低估的根本原因。

---

## 十七、第三轮补充 — agents/ 深层文件分析

### 17.1 PI-embedded-runner/ 遗漏文件（6 个，~14KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `runs.ts` | **4KB** | 多次运行管理（重试/重入逻辑） |
| `system-prompt.ts` | 3KB | Runner 级系统提示词 |
| `history.ts` | 2KB | 历史管理 |
| `session-manager-init.ts` | 1.9KB | 会话管理器初始化 |
| `session-manager-cache.ts` | 1.8KB | 会话缓存 |
| `sandbox-info.ts` | 1KB | 沙箱信息 |

### 17.2 agents/ 未被明确列出的重要文件（~177KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `subagent-announce.ts` | **18KB** | 子 Agent 公告：跨 Agent 通信、事件广播 |
| `subagent-registry.ts` | **11KB** | 子 Agent 注册表：持久化、生命周期管理 |
| `skills-install.ts` | **15KB** | 技能安装：npm 包/git 仓库/本地路径 |
| `apply-patch.ts` | **13KB** | 代码补丁应用：diff 解析、文件修改 |
| `compaction.ts` | **11KB** | 上下文压缩策略 |
| `pi-extensions/compaction-safeguard.ts` | **12KB** | 压缩安全守卫 |
| `memory-search.ts` | **10KB** | 记忆搜索 |
| `model-fallback.ts` | **10KB** | 模型失败切换链 |
| `model-scan.ts` | **14KB** | 模型扫描发现 |
| `model-auth.ts` | **11KB** | 模型认证 |
| `workspace.ts` | **9KB** | 工作区管理 |
| `session-transcript-repair.ts` | **9KB** | 会话记录修复 |
| `tool-display.ts` | **7KB** | 工具显示格式化 |
| `tool-policy.ts` | **7KB** | 工具策略 |
| `tool-images.ts` | **6KB** | 工具图片处理 |
| `tool-call-id.ts` | **6KB** | 工具调用 ID 管理 |
| `cache-trace.ts` | **8KB** | 缓存跟踪 |

### 17.3 agents/tools/ — 60 文件，499KB

第二轮审计仅列出频道相关的 5 个文件。实际 60 个文件还包含：

- `web-fetch.ts` / `web-search.ts` — Web 工具
- `file-tools.ts` / `read-file.ts` / `write-file.ts` — 文件操作工具
- `canvas-actions.ts` — Canvas 工具
- `camera-tools.ts` — 摄像头工具
- `session-status.ts` — 会话状态工具
- `subagent-spawn.ts` — 子 Agent 工具
- 等 50+ 个其他工具文件

---

## 十八、第三轮补充 — 协议与日志子系统

### 18.1 `gateway/protocol/` — 21 文件，90KB

> [!WARNING]
> Gateway WebSocket 协议的**完整定义层**，包含所有 RPC 帧格式、schema 校验、错误码。前两轮审计完全未提及。

| 文件 | 大小 | 职责 |
|------|------|------|
| `protocol/index.ts` | **19KB** | 协议主入口：消息分发、帧解析 |
| `protocol/schema/types.ts` | **11KB** | 协议类型定义 |
| `protocol/schema/protocol-schemas.ts` | **8KB** | Schema 注册表 |
| `protocol/schema/cron.ts` | **7KB** | Cron 协议帧 |
| `protocol/schema/frames.ts` | **4KB** | 帧格式定义 |
| `protocol/schema/sessions.ts` | **4KB** | 会话协议帧 |
| `protocol/schema/agents-models-skills.ts` | **5KB** | Agent/模型/技能帧 |
| `protocol/schema/channels.ts` | **3KB** | 频道帧 |
| `protocol/schema/exec-approvals.ts` | **3KB** | 审批帧 |
| 其余 12 文件 | ~26KB | 其他协议定义 |

**Go 端影响**: `internal/gateway/` 的 WebSocket 消息格式必须与此协议层**100% 兼容**。任何帧格式不一致都将导致前端断连。

### 18.2 `logging/` — 15 文件，57KB

| 文件 | 大小 | 职责 |
|------|------|------|
| `subsystem.ts` | **10KB** | 子系统 logger 创建 |
| `diagnostic.ts` | **9KB** | 诊断事件 |
| `console.ts` | **8KB** | 控制台捕获/重定向 |
| `logger.ts` | **7KB** | 核心 logger |
| `redact.ts` | **4KB** | 日志脱敏（API Key 等） |
| `parse-log-line.ts` | **1.7KB** | 日志行解析 |
| 其余 9 文件 | ~17KB | 配置、级别、状态 |

**关键隐式行为**: `redact.ts` 自动在日志输出中**屏蔽 API 密钥**，Go 端 `pkg/log/` 需等价实现。

---

## 十九、第三轮补充 — 根级文件补充

### 19.1 `src/utils.ts` — 9.3KB（前两轮未提及）

全局工具函数集，被几乎所有模块依赖：

- `jidToPhoneNumber()` / `phoneNumberToJid()` — WhatsApp JID 转换
- `lidToLegacyId()` — LID 反向映射
- `isGroup()` / `isStatus()` — 群组/状态判断
- `CONFIG_DIR` — 全局配置目录常量
- `sleep()` — 延迟函数
- `pluralize()` — 复数化
- `formatBytes()` — 字节格式化

### 19.2 `src/polls.ts` — 2KB（完全遗漏）

投票系统：WhatsApp 投票消息的创建/解析。Go 端需实现。

### 19.3 根级隐式行为补充（+3 项）

| # | 行为 | 来源文件 | Go 端等价 |
|---|------|----------|----------|
| 12 | `src/utils.ts` 全局工具函数 | `utils.ts:80-347` | `pkg/utils/` 等价实现 |
| 13 | `src/polls.ts` 投票消息 | `polls.ts` | `internal/channels/polls.go` |
| 14 | `src/compat/index.ts` Node.js 兼容层 | `compat/` | Go 无需(确认无业务逻辑) |

---

## 二十、第四轮审计 — `src/infra/` 遗漏子系统（63 文件，~270KB）

> [!CAUTION]
> 前三轮审计仅覆盖 `infra/` 中的 6 个子系统（outbound/exec-approvals/heartbeat/session-cost/state-migrations/provider-usage）。实际 `infra/` 含 120 个非测试文件共 500KB，**遗漏率 54%**。

### 20.1 自动更新系统（4 文件，~46KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `update-runner.ts` | **26KB** | 核心更新流程：版本检查→下载→解压→替换 |
| `update-check.ts` | **11KB** | 版本检查 + 频率控制 |
| `update-global.ts` | **5KB** | 全局 npm 包更新 |
| `update-startup.ts` | **4KB** | 启动时更新检查 |

**影响归入**: Phase 7 或 Phase 8

### 20.2 设备身份与配对系统（4 文件，~35KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `device-pairing.ts` | **16KB** | 多设备配对：发现、认证、状态 |
| `node-pairing.ts` | **10KB** | 远程节点配对 |
| `device-identity.ts` | **5KB** | 设备 ID 生成与持久化 |
| `device-auth-store.ts` | **3KB** | 设备认证存储 |

**影响归入**: Phase 6（与 `pairing/` 目录联动）

### 20.3 端口与网络管理（6 文件，~17KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `ports-inspect.ts` | **8KB** | 端口占用检测 |
| `ports.ts` | **3KB** | 端口分配 |
| `ports-format.ts` | **2KB** | 端口格式化 |
| `ports-lsof.ts` | **1KB** | lsof 解析 |
| `ports-types.ts` | **0.4KB** | 端口类型 |
| `ssh-tunnel.ts` | **6KB** | SSH 隧道管理 |

### 20.4 系统存在与事件（4 文件，~16KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `system-presence.ts` | **9KB** | 系统在线状态发布/订阅 |
| `diagnostic-events.ts` | **4KB** | 诊断事件收集 |
| `system-events.ts` | **3KB** | 系统事件总线 |
| `agent-events.ts` | **2KB** | Agent 事件通知 |

### 20.5 重启与进程管理（4 文件，~15KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `restart.ts` | **6KB** | 热重启逻辑 |
| `restart-sentinel.ts` | **4KB** | 重启哨兵（infinite loop 检测） |
| `gateway-lock.ts` | **7KB** | 网关单实例锁（防止多 gateway 并发） |
| `runtime-guard.ts` | **3KB** | 运行时守卫（Node 版本检查等） |

### 20.6 其余基础设施工具（~40KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `skills-remote.ts` | **11KB** | 远程技能仓库管理 |
| `control-ui-assets.ts` | **8KB** | 控制面板 UI 静态资源打包 |
| `channel-summary.ts` | **8KB** | 频道摘要生成 |
| `shell-env.ts` | **5KB** | Shell 环境变量继承 |
| `unhandled-rejections.ts` | **5KB** | 未捕获异常处理 |
| `path-env.ts` | **4KB** | PATH 环境变量管理 |
| `retry.ts` + `retry-policy.ts` | **7KB** | 通用重试策略 |
| `archive.ts` | **4KB** | 归档/解压 |
| `fetch.ts` | **2KB** | HTTP fetch 封装 |
| 其余 15 文件 | ~15KB | fs-safe, dotenv, env-file, etc. |

---

## 二十一、第四轮审计 — 完全未记录的目录级模块（~460KB）

> [!CAUTION]
> 以下 6 个目录在前三轮审计中**零提及**，但含大量业务逻辑，必须纳入重构计划。

### 21.1 `src/web/` — WhatsApp Web 频道（43 文件，185KB）

| 子模块 | 大小 | 职责 |
|--------|------|------|
| `session.ts` | **10KB** | WA 会话管理（认证状态、二维码循环、重连） |
| `media.ts` | **10KB** | 媒体收发（图片/视频/文档/语音） |
| `login-qr.ts` | **8KB** | QR 码登录流程 |
| `auth-store.ts` | **6KB** | WA 认证凭证持久化 |
| `accounts.ts` | **6KB** | 多账号管理 |
| `outbound.ts` | **6KB** | 出站消息（格式化+发送） |
| `reconnect.ts` | **2KB** | 断线重连 |
| `auto-reply/` | 26 文件 | WA 专用自动回复子系统 |
| `inbound/` | 8 文件 | 入站消息处理管线 |

**影响归入**: Phase 5（通信频道）
**隐形依赖**: `web/auto-reply/` → `agents/` → `infra/outbound/`

### 21.2 `src/tui/` — 终端 UI（24 文件，122KB）

| 子模块 | 大小 | 职责 |
|--------|------|------|
| `tui.ts` | **19KB** | TUI 主控制器（Ink 组件树） |
| `tui-command-handlers.ts` | **15KB** | 命令处理（/model /agent 等） |
| `tui-session-actions.ts` | **13KB** | 会话操作（创建/切换/删除） |
| `gateway-chat.ts` | **7KB** | Gateway 聊天交互 |
| `tui-event-handlers.ts` | **8KB** | 事件处理 |
| `tui-formatters.ts` | **7KB** | 输出格式化 |
| `components/` | ~10KB | UI 组件（searchable-select 等） |
| `theme/` | ~5KB | 主题系统 |

**影响归入**: Phase 8 或 Phase 9

### 21.3 `src/media/` — 媒体处理（11 文件，58KB）

| 子模块 | 大小 | 职责 |
|--------|------|------|
| `image-ops.ts` | **14KB** | 图片缩放/裁剪/格式转换 |
| `input-files.ts` | **11KB** | 文件输入解析 |
| `store.ts` | **8KB** | 媒体文件存储管理 |
| `parse.ts` | **7KB** | 媒体元数据解析 |
| `fetch.ts` | **6KB** | 远程媒体下载 |
| `mime.ts` | **5KB** | MIME 类型识别 |
| `server.ts` | **3KB** | 媒体服务器（本地HTTP） |

**影响归入**: Phase 7（辅助模块）
**隐形依赖**: `media/store.ts` 被 `agents/`, `web/`, `channels/` 共同依赖

### 21.4 `src/wizard/` — 初始化向导（7 文件，55KB）

| 子模块 | 大小 | 职责 |
|--------|------|------|
| `onboarding.finalize.ts` | **19KB** | 配置最终化（写入文件+启动） |
| `onboarding.ts` | **15KB** | 引导流程主控 |
| `onboarding.gateway-config.ts` | **9KB** | Gateway 配置生成 |
| `session.ts` | **7KB** | 向导会话状态 |

**影响归入**: Phase 8（CLI）

### 21.5 `src/terminal/` — 终端管理（10 文件，21KB）

包含 PTY 管理、终端检测、大小调整等。被 `agents/bash-tools` 深度依赖。

**影响归入**: Phase 4（bash-tools 依赖）

### 21.6 根级引导文件（6 文件，~13KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `entry.ts` | **5KB** | 应用入口：环境初始化 → CLI profile → respawn |
| `index.ts` | **3KB** | 模块导出 barrel |
| `version.ts` | **2KB** | 版本号管理 |
| `globals.ts` | **1KB** | 全局变量 |
| `runtime.ts` | **0.7KB** | 运行时检测 |
| `extensionAPI.ts` | **0.6KB** | 扩展 API |

**隐形行为**: `entry.ts` 的 `installProcessWarningFilter()` + `normalizeEnv()` + `normalizeWindowsArgv()` 是全局隐式初始化

---

## 二十二、第四轮审计 — 高优遗漏模块（~1.1MB）

### 22.1 `src/security/` — 安全审计系统（8 文件，131KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `audit-extra.ts` | **45KB** | 扩展安全检查（文件权限/路径注入/符号链接） |
| `audit.ts` | **37KB** | 核心安全审计引擎 |
| `fix.ts` | **15KB** | 安全问题自动修复 |
| `skill-scanner.ts` | **12KB** | 技能文件安全扫描 |
| `external-content.ts` | **8KB** | 外部内容安全策略 |

**影响归入**: Phase 4（agents/bash-tools 依赖安全检查）
**隐形依赖**: `agents/bash-tools` → `security/audit` → `security/fix`

### 22.2 `src/daemon/` — 系统服务管理（19 文件，105KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `launchd.ts` | **14KB** | macOS launchd 服务 |
| `systemd.ts` | **14KB** | Linux systemd 服务 |
| `schtasks.ts` | **12KB** | Windows 计划任务 |
| `inspect.ts` | **12KB** | 服务状态检查 |
| `service-audit.ts` | **11KB** | 服务配置审计 |

**影响归入**: Phase 8（CLI/daemon 命令）

### 22.3 `src/cron/` — 定时任务调度（22 文件，116KB）

| 文件 | 大小 | 职责 |
|------|------|------|
| `isolated-agent/run.ts` | **22KB** | 独立 Agent 运行器 |
| `service/timer.ts` | **16KB** | 定时器管理 |
| `service/store.ts` | **15KB** | 任务持久化 |
| `normalize.ts` | **13KB** | Cron 表达式标准化 |
| `service/jobs.ts` | **12KB** | Job 执行引擎 |

**影响归入**: Phase 7（辅助模块）
**隐形依赖**: `cron/isolated-agent/` → `agents/` → `infra/outbound/`

### 22.4 `src/signal/` — Signal 协议频道（14 文件，76KB）

**影响归入**: Phase 5（通信频道）

### 22.5 通信频道总量修订

| 频道 | 文件数 | 代码量 | 审计覆盖 |
|------|--------|--------|----------|
| `discord/` | — | **269KB** | 部分(channels/) |
| `telegram/` | — | **258KB** | 部分(channels/) |
| `slack/` | — | **187KB** | 部分(channels/) |
| `web/` (WhatsApp Web) | — | **185KB** | ❌ 零覆盖 |
| `line/` | — | **156KB** | ❌ 零覆盖 |
| `signal/` | — | **76KB** | ❌ 零覆盖 |
| `imessage/` | — | **55KB** | ❌ 零覆盖 |
| **频道总计** | — | **~1.2MB** | ~40% |

### 22.6 其余遗漏

| 目录 | 大小 | 职责 |
|------|------|------|
| `markdown/` | **37KB** | Markdown 解析/渲染 |
| `acp/` | **36KB** | 访问控制协议 |
| `pairing/` | **16KB** | 设备配对 |
| `link-understanding/` | **8KB** | 链接预览/解析 |
| `shared/` | **3KB** | 共享工具 |

---

## 二十三、第四轮审计 — 代码量真实规模与审计覆盖率

### 23.1 原 TypeScript 项目真实规模

```
总非测试 .ts 文件: 9.2MB (9,185,171 bytes)
```

| 区域 | 代码量 | 占比 |
|------|--------|------|
| `agents/` | ~2.0MB | 22% |
| `channels/` 相关 (discord/telegram/slack/web/line/signal/imessage/channels/) | ~1.6MB | 17% |
| `auto-reply/` | ~0.8MB | 9% |
| `config/` | ~0.5MB | 5% |
| `infra/` | ~0.5MB | 5% |
| `gateway/` | ~0.5MB | 5% |
| `commands/` | ~0.5MB | 5% |
| `cli/` | ~0.4MB | 4% |
| `security/` | ~0.1MB | 1% |
| `cron/` | ~0.1MB | 1% |
| `daemon/` | ~0.1MB | 1% |
| 其余 (~20 个目录) | ~2.0MB | 22% |

### 23.2 前三轮审计覆盖评估

| 指标 | 数值 |
|------|------|
| 总 src/ 目录数 | **50** |
| 审计报告覆盖目录数 | **~18** |
| 未覆盖目录数 | **~32** |
| 未覆盖代码量 | **~3.5MB** |
| 审计覆盖率（代码量） | **~62%** |

> [!WARNING]
> 前三轮审计覆盖了约 62% 的代码量。遗漏的 38% (~3.5MB) 中包含关键业务系统：security(131KB)、daemon(105KB)、cron(116KB)、全部非核心通信频道(~670KB)、media(58KB) 等。这些模块包含大量隐形依赖链，必须纳入 Phase 4-9 计划。
