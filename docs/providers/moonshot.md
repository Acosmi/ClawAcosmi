---
summary: "配置 Moonshot K2 与 Kimi Coding（独立供应商和密钥）"
read_when:
  - 配置 Moonshot K2（Moonshot 开放平台）与 Kimi Coding
  - 了解独立端点、密钥和模型引用
  - 需要复制粘贴的配置
title: "Moonshot AI"
status: active
arch: rust-cli+go-gateway
---

# Moonshot AI（Kimi）

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - Moonshot 供应商默认配置：**Go Gateway**（`backend/internal/agents/models/providers.go` — `moonshot`，Base URL: `https://api.moonshot.ai/v1`，默认模型: `kimi-k2.5`）
> - API Key 环境变量：`MOONSHOT_API_KEY`（Kimi Coding 回退：`KIMI_CODING_API_KEY`）
> - Onboard 流程：**Rust CLI**（`cli-rust/crates/oa-cmd-onboard/src/auth/models.rs` 中定义模型常量）

Moonshot 提供 Kimi API，带有 OpenAI 兼容端点。配置供应商并设置默认模型为 `moonshot/kimi-k2.5`，或使用 Kimi Coding 的 `kimi-coding/k2p5`。

当前 Kimi K2 模型 ID：

- `kimi-k2.5`
- `kimi-k2-0905-preview`
- `kimi-k2-turbo-preview`
- `kimi-k2-thinking`
- `kimi-k2-thinking-turbo`

```bash
openacosmi onboard --auth-choice moonshot-api-key
```

Kimi Coding：

```bash
openacosmi onboard --auth-choice kimi-code-api-key
```

注意：Moonshot 和 Kimi Coding 是独立的供应商。密钥不可互换，端点不同，模型引用也不同（Moonshot 使用 `moonshot/...`，Kimi Coding 使用 `kimi-coding/...`）。

## 配置示例（Moonshot API）

```json5
{
  env: { MOONSHOT_API_KEY: "sk-..." },
  agents: {
    defaults: {
      model: { primary: "moonshot/kimi-k2.5" },
      models: {
        "moonshot/kimi-k2.5": { alias: "Kimi K2.5" },
        "moonshot/kimi-k2-0905-preview": { alias: "Kimi K2" },
        "moonshot/kimi-k2-turbo-preview": { alias: "Kimi K2 Turbo" },
        "moonshot/kimi-k2-thinking": { alias: "Kimi K2 Thinking" },
        "moonshot/kimi-k2-thinking-turbo": { alias: "Kimi K2 Thinking Turbo" },
      },
    },
  },
  models: {
    mode: "merge",
    providers: {
      moonshot: {
        baseUrl: "https://api.moonshot.ai/v1",
        apiKey: "${MOONSHOT_API_KEY}",
        api: "openai-completions",
        models: [
          {
            id: "kimi-k2.5",
            name: "Kimi K2.5",
            reasoning: false,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 256000,
            maxTokens: 8192,
          },
          {
            id: "kimi-k2-0905-preview",
            name: "Kimi K2 0905 Preview",
            reasoning: false,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 256000,
            maxTokens: 8192,
          },
          {
            id: "kimi-k2-turbo-preview",
            name: "Kimi K2 Turbo",
            reasoning: false,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 256000,
            maxTokens: 8192,
          },
          {
            id: "kimi-k2-thinking",
            name: "Kimi K2 Thinking",
            reasoning: true,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 256000,
            maxTokens: 8192,
          },
          {
            id: "kimi-k2-thinking-turbo",
            name: "Kimi K2 Thinking Turbo",
            reasoning: true,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 256000,
            maxTokens: 8192,
          },
        ],
      },
    },
  },
}
```

## Kimi Coding

```json5
{
  env: { KIMI_API_KEY: "sk-..." },
  agents: {
    defaults: {
      model: { primary: "kimi-coding/k2p5" },
      models: {
        "kimi-coding/k2p5": { alias: "Kimi K2.5" },
      },
    },
  },
}
```

## 注意事项

- Moonshot 模型引用使用 `moonshot/<modelId>`，Kimi Coding 模型引用使用 `kimi-coding/<modelId>`。
- 如需要，可在 `models.providers` 中覆盖定价和上下文元数据。
- 如果 Moonshot 为模型发布了不同的上下文限制，请相应调整 `contextWindow`。
- 国际端点使用 `https://api.moonshot.ai/v1`，中国大陆端点使用 `https://api.moonshot.cn/v1`。
