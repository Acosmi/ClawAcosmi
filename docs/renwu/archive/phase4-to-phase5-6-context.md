# Phase 4 → Phase 5/6 上下文递交文档

> 创建日期: 2026-02-13
> 来源: Phase 4 全覆盖审计 (Round 1-5, 23 Go 文件 vs 170 TS 文件)
> 目的: 为 Phase 5/6 新窗口提供完整的待办追踪和 TS 参考

---

## 一、Phase 4 完成状态

Phase 4 (`internal/agents/`) 所有 23 个 Go 文件已通过 5 轮逐函数审计，与 TypeScript 逻辑一致性达 **83%+**，所有 P0-P1 BUG 均已修复。

### 已修复的 BUG 和 DRIFT

| ID | 文件 | 描述 | 修复位置 |
|----|------|------|----------|
| BUG-2 | `helpers/errors.go` | 精确前缀匹配缺失 | `Is*ErrorMessage` 系列函数 |
| BUG-3 | `helpers/errors.go` | 前置检查缺失 | `ClassifyFailoverReason` |
| BUG-4 | `helpers/errors.go` | 委托 + 收缩逻辑 | 错误分类链 |
| BUG-5 | `helpers/errors.go` | 模式同步 | 正则表达式匹配 |
| BUG-6 | `models/fallback.go` | 缺少 abort 中断检查 | `ExecuteWithFallback` 循环体 |
| BUG-7 | `models/fallback.go` + `selection.go` | 缺少 allowlist 白名单过滤 | `ResolveFallbackCandidates` + `BuildConfiguredAllowlistKeys` |
| BUG-8 | `models/fallback.go` | 缺少 primary 尾部 fallback | `ResolveFallbackCandidates` |
| DRIFT-1 | `helpers/errors.go` | 排除模式不匹配 TS | 正则排除列表 |
| DRIFT-2 | `helpers/errors.go` | 模式同步 | 错误消息模式 |
| DRIFT-3 | `models/selection.go` | 缺少 Google 模型 ID 归一化 | `normalizeProviderModelId` |
| NEW-4 | `models/failover_error.go` | `DescribeFailoverError` 缺少 `code` 返回值 | 已修复，返回 4 值 |

---

## 二、Phase 5 待办项 (已在代码中用 TODO 备注)

### DRIFT-4: API Key 环境变量映射不完整 [P1]

- **文件**: [providers.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/agents/models/providers.go) L74-92
- **TS 参考**: `src/agents/models-config.providers.ts` + `src/agents/model-auth.ts`
- **问题**: Go 的 `EnvApiKeyVarNames` 仅包含基础映射 (ANTHROPIC_API_KEY, OPENAI_API_KEY 等)
- **TS 完整映射**:

  ```typescript
  // models-config.providers.ts → resolveImplicitProviders()
  // 每个 provider 有完整的 env key fallback 链:
  // anthropic: ANTHROPIC_API_KEY
  // openai: OPENAI_API_KEY  
  // google: GOOGLE_API_KEY, GEMINI_API_KEY
  // mistral: MISTRAL_API_KEY
  // groq: GROQ_API_KEY
  // deepseek: DEEPSEEK_API_KEY
  // xai: XAI_API_KEY
  // amazon-bedrock: (AWS credentials chain)
  // github-copilot: (OAuth token chain)
  
  // model-auth.ts → resolveModelAuth()
  // 完整的 OAuth fallback 链 + API key 环境变量解析
  ```

- **所需工作**:
  1. 补全 `EnvApiKeyVarNames` 映射 (对照 `models-config.providers.ts` L15-89)
  2. 实现 `resolveModelAuth()` 的 OAuth token 解析链 (对照 `model-auth.ts`)
  3. 添加 AWS credentials chain 支持 (Bedrock)

### DRIFT-5: `DowngradeOpenAIReasoningBlocks` 函数未实现 [P1]

- **文件**: [helpers.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/agents/helpers/helpers.go) L82-88
- **TS 参考**: `src/agents/pi-embedded-helpers/openai.ts`
- **问题**: Go 函数当前是 stub (直接返回 nil)
- **TS 实现逻辑**:

  ```typescript
  // openai.ts → downgradeOpenAIReasoningBlocks()
  // 1. 遍历消息数组
  // 2. 查找 role=assistant 的消息中 content 包含 type="thinking" 的 block
  // 3. 将 thinking block 降级为 text block (模型不支持 reasoning 时)
  // 4. 保留其他 block 不变
  ```

- **所需工作**: 实现完整的 reasoning block 降级逻辑

---

## 三、Phase 5 待办项 (隐式供应商发现)

### NEW-5: 隐式供应商自动发现 [P3]

- **Go 文件**: [config.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/agents/models/config.go)
- **TS 参考**: `src/agents/models-config.ts` L84-147 + `src/agents/models-config.providers.ts`
- **缺失函数**:

| TS 函数 | 说明 | 对应 TS 文件位置 |
|---------|------|-----------------|
| `resolveImplicitProviders()` | 通过环境变量自动发现配置的供应商 | `models-config.providers.ts` L91-145 |
| `resolveImplicitBedrockProvider()` | AWS Bedrock 自动发现 (检查 AWS 凭证) | `models-config.providers.ts` L147-185 |
| `resolveImplicitCopilotProvider()` | GitHub Copilot 自动发现 (检查 OAuth token) | `models-config.providers.ts` L187-218 |
| `normalizeProviders()` | 供应商配置标准化 (baseUrl 清理, apiKey 注入) | `models-config.providers.ts` L60-89 |

- **实现方式**: 在 `EnsureModelsJSON` 中调用这些函数，自动合并环境中发现的供应商

---

## 四、Phase 6+ 待办项

### NEW-1: `resolveAgentModelFallbacksOverride` hasOwn 语义 [P3]

- **文件**: [scope.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/agents/scope/scope.go) L197-204
- **TS 参考**: `src/agents/agent-scope.ts` L151-164
- **问题**: TS 使用 `Object.hasOwn(raw, "fallbacks")` 区分:
  - 没有 `fallbacks` 字段 → 使用全局 fallback
  - `fallbacks: []` 空数组 → 禁用全局 fallback
- Go 的 struct 零值无法区分 nil (未设置) vs empty slice (显式空)
- **建议修复**: 将 `Model.Fallbacks` 类型从 `[]string` 改为 `*[]string`
  - `nil` 表示未设置 → 使用全局
  - `&[]string{}` 表示显式空数组 → 禁用全局

### NEW-2: 缺少 `resolveHooksGmailModel` [P3]

- **TS 参考**: `src/agents/model-selection.ts` L422-447
- **说明**: 解析 Gmail hook 处理专用模型配置 (`cfg.hooks.gmail.model`)
- **需要时机**: Phase 6+ 实现 Gmail hooks 功能时

### NEW-3: 缺少 `resolveAllowlistModelKey` [P3 - 信息]

- **TS 参考**: `src/agents/model-selection.ts` L102-108
- **说明**: 简单包装 `parseModelRef` → `modelKey` 的管道函数
- **当前状态**: Go 已在 `BuildConfiguredAllowlistKeys` 中内联了等价逻辑，**功能完整**
- **建议**: 无需单独实现，仅作记录

---

## 五、Phase 5 优先级排序

```
P1 ─── 必须在 Phase 5 完成 ───────────────────────
  1. DRIFT-4: API Key 环境变量映射补全
  2. DRIFT-5: DowngradeOpenAIReasoningBlocks 实现

P2 ─── Phase 5 中完成 ────────────────────────────
  3. NEW-5: 隐式供应商发现 (resolveImplicitProviders 系列)

P3 ─── Phase 6+ 按需实现 ─────────────────────────
  4. NEW-1: hasOwn 语义修复 (*[]string)
  5. NEW-2: resolveHooksGmailModel
  6. NEW-3: resolveAllowlistModelKey (已内联等价)
```

---

## 六、当前 Go 代码库健康状况

```
构建: ✅ go build ./internal/agents/... (0 错误)
测试: ✅ go test -race -count=1 ./internal/agents/... (10/10 包通过)
覆盖: 23/23 Go 文件已审计
一致性: 83%+ (19/23 文件完全一致)
```

---

## 七、关键 TS 参考文件索引

| Go 包 | 主要 TS 文件 | 路径 |
|-------|-------------|------|
| `scope/` | `agent-scope.ts`, `identity.ts`, `tool-policy.ts`, `session-slug.ts`, `timeout.ts`, `usage.ts` | `src/agents/` |
| `models/` | `model-selection.ts`, `model-fallback.ts`, `failover-error.ts`, `model-catalog.ts`, `models-config.ts`, `context-window-guard.ts`, `model-compat.ts` | `src/agents/` |
| `models/` (Phase 5) | `models-config.providers.ts`, `model-auth.ts` | `src/agents/` |
| `helpers/` | `pi-embedded-helpers/*.ts` | `src/agents/pi-embedded-helpers/` |
| `transcript/` | `tool-call-id.ts`, `transcript-policy.ts`, `cache-trace.ts`, `session-file-repair.ts` | `src/agents/` |
| `compaction/` | `compaction.ts` | `src/agents/` |
| `prompt/` | `system-prompt.ts` | `src/agents/` |
