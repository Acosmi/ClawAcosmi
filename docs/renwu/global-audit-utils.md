# 全局审计报告 — Utils 模块

## 概览

| 维度 | TS (`src/utils`) | Go (`backend/pkg/utils` 及其他) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 15 | 分散重构为 2 | 领域逻辑已拆包分化 |
| 总行数 | ~821 | ~263 (纯 utils) | 100% 逻辑重组 |

### 架构演进

在原版的 TypeScript 项目中，`src/utils` 作为一个典型的 "全局垃圾桶" (Junk Drawer) 存在。其中不仅包含了基础的字符串/布尔值处理，还大量耦合了特定领域的业务逻辑封装（诸如消息通道 `message-channel.ts`，指令标签 `directive-tags.ts`，队列防抖 `queue-helpers.ts`，会话上下文 `delivery-context.ts` 等）。

在 Go 语言的深度重构中，执行了**严格的领域驱动包划分 (Domain-Driven Package Splitting)**：

1. **纯正通用工具 (`pkg/utils/utils.go`)**：仅仅保留了真正的底层跨模块工具函数，比如十六进制 ID 生成 (`GenerateID`)，数字 Clamp (`ClampNumber`)，安全布尔解析 (`ParseBooleanValue`)，Shell 命令拆分 (`SplitShellArgs`) 等。这些模块彻底实现了**零外部依赖**（仅依赖 Go 标准库）。
2. **业务工具就近内聚**：
   - `delivery-context.ts` → 被吸收融入 `internal/gateway/delivery_context.go`
   - `message-channel.ts` → 被吸收融入 Gateway 的 Channel 定义。
   - `directive-tags.ts` → 被吸收融入 `internal/autoreply/reply` 中的回复生成流。
   - `queue-helpers.ts` → 被吸收融入 `internal/agents/bash` 宿主运行时的消息上报截断管线中。

## 差异清单

### P3 架构分化：解散滥用的 Utils 包

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| UTIL-1 | **包粒度与内聚性** | 全部堆积在 `src/utils` 下，产生不必要的跨模块导入（如 Gateway 引用 Utils 的 `delivery-context`又被 Utils 引用 Config）。 | `pkg/utils` 只允许放置和业务上下文无关的方法（类似标准库的补充）。原有的工具类被送回它所从属的业务包下面并成为内部函数（`internal/...`）。 | **极高价值的架构改进 (P3)**。彻底避免了 Go 中臭名昭著的循环导入问题。无需修复。 |
| UTIL-2 | **Shell 参数拆分 (`splitShellArgs`)** | 手写正则/循环状态机防止注入。 | Go 版 `utils.SplitShellArgs` 在 `pkg/utils/utils.go` 内同样实现了支持单/双引号及转义符防爆破的安全扫描。 | 无需修复。 |

## 隐藏依赖审计 (Step D)

执行了文本级别的全面结构探视：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. 环境变量** | `pkg/utils` 工具箱不再读取任何环境变量（仅限于展开 `~/` 时取 `os.UserHomeDir()`）。 | 绝对纯净，通过。 |
| **2. 并发安全** | 全部纯函数重构，没有任何全局 Map 或 Cache（`boolean.go` 中的 map 为只读的 package-level 常量数据结构）。 | 没有并发修改隐患，全协程安全。 |
| **3. 第三方包黑盒** | Go 的 `pkg/utils` 没有引入哪怕一个三方模块。 | 极简至美，满分。 |

## 下一步建议

这是一个在包结构上非常成功的领域剥离手术。"Utils 应该只包含独立于业务的对象" 这一原则被严格实施。本包不仅没有发生退化，反而清理了原本长满苔藓的历史债务。不需要执行任何修复，审计通过。
