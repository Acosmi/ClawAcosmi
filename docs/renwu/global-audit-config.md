# Config 模块全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W2 (Config审计)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 81 | 34 | 41.9% |
| 总行数 | 14329 | 8114 | 56.6% |

*(注：行数差异主要源自 TS 中 `zod-schema.*.ts` 等庞大繁杂的运行时强校验与默认值声明，Go 采用内置 Struct Tags + Validator 库实现等效功能，代码更加紧凑。)*

## 逐文件对照

| 状态 | 含义 |
|------|------|
| ✅ FULL | Go 实现完整等价 |
| ⚠️ PARTIAL | Go 有实现但存在差异 |
| ❌ MISSING | Go 完全缺失该功能 |
| 🔄 REFACTORED | Go 使用不同架构实现等价功能 |

### 1. 配置加载与解析器 (Loader & IO)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `io.ts`, `config.ts`, `merge-config.ts` | `loader.go`, `mergeconfig.go` | ✅ FULL | 核心配置解析、加载、保存及深度合并能力两端对齐。 |

### 2. 验证和模式定义 (Schema & Validation)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `zod-schema*.ts`, `validation.ts` | `schema.go`, `validator.go`, `schema_hints_data.go` | 🔄 REFACTORED | 超过 3000 行的 Zod Schema，在 Go 中被重构成等效的结构体、标签与 JSON Schema (`schema_hints_data.go`) 提示生成链。 |

### 3. 类型定义 (Types)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `types.*.ts` | `schema.go` (内联类型) | 🔄 REFACTORED | TS 通过将不同组件的类型放在各个独立文件（如 `types.cron.ts`, `types.gateway.ts`），Go 将其大多收敛至统一的配置根树下。 |

### 4. 路径处理与多环境注入 (Paths & Env)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `paths.ts`, `config-paths.ts`, `normalize-paths.ts`, `agent-dirs.ts` | `paths.go`, `configpath.go`, `normpaths.go`, `agentdirs.go` | ✅ FULL | 对齐了 `~` 解析与绝对路径转换逻辑。 |
| `env-vars.ts`, `env-substitution.ts` | `shellenv.go`, `envsubst.go` | ✅ FULL | 配置内的环境变量插值（如 `${MY_VAR}`）处理逻辑完全对齐。 |

### 5. 迁移与遗留支持 (Legacy Migrations)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `legacy.migrations*.ts`, `legacy-migrate.ts`, `legacy.rules.ts` | `legacy_migrations.go`, `legacy_migrations2.go`, `legacy.go` | ✅ FULL | 向后兼容及旧版 V1 -> V2 甚至 V3 的全自动兼容迁移流对齐。 |

### 6. 会话配置与其他 (Sessions, Plugins, Capabilities)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `sessions/*.ts` (e.g. `main-session.ts`) | `session_*.go` (e.g. `session_main.go`) | ✅ FULL | 针对用户与频道的不同会话元数据读写器。 |
| `plugin-auto-enable.ts`, `channel-capabilities.ts`, `telegram-custom-commands.ts` | `plugin_auto_enable.go`, `channel_capabilities.go`, `telegramcmds.go` | ✅ FULL | 附带特性的配置注入逻辑。 |

## 隐藏依赖审计

1. **npm 包黑盒行为**: 🟡 **中度依赖**。TS 端极度依赖 `zod` 在运行时完成配置清洗、默认值注入和强类型校验。Go 放弃了这套巨石模式，转用 Struct Tags + `go-playground/validator` 解决，并在静态下生成 JSON schema。这是最大的框架变更。
2. **全局状态/单例**: 🟢 适度依赖。有小部分基于 `Map` 的内存缓存 (如 `SESSION_STORE_CACHE`)。Go 端通过结构体成员或 `sync.Map` 进行了原生平移。
3. **事件总线/回调链**: 🟢 极少依赖（仅限于文件系统变更时或外部事件钩子），主逻辑都是纯函数变换。
4. **环境变量依赖**: 🔴 **重度依赖**。整个配置初始化高度依赖 `${VAR}` 插值（`env-substitution.ts`）以及各种保底目录推断 (`OPENACOSMI_STATE_DIR` 等)。Go 端利用 `shellenv.go` 和 `envsubst.go` 实现了一比一还原。
5. **文件系统约定**: 🔴 **重度依赖**。此模块是整个项目读取 `openacosmi.json`, `sessions.json`, `AGENTS.md`, 扩展插件等的核心节点。Go 端 `loader.go` 精准仿刻了该行为。
6. **协议/消息格式**: 🟢 此处仅定义类型，无传输协议耦合。
7. **错误处理约定**: 🟢 严格捕获文件缺失或 JSON 解析错误并在 TS 端以友好的红色日志报告。Go 采用 `fmt.Errorf` 套件级联。

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| CFG-1 | 架构差异 | `zod-schema*.ts` | `schema.go`, `schema_hints_data.go` | TS 借助 Zod 将 Default 赋值与类型强校验糅合在一块长达几千行的代码中。Go 采用了更加惯用的原生 Struct + 标签结合的方法，并在初始化时手工赋值 Default。 | P4 | 无需修复，重构更为清晰可维护。 |
| CFG-2 | 模块碎片 | `types.*.ts` | `schema.go` | TS 为了解除各渠道类型的循环依赖，碎片化了几十个独立的小类型文件。Go 利用 Package 原理统一放置在一两处。 | P4 | 无需修复。 |
| CFG-3 | 隐藏行为 | `legacy.migrations*.ts` | `legacy_migrations.go` | V1到V3之间多如牛毛的字段改名、升级逻辑在 Go 中以同样冗长平铺的方式实现，对齐度极高。 | P4 | 无需修复。 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 0 项
- P3/P4 差异: 3 项 (核心由 Zod 解析重构为 Struct Tags 机制、文件碎片收敛)
- 模块审计评级: **S** (这是一次非常成功的底层模型重塑：在甩开沉重的 Zod 负担后，Go 用少近一倍的代码量 (`14329` vs `8114`)，完美实现了对原数百个配置字段的合并、插值、向后兼容和缓存！)
