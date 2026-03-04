---
summary: "模型 Provider 概览及配置示例"
read_when:
  - 添加新 Provider 或配置认证
  - 排查模型调用失败问题
title: "模型 Provider"
status: active
arch: go-gateway+rust-cli
---

# 模型 Provider

> [!NOTE]
> **架构状态**：Provider 调用由 **Go Gateway** 执行（`backend/internal/agents/`）。
> Rust CLI 通过 `openacosmi auth`、`openacosmi models` 管理 Provider 配置。

## 内置 Provider

### Anthropic（Claude）

认证：OAuth（`openacosmi auth`）或 API 密钥（`ANTHROPIC_API_KEY`）。

```bash
openacosmi auth          # OAuth 登录
openacosmi models set anthropic/claude-sonnet-4-20250514
```

### OpenAI

认证：API 密钥（`OPENAI_API_KEY`）。

```bash
openacosmi models set openai/gpt-4.1
```

### OpenAI Codex（ChatGPT OAuth）

认证：OAuth（`openacosmi auth --provider codex`）。

### Google Gemini（API Key）

认证：`GEMINI_API_KEY`。

```bash
openacosmi models set google/gemini-2.5-pro
```

### Google Vertex / Antigravity / Gemini CLI

认证：OAuth（`openacosmi auth --provider vertex`）或 `GOOGLE_APPLICATION_CREDENTIALS`。

### Z.AI（GLM）

认证：API 密钥（`ZAI_API_KEY`）。

```bash
openacosmi models set zai/glm-4-plus
```

### OpenRouter

认证：`OPENROUTER_API_KEY`。支持免费模型扫描：

```bash
openacosmi models scan
```

### 其他内置

- **xAI**（Grok）：`XAI_API_KEY`
- **Groq**：`GROQ_API_KEY`
- **Cerebras**：`CEREBRAS_API_KEY`
- **Mistral**：`MISTRAL_API_KEY`
- **GitHub Copilot**：OAuth（`openacosmi auth --provider copilot`）
- **Vercel AI Gateway**：`VERCEL_API_KEY`

## 自定义 Provider（`models.providers`）

通过配置添加任何 OpenAI 兼容的端点：

### Moonshot AI（Kimi）

```json5
{
  models: {
    providers: {
      kimi: {
        apiBase: "https://api.moonshot.cn/v1",
        apiKeyEnv: "MOONSHOT_API_KEY",
      },
    },
  },
}
```

### Qwen Portal（通义千问）

```json5
{
  models: {
    providers: {
      qwen: {
        apiBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
        apiKeyEnv: "DASHSCOPE_API_KEY",
      },
    },
  },
}
```

### MiniMax

```json5
{
  models: {
    providers: {
      minimax: {
        apiBase: "https://api.minimax.chat/v1",
        apiKeyEnv: "MINIMAX_API_KEY",
      },
    },
  },
}
```

### 本地代理（Ollama / LM Studio / vLLM）

```json5
{
  models: {
    providers: {
      local: {
        apiBase: "http://localhost:11434/v1",
        apiKeyEnv: "OLLAMA_API_KEY",
      },
    },
  },
}
```

## 模型引用格式

- 格式：`provider/model`（如 `anthropic/claude-sonnet-4-20250514`）
- 省略 provider 时视为别名或默认 Provider，仅在模型 ID 不含 `/` 时有效

## CLI 命令参考

```bash
openacosmi auth                      # 认证向导
openacosmi models list               # 列出可用模型
openacosmi models set <model>        # 设置主模型
openacosmi models status             # 查看 Profile 状态
```
