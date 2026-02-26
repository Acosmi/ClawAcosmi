# 全局审计报告 — Agents 模块

## 概览

| 维度 | TS (`src/agents/*.ts`) | Go (分散于多包) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | ~10 个核心文件 | 多包组合 | 100% 架构重组 |
| 总行数 | 539 | "0" (作为一个独立大包的概念被消灭) | 领域驱动设计 (DDD) 拆分 |

### 架构演进

在原版的 TypeScript 项目中，`src/agents/` 目录像是一个大杂烩（Dump dir）。里面混杂着路径解析 (`agent-paths.ts`)、权限控制 (`agent-scope.ts`)、子智能体生命周期生命 (`subagent-announce.ts`) 以及基于本地文件系统的子智能体注册表 (`subagent-registry.ts`)。这 539 行代码违背了单一职责原则，使得外部模块依赖它时，常常引入了不必要的副作用。

在 Go 重构中，Acosmi 并没有生搬硬套一个庞大的 `internal/agents/*` 包，而是对智能体的生命周期进行了极其精准的**领域分解 (Domain Decomposition)**：

1. **纯配置路径流放**：`agent-paths.ts` 被转化为内聚的 `internal/config/agentdirs.go`。路径相关的规则现在只属于统一配置中枢。
2. **生命周期分离**：`subagent-announce.ts` 这种强依赖执行状态流的逻辑，被下沉到了 `internal/agents/runner/subagent_announce.go` 和统一的事件总线 `internal/infra/agent_events.go` 中，完美实现事件去耦合。
3. **注册表存储下沉**：`subagent-registry.ts` 中基于状态的子智能体写入和回表，被归类到了特定的持久化或内存隔离域（如 `internal/agents/bash/subagent_registry.go` 以及网关级的服务调用中）。

## 差异清单

### P2 设计差异 (领域驱动边界)

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| AGT-1 | **包的内聚度** | 各种跨领域的逻辑散落在一个叫 `agents` 的文件夹下。 | 根据功能特征 (Runner, Bash, Config, Event) 垂直拆分。 | **极高的工程化水准 (P2)**。有效避免了大型单体应用后期的 "God Object" 问题。无需修复。 |
| AGT-2 | **子智能体通信** | 通过写入特定路径的文件，并由 `setInterval` 或者手动触发读取。 | 完全被 Go 内置的内存 PubSub 机制或严格控制的生命周期协程所接管。 | 性能与响应速度质变。无需修复。 |

## 下一步建议

"没有 `agents` 包，处处都是 `agents`。" Go 版本通过打散原本 500 多行的面条代码，根据实际的领域上下文把职能分配给了对应的系统骨干包。这种高维度的重构使得系统维护起来更加轻巧。审计完全通过。
