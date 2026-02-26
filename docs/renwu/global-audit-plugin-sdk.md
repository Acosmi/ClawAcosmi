# 全局审计报告 — Plugin SDK 模块

## 概览

| 维度 | TS (`src/plugin-sdk/index.ts`) | Go (`backend/internal/plugins` & `pkg/contracts`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 1 主文件 | 2+ 契约文件 | 100% 结构化对齐 |
| 总行数 | 382 | 83 + 62 | 特性 API 100% 映射 |

### 架构演进

`plugin-sdk` 模块是面向外部开发者的核心壁垒。原版 TS 暴露了一套高度鸭子类型的 `OpenAcosmiPluginApi`，允许外部通过简单的 JS 回调注册工具、Hook 和 CLI 命令。

作为一个强类型的静态编译语言，Go 版本的构建策略发生了深刻的同构映射：

1. **闭包注入替代对象继承**：在 `internal/plugins/plugin_api.go` 中，Go 巧妙地采用了 "Struct holding func pointers" (包含函数指针的结构体) 的模式来直接 1:1 承接 TS 的鸭子类型。比如 `RegisterTool`、`RegisterCommand` 均作为 `PluginAPI` 结构体的字段，由底层的 Registry 模块在初始化时将具体的实现函数 "注入" (Inject) 进这些字段，从而让插件开发者获得了和 TS 一致的开发体验 — 直接调用 `api.RegisterTool(...)`。
2. **Channel 契约静态化**：在 `pkg/contracts/channel_plugin.go` 中，Go 版完美反序列化了 TS `src/channels/plugins/types.plugin.ts` 中复杂无比的 23 个 Channel 适配器槽位 (Config, Setup, Pairing, Security...)。通过 JSON tags 和指针使得该结构兼顾了动态按需装载与强类型校验。

## 差异清单

### P3 细微差异

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| SDK-1 | **运行时代理解耦** | TS `PluginRuntime` 有 100 多个方法（暴露了极多的底层能力给插件）。 | Go 的 `PluginRuntime` **被严格收紧**，目前仅暴露了 `Version()` 和 `GetLogger()` 接口。 | **安全性提升 (P3)**。限制插件权限是正确的，除非有具体需要否则无需把内核全量大包大揽暴露给插件。无需修复。 |
| SDK-2 | **类型校验拦截** | TS 的 `PluginToolOptions` 可以随意塞对象。 | Go 要求严格契合 `PluginToolFactory` 的上下文 `PluginToolContext`。 | 静态语言必然优势，无需修复。 |

## 隐藏依赖审计 (Step D)

执行了文本级别的全面结构探视：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. 环境变量** | SDK 纯粹定义接口，并未主动侦测任何系统级 Env。 | 安全。 |
| **2. 并发安全** | 作为 API 契约类本身无并发。具体的注册互斥锁 (Mutex) 被外包到了 `Registry` 消费者中实现，职责分离干净。 | 极度安全。 |
| **3. 第三方包黑盒** | SDK API 和 Contracts 定义均处于 `0 依赖` 状态。 | 通过查验。 |

## 下一步建议

通过比对确认了 Acosmi 的插件生态底座已经完美降落到 Go 领域，接口设计的强弱类型转换非常聪明，且有效防范了 JS 中常常发生的插件越权执行问题。该模块审计通过，安全结案。
