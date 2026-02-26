# 全局审计报告 — Routing 模块

## 概览

| 维度 | TS (`src/routing`) | Go (`backend/internal/routing`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 5 | 2 | **核心功能（路由）未实现** |
| 总行数 | ~646 | ~448 | 仅搬运了 Session Key 逻辑 |

### 架构演进

`routing` 模块是原版 Acosmi 的中枢之一，它控制着不同的会话来源（渠道通道的来源账号或聊天室特征）如何映射（Binding）给对应的不同 Agent，并负责组装出底层唯一性的 `Session Key`（比如给群组一个 Key，给直接私聊一个 Key 等）。

在 Go 重构中：

1. **静态键生成 (`session_key.go`) 完美还原**：该文件（长达 400+ 行）以 `1:1` 的精度还原了 TS 各类 `BuildAgentPeerSessionKey`、`ClassifySessionKeyShape` 逻辑。这是为了配合 Session 存储模块能够顺滑读写内存/数据库而硬性搬运的设施。
2. **动态路由图 (`resolve-route.go`) = 遗失 (Stubbed)**：**完全没有被 Go 移植**。TS 中包含配置驱动（`cfg.agents`）、优先级降级、以及父子线程继承检测（Thread inheritance）的核心功能 `resolveAgentRoute` 消失了。

## 差异清单

### P1 功能缺失：动态路由寻址

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| ROUTE-1 | **`ResolveAgentRoute` 控制面缺失** | 提供 `resolveAgentRoute(input)` 按配置文件进行智能路由绑定分发。 | Go 将此完全阉割。在各个通信通道（如 `whatsapp`, `imessage`, `slack`）的依赖结构体 `deps.go` 中声明了 `ResolveAgentRoute func(...)` 回调充当 **DI Stub (伪注入存根)**，运行到此处时直接判定 `if deps.ResolveAgentRoute == nil { log("DI stub") }` 跳过路由降级。 | **P1（若系统全面脱离 TS 运行则为致命级缺陷）**。由于目前 Go 网关依赖 TS 完成高阶路由计算所以可以工作，但要完成纯血 Go 化，必须将 `resolve-route.ts` 与 `bindings.ts` 用 Go 逻辑复刻并注入到 Channels 依赖中去。 |

## 隐藏依赖审计 (Step D)

执行了文本级别的全面结构探视：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. 环境变量** | `session_key.go` 属于纯粹的无副作用函数，未读取系统变量。 | 安全。 |
| **2. 并发安全** | 作为静态字符串拼接函数，全线纯函数设计，不持有或读写外部 Map，无协程竞态。 | 极度安全。 |
| **3. 第三方包黑盒** | Go `routing` 包目前处于 **零外部依赖** 状态。 | 通过查验。 |

## 下一步建议

通过深度比对证实了 `routing` 包是一个**半成品（Partial Port）**。虽然其目前承载的会话主键切分（Session Key String Utils）非常优雅健壮，但 "Router" 身为控制平面被长期外包给旧版 TS 执行始终是架构隐患。目前可以标记为审计完成（它没有 Bug），但需要往未结事项/TODO 面板中补充：「复活 Go 的路由绑定策略分发引擎」。
