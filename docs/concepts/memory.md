---
summary: "记忆系统：工作区文件、自动刷写和向量搜索"
read_when:
  - 需要了解 OpenAcosmi 记忆如何工作
  - 修改记忆刷写、搜索或 embedding
title: "记忆"
status: active
arch: go-gateway
---

# 记忆

> [!IMPORTANT]
> **架构状态**：记忆系统由 **Go Gateway** 实现（`backend/internal/memory/`）。

记忆是工作区中的纯 **Markdown 文件**。模型只"记住"**写入磁盘**的内容。

## 文件布局

- `memory/YYYY-MM-DD.md` — 每日记忆日志（每天一个文件）。
- `MEMORY.md`（可选）— 精选的长期记忆。

Agent 在自然对话中通过写入这些文件存储记忆。OpenAcosmi 不强制格式，但鼓励结构化笔记。

## 写入记忆的时机

- Agent 主动决定在对话中记录有价值的信息时。
- 自动记忆刷写（压缩前的静默 Agent 轮次）。

## 自动记忆刷写

在压缩（总结旧历史）前，OpenAcosmi 可运行一次**静默记忆刷写**轮次。Agent 被提示将重要上下文存储到记忆文件，然后压缩运行。

配置：

```json5
{
  agent: {
    memory: {
      autoFlush: true,  // 默认开启
    },
  },
}
```

## 向量记忆搜索

OpenAcosmi 支持通过 embedding 向量搜索历史记忆。

### 支持的 Embedding Provider

- OpenAI（`text-embedding-3-small`）
- Google Gemini（`embedding-001`）
- Voyage AI
- 本地 embedding（自动下载的 ONNX 模型）

### 搜索工具

- `memory_search` — 语义搜索记忆文件。
- `memory_get` — 获取特定日期的记忆条目。

### 配置

```json5
{
  agent: {
    memory: {
      search: {
        enabled: true,
        provider: "openai",  // 或 "gemini" | "voyage" | "local"
      },
    },
  },
}
```

## 混合搜索

结合 BM25（关键词）和向量相似度的混合搜索模式。

## Embedding 缓存

- Embedding 结果缓存到按文件的 `.cache` 文件以避免重复计算。
- 缓存在文件内容变化时自动失效。

## 本地 Embedding

- 默认使用 ONNX 格式的轻量模型。
- 首次使用时自动下载到 `~/.openacosmi/models/`。
- 无需外部 API 调用。

## 自定义 OpenAI 兼容端点

```json5
{
  agent: {
    memory: {
      search: {
        provider: "custom",
        apiBase: "http://localhost:8080/v1",
        model: "my-embedding-model",
      },
    },
  },
}
```

## 会话记忆搜索（实验性）

实验性功能：搜索会话转录历史（不仅是记忆文件）。

## 记忆 vs 上下文

- **记忆**：持久化在磁盘上的 Markdown 文件，跨会话生存。
- **上下文**：当前会话窗口中模型可见的内容。
- 记忆通过 bootstrap 注入或搜索工具进入上下文。
