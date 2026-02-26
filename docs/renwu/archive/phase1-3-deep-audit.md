# Phase 1-3 深度二次审计报告

> 日期: 2026-02-12 | 审计人: AI 高级全栈架构师视角
> 范围: Phase 1 (配置与类型), Phase 2 (网关基础层), Phase 3 (网关业务层)

---

## 一、审计概览

### 1.1 审计背景

Phase 4-9 深度审计发现原计划存在大量遗漏（19 个模块、15 条隐藏依赖链）。本报告对已完成重构的 Phase 1-3 执行独立二次审计，验证 TS→Go 移植的完整性，并检查 Go 代码健康度。

### 1.2 审计方法

1. **TS 源码逐文件扫描** — 列出 Phase 1-3 涉及的所有 TS 文件及其核心能力
2. **Go 代码对照** — 逐一检查 Go 端是否有等价实现
3. **Go 代码健康度** — 检查已实现代码的并发安全、错误处理、边界条件
4. **分段输出** — 避免上下文过载

### 1.3 关键发现总结

| 发现类别 | 数量 | 影响 |
|----------|------|------|
| Phase 1 遗漏文件/能力 | **18 项** | 🔴 高 — 配置系统功能不完整 |
| Phase 2 遗漏文件/能力 | **14 项** | 🔴 高 — 网关基础功能缺失 |
| Phase 3 遗漏文件/能力 | **22 项** | 🔴 高 — 网关业务层大量缺失 |
| Go 代码潜在 Bug | **6 项** | 🟡 中 — 需逐一验证修复 |
| Go 代码健康项（已良好） | **8 项** | ✅ 已达标 |

---

## 二、Phase 1 审计：配置与类型系统

### 2.1 TS 源码范围 vs Go 实现对比

#### Phase 1.1 — 核心类型定义 (`src/types/` → `pkg/types/`)

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| `types/` 9 个 `.d.ts` 声明文件 | N/A — 类型声明，不需移植 | ✅ 无需处理 |

> [!TIP]
> `src/types/` 目录仅包含 `.d.ts` 类型声明文件（node-pty, pdfjs 等第三方包类型），不含业务逻辑，无需 Go 移植。真正的配置类型定义在 `src/config/types.*.ts` 中。

**实际类型来源**: `src/config/types.*.ts` (20+ 文件) → `pkg/types/` (31 文件)

| TS 类型文件 | Go 等价 | 状态 |
|------------|---------|------|
| `types.base.ts` (5.3KB) | `types.go` (13.4KB) | ✅ 已覆盖 |
| `types.agents.ts` (2.8KB) | `types_agents.go` (3.6KB) | ✅ 已覆盖 |
| `types.agent-defaults.ts` (11.2KB) | `types_agent_defaults.go` (11.8KB) | ✅ 已覆盖 |
| `types.channels.ts` (1.3KB) | `types_channels.go` (1.3KB) | ✅ 已覆盖 |
| `types.discord.ts` (6.5KB) | `types_discord.go` (6.8KB) | ✅ 已覆盖 |
| `types.gateway.ts` (8.5KB) | `types_gateway.go` (8.7KB) | ✅ 已覆盖 |
| `types.hooks.ts` (3.2KB) | `types_hooks.go` (6.2KB) | ✅ 已覆盖（更完整） |
| `types.tools.ts` (16.1KB) | `types_tools.go` (20.6KB) | ✅ 已覆盖（更完整） |
| `types.messages.ts` (4.4KB) | `types_messages.go` (3.6KB) | ✅ 已覆盖 |
| `types.models.ts` (1.4KB) | `types_models.go` (3.7KB) | ✅ 已覆盖（更完整） |
| `types.openacosmi.ts` (3.7KB) | `types_openacosmi.go` (4.6KB) | ✅ 已覆盖 |
| `types.sandbox.ts` (2.6KB) | `types_sandbox.go` (2.6KB) | ✅ 已覆盖 |
| `types.skills.ts` (0.9KB) | `types_skills.go` (1.2KB) | ✅ 已覆盖 |
| `types.telegram.ts` (7.2KB) | `types_telegram.go` (6.4KB) | ✅ 已覆盖 |
| `types.slack.ts` (5.9KB) | `types_slack.go` (6.2KB) | ✅ 已覆盖 |
| `types.whatsapp.ts` (6.6KB) | `types_whatsapp.go` (3.5KB) | ⚠️ 部分缺失 |
| `types.tts.ts` (2.5KB) | `types_tts.go` (4.0KB) | ✅ 已覆盖 |
| `types.memory.ts` (1.1KB) | `types_memory.go` (2.6KB) | ✅ 已覆盖 |
| `types.plugins.ts` (0.9KB) | `types_plugins.go` (1.5KB) | ✅ 已覆盖 |

**Phase 1.1 结论**: ✅ 类型系统移植基本完整，覆盖率 **95%+**。

> [!NOTE]
> `types_whatsapp.go` (3.5KB) 比 TS 版 (6.6KB) 小约 47%，可能缺少部分 WhatsApp 特定类型（如 LID 映射、群组元数据等）。建议 Phase 5 实施前补充。

---

#### Phase 1.2 — 配置加载 (`src/config/` → `internal/config/`)

这是 **差距最大的区域**。TS `src/config/` 包含 **125 个文件**（含测试），Go `internal/config/` 仅有 **29 个文件**。

**已完成（Go 已实现）**:

| TS 文件 | Go 文件 | 行数对比 | 状态 |
|---------|---------|---------|------|
| `io.ts` (617L) | `loader.go` (562L) | 91% | ✅ 核心管道已完成 |
| `defaults.ts` (471L) | `defaults.go` (285L) | 60% | ⚠️ 部分函数缺失 |
| `includes.ts` (250L) | `includes.go` (7KB) | 100% | ✅ 完整 |
| `env-substitution.ts` (122L) | `envsubst.go` (4KB) | 100% | ✅ 完整 |
| `config-paths.ts` (77L) | `configpath.go` (3.2KB) | 100% | ✅ 完整 |
| `paths.ts` (244L) | `paths.go` (6.2KB) | 100% | ✅ 完整 |
| `normalize-paths.ts` (52L) | `normpaths.go` (2.6KB) | 100% | ✅ 完整 |
| `runtime-overrides.ts` (58L) | `overrides.go` (3.9KB) | 100% | ✅ 完整 |
| `agent-dirs.ts` (107L) | `agentdirs.go` (5.6KB) | 100% | ✅ 完整 |
| `version.ts` (40L) | `version.go` (2.0KB) | 100% | ✅ 完整 |
| `legacy*.ts` (5 文件) | `legacy.go`+`legacy_migrations*.go` | 95% | ✅ 基本完整 |

**缺失（Go 未实现 / 严重不足）**:

| # | TS 文件 | 大小 | 核心功能 | Go 状态 | 影响 |
|---|---------|------|----------|---------|------|
| 1 | `schema.ts` | **55.8KB** (1115L) | Zod schema → JSON Schema + UI Hints | `schema.go` 仅 5.3KB (hint 标记) | 🔴 **严重缺失** |
| 2 | `validation.ts` | **10.9KB** (362L) | 插件感知验证 + heartbeat/avatar 校验 | `validator.go` 仅 5.5KB (基础验证) | 🔴 缺失 |
| 3 | `plugin-auto-enable.ts` | **12.3KB** (456L) | 按配置自动启用插件 (31 函数) | ❌ 零实现 | 🔴 缺失 |
| 4 | `group-policy.ts` | **6.2KB** (214L) | 频道群组策略/工具权限 | ❌ 零实现 | 🔴 缺失 |
| 5 | `redact-snapshot.ts` | **5.7KB** (169L) | 敏感字段脱敏 + 还原 | ❌ 零实现 | 🔴 安全相关 |
| 6 | `sessions/store.ts` | **15.7KB** (500+L) | Session 持久化存储 | ❌ 零实现 | 🔴 缺失 |
| 7 | `sessions/transcript.ts` | **4.2KB** | 会话记录管理 | ❌ 零实现 | 🟡 |
| 8 | `sessions/metadata.ts` | **5.1KB** | 会话元数据 | ❌ 零实现 | 🟡 |
| 9 | `sessions/reset.ts` | **5.4KB** | 会话重置 | ❌ 零实现 | 🟡 |
| 10 | `sessions/group.ts` | **3.5KB** | 群组会话 | ❌ 零实现 | 🟡 |
| 11 | `sessions/paths.ts` | **2.9KB** | 会话路径解析 | ❌ 零实现 | 🟡 |
| 12 | `zod-schema.ts` | **19.8KB** | Zod 验证 schema 主文件 | ❌ 零实现 | 🔴 |
| 13 | `zod-schema.core.ts` | **16.2KB** | 核心 schema 定义 | ❌ 零实现 | 🔴 |
| 14 | `zod-schema.providers-core.ts` | **30.2KB** | Provider schema | ❌ 零实现 | 🔴 |
| 15 | `zod-schema.agent-runtime.ts` | **17.4KB** | Agent 运行时 schema | ❌ 零实现 | 🔴 |
| 16 | `markdown-tables.ts` | **2.3KB** | 配置导出 markdown 表格 | ❌ 零实现 | 🟢 低优 |
| 17 | `channel-capabilities.ts` | **2.7KB** | 频道能力检测 | ❌ 零实现 | 🟡 |
| 18 | `talk.ts` | **1.2KB** | Talk API key 解析 | ❌ 零实现 | 🟡 |

> [!CAUTION]
> **最严重遗漏**: `schema.ts` (55KB) 是整个配置 UI 的核心。它将 Zod schema 转换为 JSON Schema + UI Hints，驱动 Web 控制台的配置编辑器。Go 端 `schema.go` 仅实现了字段 hint 标记（5.3KB），缺少：
>
> - JSON Schema 生成（从 Go struct 反射）
> - 200+ 个字段的 UI 标签/帮助文本/分组
> - 插件/频道元数据注入
> - 动态 schema 扩展

**defaults.go 缺失函数**:

| 缺失函数 | TS 行号 | 功能 | 优先级 |
|----------|---------|------|--------|
| `resolvePrimaryModelRef()` | L96-106 | 解析主模型引用 | P1 |
| `resolveAnthropicDefaultAuthMode()` | L56-94 | Anthropic 认证模式 | P2 |
| `applyTalkApiKey()` | L154-170 | Talk API key 回退 | P2 |
| `resolveModelCost()` | L44-54 | 模型成本解析 | P2 |

---

#### Phase 1.3 — 全局工具函数 (`src/utils.ts` + `src/utils/` → `pkg/utils/`)

| TS 文件 | Go 等价 | 状态 |
|---------|---------|------|
| `utils.ts` (348L, 33 函数) | `pkg/utils/utils.go` (5.8KB) | ⚠️ **严重不足** |
| `utils/message-channel.ts` (4.6KB) | ❌ | 🔴 缺失 |
| `utils/delivery-context.ts` (4.3KB) | ❌ | 🔴 缺失 |
| `utils/queue-helpers.ts` (3.8KB) | ❌ | 🔴 缺失 |
| `utils/usage-format.ts` (2.3KB) | ❌ | 🟡 缺失 |
| `utils/directive-tags.ts` (2.0KB) | ❌ | 🟡 缺失 |
| `utils/transcript-tools.ts` (1.9KB) | ❌ | 🟡 缺失 |
| `utils/shell-argv.ts` (1.1KB) | ❌ | 🟡 缺失 |
| `utils/boolean.ts` (1.0KB) | ❌ | 🟡 缺失 |
| `utils/provider-utils.ts` (1.0KB) | ❌ | 🟡 缺失 |

---

## 三、Phase 2 审计：网关基础层

### 3.1 TS 源码范围

Phase 2 覆盖 `src/gateway/` 基础设施层，TS 端共 **129 个文件**，Go 端 `internal/gateway/` 仅 **24 个文件**。

### 3.2 已完成（Go 已实现）

| TS 文件 | Go 文件 | 行数对比 | 说明 |
|---------|---------|---------|------|
| `client.ts` (15.5KB) | `ws.go` (292L) | 55% | WS 客户端：连接/重连/心跳 ✅ |
| `auth.ts` (8.5KB) | `auth.go` (275L) | 70% | Token/密码认证 ✅ |
| `net.ts` (3.5KB) | `net.go` (135L) | 85% | IP 解析/信任代理 ✅ |
| `boot.ts` (11.2KB) | `boot.go` (380L) | 60% | 网关启动流程 ⚠️ 部分缺失 |
| `server-chat.ts` (414L) | `chat.go` (417L) | 100% | Agent事件处理 ✅ |

### 3.3 缺失（Go 未实现）

| # | TS 文件/目录 | 大小 | 核心功能 | Go 状态 |
|---|-------------|------|----------|---------|
| 1 | `server-http.ts` | **13.8KB** (451L) | HTTP 服务器 + 路由分发 (hooks, canvas, OpenAI, plugins) | ⚠️ 仅基础路由 |
| 2 | `openai-http.ts` | **15.3KB** | OpenAI Chat Completions 兼容 API | ❌ 零实现 |
| 3 | `openresponses-http.ts` | **8.2KB** | OpenAI Responses API 兼容 | ❌ 零实现 |
| 4 | `hooks.ts` | **11.4KB** | Webhook 分发 (wake/agent/message) | ❌ 零实现 |
| 5 | `config-reload.ts` | **4.6KB** | 配置热重载 + 广播 | ❌ 零实现 |
| 6 | `server-broadcast.ts` | **9.8KB** | 多客户端事件广播 | `broadcast.go` 部分 |
| 7 | `server-node-events.ts` | **6.3KB** | 节点事件路由 | ❌ 零实现 |
| 8 | `server-channels.ts` | **22.5KB** | 频道管理 (connect/disconnect/status) | ❌ 零实现 |
| 9 | `tools-invoke-http.ts` | **5.8KB** | 工具 HTTP 调用代理 | ❌ 零实现 |
| 10 | `ws-log.ts` | **3.2KB** | WS 消息格式化日志 | ❌ 零实现 |
| 11 | `http-common.ts` | **2.1KB** | HTTP 通用函数 | ❌ 零实现 |
| 12 | `http-utils.ts` | **1.8KB** | Header 解析工具 | ❌ 零实现 |
| 13 | `control-ui.ts` | **5.6KB** | Web 控制台 UI 静态文件服务 | ❌ 零实现 |
| 14 | `tls-simple.ts` | **2.1KB** | 自签名 TLS 证书 | ❌ 零实现 |

### 3.4 server-methods/ 完全缺失（41 文件, 267KB）

`server-methods/` 是网关的 **核心 API 方法注册层**，实现了所有 WS 协议方法。Go 端 **零实现**。

| TS 文件 | 大小 | 功能 |
|---------|------|------|
| `chat.ts` | **22.5KB** | 聊天消息处理（最大文件） |
| `agents.ts` | **15.0KB** | Agent CRUD + 列表 |
| `agent.ts` | **17.8KB** | 单 Agent 操作 |
| `sessions.ts` | **15.1KB** | Session 列表/预览/操作 |
| `usage.ts` | **26.7KB** | 用量统计（最大文件） |
| `send.ts` | **11.5KB** | 消息发送 |
| `config.ts` | **13.1KB** | 配置 get/set/apply/patch |
| `channels.ts` | **10.4KB** | 频道管理 |
| `nodes.ts` | **17.2KB** | 节点管理 |
| `browser.ts` | **8.5KB** | 浏览器工具 |
| `exec-approvals.ts` | **6.9KB** | 执行审批 |
| `skills.ts` | **6.7KB** | 技能管理 |
| `devices.ts` | **5.5KB** | 设备管理 |
| `cron.ts` | **6.9KB** | 定时任务 |
| `logs.ts` | **4.8KB** | 日志查询 |
| `system.ts` | **5.5KB** | 系统信息 |
| `wizard.ts` | **4.4KB** | 配置向导 |
| ...其余 24 文件 | ~55KB | 各类辅助方法 |

> [!CAUTION]
> `server-methods/` 是网关的 **灵魂**，定义了客户端可调用的所有方法。缺少它意味着 Web 控制台和外部集成 **完全无法工作**。

### 3.5 protocol/ 完全缺失（21 文件, 24KB）

| TS 文件 | 大小 | 功能 |
|---------|------|------|
| `protocol/index.ts` | **19.7KB** | WS 协议类型定义（所有方法签名） |
| `protocol/schema/` (17 文件) | ~18KB | 协议 Zod 验证 schema |

### 3.6 server/ 基础设施（11 文件, 24KB）

| TS 文件 | 大小 | Go 状态 |
|---------|------|---------|
| `ws-connection.ts` | **8.5KB** | `connection.go` ⚠️ 部分 |
| `health-state.ts` | **2.5KB** | ❌ |
| `hooks.ts (server/)` | **3.8KB** | ❌ |
| `plugins-http.ts` | **1.9KB** | ❌ |
| `http-listen.ts` | **1.1KB** | ❌ |
| `tls.ts` | **0.5KB** | ❌ |
| `close-reason.ts` | **0.4KB** | ❌ |

---

## 四、Phase 3 审计：网关业务层

Phase 3 的 TS 文件大多和 Phase 2 在同一 `src/gateway/` 目录下。

### 4.1 已完成

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| `server-chat.ts` (414L) | `chat.go` (417L) | ✅ 完整 |

### 4.2 严重缺失

| # | TS 文件 | 大小 | Go 状态 |
|---|---------|------|---------|
| 1 | `session-utils.ts` | **22.2KB** (734L, 27 函数) | ❌ 零实现 |
| 2 | `session-utils.types.ts` | **4.4KB** | ❌ |
| 3 | `session-utils.fs.ts` | **7.8KB** | ❌ |
| 4 | `server-channels.ts` | **22.5KB** | ❌ |

`session-utils.ts` 包含 27 个关键函数:

| 函数 | 说明 |
|------|------|
| `resolveIdentityAvatarUrl()` | 头像 URL 解析 |
| `deriveSessionTitle()` | 会话标题推导 |
| `loadSessionEntry()` | 加载会话条目 |
| `classifySessionKey()` | 会话类型分类 |
| `listAgentsForGateway()` | 网关 Agent 列表 |
| `resolveSessionStoreKey()` | 存储 key 解析 |
| `resolveGatewaySessionStoreTarget()` | 存储目标解析 |
| `loadCombinedSessionStoreForGateway()` | 合并存储加载 |
| `listSessionsFromStore()` | 会话列表查询 (180 行) |
| ...其余 18 个 | 会话管理各功能 |

**`utils.ts` 中已实现 vs 缺失**:

| 已实现 (Go) | 缺失 (Go) |
|------------|----------|
| `ensureDir` | `withWhatsAppPrefix`, `normalizeE164` |
| `clampNumber`, `clampInt` | `toWhatsappJid`, `jidToE164` (JID↔E164 转换) |
| `normalizePath` | `resolveJidToE164` (异步 LID 反查) |
| `sleep` | `isSelfChatMode` (WhatsApp 自聊模式) |
| `resolveConfigDir` | `sliceUtf16Safe`, `truncateUtf16Safe` (UTF-16 安全截断) |
| | `resolveUserPath`, `shortenHomePath` |
| | `formatTerminalLink` (终端超链接) |
| | `CONFIG_DIR` 全局常量 |

> [!WARNING]
> `message-channel.ts` 是一个关键的跨模块工具文件（Phase 4-9 审计已标注其隐藏依赖链 4.13），被 agents、gateway、PI Runner 等核心模块广泛使用。Phase 1 应该包含它但完全缺失。

---

## 五、Go 代码健康度评估

### 5.1 已实现代码的良好实践 ✅

| 文件 | 评估项 | 评价 |
|------|--------|------|
| `loader.go` | JSON5 解析用 hujson 库 | ✅ 正确选型 |
| `loader.go` | 配置缓存 + RWMutex | ✅ 并发安全 |
| `loader.go` | 原子写入 + 备份轮换 | ✅ 数据安全 |
| `includes.go` | 循环引用检测 | ✅ 完整 |
| `ws.go` | 指数退避重连（非递归循环） | ✅ 避免 goroutine 泄漏 |
| `ws.go` | Ping/Pong 心跳 + ReadDeadline | ✅ 连接健康检测 |
| `chat.go` | Delta 节流 (150ms) | ✅ 避免广播风暴 |
| `defaults.go` | 指针类型可选字段 | ✅ 正确区分零值和未设置 |

### 5.2 潜在 Bug / 需关注项 🟡

| # | 文件 | 问题 | 影响 | 建议 |
|---|------|------|------|------|
| 1 | `defaults.go` L89-98 | `applySessionDefaults` 强制覆盖 `mainKey="main"`，但 TS 版会 log 警告再覆盖。Go 版静默覆盖可能导致用户困惑 | 低 | 添加 warn 日志 |
| 2 | `defaults.go` L148-190 | `applyContextPruningDefaults` 仅在 `ContextPruning != nil` 时触发，但 TS 版在 nil 时也会创建默认对象 | 中 | 对齐 TS 行为 |
| 3 | `loader.go` L469-482 | `getCached` 无并发读保护（虽然外层有 RWMutex） — 确认调用链全部在锁内 | 低 | 审查调用链 |
| 4 | `ws.go` L72-127 | `Connect()` 获取 mutex 后发送 connect 帧，若网络慢会长时间持锁 | 中 | 考虑缩小锁范围 |
| 5 | `chat.go` L109-131 | `handleDelta` 节流逻辑使用 `time.Now()` 但未加锁保护 `deltaSentAt` | 中 | 检查 data race |
| 6 | `loader.go` L355-467 | `applyConfigPipeline` 长达 112 行，管道步骤参数传递复杂 | 低 | 可考虑拆分 |

### 5.3 infra 层对比 (`src/infra/` → `internal/infra/`)

| TS 文件 | 大小 | Go 状态 |
|---------|------|---------|
| `home-dir.ts` | 3.2KB | ✅ 已覆盖 |
| `os.ts` | 1.5KB | ✅ 已覆盖 |
| `bonjour/` | 15KB+ | ✅ 已实现 |
| `heartbeat.ts` | 8.7KB | `heartbeat.go` ⚠️ 基础 |
| `system-events.ts` | 4.2KB | ✅ 已覆盖 |
| `agent-events.ts` | 6.5KB | ❌ 零实现 |
| `heartbeat-visibility.ts` | 2.3KB | ❌ 零实现 |
| `sleep-detect.ts` | 1.8KB | ❌ 零实现 |
| `update-check.ts` | 5.6KB | ❌ |
| `cli-support.ts` | 3.4KB | ❌ |

---

## 六、修复行动计划

### 6.1 优先级 P0 — 阻碍后续 Phase 的缺失项

| 序号 | 缺失项 | 建议归属 Phase | 预估工作量 |
|------|--------|---------------|-----------|
| 1 | `redact-snapshot.ts` → Go | Phase 1 补丁 | 2h |
| 2 | `defaults.go` 缺失函数 (4 个) | Phase 1 补丁 | 3h |
| 3 | `plugin-auto-enable.ts` → Go | Phase 1 补丁 | 6h |
| 4 | `session-utils.ts` → Go | Phase 3 补丁 | 8h |
| 5 | `group-policy.ts` → Go | Phase 1 补丁 | 3h |
| 6 | Go Bug #2 (contextPruning defaults) | Phase 1 补丁 | 1h |

### 6.2 优先级 P1 — 网关可用性

| 序号 | 缺失项 | 建议归属 Phase | 预估工作量 |
|------|--------|---------------|-----------|
| 7 | `protocol/index.ts` 类型定义 | Phase 2 补丁 | 4h |
| 8 | `server-methods/config.ts` | Phase 3 补丁 | 4h |
| 9 | `server-methods/chat.ts` | Phase 3 补丁 | 6h |
| 10 | `server-methods/sessions.ts` | Phase 3 补丁 | 4h |
| 11 | `server-methods/agents.ts` | Phase 3 补丁 | 4h |
| 12 | `config-reload.ts` | Phase 2 补丁 | 3h |
| 13 | `hooks.ts` | Phase 2 补丁 | 4h |

### 6.3 优先级 P2 — 功能完整性

| 序号 | 缺失项 | 建议归属 Phase | 预估 |
|------|--------|---------------|------|
| 14 | `config/sessions/` (13 文件) | Phase 1 或 3 | 12h |
| 15 | `zod-schema*.ts` → Go 验证 | Phase 1 延后 | 16h |
| 16 | `schema.ts` → JSON Schema 生成 | Phase 1 延后 | 20h |
| 17 | `openai-http.ts` | Phase 2 延后 | 8h |
| 18 | `server-methods/` 剩余 30+ 文件 | Phase 3 延后 | 40h |
| 19 | `utils/` 剩余工具函数 | 按需补充 | 8h |

### 6.4 总工作量估算

| 优先级 | 项数 | 预估总工时 |
|--------|------|-----------|
| P0 | 6 | ~23h |
| P1 | 7 | ~29h |
| P2 | 6 | ~104h |
| **总计** | **19** | **~156h** |

---

## 七、结论

### Phase 1-3 完成度评估

| Phase | 声称完成度 | 实际完成度 | 差距 |
|-------|-----------|-----------|------|
| Phase 1 (配置和类型) | 100% | **~55%** | 缺少 schema/validation/sessions/plugin-auto-enable |
| Phase 2 (网关基础层) | 100% | **~25%** | 缺少 HTTP 路由、server-methods、protocol |
| Phase 3 (网关业务层) | 100% | **~15%** | 仅 server-chat.ts 完成，其余均缺失 |

> [!IMPORTANT]
> **核心结论**: Phase 1-3 的实际完成度远低于预期。Phase 1 的类型系统和配置加载核心管道已完成（这是最重要的基础），但 Phase 2-3 的网关层仅完成了骨架。在推进 Phase 4 之前，需优先补全 P0 级别的 6 项缺失，以确保后续 Phase 不会因基础缺陷而阻塞。
