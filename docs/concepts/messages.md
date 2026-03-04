---
summary: "消息流：入站去重、会话路由、队列和推理可见性"
read_when:
  - 修改入站消息处理或出站回复管道
title: "消息流"
status: active
arch: go-gateway
---

# 消息流

> [!NOTE]
> **架构状态**：消息处理由 **Go Gateway** 实现（`backend/internal/autoreply/`、`backend/internal/outbound/`）。

## 高层流程

```
入站消息
  → 路由/绑定 → 选择 Agent
  → 会话键解析
  → 命令队列（串行化）
  → Agent 运行（模型 + 工具）
  → 出站回复
```

## 入站去重

Gateway 维护短时去重缓存。通道重连或重试时重复的消息 ID 被静默丢弃。

## 入站防抖

按通道可配置的消息合并延迟。快速连续的消息在触发 Agent 运行前合并：

```json5
{
  queue: {
    debounceMs: 1500,  // 等待 1.5 秒后再触发
  },
}
```

## 会话与设备

- Gateway 拥有会话状态。
- 多设备（手机 + 桌面）可映射到同一会话键。
- 会话历史在设备间可能不完全同步，但 Gateway 维护权威记录。

## 入站消息体

Gateway 为每条入站消息构建多种表示：

- **Body**：最终发给模型的消息（包含 envelope 头、引用上下文等）。
- **CommandBody**：提取的斜杠命令。
- **RawBody**：原始文本（不含信封包装）。

## 队列与跟进模式

参见 [命令队列](/concepts/queue)。

## 流式、分块与批处理

参见 [流式与分块](/concepts/streaming)。

## 推理可见性

- `/reasoning off`：不发送推理内容（默认）。
- `/reasoning text`：推理作为文本附加到回复。
- `/reasoning stream`：推理通过 Draft 气泡流式传输（仅 Telegram）。

## 出站消息前缀

- 工具摘要前缀（如 `🔧 exec ...`）。
- Block streaming 的回复片段。
- 推理内容（启用时）。

## 回复格式

- **普通回复**：文本消息。
- **线程回复**：Slack/Discord 线程或 Telegram 论坛主题。
- **引用回复**：使用 `replyToId` 引用原始消息（支持的通道）。
