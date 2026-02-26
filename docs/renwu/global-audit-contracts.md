# 全局审计报告 — Contracts 模块

## 概览

| 维度 | TS | Go (`backend/pkg/contracts`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 散落在 `types.ts` 等文件中 | 4 个契约定义文件 | 架构层面 100% 对齐 |
| 总行数 | 0 (未独立建包) | 452 | 依赖反转 (DIP) 原生化 |

### 架构演进

在原版的 TypeScript 中，不存在独立的 `contracts` 模块代码统计量（0 行）。由于 TypeScript 采用的是结构化类型（Duck Typing）和极其宽松的循环导入（ESM 允许一定程度的循环展开），频道（Channels）的核心处理逻辑可以直接引用频道的具体实现，或者简单地 import 一个 interface 就完事。

在 Go 重构中，为了彻底打破核心业务管线（如 `gateway`, `autoreply`，负责调度分发）与具体的频道实现（如 `telegram`, `discord`, `msteams`）之间的**循环依赖死锁 (Import Cycles)**，Go 版专门构建了 `pkg/contracts`。

1. **依赖反转 (Dependency Inversion)**：原本在 `channels` 里定义的 20 多种不同适配器 (Adapters) 的类型声明全被抽离到了 `pkg/contracts/channel_adapters.go`（例如 `ChannelConfigAdapter`, `ChannelOutboundAdapter`）。
2. **纯粹的 Interface 层**：这个包不包含任何具体的业务逻辑实现，只有纯接口 (`interface`) 和核心胶水结构体。`internal/gateway` 依赖 `contracts`，`internal/channels` 各个子包实现 `contracts`。

## 差异清单

### P2 设计差异 (语言级解耦合)

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| CON-1 | **接口隔离原则** | 大量充斥着巨型联合结构体和 `Partial<Plugin>`。 | 被极度细致地拆分为了 `channel_adapters.go`、`channel_plugin.go`、`channel_types.go`。各个 Adapter 都是小粒度的 Go `interface`。 | **架构干净程度极大提升 (P2)**。Go 强迫开发者梳理接口职责，杜绝了 TS 里牵一发而动全身的隐式耦合。无需修复。 |
| CON-2 | **插件契约注册** | 依赖任意属性赋值。 | `ChannelPlugin` 实体通过明确赋值内部被忽略序列化的槽位(`json:"-"`)与外部建立交互墙。 | 无需修复。 |

## 隐藏依赖审计 (Step D)

执行了文本级别的全面结构探视：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. 环境变量** | 核心接口定义文件，无任何运行时环境挂载。 | 纯粹，安全。 |
| **2. 并发安全** | 作为契约类型，不持有任何运行时状态，并发安全性由各具体实现的 Adapter 负责。 | 极度安全。 |
| **3. 第三方包黑盒** | 没有引入任何第三方依赖。 | 优秀的基层设计。 |

## 下一步建议

通过抽出独立的 `contracts` 包，Acosmi 解决了一个 Go 语言中开发大规模插件化系统的历史级难题 — 循环导入。这种基于 Interface 的隔离模式非常成熟。该模块审计通过，安全结案。
