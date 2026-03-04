---
title: "Cloudflare AI Gateway"
summary: "Cloudflare AI Gateway 设置（认证与模型选择）"
read_when:
  - 使用 Cloudflare AI Gateway
  - 需要 Account ID、Gateway ID 或 API Key 环境变量
status: active
arch: rust-cli+go-gateway
---

# Cloudflare AI Gateway

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - API Key 环境变量 `CLOUDFLARE_AI_GATEWAY_API_KEY` 由 **Go Gateway** 解析（`backend/internal/agents/models/providers.go`）
> - Onboard 流程由 **Rust CLI** 实现（`cli-rust/crates/oa-cmd-auth/src/auth_choice.rs` — `CloudflareAiGatewayApiKey`）

Cloudflare AI Gateway 位于供应商 API 前端，允许你添加分析、缓存和控制。OpenAcosmi 通过你的 Gateway 端点使用 Anthropic Messages API 访问 Anthropic 模型。

- 供应商：`cloudflare-ai-gateway`
- Base URL：`https://gateway.ai.cloudflare.com/v1/<account_id>/<gateway_id>/anthropic`
- 默认模型：`cloudflare-ai-gateway/claude-sonnet-4-5`
- API Key：`CLOUDFLARE_AI_GATEWAY_API_KEY`（通过 Gateway 发送请求时使用的供应商 API Key）

对于 Anthropic 模型，使用你的 Anthropic API Key。

## 快速开始

1. 设置供应商 API Key 和 Gateway 详情：

```bash
openacosmi onboard --auth-choice cloudflare-ai-gateway-api-key
```

1. 设置默认模型：

```json5
{
  agents: {
    defaults: {
      model: { primary: "cloudflare-ai-gateway/claude-sonnet-4-5" },
    },
  },
}
```

## 非交互模式示例

```bash
openacosmi onboard --non-interactive \
  --mode local \
  --auth-choice cloudflare-ai-gateway-api-key \
  --cloudflare-ai-gateway-account-id "your-account-id" \
  --cloudflare-ai-gateway-gateway-id "your-gateway-id" \
  --cloudflare-ai-gateway-api-key "$CLOUDFLARE_AI_GATEWAY_API_KEY"
```

## 认证 Gateway

如果在 Cloudflare 中启用了 Gateway 认证，需要添加 `cf-aig-authorization` header（除了供应商 API Key 之外）。

```json5
{
  models: {
    providers: {
      "cloudflare-ai-gateway": {
        headers: {
          "cf-aig-authorization": "Bearer <cloudflare-ai-gateway-token>",
        },
      },
    },
  },
}
```

## 环境注意事项

如果 Gateway 作为守护进程运行（launchd/systemd），请确保 `CLOUDFLARE_AI_GATEWAY_API_KEY` 对该进程可用（例如在 `~/.openacosmi/.env` 或通过 `env.shellEnv`）。
