---
summary: "在本地 LLM 上运行 OpenAcosmi（LM Studio、vLLM、LiteLLM）"
read_when:
  - 使用本地 GPU 服务模型
  - 接入 LM Studio 或 OpenAI 兼容代理
title: "本地模型"
---

# 本地模型

> [!IMPORTANT]
> **架构状态**：本地模型由 **Go Gateway** 的模型路由系统支持，
> 通过 `models.providers` 配置自定义端点。

本地可行，但 OpenAcosmi 需要大上下文 + 强防注入能力。建议 **≥2 台满配 Mac Studio 或等效 GPU**。
使用**最大/完整模型变体**；激进量化提高注入风险。

## 推荐：LM Studio + MiniMax M2.1

```json5
{
  agents: {
    defaults: {
      model: { primary: "lmstudio/minimax-m2.1-gs32" },
    },
  },
  models: {
    mode: "merge",
    providers: {
      lmstudio: {
        baseUrl: "http://127.0.0.1:1234/v1",
        apiKey: "lmstudio",
        api: "openai-responses",
        models: [{
          id: "minimax-m2.1-gs32",
          name: "MiniMax M2.1 GS32",
          cost: { input: 0, output: 0 },
          contextWindow: 196608,
        }],
      },
    },
  },
}
```

## 混合配置：托管主力 + 本地回退

```json5
{
  agents: {
    defaults: {
      model: {
        primary: "anthropic/claude-sonnet-4-5",
        fallbacks: ["lmstudio/minimax-m2.1-gs32"],
      },
    },
  },
}
```

保持 `models.mode: "merge"` 使托管模型作为回退可用。

## 其他 OpenAI 兼容代理

vLLM、LiteLLM 等只需暴露 `/v1` 端点即可接入。

## 故障排除

- Gateway 能访问代理？`curl http://127.0.0.1:1234/v1/models`
- 模型未加载？重新加载；冷启动是常见"挂起"原因。
- 本地模型跳过提供商侧过滤；保持 agent 范围窄并启用压缩。
