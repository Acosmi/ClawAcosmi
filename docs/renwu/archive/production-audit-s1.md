# S1 审计：Phase 1-3（config / infra / gateway 核心层）

> 审计日期：2026-02-18 | 方法：逐文件 TS↔Go 对照

---

## Phase 1: 类型系统与配置

### 1.1 TS config/ 文件规模

| 分类 | TS 文件数 | TS 行数 | Go 对应 |
|------|-----------|---------|---------|
| schema/zod 验证 | 10 | ~3,700 | schema.go + validator.go + schema_hints_data.go |
| types.*.ts 类型 | 25 | ~3,000 | pkg/types/ 30 文件 3,080L ✅ |
| io/loader | 1 | 616 | loader.go 694L ✅ |
| defaults | 1 | 470 | defaults.go 473L ✅ |
| legacy 迁移 | 6 | ~1,000 | legacy*.go ~1,274L ✅ |
| paths/env | 4 | ~430 | paths.go+envsubst+configpath+shellenv ✅ |
| 其他辅助 | ~40 | ~4,113 | 见下方逐文件对照 |
| **config/sessions/** | **9** | **1,485** | **sessions/ 7 文件 1,672L ✅** |
| **合计** | **~96** | **~14,814** | **~11,928L (80%)** |

### 1.2 关键文件对照

| TS 文件 | 行数 | Go 文件 | 行数 | 状态 |
|---------|------|---------|------|------|
| schema.ts | 1,114 | schema.go (298) + schema_hints_data.go (618) | 916 | ✅ |
| zod-schema.ts | 629 | validator.go (330) | 330 | ⚠️ 部分 |
| zod-schema.core.ts | 511 | 内联 validator.go | — | ⚠️ |
| zod-schema.providers-core.ts | 838 | 内联 validator.go | — | ⚠️ |
| zod-schema.agent-runtime.ts | 573 | 内联 validator.go | — | ⚠️ |
| io.ts | 616 | loader.go | 694 | ✅ |
| defaults.ts | 470 | defaults.go | 473 | ✅ |
| validation.ts | 361 | validator.go 内含 | — | ⚠️ |
| plugin-auto-enable.ts | 455 | plugin_auto_enable.go | 528 | ✅ |
| group-policy.ts | 213 | grouppolicy.go | 484 | ✅ |
| redact-snapshot.ts | 168 | redact.go | 270 | ✅ |
| includes.ts | 249 | includes.go | 279 | ✅ |
| paths.ts | 274 | paths.go | 249 | ✅ |

### 1.3 Phase 1 评估

**真实完成度：~85%**

- ✅ 类型系统 (pkg/types/)：TS 25 type 文件 → Go 30 文件，完全覆盖
- ✅ 配置加载/默认值/迁移：对齐
- ⚠️ **Zod Schema 验证**：TS 有 ~2,551L 验证规则，Go validator.go 仅 330L
  - **已在 Phase 12 W7 完成深度约束验证+语义验证**（见 deferred-clearance-task.md）
  - 当前差异主要为验证规则的密度差（TS Zod 代码冗长 vs Go 简洁）
- ✅ sessions/：完全覆盖

> **结论**：Phase 1 无新增缺失项。Zod 验证差异属预期（Go 实现更紧凑）。

---

## Phase 2: 基础设施层

### 2.1 规模对比

| 维度 | TS | Go | 比率 |
|------|----|----|------|
| infra/ 根文件 | 94 (17,485L) | 10 (2,344L) | 13% |
| infra/outbound/ | 19 (4,851L) | outbound/ 4 (1,454L) | 30% |
| **合计** | **~22,336L** | **~3,798L** | **17%** |

### 2.2 关键缺失（❌ Go 中未找到）

| TS 文件 | 行数 | 功能 | 状态 |
|---------|------|------|------|
| session-cost-usage.ts | 1,092 | 会话成本/用量追踪 | ❌ |
| state-migrations.ts | 970 | 跨版本状态迁移 | ❌ |
| provider-usage.*.ts | ~7 文件 ~900L | Provider 用量 API | ❌ |
| update-runner.ts | 912 | 自动更新运行器 | ❌ |
| device-pairing.ts | 558 | 设备配对 | ✅ gateway/ |
| exec-approval-forwarder.ts | 352 | 审批转发 | ⚠️ 部分 |
| ssh-tunnel.ts | 213 | SSH 隧道 | ❌ |
| skills-remote.ts | 361 | 远程技能 | ❌ |
| node-pairing.ts | 336 | 节点配对 | ❌ |
| control-ui-assets.ts | 274 | 控制 UI 资源 | ⚠️ |

### 2.3 已覆盖文件

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| exec-approvals.ts (1,541L) | exec_approvals.go (230L) | ⚠️ 15% |
| heartbeat-runner.ts (1,030L) | heartbeat.go (356L) | ⚠️ 35% |
| bonjour-discovery.ts (603L) | discovery.go (352L) | ✅ 58% |
| ports*.ts (~480L) | ports.go (449L) | ✅ |
| system-events.ts (109L) | system_events.go (165L) | ✅ |
| agent-events.ts (83L) | agent_events.go (154L) | ✅ |

### 2.4 Phase 2 隐藏依赖

- **exec-approvals.ts** → `exec-approval-forwarder.ts` → gateway 双通道
- **heartbeat-runner.ts** → outbound/deliver.ts → 频道投递
- **session-cost-usage.ts** → 7 个 provider-usage 适配器 → 外部 API
- **state-migrations.ts** → 全局文件系统操作

### 2.5 Phase 2 评估

**真实完成度：~30%**

> [!WARNING]
> infra/ 是差距最大的模块之一。但需注意：
>
> 1. 部分功能（如 update/ssh/node-pairing）可能非 MVP 必须
> 2. session-cost-usage + provider-usage **直接影响用量计费**
> 3. state-migrations **影响版本升级兼容性**

---

## Phase 3: 网关核心层

### 3.1 规模对比

| 维度 | TS | Go | 比率 |
|------|----|----|------|
| gateway/ 根 | 73 (14,660L) | 55 (17,534L) | ✅ 120% |
| server-methods/ | 30 (7,368L) | 内含 server_methods_*.go | ✅ |
| protocol/ | 20 (2,800L) | protocol.go (270L) | ⚠️ 10% |
| server/ | 10 (1,629L) | 内含 gateway/ | ✅ |

### 3.2 server-methods 对照

| TS 方法文件 | 行数 | Go 对应 | 状态 |
|------------|------|---------|------|
| usage.ts | 822 | server_methods_usage.go (807L) | ✅ |
| chat.ts | 694 | server_methods_chat.go (509L) | ✅ |
| sessions.ts | 482 | server_methods_sessions.go (701L) | ✅ |
| config.ts | 460 | server_methods_config.go (285L) | ⚠️ |
| send.ts | 364 | server_methods_send.go (227L) | ⚠️ |
| agents.ts | 507 | server_methods_agents.go (576L) | ✅ |
| agent.ts | 515 | server_methods_agent_rpc.go (316L) | ⚠️ |
| exec-approvals.ts | 242 | server_methods_exec_approvals.go (232L) | ✅ |
| nodes.ts | 537 | **stubs** | ❌ |
| skills.ts | 216 | **stubs** | ❌ |
| devices.ts | 190 | **stubs** | ❌ |
| cron.ts | 227 | **stubs** | ❌ |
| tts.ts | 157 | **stubs** | ❌ |
| browser.ts | 277 | **stubs** | ❌ |
| wizard.ts | 139 | wizard_*.go ✅ | ✅ |

### 3.3 Phase 3 评估

**真实完成度：~70%**

- ✅ 核心方法（chat/sessions/agents/usage）完整
- ✅ WS 服务器 + 认证 + 会话管理 完整
- ❌ **~40 个 stub 方法**（nodes/skills/devices/cron/tts/browser）
- ⚠️ protocol/ 20 文件 → 仅 protocol.go 270L（信息大量内联）

---

## S1 汇总

| Phase | TS 行数 | Go 行数 | 真实完成度 |
|-------|---------|---------|-----------|
| Phase 1 (config) | ~14,814 | ~11,928 | **~85%** |
| Phase 2 (infra) | ~22,336 | ~3,798 | **~30%** |
| Phase 3 (gateway) | ~26,457 | ~17,534 | **~70%** |

> **S1 结论**：Phase 1 健康。Phase 3 核心完成但有 ~40 stub。
> **Phase 2 是最大风险** — session-cost-usage/state-migrations/provider-usage 完全缺失。
