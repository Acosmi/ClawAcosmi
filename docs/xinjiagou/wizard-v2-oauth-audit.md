# 向导 V2 + OAuth 全链路审计报告

> 审计时间: 2026-03-03  
> 审计范围: 10 个源文件，追踪完整链路

---

## 一、已修复问题

### 1.1 Google OAuth Scope 错误 ✅

- **文件**: `backend/internal/agents/auth/google_oauth.go`
- **根因**: `GeminiOAuthScope` 设为 `generative-language`，但该 scope 未在 gemini-cli ClientID 的 GCP OAuth 同意屏幕中注册
- **表现**: Google 返回 403 `restricted_client` / `Unregistered scope`
- **修复**: 改为 `cloud-platform`，添加 `openid` + `userinfo.email` + `userinfo.profile`（与 gemini-cli 一致）

### 1.2 Google OAuth Client Secret 缺失 ✅

- **文件**: `backend/internal/agents/auth/google_oauth.go` + `oauth_web_flow.go`
- **根因**: Google Desktop OAuth 即使是 PKCE 公共客户端，token exchange 也必须提供 `client_secret`
- **表现**: Token exchange 返回 `"invalid_request" "client_secret is missing"`
- **修复**: 添加 `GeminiOAuthClientSecret = "GOCSPX-..."` 常量（来自 gemini-cli Apache-2.0 开源）

### 1.3 OAuth 按钮事件处理 ✅

- **文件**: `ui/src/ui/views/wizard-v2.ts`
- **根因**: Lit 模板中使用全局 `event` 变量而非 `e: Event` 参数
- **修复**: `@click=${async (e: Event) => { const btn = e.currentTarget... }}`

### 1.4 Provider ID 前端→后端映射 ✅

- **文件**: `backend/internal/gateway/server_methods_wizard_v2.go`
- **根因**: 前端 `qwen` ≠ 后端 `qwen-portal`，`minimax` ≠ `minimax-portal`
- **修复**: 添加 `oauthProviderMapping` 映射表

### 1.5 向导关闭行为 ✅

- **文件**: `ui/src/ui/views/wizard-v2.ts`
- **修复**: 移除 overlay backdrop 点击关闭，仅保留 X 按钮

### 1.6 wizard.v2.apply 未调用 ✅

- **文件**: `ui/src/ui/views/wizard-v2.ts`
- **根因**: `nextStep()` 仅做 `stepIndex++`，从未调用 `wizard.v2.apply` RPC
- **修复**: 改为 async，调用 `state.client!.request("wizard.v2.apply", payload)`

### 1.7 向导卡在 20% ✅

- **文件**: `ui/src/ui/views/wizard-v2.ts`
- **根因**: 后端 `ScheduleRestart` 杀掉 WS 连接，前端 `await request` 永远收不到响应
- **修复**: `Promise.race` + 5s 超时兜底

---

## 二、未修复 — 致命问题

### 2.1 🔴 OAuth Token 未持久化

- **文件**: `server_methods_wizard_v2.go:151`
- **代码**: `auth.RunOAuthWebFlow(oauthCtx, oauthCfg, nil)` — 第三参数 `AuthStore` 传了 `nil`
- **影响**: OAuth 授权成功 → token 换到了 → **从未写入磁盘** → 关闭页面后 token 丢失
- **对比**: `setup_auth_apply.go:235` 正确传了 `params.AuthStore`
- **修复建议**: 从 `ctx.Context` 获取 AuthStore 传入

### 2.2 🔴 `zhipu` 前端 ID 与后端 `zai` 不匹配

- **前端**: `wizard-v2-providers.ts` 定义 `id: "zhipu"`
- **后端**: `providers.go:93` 用 `"zai" → "ZAI_API_KEY"`，`providerDefaults` 中无 `zhipu`
- **影响**: 用户配置智谱 API Key → config 写入 key `zhipu` → 后端查 `zai` → **找不到，API 失败**
- **修复建议**: 前端改 `zhipu` → `zai`，或后端添加 `zhipu` 别名

### 2.3 🔴 `qwen` 路由断裂（BaseURL 缺失）

- **前端**: `primaryConfig["qwen"] = "<apiKey>"`
- **后端**: `applyProviderConfigs` 写入 `cfg.Models.Providers["qwen"]`
- **但**: `GetProviderDefaults("qwen")` 返回 nil（只有 `qwen-portal` 有默认 BaseURL）
- **影响**: Qwen 配置写入了但 BaseURL 为空 → API 调用失败
- **修复建议**: 添加 `"qwen"` 到 `providerDefaults`，或在 `applyProviderConfigs` 中加映射

### 2.4 🟡 MiniMax OAuth 无 ClientID

- **文件**: `oauth_web_flow.go:75-81`
- **代码**: `"minimax-portal"` 注册表条目中 `ClientID` 为空
- **影响**: `RunOAuthWebFlow` 检查 `ClientID == ""` → 直接报错 → MiniMax OAuth 永远失败
- **修复建议**: 获取 MiniMax 官方 OAuth ClientID 并填入

### 2.5 🟡 OAuth Token 被当 API Key 使用

- **文件**: `wizard-v2.ts:277`
- **代码**: `configMap[p.id] = res.accessToken` — 将 OAuth 临时 access_token 存入 primaryConfig
- **影响**: access_token 约 1 小时过期 → 写入 config 后无法自动刷新 → 过期后 API 失败
- **正确做法**: OAuth 凭据（access + refresh + expires）应存入 AuthStore，不应当 API Key 用

### 2.6 🟢 部分 Provider 缺少默认 BaseURL

- **文件**: `providers.go:29-78`
- **缺失**: `openai`、`anthropic`、`xai` 无 `providerDefaults` 条目
- **影响**: 向导配置后 config 中无 BaseURL，依赖其他地方的兜底逻辑

---

## 三、Provider ID 全量对照表

| 前端 ID | 后端 Defaults | 后端 EnvKey | 环境变量 | 状态 |
|---------|-------------|-----------|---------|------|
| `google` | ✅ `google` | ✅ `google` | `GEMINI_API_KEY` | ✅ |
| `qwen` | ❌ 无 | ⚠️ `qwen` | `DASHSCOPE_API_KEY` | 🔴 无 BaseURL |
| `minimax` | ✅ `minimax` | ✅ `minimax` | `MINIMAX_API_KEY` | ✅ |
| `deepseek` | ✅ `deepseek` | ✅ `deepseek` | `DEEPSEEK_API_KEY` | ✅ |
| `doubao` | ✅ `doubao` | ✅ `doubao` | `ARK_API_KEY` | ✅ |
| `zhipu` | ❌ 无 | ❌ 应为 `zai` | `ZAI_API_KEY` | 🔴 断裂 |
| `moonshot` | ✅ `moonshot` | ✅ `moonshot` | `MOONSHOT_API_KEY` | ✅ |
| `openai` | ❌ 无 | ✅ `openai` | `OPENAI_API_KEY` | ⚠️ 无默认 URL |
| `anthropic` | ❌ 无 | ✅ `anthropic` | `ANTHROPIC_API_KEY` | ⚠️ 无默认 URL |
| `xai` | ❌ 无 | ✅ `xai` | `XAI_API_KEY` | ⚠️ 无默认 URL |
| `ollama` | ✅ `ollama` | ✅ `ollama` | `OLLAMA_API_KEY` | ✅ |
| `custom-openai` | ❌ 无 | ❌ 无 | 用户自定义 | ⚠️ 依赖用户输入 |

---

## 四、修复优先级

| # | Bug | 严重度 | 状态 |
|---|-----|-------|------|
| 1 | OAuth Token nil AuthStore | 🔴 致命 | 未修 |
| 2 | zhipu→zai ID 断裂 | 🔴 致命 | 未修 |
| 3 | qwen 无默认 BaseURL | 🔴 致命 | 未修 |
| 4 | MiniMax OAuth 无 ClientID | 🟡 高 | 未修 |
| 5 | OAuth token 当 API Key 用 | 🟡 高 | 未修 |
| 6 | Google OAuth scopes+secret | 🟡 | ✅ 已修 |
| 7 | WS 断开卡 20% | 🟡 | ✅ 已修 |
| 8 | 部分 Provider 无默认 URL | 🟢 低 | 未修 |

---

## 五、涉及文件清单

| 文件 | 角色 |
|------|------|
| `ui/src/ui/views/wizard-v2.ts` | 前端向导主逻辑 |
| `ui/src/ui/views/wizard-v2-providers.ts` | 前端 Provider 定义 |
| `backend/internal/gateway/server_methods_wizard_v2.go` | 后端 RPC 处理器 |
| `backend/internal/agents/auth/oauth_web_flow.go` | OAuth 流程引擎 |
| `backend/internal/agents/auth/google_oauth.go` | Google OAuth 常量 |
| `backend/internal/agents/auth/qwen_oauth.go` | Qwen OAuth 常量 |
| `backend/internal/agents/models/providers.go` | Provider 默认配置 |
| `backend/internal/gateway/ws_server.go` | WS 连接生命周期 |
| `backend/cmd/openacosmi/setup_auth_apply.go` | CLI 认证应用（参考） |
| `backend/internal/config/loader.go` | 配置加载+Keyring |
