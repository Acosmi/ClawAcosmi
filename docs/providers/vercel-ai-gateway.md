---
title: "Vercel AI Gateway"
summary: "Vercel AI Gateway 设置（认证与模型选择）"
read_when:
  - 使用 Vercel AI Gateway
  - 需要 API Key 环境变量或 CLI 认证选项
status: active
arch: rust-cli+go-gateway
---

# Vercel AI Gateway

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - API Key 环境变量 `AI_GATEWAY_API_KEY` 由 **Go Gateway** 解析（`backend/internal/agents/models/providers.go`）
> - Onboard 流程由 **Rust CLI** 实现

[Vercel AI Gateway](https://vercel.com/ai-gateway) 提供统一 API，通过单一端点访问数百个模型。

- 供应商：`vercel-ai-gateway`
- 认证：`AI_GATEWAY_API_KEY`
- API：Anthropic Messages 兼容

## 快速开始

1. 设置 API Key（推荐：为 Gateway 存储）：

```bash
openacosmi onboard --auth-choice ai-gateway-api-key
```

1. 设置默认模型：

```json5
{
  agents: {
    defaults: {
      model: { primary: "vercel-ai-gateway/anthropic/claude-opus-4.6" },
    },
  },
}
```

## 非交互模式示例

```bash
openacosmi onboard --non-interactive \
  --mode local \
  --auth-choice ai-gateway-api-key \
  --ai-gateway-api-key "$AI_GATEWAY_API_KEY"
```

## 环境注意事项

如果 Gateway 作为守护进程运行（launchd/systemd），请确保 `AI_GATEWAY_API_KEY` 对该进程可用（例如在 `~/.openacosmi/.env` 或通过 `env.shellEnv`）。
