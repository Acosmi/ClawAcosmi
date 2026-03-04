---
summary: "OpenAcosmi 支持的模型供应商（LLM）快速入门"
read_when:
  - 选择模型供应商
  - 需要 LLM 认证和模型选择的快速示例
title: "模型供应商快速入门"
status: active
arch: rust-cli+go-gateway
---

# 模型供应商

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - 供应商配置与 API Key 解析由 **Go Gateway** 处理（`backend/internal/agents/models/providers.go`）
> - Onboard/认证命令由 **Rust CLI** 实现（`oa-cmd-onboard`、`oa-cmd-auth`）

OpenAcosmi 支持多种 LLM 供应商。选择一个供应商，完成认证，然后将默认模型设置为 `provider/model`。

## 推荐：Venice（Venice AI）

Venice 是我们推荐的隐私优先推理设置，同时可选用 Opus 处理最困难的任务。

- 默认模型：`venice/llama-3.3-70b`
- 最佳综合：`venice/claude-opus-45`（Opus 仍是最强模型）

详见 [Venice AI](/providers/venice)。

## 快速开始（两步）

1. 通过供应商认证（通常使用 `openacosmi onboard`）。
2. 设置默认模型：

```json5
{
  agents: { defaults: { model: { primary: "anthropic/claude-opus-4-6" } } },
}
```

## 支持的供应商（初始集）

- [OpenAI（API + Codex）](/providers/openai)
- [Anthropic（API + Claude Code CLI）](/providers/anthropic)
- [OpenRouter](/providers/openrouter)
- [Vercel AI Gateway](/providers/vercel-ai-gateway)
- [Cloudflare AI Gateway](/providers/cloudflare-ai-gateway)
- [Moonshot AI（Kimi + Kimi Coding）](/providers/moonshot)
- [Synthetic](/providers/synthetic)
- [OpenAcosmi Zen](/providers/openacosmi)
- [Z.AI](/providers/zai)
- [GLM 模型](/providers/glm)
- [MiniMax](/providers/minimax)
- [Venice（Venice AI）](/providers/venice)
- [Amazon Bedrock](/providers/bedrock)
- [千帆 Qianfan](/providers/qianfan)

有关完整供应商目录（xAI、Groq、Mistral 等）和高级配置，请参见 [模型供应商](/concepts/model-providers)。
