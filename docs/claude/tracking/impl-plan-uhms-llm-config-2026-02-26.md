---
document_type: Tracking
status: In Progress
created: 2026-02-26
last_updated: 2026-02-26
audit_report: Pending
skill5_verified: true
---

# 记忆系统独立 LLM 模型配置

## 概述

为 UHMS 记忆系统添加独立 LLM 模型配置能力。
用户必须在记忆管理页面显式配置 LLM provider/model/apiKey，切换后仅影响记忆操作（分类、摘要、压缩、提取），不影响 agent。

## Online Verification Log

### LLM Provider Hot-Swap 安全性
- **Query**: Go interface assignment goroutine safety
- **Source**: Go Memory Model (https://go.dev/ref/mem)
- **Key finding**: interface 赋值为单指针宽度操作，但非原子；通过现有 mu.Lock 保护写入，
  safeGo 启动的新 goroutine 读新值，正在运行的旧 goroutine 持有旧接口引用，无竞态。
- **Verified date**: 2026-02-26

### 各 LLM Provider 官方 API 地址
- **Query**: LLM provider official API base URLs 2026
- **Sources**: 各厂商官方文档
- **Key findings**:
  - Anthropic: `https://api.anthropic.com`
  - OpenAI: `https://api.openai.com/v1`
  - DeepSeek: `https://api.deepseek.com`
  - Google Gemini: `https://generativelanguage.googleapis.com/v1beta`
  - Ollama: `http://localhost:11434`
  - Groq: `https://api.groq.com/openai/v1`
  - Mistral: `https://api.mistral.ai/v1`
  - Together AI: `https://api.together.xyz/v1`
  - OpenRouter: `https://openrouter.ai/api/v1`
- **Verified date**: 2026-02-26

## 修改清单

### Phase 1: 初始实现 (已完成)

#### 后端 (5 files)

- [x] **P1: `backend/pkg/types/types_memory.go`** — `MemoryUHMSConfig` 加 3 字段
  - `LLMProvider string` — 空=未配置
  - `LLMModel string` — 空=按 provider 默认
  - `LLMBaseURL string` — 空=使用 provider 默认 URL

- [x] **P2: `backend/internal/memory/uhms/manager.go`** — 热替换方法
  - `SetLLMProvider(llm LLMProvider)` — 加锁写入 m.llm
  - `LLMInfo() (provider, model string)` — RLock 读取当前 adapter

- [x] **P3: `backend/internal/gateway/server.go`** — 初始化逻辑重构
  - 抽取 `buildUHMSLLMAdapter(uhmsCfg, fullCfg) LLMProvider` 辅助函数
  - 抽取 `defaultModelForProvider(provider) string` 默认模型映射
  - 初始化优先使用 UHMS 独立配置，fallback 到 agent auto-select

- [x] **P4: `backend/internal/gateway/server_methods_uhms.go`** — 2 个新 RPC
  - `memory.uhms.llm.get` — 返回当前配置 + 可用 provider 列表
  - `memory.uhms.llm.set` — 持久化 + 热替换 + CompactionClient 管理
  - `collectUHMSLLMProviders()` — 收集已知 + 配置中的 provider 列表

- [x] **P5: `backend/internal/gateway/server_methods.go`** — 授权注册

#### 前端 (5 files)

- [x] **P6: `ui/src/ui/controllers/memory.ts`** — 控制器
- [x] **P7: `ui/src/ui/views/memory.ts`** — 视图
- [x] **P8: `ui/src/ui/app-render.ts`** — Props 接线
- [x] **P9: `ui/src/ui/app.ts` + `app-view-state.ts`** — 状态字段
- [x] **P10: `ui/src/ui/locales/en.ts` + `zh.ts`** — i18n

### Phase 2: Bug 修复 — 3 个 Bug (已完成)

#### B1: fallback 不复制 baseUrl (已修复)
- **根因**: `buildUHMSLLMAdapter()` fallback 路径 `providerCandidate` 无 `baseURL` 字段
- **影响**: DeepSeek key 发到 OpenAI 默认 URL → `invalid_request_error`
- **修复**: `providerCandidate` 增加 `baseURL`，传入 `LLMClientAdapter`
- **文件**: `backend/internal/gateway/server.go`

#### B2: 无独立 API Key 字段 (已修复)
- **根因**: `MemoryUHMSConfig` 没有 `LLMApiKey` 字段
- **修复**: 新增 `LLMApiKey string`；`buildUHMSLLMAdapter` 优先使用独立 key
- **文件**: `backend/pkg/types/types_memory.go`, `backend/internal/gateway/server.go`

#### B3: 前端缺 API Key 输入 (已修复)
- **修复**: RPC set/get 支持 apiKey；前端 password 输入框 + 清除按钮；翻译
- **文件**: `server_methods_uhms.go`, `controllers/memory.ts`, `views/memory.ts`, `app-render.ts`, locales

### Phase 3: 移除"跟随系统" + UI 修复 + Provider URL 补全 (已完成)

#### 移除"跟随智能体"模式
- **根因**: 自动选择 fallback 从 agent providers 中选错 provider (deepseek 字母序在 google 前)
- **修复**:
  - `buildUHMSLLMAdapter()` 删除整个 fallback 路径 (~30 LOC)，未配置 provider 返回 nil
  - `handleUHMSLLMSet` 清空 provider 时 `SetLLMProvider(nil)` (降级)
  - `handleUHMSLLMGet` 删除 `isCustom` 字段和 UHMSManager fallback
  - 前端删除 radio buttons，始终显示配置表单
  - 删除 `isCustom` from MemoryLLMConfig type
  - 删除 `memory.llmFollowAgent` / `memory.llmCustom` 翻译，新增 `memory.llmNotConfigured`
  - 移除未使用的 `sort` import
- **文件**: server.go, server_methods_uhms.go, controllers/memory.ts, views/memory.ts, locales

#### API Key 输入消失修复
- **根因 1**: `.value=${""}` 每次 Lit 渲染都清空输入框
- **根因 2**: 切换 provider/model 时 `onLLMConfigSave` 不传 apiKey，后端收到 `""` 覆盖已存 key
- **修复**:
  - 前端: 移除 `.value=${""}` 绑定
  - 后端: `_, apiKeyProvided := ctx.Params["apiKey"]`，仅显式传入时更新
- **文件**: server_methods_uhms.go, views/memory.ts

#### Provider 默认 URL 补全
- **修复**:
  - `UHMSLLMProvider` 新增 `DefaultBaseURL string` 字段
  - 新增 `defaultBaseURLForProvider()` — 9 个 provider 官方 URL
  - `defaultModelForProvider()` 扩展: +google, groq, mistral, together, openrouter
  - `collectUHMSLLMProviders()` 扩展: 9 个已知 provider (原 4 个)
  - 前端: 切换 provider 自动填 URL + baseUrl placeholder 显示默认地址
  - 前端 type: providers 增加 `defaultBaseUrl` 字段
- **文件**: server_methods_uhms.go, server.go, controllers/memory.ts, views/memory.ts

## 全部修改文件清单

| 文件 | Phase | 变更 |
|------|-------|------|
| `backend/pkg/types/types_memory.go` | 1+2 | +LLMApiKey 字段 |
| `backend/internal/gateway/server.go` | 1+2+3 | buildUHMSLLMAdapter 简化 (无 fallback), defaultModelForProvider 扩展 |
| `backend/internal/gateway/server_methods_uhms.go` | 1+2+3 | apiKey RPC, defaultBaseURLForProvider, 9 provider, UHMSLLMProvider.DefaultBaseURL |
| `backend/internal/gateway/server_methods.go` | 1 | RPC 注册 |
| `backend/internal/memory/uhms/manager.go` | 1 | SetLLMProvider, LLMInfo |
| `ui/src/ui/controllers/memory.ts` | 1+2+3 | +hasOwnApiKey, +defaultBaseUrl, -isCustom, +apiKey param |
| `ui/src/ui/views/memory.ts` | 1+2+3 | 删 radio buttons, +API key 输入, provider 切换填 URL |
| `ui/src/ui/app-render.ts` | 1+3 | 透传 apiKey |
| `ui/src/ui/app.ts` + `app-view-state.ts` | 1 | 状态字段 |
| `ui/src/ui/locales/en.ts` + `zh.ts` | 1+2+3 | +4 apiKey 翻译, -2 follow/custom, +1 notConfigured |

## 验证结果

- [x] Go 编译: `CGO_ENABLED=0 go build ./...` — 通过
- [x] TS 类型检查: 无新增错误
- [x] 未配置 provider → 返回 nil，UHMS 降级 (截取代替摘要，不提取记忆)
- [x] 配置 provider/model/apiKey → 持久化到 config.json → 重启生效
- [x] 切换 provider → 自动填充 defaultModel + defaultBaseUrl
- [x] API Key 输入 → 保存 → 切换其他字段 → key 不被清除
- [x] 清除 API Key → 回退到从 agent provider 查找

## 关键设计决策

| 决策 | 选择 | 原因 |
|---|---|---|
| 存储位置 | `openacosmi.config.json` | 复用 ConfigLoader，重启后持久 |
| 热替换 vs 重启 | 热替换 (SetLLMProvider) | 参考 SetCompactionClient 模式 |
| provider="" 语义 | 未配置 (返回 nil) | 移除了"跟随系统"自动选择，避免选错 provider |
| API Key 来源 | UHMS 独立 key > Models.Providers[name].APIKey | 用户可为 UHMS 单独配置不同 key |
| apiKey RPC 更新策略 | 仅 params 中存在时更新 | 避免 provider/model 变更时误清 key |
| 前端 API Key 输入 | write-only password 字段 | 安全: 不回显 key，只显示 hasOwnApiKey 状态 |
| 默认 URL | 9 个 provider 官方地址 | 切换 provider 无需手动查 URL |
| LLM 未配置降级 | 会话归档用截取，不提取记忆 | session_committer.go 已处理 llm==nil |
