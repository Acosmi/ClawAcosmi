# providers 模块架构文档

> 最后更新：2026-02-19

## 一、模块概述

providers 模块实现第三方 LLM 提供商的 OAuth 认证和模型发现。当前覆盖 GitHub Copilot（完整 Device Flow + API token 交换）和 Qwen Portal（OAuth refresh_token）。模块位于 `internal/agents/auth/` 和 `internal/agents/models/`，为 setup 流程和隐式供应商发现提供底层支持。

## 二、原版实现（TypeScript）

### 源文件列表

| 文件 | 行数 | 职责 |
|------|------|------|
| `github-copilot-auth.ts` | 185 | 设备码 OAuth 流程（RFC 8628） |
| `github-copilot-token.ts` | 133 | GitHub → Copilot API token 交换 + JSON 缓存 |
| `github-copilot-models.ts` | 42 | 默认 Copilot 模型定义（7 个 OpenAI 模型） |
| `qwen-portal-oauth.ts` | 55 | Qwen OAuth2 refresh_token 刷新 |

### 核心逻辑摘要

- **GitHub Copilot**：完整 Device Flow — (1) POST 获取 device_code + user_code → (2) 用户浏览器授权 → (3) 轮询获取 access_token → (4) 用 access_token 交换 Copilot API token（GET `/copilot_internal/v2/token`）→ (5) 从 token 中解析 `proxy-ep` 获取 API base URL
- **Qwen Portal**：POST `chat.qwen.ai/api/v1/oauth2/token` 用 refresh_token 获取新 access_token

## 三、依赖分析

### 显式依赖图

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `auth.AuthStore` | 值 | ↓ | 凭据存储 |
| `auth.OAuthCredentials` | 类型 | ↓ | OAuth 凭据结构体 |
| `config.ResolveStateDir` | 值 | ↓ | 缓存路径解析 |
| `types.ModelDefinitionConfig` | 类型 | ↓ | 模型定义 |
| `models.implicit_providers` | 值 | ↑ | 消费 Copilot 模型列表 |
| `cmd/openacosmi/setup_auth_apply` | 值 | ↑ | 消费 Device Flow 函数 |

### 隐藏依赖审计

| 类别 | 结果 | Go 等价方案 |
|------|------|-------------|
| npm 包黑盒行为 | ✅ 无（仅 `fetch` + `@clack/prompts`） | net/http + tui.WizardPrompter |
| 全局状态/单例 | ✅ 无 | — |
| 事件总线/回调链 | ✅ 无 | — |
| 环境变量依赖 | ⚠️ `GITHUB_COPILOT_TOKEN` | os.Getenv（implicit_providers.go 已读取）|
| 文件系统约定 | ⚠️ `{stateDir}/credentials/github-copilot.token.json` | config.ResolveStateDir() + filepath.Join |
| 协议/消息格式 | ⚠️ Device Flow + token 中 `proxy-ep=` 解析 | url.Values + regexp |
| 错误处理约定 | ⚠️ 4 种轮询错误码 | switch 分支完全覆盖 |

## 四、重构实现（Go）

### 文件结构

| 文件 | 行数 | 对应原版 |
|------|------|----------|
| `auth/github_copilot_auth.go` | 203 | `github-copilot-auth.ts` |
| `auth/github_copilot_token.go` | 252 | `github-copilot-token.ts` |
| `auth/qwen_oauth.go` | 117 | `qwen-portal-oauth.ts` |
| `models/github_copilot_models.go` | 60 | `github-copilot-models.ts` |
| `auth/providers_test.go` | ~180 | 新增测试 |
| `models/github_copilot_models_test.go` | 87 | 新增测试 |

### 接口定义

```go
// OAuthTokenRefresher — 外部注入的 token 刷新器（已有）
type OAuthTokenRefresher interface {
    RefreshToken(provider string, refreshToken string) (*OAuthCredentials, error)
}

// QwenPortalRefresher — 实现 OAuthTokenRefresher
type QwenPortalRefresher struct { Client *http.Client }
```

### 核心函数

| 函数 | 说明 |
|------|------|
| `RequestCopilotDeviceCode` | POST device/code |
| `PollForCopilotAccessToken` | 轮询获取 access token |
| `StoreCopilotAuthProfile` | 存入 AuthStore |
| `ResolveCopilotApiToken` | 交换 Copilot API token（含缓存） |
| `DeriveCopilotApiBaseUrlFromToken` | 解析 proxy-ep 获取 API URL |
| `GetDefaultCopilotModelIDs` | 返回 7 个默认模型 |
| `BuildCopilotModelDefinition` | 构建单个模型配置 |
| `RefreshQwenPortalCredentials` | Qwen OAuth refresh |

## 五、差异对照

| 维度 | 原版 TS | 重构 Go |
|------|---------|---------|
| HTTP 客户端 | `fetch` | `net/http`（可注入 `*http.Client`） |
| 错误处理 | throw Error | `error` 返回值 |
| 缓存 I/O | `loadJsonFile`/`saveJsonFile` | `os.ReadFile`/`os.WriteFile` |
| 并发安全 | 单线程 | `http.Client` 天然并发安全 |
| 终端交互 | `@clack/prompts` | `tui.WizardPrompter` |

## 六、Rust 下沉候选

| 函数/模块 | 优先级 | 原因 |
|-----------|--------|------|
| (无) | — | 纯 I/O 绑定模块，无计算密集逻辑 |

## 七、测试覆盖

| 测试类型 | 覆盖范围 | 状态 |
|----------|----------|------|
| 单元测试 | proxy-ep 解析、token TTL、响应解析、profile 存储 | ✅ |
| 单元测试 | 模型定义不可变性、字段验证、默认列表 | ✅ |
| 单元测试 | Qwen refresh 边界（nil/empty） | ✅ |
| 集成测试 | GitHub/Qwen API 调用（需运行时） | ⏭️ |
