---
summary: "在 OpenAcosmi 中使用 Z.AI（GLM 模型）"
read_when:
  - 使用 Z.AI / GLM 模型
  - 需要 ZAI_API_KEY 设置
title: "Z.AI"
status: active
arch: rust-cli+go-gateway
---

# Z.AI

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - API Key 环境变量：`XAI_API_KEY`（回退：`ZAI_API_KEY`），由 **Go Gateway** 解析（`backend/internal/agents/models/providers.go`）
> - Onboard 流程由 **Rust CLI** 实现

Z.AI 是 **GLM** 模型的 API 平台。它为 GLM 提供 REST API，使用 API Key 认证。在 Z.AI 控制台创建 API Key。OpenAcosmi 使用 `zai` 供应商。

## CLI 设置

```bash
openacosmi onboard --auth-choice zai-api-key
# 或非交互模式
openacosmi onboard --zai-api-key "$ZAI_API_KEY"
```

## 配置示例

```json5
{
  env: { ZAI_API_KEY: "sk-..." },
  agents: { defaults: { model: { primary: "zai/glm-4.7" } } },
}
```

## 注意事项

- GLM 模型以 `zai/<model>` 格式引用（例如：`zai/glm-4.7`）。
- 参见 [GLM 模型](/providers/glm) 了解模型系列概览。
- Z.AI 使用 Bearer 认证和你的 API Key。
