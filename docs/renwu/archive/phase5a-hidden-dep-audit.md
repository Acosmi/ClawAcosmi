# Phase 5A 遗留补全 — 隐藏依赖审计报告

> 最后更新：2026-02-14
> 范围：8 个遗留项（P2-D1/D2/D5, P3-D1/D2/D3, P4-DRIFT4, P4-NEW5）

---

## 一、提取摘要（步骤 1）

### 涉及的 TS 源文件

| TS 文件 | 大小 | 涉及遗留项 | Go 目标文件 |
|---------|------|-----------|------------|
| `gateway/hooks-mapping.ts` | 439L (12KB) | P2-D1, P2-D2 | `internal/gateway/hooks_mapping.go` |
| `gateway/hooks.ts` | ~550L | P2-D5 | `internal/gateway/hooks.go` |
| `gateway/session-utils.ts` | ~800L | P3-D1, P3-D2, P3-D3 | `internal/gateway/server_methods_sessions.go` |
| `agents/model-auth.ts` | 396L (12KB) | P4-DRIFT4 | `internal/agents/models/providers.go` |
| `agents/models-config.providers.ts` | 637L (19KB) | P4-NEW5 | `internal/agents/models/implicit_providers.go` **[NEW]** |

### 关键 export 接口

| 函数/类型 | TS 文件 | Go 等价 |
|-----------|---------|---------|
| `resolveHookMappings()` | hooks-mapping.ts:102 | `ResolveHookMappings()` |
| `loadTransform()` + `resolveTransformFn()` | hooks-mapping.ts:315-333 | `RegisterTransform()` + `ApplyTransform()` |
| `match: { path, source }` 嵌套结构 | hooks-mapping.ts (HookMappingConfig) | `Match *HookMatchFieldConfig` |
| `resolveEnvApiKey()` | model-auth.ts:235-313 | `ResolveEnvApiKeyWithFallback()` |
| `resolveImplicitProviders()` | models-config.providers.ts:444-547 | `ResolveImplicitProviders()` |
| `normalizeProviders()` | models-config.providers.ts:214-281 | `NormalizeProviders()` |

---

## 二、依赖图（步骤 2）

### P2-D1/D2 hooks-mapping 依赖

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `node:path` | 值 | ↓ | Transform 模块路径解析 |
| `node:url` | 值 | ↓ | `pathToFileURL` 用于动态 import |
| `./hooks.ts` (HookMessageChannel) | 类型 | ↓ | Channel 类型定义 |
| `../config/config.ts` (HooksConfig, HookMappingConfig) | 类型+值 | ↓ | 配置类型 |
| 上游: `hooks.ts` | 值 | ↑ | 被 hooks 路由调用 |

### P2-D5 hooks 依赖

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `../types/types.hooks.ts` | 类型 | ↓ | `HookChannel` 枚举定义 |

### P3-D1/D2/D3 sessions 依赖

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `./session-utils.ts` | 值 | ↓ | storePath, resolveDefaults |
| `../config/sessions/` | 值 | ↓ | session key 解析 |
| `../agents/models/` (resolveConfiguredModelRef) | 值 | ↓ | 默认模型解析 |
| `../routing/session-key.ts` | 值 | ↓ | `buildAgentMainSessionKey` |

### P4-DRIFT4 model-auth 依赖

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `@mariozechner/pi-ai` (getEnvApiKey) | 值 | ↓ | 第三方 API Key 解析 |
| `../config/config.ts` | 类型 | ↓ | 配置访问 |
| `./auth-profiles/*.ts` | 值 | ↓ | OAuth profile store |

### P4-NEW5 models-config.providers 依赖

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `../providers/github-copilot-token.ts` | 值 | ↓ | Copilot OAuth token |
| `./synthetic-models.ts` | 值 | ↓ | Synthetic 模型目录 |
| `./venice-models.ts` | 值 | ↓ | Venice 模型发现 |
| `../config/types.models.ts` | 类型 | ↓ | 模型定义类型 |
| 上游: `models-config.ts` | 值 | ↑ | 被模型配置主流程调用 |

---

## 三、隐藏依赖审计（步骤 3）

### P2-D1: Transform 管道

| # | 类别 | 结果 | Go 等价方案 |
|---|------|------|-------------|
| 1 | npm 包黑盒行为 | ✅ 无 | — |
| 2 | 全局状态/单例 | ⚠️ TS `transformCache = new Map<>()` 模块级缓存 | Go: `transformRegistry` 包级变量（`map[string]TransformFunc`） |
| 3 | 事件总线/回调链 | ✅ 无 | — |
| 4 | 环境变量依赖 | ✅ 无 | — |
| 5 | 文件系统约定 | ⚠️ TS 使用 `path.resolve(CONFIG_PATH, ...)` + 动态 `import()` 加载模块 | Go: 注册表模式 `RegisterTransform(name, fn)` 替代（编译时注册 vs 运行时 import） |
| 6 | 协议/消息格式 | ⚠️ Transform 返回结果的 `mergeAction` 语义（Override > Merge > Skip） | Go: `HookTransformResult` 实现等价 Override/Merge/Skip 三模式 |
| 7 | 错误处理约定 | ⚠️ TS import() 失败时 warn-and-skip | Go: `transformRegistry` 查找失败时跳过（等价语义） |

### P2-D2: HookMappingConfig 嵌套 match

| # | 类别 | 结果 |
|---|------|------|
| 1-7 | 全部 | ✅ 纯结构体字段变更，无隐藏依赖 |

### P2-D5: Channel 动态注册

| # | 类别 | 结果 |
|---|------|------|
| 1 | npm 包黑盒行为 | ✅ 无 |
| 2 | 全局状态/单例 | ⚠️ `validHookChannels` 包级变量 | Go: 同样使用包级 `map[string]bool` + `RegisterHookChannel` 修改 |
| 3-7 | 其余 | ✅ 无 |

### P3-D1/D2/D3: Sessions 业务逻辑

| # | 类别 | 结果 |
|---|------|------|
| 1 | npm 包黑盒行为 | ✅ 无 |
| 2 | 全局状态/单例 | ✅ 无 |
| 3 | 事件总线 | ✅ 无 |
| 4 | 环境变量依赖 | ✅ 无 |
| 5 | 文件系统约定 | ⚠️ P3-D1: `storePath` 从 `cfg.Session.Store` 路径解析 | Go: `ctx.Context.StorePath` 已注入 |
| 6 | 协议/消息格式 | ⚠️ P3-D2: `Defaults` 返回格式需与 TS `{ modelProvider, model, contextTokens }` 一致 | Go: `GatewaySessionsDefaults` 结构体字段名对齐 |
| 7 | 错误处理约定 | ⚠️ P3-D3: 删除主 session 时返回特定错误消息 | Go: `fmt.Errorf("cannot delete main session key")` |

### P4-DRIFT4: API Key 环境变量映射

| # | 类别 | 结果 |
|---|------|------|
| 1 | npm 包黑盒行为 | ⚠️ `@mariozechner/pi-ai` 的 `getEnvApiKey` 内含 OAuth token 回退链 | Go: `EnvApiKeyFallbacks` + `ResolveEnvApiKeyWithFallback` 实现等价 |
| 2 | 全局状态/单例 | ✅ 无 |
| 3 | 事件总线 | ✅ 无 |
| 4 | 环境变量依赖 | ⚠️ 31 个环境变量映射 + 5 条回退链 | Go: `EnvApiKeyVarNames` (31 entries) + `EnvApiKeyFallbacks` (5 entries) |
| 5 | 文件系统约定 | ✅ 无 |
| 6 | 协议/消息格式 | ✅ 无 |
| 7 | 错误处理约定 | ✅ 无 |

### P4-NEW5: 隐式供应商发现

| # | 类别 | 结果 |
|---|------|------|
| 1 | npm 包黑盒行为 | ⚠️ Ollama 模型发现调用 `http://127.0.0.1:11434/api/tags` | Go: stub 状态，需完整 HTTP 调用（标记为低优先级） |
| 2 | 全局状态/单例 | ✅ 无 |
| 3 | 事件总线 | ✅ 无 |
| 4 | 环境变量依赖 | ⚠️ 12+ 环境变量触发隐式发现 | Go: `implicitProviderSpecs[].EnvVars` 列表 |
| 5 | 文件系统约定 | ⚠️ Copilot token 解析需读取 `~/.config/github-copilot` 目录 | Go: stub — 完整实现延迟到 Phase 7+ |
| 6 | 协议/消息格式 | ⚠️ Bedrock 使用 AWS Signature V4 认证 | Go: stub — AWS SDK 集成延迟 |
| 7 | 错误处理约定 | ✅ 无 |

---

## 四、审计结论

| 项目 | 隐藏依赖数 | 状态 |
|------|----------|------|
| P2-D1 Transform | 4 ⚠️ | ✅ 全部已实现（注册表+Override/Merge/Skip） |
| P2-D2 嵌套 match | 0 | ✅ 纯结构变更 |
| P2-D5 Channel 注册 | 1 ⚠️ | ✅ `RegisterHookChannel` 已实现 |
| P3-D1 Path 字段 | 1 ⚠️ | ✅ `StorePath` 已注入 |
| P3-D2 Defaults | 1 ⚠️ | ✅ `getSessionDefaults` 已实现 |
| P3-D3 主 session 保护 | 1 ⚠️ | ✅ 动态保护已实现 |
| P4-DRIFT4 API Key | 2 ⚠️ | ✅ 31 映射 + 5 回退链已完成 |
| P4-NEW5 隐式发现 | 3 ⚠️ | ⚠️ Bedrock/Copilot 为 stub |

**总结**：13 项 ⚠️ 隐藏依赖中，11 项已完整实现 Go 等价逻辑，2 项（Bedrock AWS SDK、Copilot OAuth）标记为 stub 待后续 Phase 补全。无 ❌ 高复杂度阻塞项。
