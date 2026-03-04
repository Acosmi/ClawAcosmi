---
summary: "OpenAcosmi 支持的模型供应商（LLM）概览"
read_when:
  - 选择模型供应商
  - 快速了解支持的 LLM 后端
title: "模型供应商"
status: active
arch: rust-cli+go-gateway
---

# 模型供应商

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - 模型供应商配置与 API Key 解析由 **Go Gateway** 负责（`backend/internal/agents/models/providers.go`）
> - 模型目录查询通过 Gateway RPC 方法 `models.list`（`backend/internal/gateway/server_methods_models.go`）
> - CLI 命令 `openacosmi onboard`（Rust `oa-cmd-onboard`）、`openacosmi models`（Rust `oa-cmd-models`）处理用户交互
> - 认证流程由 Rust CLI `oa-cmd-auth` crate 实现（API Key、OAuth、setup-token）

OpenAcosmi 支持多种 LLM 供应商。选择一个供应商，完成认证，然后将默认模型设置为 `provider/model`。

寻找聊天通道文档（WhatsApp / Telegram / Discord / Slack / Mattermost / 飞书 / 钉钉 / 企业微信等）？请查看 [通道](/channels)。

## 推荐：Venice（Venice AI）

Venice 是我们推荐的隐私优先推理设置，同时可选用 Opus 处理复杂任务。

- 默认模型：`venice/llama-3.3-70b`
- 最佳综合：`venice/claude-opus-45`（Opus 仍是最强模型）

详见 [Venice AI](/providers/venice)。

## 快速开始

1. 通过供应商认证（通常使用 `openacosmi onboard`）。
2. 设置默认模型：

```json5
{
  agents: { defaults: { model: { primary: "anthropic/claude-opus-4-6" } } },
}
```

## 供应商文档

- [OpenAI（API + Codex）](/providers/openai)
- [Anthropic（API + Claude Code CLI）](/providers/anthropic)
- [通义千问 Qwen（OAuth）](/providers/qwen)
- [OpenRouter](/providers/openrouter)
- [Vercel AI Gateway](/providers/vercel-ai-gateway)
- [Cloudflare AI Gateway](/providers/cloudflare-ai-gateway)
- [Moonshot AI（Kimi + Kimi Coding）](/providers/moonshot)
- [OpenAcosmi Zen](/providers/openacosmi)
- [Amazon Bedrock](/providers/bedrock)
- [Z.AI](/providers/zai)
- [小米 Xiaomi](/providers/xiaomi)
- [GLM 模型](/providers/glm)
- [MiniMax](/providers/minimax)
- [Venice（Venice AI，隐私优先）](/providers/venice)
- [Ollama（本地模型）](/providers/ollama)
- [千帆 Qianfan](/providers/qianfan)

## 语音转写供应商

- [Deepgram（音频转写）](/providers/deepgram)

## 社区工具

- [Claude Max API Proxy](/providers/claude-max-api-proxy) — 将 Claude Max/Pro 订阅用作 OpenAI 兼容 API 端点

有关完整供应商目录（xAI、Groq、Mistral 等）和高级配置，请参见 [模型供应商](/concepts/model-providers)。
