---
summary: "在 OpenAcosmi 中使用 Synthetic 的 Anthropic 兼容 API"
read_when:
  - 使用 Synthetic 作为模型供应商
  - 需要 Synthetic API Key 或 Base URL 设置
title: "Synthetic"
status: active
arch: rust-cli+go-gateway
---

# Synthetic

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - API Key 环境变量 `SYNTHETIC_API_KEY` 由 **Go Gateway** 解析（`backend/internal/agents/models/providers.go`）
> - Onboard 流程由 **Rust CLI** 实现

Synthetic 提供 Anthropic 兼容端点。OpenAcosmi 将其注册为 `synthetic` 供应商，使用 Anthropic Messages API。

## 快速设置

1. 设置 `SYNTHETIC_API_KEY`（或运行下方向导）。
2. 运行 onboarding：

```bash
openacosmi onboard --auth-choice synthetic-api-key
```

默认模型设置为：

```
synthetic/hf:MiniMaxAI/MiniMax-M2.1
```

## 配置示例

```json5
{
  env: { SYNTHETIC_API_KEY: "sk-..." },
  agents: {
    defaults: {
      model: { primary: "synthetic/hf:MiniMaxAI/MiniMax-M2.1" },
      models: { "synthetic/hf:MiniMaxAI/MiniMax-M2.1": { alias: "MiniMax M2.1" } },
    },
  },
  models: {
    mode: "merge",
    providers: {
      synthetic: {
        baseUrl: "https://api.synthetic.new/anthropic",
        apiKey: "${SYNTHETIC_API_KEY}",
        api: "anthropic-messages",
        models: [
          {
            id: "hf:MiniMaxAI/MiniMax-M2.1",
            name: "MiniMax M2.1",
            reasoning: false,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 192000,
            maxTokens: 65536,
          },
        ],
      },
    },
  },
}
```

注意：OpenAcosmi 的 Anthropic 客户端会在 Base URL 后追加 `/v1`，因此使用 `https://api.synthetic.new/anthropic`（而非 `/anthropic/v1`）。如果 Synthetic 更改了 Base URL，请覆盖 `models.providers.synthetic.baseUrl`。

## 模型目录

以下所有模型费用为 `0`（输入/输出/缓存）。

| 模型 ID                                                | 上下文窗口 | 最大 Token | 推理    | 输入         |
| ------------------------------------------------------ | ---------- | ---------- | ------- | ------------ |
| `hf:MiniMaxAI/MiniMax-M2.1`                            | 192000     | 65536      | false   | text         |
| `hf:moonshotai/Kimi-K2-Thinking`                       | 256000     | 8192       | true    | text         |
| `hf:zai-org/GLM-4.7`                                   | 198000     | 128000     | false   | text         |
| `hf:deepseek-ai/DeepSeek-R1-0528`                      | 128000     | 8192       | false   | text         |
| `hf:deepseek-ai/DeepSeek-V3-0324`                      | 128000     | 8192       | false   | text         |
| `hf:deepseek-ai/DeepSeek-V3.1`                         | 128000     | 8192       | false   | text         |
| `hf:deepseek-ai/DeepSeek-V3.1-Terminus`                | 128000     | 8192       | false   | text         |
| `hf:deepseek-ai/DeepSeek-V3.2`                         | 159000     | 8192       | false   | text         |
| `hf:meta-llama/Llama-3.3-70B-Instruct`                 | 128000     | 8192       | false   | text         |
| `hf:meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8` | 524000     | 8192       | false   | text         |
| `hf:moonshotai/Kimi-K2-Instruct-0905`                  | 256000     | 8192       | false   | text         |
| `hf:openai/gpt-oss-120b`                               | 128000     | 8192       | false   | text         |
| `hf:Qwen/Qwen3-235B-A22B-Instruct-2507`                | 256000     | 8192       | false   | text         |
| `hf:Qwen/Qwen3-Coder-480B-A35B-Instruct`               | 256000     | 8192       | false   | text         |
| `hf:Qwen/Qwen3-VL-235B-A22B-Instruct`                  | 250000     | 8192       | false   | text + image |
| `hf:zai-org/GLM-4.5`                                   | 128000     | 128000     | false   | text         |
| `hf:zai-org/GLM-4.6`                                   | 198000     | 128000     | false   | text         |
| `hf:deepseek-ai/DeepSeek-V3`                           | 128000     | 8192       | false   | text         |
| `hf:Qwen/Qwen3-235B-A22B-Thinking-2507`                | 256000     | 8192       | true    | text         |

## 注意事项

- 模型引用使用 `synthetic/<modelId>` 格式。
- 如果启用了模型允许列表（`agents.defaults.models`），需添加你计划使用的所有模型。
- 参见 [模型供应商](/concepts/model-providers) 了解供应商规则。
