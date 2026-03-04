---
summary: "在 OpenAcosmi 中通过 API Key 或 setup-token 使用 Anthropic Claude"
read_when:
  - 使用 Anthropic 模型
  - 使用 setup-token 代替 API Key
title: "Anthropic"
status: active
arch: rust-cli+go-gateway
---

# Anthropic（Claude）

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - 认证向导由 **Rust CLI** 实现（`cli-rust/crates/oa-cmd-auth/src/apply/api_providers.rs`）
> - API Key 环境变量解析由 **Go Gateway** 处理（`backend/internal/agents/models/providers.go`）
> - 环境变量名：`ANTHROPIC_API_KEY`，OAuth 回退：`ANTHROPIC_OAUTH_TOKEN`

Anthropic 是 **Claude** 模型系列的开发者，通过 API 提供访问。
在 OpenAcosmi 中，你可以使用 API Key 或 **setup-token** 进行认证。

## 方式 A：Anthropic API Key

**适用场景：** 标准 API 访问和按量计费。
在 Anthropic Console 中创建 API Key。

### CLI 设置

```bash
openacosmi onboard
# 选择：Anthropic API key

# 或非交互模式
openacosmi onboard --anthropic-api-key "$ANTHROPIC_API_KEY"
```

### 配置示例

```json5
{
  env: { ANTHROPIC_API_KEY: "sk-ant-..." },
  agents: { defaults: { model: { primary: "anthropic/claude-opus-4-6" } } },
}
```

## 提示缓存（Anthropic API）

OpenAcosmi 支持 Anthropic 的提示缓存功能。此功能仅适用于 **API 访问**；订阅认证不支持缓存设置。

### 配置

在模型配置中使用 `cacheRetention` 参数：

| 值      | 缓存时长 | 描述                                |
| ------- | -------- | ----------------------------------- |
| `none`  | 无缓存   | 禁用提示缓存                         |
| `short` | 5 分钟   | API Key 认证的默认值                  |
| `long`  | 1 小时   | 扩展缓存（需要 beta 标志）            |

```json5
{
  agents: {
    defaults: {
      models: {
        "anthropic/claude-opus-4-6": {
          params: { cacheRetention: "long" },
        },
      },
    },
  },
}
```

### 默认行为

使用 Anthropic API Key 认证时，OpenAcosmi 会自动为所有 Anthropic 模型设置 `cacheRetention: "short"`（5 分钟缓存）。你可以在配置中显式设置 `cacheRetention` 来覆盖此默认值。

### 旧版参数

旧版 `cacheControlTtl` 参数仍受支持以保持向后兼容：

- `"5m"` 映射为 `short`
- `"1h"` 映射为 `long`

建议迁移到新的 `cacheRetention` 参数。

OpenAcosmi 在 Anthropic API 请求中包含 `extended-cache-ttl-2025-04-11` beta 标志；如果你覆盖了供应商 headers，请保留此标志（参见 [Gateway 配置](/gateway/configuration)）。

## 方式 B：Claude setup-token

**适用场景：** 使用 Claude 订阅。

### 获取 setup-token

setup-token 由 **Claude Code CLI** 创建（非 Anthropic Console）。你可以在**任何机器上**运行：

```bash
claude setup-token
```

将 token 粘贴到 OpenAcosmi（向导中选择 **Anthropic token (paste setup-token)**），或在 Gateway 主机上运行：

```bash
openacosmi models auth setup-token --provider anthropic
```

如果你在不同机器上生成了 token，可以粘贴：

```bash
openacosmi models auth paste-token --provider anthropic
```

### CLI 设置（setup-token）

```bash
# 在 onboarding 时粘贴 setup-token
openacosmi onboard --auth-choice setup-token
```

### 配置示例（setup-token）

```json5
{
  agents: { defaults: { model: { primary: "anthropic/claude-opus-4-6" } } },
}
```

## 注意事项

- 使用 `claude setup-token` 生成 token 并粘贴，或在 Gateway 主机上运行 `openacosmi models auth setup-token`。
- 如果 Claude 订阅出现 "OAuth token refresh failed …" 错误，请重新使用 setup-token 认证。参见 [Gateway 故障排查](/gateway/troubleshooting#oauth-token-refresh-failed-anthropic-claude-subscription)。
- 认证详情与复用规则参见 [OAuth 概念](/concepts/oauth)。

## 故障排查

**401 错误 / token 突然失效**

- Claude 订阅认证可能过期或被撤销。重新运行 `claude setup-token` 并在 **Gateway 主机**上粘贴。
- 如果 Claude CLI 登录位于不同机器，使用 `openacosmi models auth paste-token --provider anthropic` 在 Gateway 主机上操作。

**未找到供应商 "anthropic" 的 API Key**

- 认证是**按 Agent 分离**的。新 Agent 不会继承主 Agent 的密钥。
- 为该 Agent 重新运行 onboarding，或在 Gateway 主机上粘贴 setup-token / API Key，然后使用 `openacosmi models status` 验证。

**未找到 profile `anthropic:default` 的凭据**

- 运行 `openacosmi models status` 查看当前激活的认证 profile。
- 重新运行 onboarding，或为该 profile 粘贴 setup-token / API Key。

**所有认证 profile 不可用（冷却中/不可用）**

- 使用 `openacosmi models status --json` 检查 `auth.unusableProfiles`。
- 添加另一个 Anthropic profile 或等待冷却结束。

更多信息：[Gateway 故障排查](/gateway/troubleshooting) 和 [常见问题](/help/faq)。

## 代码位置参考

| 组件 | 位置 |
|------|------|
| API Key 环境变量映射 | `backend/internal/agents/models/providers.go` |
| OAuth 回退链 | `backend/internal/agents/models/providers.go` `EnvApiKeyFallbacks` |
| 认证向导（Rust CLI） | `cli-rust/crates/oa-cmd-auth/src/apply/api_providers.rs` |
| Onboard 流程（Rust CLI） | `cli-rust/crates/oa-cmd-onboard/src/auth/` |
