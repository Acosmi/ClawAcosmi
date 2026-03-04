---
summary: "上下文窗口与压缩：OpenAcosmi 如何让会话保持在模型限制内"
read_when:
  - 需要了解自动压缩和 /compact 命令
  - 调试长会话触及上下文限制
title: "压缩（Compaction）"
status: active
arch: go-gateway
---

# 上下文窗口与压缩

> [!NOTE]
> **架构状态**：压缩逻辑由 **Go Gateway** 实现（`backend/internal/agents/runner/`）。
> Rust CLI 不参与压缩处理。

每个模型都有一个**上下文窗口**（可看到的最大 token 数）。长时间运行的聊天会累积消息和工具结果；当窗口接近满载时，OpenAcosmi 会**压缩**旧历史以保持在限制内。

## 什么是压缩

压缩将**旧对话总结**为紧凑的摘要条目，保持近期消息不变。摘要存储在会话历史中，后续请求使用：

- 压缩摘要
- 压缩点之后的近期消息

压缩会**持久化**在会话的 JSONL 历史中。

## 配置

参见 [压缩配置与模式](/concepts/compaction) 中的 `agents.defaults.compaction` 设置。

## 自动压缩（默认开启）

当会话接近或超过模型的上下文窗口时，OpenAcosmi 触发自动压缩，并可使用压缩后的上下文重试原始请求。

你会看到：

- 详细模式下的 `🧹 自动压缩完成`
- `/status` 显示 `🧹 压缩次数: <count>`

压缩前，OpenAcosmi 可运行一次**静默记忆刷写**轮次，将持久笔记存储到磁盘。参见 [记忆](/concepts/memory)。

## 手动压缩

使用 `/compact`（可选附加指令）强制执行压缩：

```
/compact 聚焦于决策和未解问题
```

## 上下文窗口来源

上下文窗口是模型特定的。OpenAcosmi 使用配置的 Provider 目录中的模型定义来确定限制。

## 压缩 vs 剪枝

- **压缩**：总结并**持久化**在 JSONL 中。
- **会话剪枝**：仅修剪旧的**工具结果**，**在内存中**按请求处理。

参见 [会话剪枝](/concepts/session-pruning)。

## 提示

- 当会话感觉过时或上下文膨胀时使用 `/compact`。
- 大工具输出已被截断；剪枝可进一步减少工具结果堆积。
- 如需全新开始，`/new` 或 `/reset` 启动新会话 ID。
