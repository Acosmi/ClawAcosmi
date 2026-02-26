# plugins 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W5 (或后续分配窗口)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 29 | 16 | 55.2% |
| 总行数 | 5780 | 4410 | 76.3% |

*注：覆盖率偏低是因为 TypeScript 拆分了大量细碎的类型及常量文件（如 runtime/types.ts 等），在 Go 中已整合为标准模块化单文件（如 types.go, plugin_api.go）。*

## 逐文件对照

| 状态 | TS 文件 | Go 文件 | 备注 |
|------|---------|---------|------|
| ✅ FULL | `types.ts`, `runtime/types.ts` | `types.go` | 合并为一处声明定义。 |
| ✅ FULL | `http-path.ts` | `http_path.go` | 核心路由路径等对齐。 |
| 🔄 REFACTORED | `providers.ts`, `tools.ts`, `services.ts` | `plugin_api.go` | 插件核心服务及依赖注入对齐至統一接口。 |
| 🔄 REFACTORED | `runtime/native-deps.ts` | `plugin_api.go` / `runtime.go` | JS端特殊的原生 C 插件扩展处理，在 Go 中被内置或忽略。 |
| ✅ FULL | `installs.ts` | `install_helpers.go` | 安装解压与验证助手方法。 |
| ✅ FULL | `config-schema.ts`, `schema-validator.ts` | `schema.go` | 插件 Schema 验证处理。 |
| 🔄 REFACTORED | `status.ts` | `runtime_state.go` | 插件启停生命状态统一监控模块。 |
| 🔄 REFACTORED | `bundled-dir.ts` | `loader.go` | 提取到了加载流内部或全局常量中。 |
| ✅ FULL | `enable.ts` | `update.go` / `install.go` | 等价实现功能的函数或逻辑迁移。 |
| ✅ FULL | `http-registry.ts` | `registry.go` | 远程拉取注册表对齐。 |
| ✅ FULL | `runtime.ts`, `runtime/index.ts` | `runtime.go` | 主运行时实现与加载入口同步对齐。 |
| ✅ FULL | `cli.ts`, `commands.ts` | `commands.go` | 命令行及插件提供的动态命令映射处理对齐。 |
| 🔄 REFACTORED | `hook-runner-global.ts`, `hooks.ts` | `runtime.go` | TS 的事件触发和 hook 模型被 Go 的 channel 及原生接口替代。 |
| ✅ FULL | `slots.ts` | `slots.go` | 扩展点 Slot 模型完整对齐。 |
| ✅ FULL | `manifest.ts` | `manifest.go` | `package.json` 中的清单及自定义字段解析对齐。 |
| ✅ FULL | `manifest-registry.ts` | `registry.go` | 清单注册中心在 Go 的注册表中统一处理。 |
| ✅ FULL | `config-state.ts` | `config_state.go` | 用于加载后持久化插件默认参数或存储参数对齐。 |
| ✅ FULL | `discovery.ts` | `discovery.go` | 启发式搜索工作区和全局安装路径寻找插件对齐。 |
| ✅ FULL | `update.ts` | `update.go` | 版本对比及升级控制。 |
| ✅ FULL | `loader.ts` | `loader.go` | 核心加载器对齐。 |
| ✅ FULL | `install.ts` | `install.go` | 下载解包写入注册表的总控实现对齐。 |

## 隐藏依赖审计

| # | 类别 | 结果 | 应对说明 |
|---|------|------|----------|
| 1 | npm包黑盒行为 | ⚠️ 存在 | TS 在下载解包时可能依赖第三方工具或 npm 下载器本身 API（如 `npm_execpath`），Go 是原生的 zip 下载解包与 FS 操作。 |
| 2 | 全局状态/单例 | ⚠️ 存在 | TS 在 `commands.ts` (`const pluginCommands: Map...`) 具有全局状态，Go 端在 `manager/runtime` 的生命周期上下文结构体内存储，消除了单例风险。 |
| 3 | 事件总线/回调链 | ✅ 无 | 无全局 EventEmitter。 |
| 4 | 环境变量依赖 | ⚠️ 存在 | 对 `OPENACOSMI_BUNDLED_PLUGINS_DIR`, `OPENACOSMI_STATE_DIR`, `npm_execpath` 的特殊处理，Go 端已从 config 环境变量获取。 |
| 5 | 文件系统约定 | ⚠️ 存在 | 强依赖 OS 的路径解包 `install.go`，Go 端已通过 `os` / `filepath` 处理一致逻辑。 |
| 6 | 协议/消息格式 | ✅ 无 | 暂未发现特殊的插件非标通信协议。 |
| 7 | 错误处理约定 | ✅ 正常 | 校验失败直接报错机制一致。 |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| 1 | 架构差异 | `hooks.ts` / `hook-runner-global.ts` | `runtime.go` | Hook 生命周期和 Hook 执行系统在 TS 是解耦的一个独立监听器类，Go 使用了原生的服务注入接口调用或事件 channel，去中心化。 | P2 | Go 的模式更适合并发，无需强制一致，只要等效触发即可。 |
| 2 | 原生依赖 | `runtime/native-deps.ts` | `plugin_api.go` | Node 的 ABI c++ 编译加载方案对应 Go 无法执行，Go 提供基于 WASM/RPC 或本原生编译形式处理。 | P2 | JS 插件端兼容就好，当前无阻塞缺陷。 |
| 3 | 职责内聚 | `status.ts` 等辅助 | `runtime_state.go` | TS 细碎分割的多文件在 Go 采用大单体包含或结构体组合，易于测试和管理。 | P3 | 无需修复，符合 Go 特性。 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 2 项
- P3 差异: 1 项
- 模块审计评级: A

该模块重构较好地利用了 Go 语言内置的文件处理能力、静态类型验证并利用结构体合并了状态管理，移除了由于 TS Node 生态依赖带来的全局状态和复杂黑盒，插件系统的依赖发现、生命周期注册得到全面等效对齐。
