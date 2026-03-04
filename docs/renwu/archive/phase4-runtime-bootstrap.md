# Phase 4 Agent Runtime 补全 — Bootstrap 上下文

> **目的**：在新会话中完成 Phase 4 Agent 运行时层的 Go 移植，以解锁 Phase 6 Cron C2（isolated-agent runner）。

## 1. 问题背景

Phase 6 Cron 服务层（C1）已完成（17 Go 文件），但 C2（`src/cron/isolated-agent/run.ts`，597L）依赖 Phase 4 的 Agent 运行时函数。这些函数在 Go 侧尚未实现。

C1 已通过依赖注入预留了接入点：

```go
// internal/cron/service_state.go
type CronServiceDeps struct {
    RunIsolatedAgentJob func(params IsolatedAgentJobParams) (*IsolatedAgentJobResult, error)
}
```

## 2. 缺失函数清单

| 函数 | TS 文件 | 行数 | Go 预期位置 |
|------|---------|------|-------------|
| `runEmbeddedPiAgent` | `agents/pi-embedded-runner/run.ts` | ~800L | `internal/agents/runner/` |
| `runCliAgent` | `agents/cli-runner.ts` | 363L | `internal/agents/exec/` |
| `ensureAgentWorkspace` | `agents/workspace.ts` L127-200 | ~73L | `internal/agents/workspace/` (文件已存在，缺此函数) |
| `runSubagentAnnounceFlow` | `agents/subagent-announce.ts` L367-572 | ~205L | `internal/agents/runner/` |
| `deliverOutboundPayloads` | `infra/outbound/deliver.ts` L179-375 | ~196L | `internal/infra/outbound/` |

### 深层依赖：`pi-embedded-runner/` 子系统（24 文件）

```
src/agents/pi-embedded-runner/
├── run.ts              ← runEmbeddedPiAgent 的真正实现
├── runs.ts             ← 运行状态管理
├── types.ts            ← EmbeddedPiRunResult 等类型
├── session-manager-init.ts
├── session-manager-cache.ts
├── system-prompt.ts
├── model.ts
├── history.ts
├── compact.ts
├── extra-params.ts
├── extensions.ts
├── tool-split.ts
├── tool-result-truncation.ts
├── sandbox-info.ts
├── lanes.ts
├── logger.ts
├── cache-ttl.ts
├── google.ts
├── abort.ts
└── utils.ts
```

## 3. 已完成的 Phase 4 Go 代码

| Go 包 | 内容 | 状态 |
|--------|------|------|
| `agents/scope/` | `ResolveAgentConfig`, `ResolveAgentDir`, `ResolveAgentWorkspaceDir`, `ResolveDefaultAgentId` | ✅ |
| `agents/models/` | `ResolveConfiguredModelRef`, `RunWithModelFallback`, `LoadModelCatalog`, `ResolveAllowedModelRef` | ✅ |
| `agents/datetime/` | 时间格式化 | ✅ |
| `agents/workspace/` | 引导文件加载、工作区解析（缺 `EnsureAgentWorkspace`） | ⚠️ |
| `agents/transcript/` | 会话转录 | ✅ |
| `agents/helpers/` | 辅助函数 | ✅ |
| `agents/runner/` | **空目录** | ❌ |
| `agents/exec/` | **空目录** | ❌ |
| `agents/stream/` | **空目录** | ❌ |

## 4. `run.ts` (C2) 调用链分析

```
runCronIsolatedAgentTurn (cron/isolated-agent/run.ts)
├── resolveAgentConfig        ✅ scope/
├── resolveAgentDir           ✅ scope/
├── resolveAgentWorkspaceDir  ✅ scope/
├── ensureAgentWorkspace      ❌ workspace/ (缺此函数)
├── resolveConfiguredModelRef ✅ models/
├── loadModelCatalog          ✅ models/
├── resolveAllowedModelRef    ✅ models/
├── resolveAgentTimeoutMs     ✅ scope/
├── resolveThinkingDefault    ✅ models/
├── runWithModelFallback      ✅ models/
│   ├── runCliAgent           ❌ exec/ (空)
│   └── runEmbeddedPiAgent    ❌ runner/ (空)
├── resolveCronDeliveryPlan   ✅ cron/ (C1 已实现)
├── resolveDeliveryTarget     ❌ (cron/isolated-agent/delivery-target.ts)
├── runSubagentAnnounceFlow   ❌ (agents/subagent-announce.ts)
└── deliverOutboundPayloads   ❌ (infra/outbound/deliver.ts)
```

标记 ✅ = Go 已实现，❌ = 需补全

## 5. 推荐实现顺序

### Step 1: `EnsureAgentWorkspace`（最小依赖）

- 在 `internal/agents/workspace/workspace.go` 中添加
- 功能：目录创建 + 引导文件写入 + optional git init
- TS: `workspace.ts` L127-200

### Step 2: Agent Runner 框架

- 创建 `internal/agents/runner/types.go` — `EmbeddedPiRunResult` 等类型
- 创建 `internal/agents/runner/run.go` — `runEmbeddedPiAgent` stub/实现
- 这是最大的工作量（800L+ TS，依赖 Pi AI SDK）

### Step 3: CLI Runner

- 创建 `internal/agents/exec/cli_runner.go`
- TS: `cli-runner.ts` (363L)

### Step 4: Outbound Delivery

- 创建 `internal/infra/outbound/deliver.go`
- TS: `deliver.ts` (376L)，依赖各渠道 SDK

### Step 5: Subagent Announce

- 创建 `internal/agents/runner/subagent_announce.go`
- TS: `subagent-announce.ts` L367-572

### Step 6: Cron C2

- 完成上述后，创建 `internal/cron/isolated_agent.go`
- 实现 `RunIsolatedAgentJob` 并注入 `CronServiceDeps`

## 6. 关键文件引用

```
# TS 源码
src/agents/pi-embedded-runner/run.ts       # runEmbeddedPiAgent 主逻辑
src/agents/pi-embedded-runner/types.ts     # 结果类型定义
src/agents/cli-runner.ts                   # CLI agent 运行
src/agents/workspace.ts                    # 工作区管理
src/agents/subagent-announce.ts            # 子 agent 广播
src/infra/outbound/deliver.ts              # 出站投递
src/cron/isolated-agent/run.ts             # C2 主逻辑 (597L)
src/cron/isolated-agent/session.ts         # cron 会话解析
src/cron/isolated-agent/delivery-target.ts # 投递目标解析
src/cron/isolated-agent/helpers.ts         # 辅助函数

# Go 已完成代码
backend/internal/agents/scope/scope.go
backend/internal/agents/models/selection.go
backend/internal/agents/models/fallback.go
backend/internal/agents/workspace/workspace.go
backend/internal/cron/service_state.go     # C2 接入点
```

## 7. 工作流

请遵循 `/refactor` 工作流 — 六步循环法：Extract → Dependency Graph → Hidden Dependency Audit → Analyze → Rewrite → Verify。
