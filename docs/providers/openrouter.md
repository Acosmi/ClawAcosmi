---
summary: "通过 OpenRouter 统一 API 在 OpenAcosmi 中访问多种模型"
read_when:
  - 需要单个 API Key 访问多种 LLM
  - 通过 OpenRouter 运行模型
title: "OpenRouter"
status: active
arch: rust-cli+go-gateway
---

# OpenRouter

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - API Key 环境变量 `OPENROUTER_API_KEY` 由 **Go Gateway** 解析（`backend/internal/agents/models/providers.go`）

OpenRouter 提供**统一 API**，通过单一端点和 API Key 将请求路由到多种模型。它兼容 OpenAI，大多数 OpenAI SDK 只需切换 Base URL 即可使用。

## CLI 设置

```bash
openacosmi onboard --auth-choice apiKey --token-provider openrouter --token "$OPENROUTER_API_KEY"
```

## 配置示例

```json5
{
  env: { OPENROUTER_API_KEY: "sk-or-..." },
  agents: {
    defaults: {
      model: { primary: "openrouter/anthropic/claude-sonnet-4-5" },
    },
  },
}
```

## 注意事项

- 模型引用格式为 `openrouter/<provider>/<model>`。
- 更多模型/供应商选项参见 [模型供应商](/concepts/model-providers)。
- OpenRouter 底层使用 Bearer token 和你的 API Key。
