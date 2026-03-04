---
summary: "在 OpenAcosmi 中使用 Ollama（本地 LLM 运行时）"
read_when:
  - 使用本地模型运行 OpenAcosmi
  - 需要 Ollama 设置和配置指南
title: "Ollama"
status: active
arch: rust-cli+go-gateway
---

# Ollama

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - Ollama 供应商默认配置：**Go Gateway**（`backend/internal/agents/models/providers.go` — `ollama`，Base URL: `http://127.0.0.1:11434/v1`）
> - API Key 环境变量：`OLLAMA_API_KEY`
> - 模型自动发现由 Go Gateway 实现

Ollama 是一个本地 LLM 运行时，可以轻松在你的机器上运行开源模型。OpenAcosmi 与 Ollama 的 OpenAI 兼容 API 集成，并且在你使用 `OLLAMA_API_KEY`（或认证 profile）且未定义显式 `models.providers.ollama` 条目时，可以**自动发现支持工具的模型**。

## 快速开始

1. 安装 Ollama：[https://ollama.ai](https://ollama.ai)

2. 拉取模型：

```bash
ollama pull gpt-oss:20b
# 或
ollama pull llama3.3
# 或
ollama pull qwen2.5-coder:32b
# 或
ollama pull deepseek-r1:32b
```

1. 为 OpenAcosmi 启用 Ollama（任意值即可；Ollama 不需要真正的密钥）：

```bash
# 设置环境变量
export OLLAMA_API_KEY="ollama-local"

# 或在配置文件中设置
openacosmi config set models.providers.ollama.apiKey "ollama-local"
```

1. 使用 Ollama 模型：

```json5
{
  agents: {
    defaults: {
      model: { primary: "ollama/gpt-oss:20b" },
    },
  },
}
```

## 模型发现（隐式供应商）

设置 `OLLAMA_API_KEY`（或认证 profile）且**未**定义 `models.providers.ollama` 时，OpenAcosmi 从本地 Ollama 实例（`http://127.0.0.1:11434`）发现模型：

- 查询 `/api/tags` 和 `/api/show`
- 仅保留报告 `tools` 能力的模型
- 当模型报告 `thinking` 时标记 `reasoning`
- 从 `model_info["<arch>.context_length"]` 读取 `contextWindow`（如可用）
- `maxTokens` 设为上下文窗口的 10 倍
- 所有费用设为 `0`

这避免了手动模型条目，同时保持目录与 Ollama 能力对齐。

查看可用模型：

```bash
ollama list
openacosmi models list
```

添加新模型只需用 Ollama 拉取：

```bash
ollama pull mistral
```

新模型将被自动发现并可用。

如果显式设置了 `models.providers.ollama`，自动发现将被跳过，需手动定义模型（见下方）。

## 配置

### 基本设置（隐式发现）

启用 Ollama 最简单的方式是通过环境变量：

```bash
export OLLAMA_API_KEY="ollama-local"
```

### 显式设置（手动模型）

在以下情况使用显式配置：

- Ollama 运行在其他主机/端口。
- 需要强制指定上下文窗口或模型列表。
- 需要包含不报告工具支持的模型。

```json5
{
  models: {
    providers: {
      ollama: {
        // 使用包含 /v1 的主机地址（OpenAI 兼容 API）
        baseUrl: "http://ollama-host:11434/v1",
        apiKey: "ollama-local",
        api: "openai-completions",
        models: [
          {
            id: "gpt-oss:20b",
            name: "GPT-OSS 20B",
            reasoning: false,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 8192,
            maxTokens: 81920
          }
        ]
      }
    }
  }
}
```

如果设置了 `OLLAMA_API_KEY`，可以省略供应商条目中的 `apiKey`，OpenAcosmi 会自动填充以进行可用性检查。

### 自定义 Base URL（显式配置）

如果 Ollama 运行在不同的主机或端口（显式配置会禁用自动发现，需手动定义模型）：

```json5
{
  models: {
    providers: {
      ollama: {
        apiKey: "ollama-local",
        baseUrl: "http://ollama-host:11434/v1",
      },
    },
  },
}
```

### 模型选择

配置完成后，所有 Ollama 模型均可用：

```json5
{
  agents: {
    defaults: {
      model: {
        primary: "ollama/gpt-oss:20b",
        fallbacks: ["ollama/llama3.3", "ollama/qwen2.5-coder:32b"],
      },
    },
  },
}
```

## 高级

### 推理模型

当 Ollama 在 `/api/show` 中报告 `thinking` 时，OpenAcosmi 将模型标记为推理能力：

```bash
ollama pull deepseek-r1:32b
```

### 模型费用

Ollama 免费且在本地运行，所有模型费用设为 $0。

### 流式配置

由于底层 SDK 与 Ollama 响应格式存在[已知问题](https://github.com/badlogic/pi-mono/issues/1205)，Ollama 模型**默认禁用流式传输**。这可以防止使用工具模型时出现损坏的响应。

禁用流式传输后，响应将一次性交付（非流式模式），避免交错的内容/推理增量导致输出混乱。

#### 重新启用流式传输（高级）

如果要为 Ollama 重新启用流式传输（可能导致工具模型出现问题）：

```json5
{
  agents: {
    defaults: {
      models: {
        "ollama/gpt-oss:20b": {
          streaming: true,
        },
      },
    },
  },
}
```

#### 为其他供应商禁用流式传输

如需要，也可以为任何供应商禁用流式传输：

```json5
{
  agents: {
    defaults: {
      models: {
        "openai/gpt-4": {
          streaming: false,
        },
      },
    },
  },
}
```

### 上下文窗口

对于自动发现的模型，OpenAcosmi 使用 Ollama 报告的上下文窗口（如可用），否则默认为 `8192`。可以在显式供应商配置中覆盖 `contextWindow` 和 `maxTokens`。

## 故障排查

### Ollama 未被检测到

确保 Ollama 正在运行，已设置 `OLLAMA_API_KEY`（或认证 profile），且**未**定义显式 `models.providers.ollama` 条目：

```bash
ollama serve
```

并确保 API 可访问：

```bash
curl http://localhost:11434/api/tags
```

### 无可用模型

OpenAcosmi 仅自动发现报告工具支持的模型。如果你的模型未列出：

- 拉取一个支持工具的模型，或
- 在 `models.providers.ollama` 中显式定义该模型。

添加模型：

```bash
ollama list          # 查看已安装的模型
ollama pull gpt-oss:20b  # 拉取工具模型
ollama pull llama3.3     # 或其他模型
```

### 连接被拒绝

检查 Ollama 是否运行在正确端口：

```bash
# 检查 Ollama 是否运行
ps aux | grep ollama

# 或重启 Ollama
ollama serve
```

### 响应损坏或输出中出现工具名称

如果使用 Ollama 模型时看到包含工具名称（如 `sessions_send`、`memory_get`）的混乱响应或碎片文本，这是由于上游 SDK 的流式响应问题。最新版 OpenAcosmi 已**默认修复**此问题（禁用 Ollama 模型的流式传输）。

如果手动启用了流式传输并遇到此问题：

1. 移除 Ollama 模型条目中的 `streaming: true` 配置，或
2. 为 Ollama 模型显式设置 `streaming: false`（参见[流式配置](#流式配置)）

## 另请参阅

- [模型供应商](/concepts/model-providers) — 所有供应商概览
- [模型选择](/concepts/models) — 如何选择模型
- [配置](/gateway/configuration) — 完整配置参考
