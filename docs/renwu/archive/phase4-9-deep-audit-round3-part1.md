# 第三轮深度审计报告 — Part 1: 遗漏的子系统

> 基于第二轮审计报告，对 `src/` 全部 50 个子目录逐文件排查后发现的补充遗漏。

---

## 一、`src/infra/` 遗漏子系统（155 文件，前两轮仅提及 3 处）

### 1.1 出站消息管线 `infra/outbound/` — 31 文件，完全遗漏

> [!CAUTION]
> 这是一个**完整的消息出站管线**，负责所有 Agent 回复从生成到最终投递到频道的全链路。前两轮审计**完全未提及**。

| 文件 | 大小 | 职责 |
|------|------|------|
| `message-action-runner.ts` | **33KB** | 出站消息动作执行器：threading、重试、分片 |
| `outbound-session.ts` | **29KB** | 出站会话管理：投递目标解析、频道选择 |
| `target-resolver.ts` | **14KB** | 投递目标解析器：频道→账户→线程 |
| `deliver.ts` | **12KB** | 投递执行器：频道适配、错误处理 |
| `targets.ts` | **10KB** | 投递目标定义与管理 |
| `message.ts` | **8KB** | 出站消息构建 |
| `outbound-policy.ts` | **6KB** | 出站策略（速率限制、过滤） |
| `outbound-send-service.ts` | **5KB** | 发送服务封装 |
| 其余 23 文件 | ~40KB | 辅助函数、测试、类型 |

**影响**: 这是 Agent 回复→用户 的**唯一通道**。Go 端 `internal/outbound/` 已有 4 文件，但远不足以覆盖此管线。

**建议归入**: Phase 4（与 PI Runner 同步实现）

---

### 1.2 执行审批系统 `infra/exec-approvals.ts` — 41KB

| 文件 | 大小 | 职责 |
|------|------|------|
| `exec-approvals.ts` | **41KB** | 完整审批流：策略匹配、滑动窗口、持久化、超时 |
| `exec-approval-forwarder.ts` | **10KB** | 审批请求转发到远程节点 |
| `exec-safety.ts` | **1KB** | 安全检查辅助 |

**隐藏行为**:

- 审批策略支持 glob 匹配、正则、命令前缀
- 滑动窗口 auto-approve（N 次确认后自动放行同类命令）
- 持久化到文件系统（审批记录可恢复）

**影响**: Bash 工具执行（Phase 4.9）**强依赖**此系统。第二轮审计仅在依赖链 4.12 中提到 `sandbox/`，但未识别 `exec-approvals` 本身。

---

### 1.3 心跳系统 `infra/heartbeat-runner.ts` — 33KB

**职责**:

- 定时心跳检测（可配置间隔）
- 心跳消息格式化与投递
- ACK 最大字符数限制
- 投递目标优先级选择（优先 delivery-target）
- 可见性控制（静默/可见/条件触发）

**被谁依赖**: `gateway/server.impl.ts`、`cron/`

**影响**: 心跳是服务存活检测的核心机制，Go 端需等价的 goroutine + ticker 实现。

---

### 1.4 会话成本/用量系统 `infra/session-cost-usage.ts` — 33KB

**职责**:

- 按会话追踪 Token 用量和费用
- 按 provider 分类统计
- 费用格式化与报告
- 用量阈值告警

**被谁依赖**: `gateway/server-methods/usage.ts` (26KB)、PI Runner

---

### 1.5 状态迁移系统 `infra/state-migrations.ts` — 29KB

**职责**:

- 跨版本状态文件迁移（类似数据库 migration）
- 多步迁移链（按版本号排序执行）
- 回滚支持

**影响**: 每次版本升级时自动执行，Go 端必须实现等价逻辑。

---

### 1.6 Provider 用量子系统 `infra/provider-usage.*` — 10+ 文件，~65KB

| 文件 | 大小 | 职责 |
|------|------|------|
| `provider-usage.fetch.antigravity.ts` | 9KB | Antigravity API 用量获取 |
| `provider-usage.fetch.minimax.ts` | 9KB | MiniMax API 用量获取 |
| `provider-usage.auth.ts` | 7KB | 用量 API 认证 |
| `provider-usage.fetch.claude.ts` | 5KB | Claude API 用量获取 |
| `provider-usage.format.ts` | 4KB | 用量格式化 |
| `provider-usage.load.ts` | 3KB | 用量数据加载 |
| 其余 4 文件 | ~8KB | Codex/Copilot/Gemini/ZAI |

**影响**: 每个 LLM provider 都需要独立的用量获取逻辑，Go 端需等价的 HTTP 客户端。

---

### 1.7 网络发现子系统 — Bonjour + Tailscale，32KB

| 文件 | 大小 | 职责 |
|------|------|------|
| `bonjour-discovery.ts` | **16KB** | mDNS/Bonjour 服务发现 |
| `bonjour.ts` | 8KB | Bonjour 服务注册 |
| `tailscale.ts` | **15KB** | Tailscale VPN 集成 |
| `widearea-dns.ts` | 6KB | 广域 DNS 解析 |

**影响**: 多节点服务发现和网络穿透的核心。Go 替代: `github.com/hashicorp/mdns` + Tailscale API。

---

### 1.8 设备配对系统 `infra/device-pairing.ts` — 15KB

**职责**:

- 设备配对握手协议
- 配对码生成与验证
- 配对状态持久化

**被谁依赖**: `gateway/`、`pairing/`

---

### 1.9 更新系统 `infra/update-*` — 25KB+

| 文件 | 大小 | 职责 |
|------|------|------|
| `update-runner.ts` | **25KB** | 自动更新执行器 |
| `update-check.ts` | 11KB | 版本检查 |
| `update-global.ts` | 4KB | 全局更新 |
| `update-startup.ts` | 3KB | 启动时更新检查 |
| `update-channels.ts` | 2KB | 更新频道 |

---

## 二、`src/config/` 遗漏子系统

### 2.1 Zod Schema 验证子系统 — 12 文件，106KB 总量

> [!WARNING]
> `config/` 的 Zod schemas 是**运行时配置验证**的核心。第二轮审计未提及此子系统。

| 文件 | 大小 | 职责 |
|------|------|------|
| `zod-schema.providers-core.ts` | **30KB** | Provider 配置验证 |
| `zod-schema.ts` | **19KB** | 主 schema 入口 |
| `zod-schema.agent-runtime.ts` | **17KB** | Agent 运行时配置验证 |
| `zod-schema.core.ts` | **16KB** | 核心配置验证 |
| `zod-schema.agent-defaults.ts` | 6KB | Agent 默认值验证 |
| 其余 7 文件 | ~18KB | 各模块 schema |

**Go 端等价**: 使用 `github.com/go-playground/validator` 或自定义验证逻辑。

### 2.2 `config/schema.ts` — 55KB（单文件最大之一）

此文件定义了**完整的 OpenAcosmi 配置类型系统**。第二轮审计的"文件大小低估"清单中**未包含此文件**。

**Go 端**: 已在 `pkg/types/` 部分覆盖，但需对照 55KB 原文逐字段验证完整性。

### 2.3 Legacy 迁移子系统 — 7 文件，43KB

| 文件 | 大小 |
|------|------|
| `legacy.migrations.part-2.ts` | 14KB |
| `legacy.migrations.part-1.ts` | 12KB |
| `legacy.migrations.part-3.ts` | 6KB |
| `legacy.rules.ts` | 4KB |
| `legacy.shared.ts` | 3KB |

**影响**: 旧版配置格式自动迁移。Go 端若需支持已有用户数据，必须实现。

---

## 三、`src/gateway/` 遗漏子系统

### 3.1 Gateway Server Methods — 41 文件，270KB+

> [!CAUTION]
> `gateway/server-methods/` 是**网关 RPC 方法的完整实现层**，前两轮审计完全未提及。

**Top 10 大文件**:

| 文件 | 大小 | 职责 |
|------|------|------|
| `usage.ts` | **26KB** | 用量查询/统计方法 |
| `chat.ts` | **22KB** | 聊天消息处理方法 |
| `agent.ts` | **17KB** | Agent 管理方法 |
| `nodes.ts` | **17KB** | 节点管理方法 |
| `sessions.ts` | **15KB** | 会话管理方法 |
| `config.ts` | 13KB | 配置管理方法 |
| `channels.ts` | 10KB | 频道管理方法 |
| `send.ts` | 11KB | 消息发送方法 |
| `exec-approvals.ts` | 6KB | 审批管理方法 |
| `cron.ts` | 6KB | 定时任务方法 |

**影响**: Go 端 `internal/gateway/` 已有 41 文件，但需逐一对照 TS 的 server-methods 确认方法完整性。

### 3.2 `server.impl.ts` — 21KB（Gateway 核心）

Gateway 服务器实现入口，组装所有子模块。

### 3.3 `openresponses-http.ts` — 27KB

OpenAI Responses API 兼容层。

### 3.4 `session-utils.ts` — 22KB + `session-utils.fs.ts` — 12KB

会话工具函数，Go 端已部分迁移。

---

## 四、修订后的遗漏模块总数

| 审计轮次 | 遗漏模块数 | 新增 |
|----------|-----------|------|
| 第一轮 | 15 个 | — |
| 第二轮 | 19 个 (+4) | node-host, canvas-host, plugin-sdk, macos |
| **第三轮** | **28 个 (+9)** | 见下表 |

**第三轮新增遗漏模块**:

| # | 模块 | 文件数 | 总量 | 建议 Phase |
|---|------|--------|------|-----------|
| 20 | `infra/outbound/` | 31 | ~160KB | **4** |
| 21 | `infra/exec-approvals` | 3 | ~52KB | **4** |
| 22 | `infra/heartbeat-*` | 5 | ~40KB | **4** |
| 23 | `infra/session-cost-usage` | 1 | 33KB | **4** |
| 24 | `infra/state-migrations` | 1 | 29KB | **4** |
| 25 | `infra/provider-usage.*` | 10+ | ~65KB | **4** |
| 26 | `infra/bonjour+tailscale+dns` | 5 | ~47KB | **6** |
| 27 | `config/zod-schema.*` | 12 | 106KB | **4** |
| 28 | `gateway/server-methods/` | 41 | ~270KB | **4** |

---

> **Part 2 将覆盖：隐藏依赖链补充 + agents/ 深层文件分析**
