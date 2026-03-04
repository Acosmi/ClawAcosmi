---
summary: "在 OpenAcosmi 中使用 Venice AI 隐私优先模型"
read_when:
  - 使用隐私优先推理
  - 需要 Venice AI 设置指南
title: "Venice AI"
status: active
arch: rust-cli+go-gateway
---

# Venice AI（推荐隐私方案）

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - Venice 模型定义在 **Go Gateway**（`backend/internal/agents/models/venice_models.go`）
> - API Key 环境变量 `VENICE_API_KEY` 由 Go Gateway 解析（`backend/internal/agents/models/providers.go`）
> - Onboard 流程由 **Rust CLI** 实现

**Venice** 是我们推荐的隐私优先推理方案，可选匿名访问专有模型。

Venice AI 提供隐私优先的 AI 推理，支持无审查模型和通过匿名代理访问主要专有模型。所有推理默认私密——不会基于你的数据训练，不会记录日志。

## 隐私模式

Venice 提供两个隐私级别——了解这一点是选择模型的关键：

| 模式       | 描述                                                        | 模型                                           |
| ---------- | ----------------------------------------------------------- | ---------------------------------------------- |
| **私密**   | 完全私密。提示/响应**从不存储或记录**。临时性处理。              | Llama、Qwen、DeepSeek、Venice Uncensored 等    |
| **匿名化** | 通过 Venice 代理，元数据被剥离。底层供应商看到匿名请求。         | Claude、GPT、Gemini、Grok、Kimi、MiniMax       |

## 设置

### 1. 获取 API Key

1. 在 [venice.ai](https://venice.ai) 注册
2. 进入 **Settings → API Keys → Create new key**
3. 复制 API Key（格式：`vapi_xxxxxxxxxxxx`）

### 2. 配置 OpenAcosmi

**方式 A：环境变量**

```bash
export VENICE_API_KEY="vapi_xxxxxxxxxxxx"
```

**方式 B：交互式设置（推荐）**

```bash
openacosmi onboard --auth-choice venice-api-key
```

**方式 C：非交互模式**

```bash
openacosmi onboard --non-interactive \
  --auth-choice venice-api-key \
  --venice-api-key "vapi_xxxxxxxxxxxx"
```

### 3. 验证设置

```bash
openacosmi chat --model venice/llama-3.3-70b "Hello, are you working?"
```

## 模型选择

- **默认推荐**：`venice/llama-3.3-70b` — 私密、均衡性能。
- **最佳质量**：`venice/claude-opus-45` — 适合困难任务（Opus 仍是最强模型）。
- **隐私**：选择"私密"模型获得完全私密推理。
- **能力**：选择"匿名化"模型通过 Venice 代理访问 Claude、GPT、Gemini。

切换默认模型：

```bash
openacosmi models set venice/claude-opus-45
openacosmi models set venice/llama-3.3-70b
```

列出所有可用模型：

```bash
openacosmi models list | grep venice
```

## 使用 `openacosmi configure` 配置

1. 运行 `openacosmi configure`
2. 选择 **Model/auth**
3. 选择 **Venice AI**

## 模型推荐

| 使用场景           | 推荐模型                          | 原因                              |
| ------------------ | --------------------------------- | --------------------------------- |
| **通用聊天**       | `llama-3.3-70b`                   | 全面均衡，完全私密                 |
| **最佳质量**       | `claude-opus-45`                  | Opus 在困难任务中仍是最强          |
| **编程**           | `qwen3-coder-480b-a35b-instruct`  | 代码优化，262k 上下文              |
| **视觉任务**       | `qwen3-vl-235b-a22b`              | 最佳私密视觉模型                   |
| **无审查**         | `venice-uncensored`               | 无内容限制                         |
| **快速+低成本**    | `qwen3-4b`                        | 轻量级，仍有能力                   |
| **复杂推理**       | `deepseek-v3.2`                   | 强推理能力，私密                   |

## 可用模型（共 25 个）

### 私密模型（15 个）— 完全私密，无日志

| 模型 ID                          | 名称                    | 上下文（token） | 特性                    |
| -------------------------------- | ----------------------- | --------------- | ----------------------- |
| `llama-3.3-70b`                  | Llama 3.3 70B           | 131k            | 通用                    |
| `llama-3.2-3b`                   | Llama 3.2 3B            | 131k            | 快速、轻量              |
| `hermes-3-llama-3.1-405b`        | Hermes 3 Llama 3.1 405B | 131k            | 复杂任务                |
| `qwen3-235b-a22b-thinking-2507`  | Qwen3 235B Thinking     | 131k            | 推理                    |
| `qwen3-235b-a22b-instruct-2507`  | Qwen3 235B Instruct     | 131k            | 通用                    |
| `qwen3-coder-480b-a35b-instruct` | Qwen3 Coder 480B        | 262k            | 编程                    |
| `qwen3-next-80b`                 | Qwen3 Next 80B          | 262k            | 通用                    |
| `qwen3-vl-235b-a22b`             | Qwen3 VL 235B           | 262k            | 视觉                    |
| `qwen3-4b`                       | Venice Small (Qwen3 4B) | 32k             | 快速、推理              |
| `deepseek-v3.2`                  | DeepSeek V3.2           | 163k            | 推理                    |
| `venice-uncensored`              | Venice Uncensored       | 32k             | 无审查                  |
| `mistral-31-24b`                 | Venice Medium (Mistral) | 131k            | 视觉                    |
| `google-gemma-3-27b-it`          | Gemma 3 27B Instruct    | 202k            | 视觉                    |
| `openai-gpt-oss-120b`            | OpenAI GPT OSS 120B     | 131k            | 通用                    |
| `zai-org-glm-4.7`                | GLM 4.7                 | 202k            | 推理、多语言            |

### 匿名化模型（10 个）— 通过 Venice 代理

| 模型 ID                  | 原始模型          | 上下文（token） | 特性              |
| ------------------------ | ----------------- | --------------- | ----------------- |
| `claude-opus-45`         | Claude Opus 4.5   | 202k            | 推理、视觉        |
| `claude-sonnet-45`       | Claude Sonnet 4.5 | 202k            | 推理、视觉        |
| `openai-gpt-52`          | GPT-5.2           | 262k            | 推理              |
| `openai-gpt-52-codex`    | GPT-5.2 Codex     | 262k            | 推理、视觉        |
| `gemini-3-pro-preview`   | Gemini 3 Pro      | 202k            | 推理、视觉        |
| `gemini-3-flash-preview` | Gemini 3 Flash    | 262k            | 推理、视觉        |
| `grok-41-fast`           | Grok 4.1 Fast     | 262k            | 推理、视觉        |
| `grok-code-fast-1`       | Grok Code Fast 1  | 262k            | 推理、编程        |
| `kimi-k2-thinking`       | Kimi K2 Thinking  | 262k            | 推理              |
| `minimax-m21`            | MiniMax M2.1      | 202k            | 推理              |

## 模型发现

设置 `VENICE_API_KEY` 后，OpenAcosmi 自动从 Venice API 发现模型。如果 API 不可达，会回退到静态目录。

`/models` 端点是公开的（列出不需要认证），但推理需要有效的 API Key。

## 流式传输与工具支持

| 特性             | 支持情况                                                |
| ---------------- | ------------------------------------------------------- |
| **流式传输**     | ✅ 所有模型                                             |
| **函数调用**     | ✅ 大部分模型（检查 API 中的 `supportsFunctionCalling`） |
| **视觉/图片**    | ✅ 标记为"视觉"特性的模型                                |
| **JSON 模式**    | ✅ 通过 `response_format` 支持                          |

## 配置文件示例

```json5
{
  env: { VENICE_API_KEY: "vapi_..." },
  agents: { defaults: { model: { primary: "venice/llama-3.3-70b" } } },
  models: {
    mode: "merge",
    providers: {
      venice: {
        baseUrl: "https://api.venice.ai/api/v1",
        apiKey: "${VENICE_API_KEY}",
        api: "openai-completions",
        models: [
          {
            id: "llama-3.3-70b",
            name: "Llama 3.3 70B",
            reasoning: false,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 131072,
            maxTokens: 8192,
          },
        ],
      },
    },
  },
}
```

## 故障排查

### API Key 未被识别

```bash
echo $VENICE_API_KEY
openacosmi models list | grep venice
```

确保密钥以 `vapi_` 开头。

### 模型不可用

Venice 模型目录动态更新。运行 `openacosmi models list` 查看当前可用模型。部分模型可能暂时离线。

### 连接问题

Venice API 位于 `https://api.venice.ai/api/v1`。确保你的网络允许 HTTPS 连接。

## 相关链接

- [Venice AI](https://venice.ai)
- [API 文档](https://docs.venice.ai)
- [定价](https://venice.ai/pricing)
- [状态](https://status.venice.ai)
