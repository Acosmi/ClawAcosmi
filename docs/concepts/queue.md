---
summary: "命令队列设计：序列化入站自动回复运行"
read_when:
  - 修改队列模式或并发行为
  - 排查消息排队或丢失问题
title: "命令队列"
status: active
arch: go-gateway
---

# 命令队列

> [!NOTE]
> **架构状态**：命令队列由 **Go Gateway** 实现（`backend/internal/agents/runner/queue.go`）。

## 目的

防止多条消息同时到达时导致的 Agent 运行冲突。进程内 FIFO 队列按会话键串行化运行。

## 工作方式

- 按**会话键**的通道串行化：同一会话同时只能有一个 Agent 运行。
- **全局通道并发上限**：限制跨会话的总并行运行数。
- 排队的消息在当前运行结束后执行。

## 队列模式

- `steer`：注入当前运行（在下一个工具调用后引入）。
- `followup`：排队等当前运行结束后开始新一轮。
- `collect`（默认）：合并所有排队消息为单轮跟进。
- `steer-backlog`：如同 steer，但溢出到 collect。
- `interrupt`（旧版）：中止当前运行并开始新一轮。

## 队列选项

```json5
{
  queue: {
    mode: "collect",
    debounceMs: 1500,    // 合并延迟
    cap: 10,             // 最大排队消息数
    drop: "old",         // 超出时丢弃：old / new / summarize
  },
}
```

## Per-Session 覆盖

在聊天中使用 `/queue <mode>` 临时覆盖当前会话的队列模式。

## 作用域保证

- 队列状态不跨 Gateway 重启持久化（进程内）。
- Gateway 重启清空所有排队消息。
