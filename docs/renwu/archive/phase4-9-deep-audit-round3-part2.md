# 第三轮深度审计报告 — Part 2: 隐藏依赖链补充 + agents/ 深层分析

---

## 五、新增隐藏依赖链（+5 条，累计 20 条）

### 4.16 出站消息管线 → channels + agents 三方耦合

```
infra/outbound/message-action-runner.ts (33KB)
  → infra/outbound/outbound-session.ts (29KB, 投递会话)
  → infra/outbound/target-resolver.ts (14KB, 目标解析)
  → infra/outbound/deliver.ts (12KB, 投递执行)
  → channels/registry.ts (频道查找)
  → agents/pi-embedded-runner/ (Agent 回调)
```

**影响**: 出站管线是 Agent 输出到用户的**唯一桥梁**。它同时依赖 channels 和 agents，形成 agents→outbound→channels 的**单向链**（非循环）。Go 端需在 Phase 4 中实现此管线，置于 `internal/outbound/`。

### 4.17 执行审批 → gateway + node-host 双通道

```
infra/exec-approvals.ts (41KB)
  → infra/exec-approval-forwarder.ts (10KB)
    → gateway/server-methods/exec-approval.ts (4KB)
    → node-host/runner.ts (审批快照)
  → agents/bash-tools.exec.ts (审批检查)
  → agents/sandbox/*.ts (沙箱审批)
```

**影响**: 审批系统同时服务于 **本地执行** 和 **远程节点执行**。第二轮审计的 bash-tools 链路（4.12）遗漏了 `exec-approvals.ts` 本身。

### 4.18 心跳 → outbound + session 联动

```
infra/heartbeat-runner.ts (33KB)
  → infra/outbound/deliver.ts (投递心跳消息)
  → infra/heartbeat-visibility.ts (可见性判断)
  → infra/heartbeat-events.ts (事件总线)
  → config/sessions/ (会话状态查询)
```

**影响**: 心跳不只是 ping/pong，而是通过 outbound 管线**投递格式化消息**到指定频道。

### 4.19 会话成本 → provider-usage → 外部 API

```
infra/session-cost-usage.ts (33KB)
  → infra/provider-usage.fetch.*.ts (7 个 provider 适配器)
  → infra/provider-usage.auth.ts (API 认证)
  → infra/provider-usage.format.ts (格式化)
  → gateway/server-methods/usage.ts (26KB, 查询接口)
```

**影响**: 成本追踪系统依赖 7 个独立的 provider API 适配器。Go 端需为每个 provider 编写 HTTP 客户端。

### 4.20 配置验证 → zod-schema → 运行时类型守卫

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

## 六、agents/ 深层文件分析

### 6.1 第二轮审计低估的 pi-embedded-runner/ 子文件

| 文件 | 大小 | 第二轮提及? | 职责 |
|------|------|-----------|------|
| `run.ts` | **34KB** | ✅ | 主运行循环 |
| `compact.ts` | **18KB** | ✅ | 上下文压缩 |
| `google.ts` | **12KB** | ✅ | Google 模型处理 |
| `tool-result-truncation.ts` | **11KB** | ✅ | 工具结果截断 |
| `model.ts` | **7KB** | ✅ | 模型解析 |
| `extra-params.ts` | **4KB** | ✅ | 额外参数 |
| `runs.ts` | **4KB** | ❌ | 多次运行管理 |
| `system-prompt.ts` | **3KB** | ❌ | Runner 级系统提示词 |
| `history.ts` | **2KB** | ❌ | 历史管理 |
| `session-manager-init.ts` | **1.9KB** | ❌ | 会话管理器初始化 |
| `session-manager-cache.ts` | **1.8KB** | ❌ | 会话缓存 |
| `sandbox-info.ts` | **1KB** | ❌ | 沙箱信息 |
| `extensions.ts` | **3.7KB** | ✅ | 扩展点 |

**遗漏 6 个文件**，总量 ~14KB。虽然单个文件不大，但 `runs.ts` 管理多次运行的**重试/重入逻辑**，`session-manager-*` 管理**会话缓存生命周期**。

### 6.2 第二轮未提及的 agents/ 重要文件

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

**合计 ~177KB 未被第二轮审计明确列出的文件**。

### 6.3 agents/tools/ — 60 文件，499KB

第二轮审计提到了 `tools/` 但仅列出了频道相关的 5 个文件。实际 60 个文件中还包含：

- `web-fetch.ts` / `web-search.ts` — Web 工具
- `file-tools.ts` / `read-file.ts` / `write-file.ts` — 文件操作工具
- `canvas-actions.ts` — Canvas 工具
- `camera-tools.ts` — 摄像头工具
- `session-status.ts` — 会话状态工具
- `subagent-spawn.ts` — 子 Agent 工具
- 等 50+ 个其他工具文件

---

## 七、修订后的文件大小严重低估清单（+8 个）

| 文件 | 实际大小 | 来源 |
|------|----------|------|
| `config/schema.ts` | **55KB** | 第三轮发现 |
| `infra/exec-approvals.ts` | **41KB** | 第三轮发现 |
| `infra/outbound/message-action-runner.ts` | **33KB** | 第三轮发现 |
| `infra/heartbeat-runner.ts` | **33KB** | 第三轮发现 |
| `infra/session-cost-usage.ts` | **33KB** | 第三轮发现 |
| `config/zod-schema.providers-core.ts` | **30KB** | 第三轮发现 |
| `infra/outbound/outbound-session.ts` | **29KB** | 第三轮发现 |
| `infra/state-migrations.ts` | **29KB** | 第三轮发现 |

---

> **Part 3 将覆盖：gateway/ 协议层分析、根级文件隐式行为补充、SKILL.md 更新**
