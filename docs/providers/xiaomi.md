---
summary: "在 OpenAcosmi 中使用小米 MiMo（mimo-v2-flash）"
read_when:
  - 使用小米 MiMo 模型
  - 需要 XIAOMI_API_KEY 设置
title: "小米 MiMo"
status: active
arch: rust-cli+go-gateway
---

# 小米 MiMo

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - 小米供应商默认配置：**Go Gateway**（`backend/internal/agents/models/providers.go` — `xiaomi`，Base URL: `https://api.xiaomimimo.com/anthropic`，默认模型: `mimo-v2-flash`）
> - API Key 环境变量：`TIZI_API_KEY`（对应 providers.go 中的 `xiaomi` 映射）

小米 MiMo 是 **MiMo** 模型的 API 平台。它提供兼容 OpenAI 和 Anthropic 格式的 REST API，使用 API Key 认证。在[小米 MiMo 控制台](https://platform.xiaomimimo.com/#/console/api-keys)创建 API Key。OpenAcosmi 使用 `xiaomi` 供应商。

## 模型概览

- **mimo-v2-flash**：262144 token 上下文窗口，Anthropic Messages API 兼容。
- Base URL：`https://api.xiaomimimo.com/anthropic`
- 授权：`Bearer $XIAOMI_API_KEY`

## CLI 设置

```bash
openacosmi onboard --auth-choice xiaomi-api-key
# 或非交互模式
openacosmi onboard --auth-choice xiaomi-api-key --xiaomi-api-key "$XIAOMI_API_KEY"
```

## 配置示例

```json5
{
  env: { XIAOMI_API_KEY: "your-key" },
  agents: { defaults: { model: { primary: "xiaomi/mimo-v2-flash" } } },
  models: {
    mode: "merge",
    providers: {
      xiaomi: {
        baseUrl: "https://api.xiaomimimo.com/anthropic",
        api: "anthropic-messages",
        apiKey: "XIAOMI_API_KEY",
        models: [
          {
            id: "mimo-v2-flash",
            name: "Xiaomi MiMo V2 Flash",
            reasoning: false,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 262144,
            maxTokens: 8192,
          },
        ],
      },
    },
  },
}
```

## 注意事项

- 模型引用：`xiaomi/mimo-v2-flash`。
- 当 `XIAOMI_API_KEY` 被设置（或存在认证 profile）时，供应商会自动注入。
- 参见 [模型供应商](/concepts/model-providers) 了解供应商规则。
