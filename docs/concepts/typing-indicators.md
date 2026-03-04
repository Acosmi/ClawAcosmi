---
summary: "输入指示器：何时显示以及如何配置"
read_when:
  - 修改输入指示器行为或默认值
title: "输入指示器"
status: active
arch: go-gateway
---

# 输入指示器

> [!NOTE]
> **架构状态**：输入指示器由 **Go Gateway** 实现（`backend/internal/outbound/`）。

运行活跃时向聊天通道发送输入指示器。通过 `agents.defaults.typingMode` 控制**何时**开始，`typingIntervalSeconds` 控制**刷新频率**。

## 默认行为

未设置 `typingMode` 时，OpenAcosmi 保持旧版行为：

- **私聊**：模型循环开始后立即显示。
- **带 @提及的群聊**：立即显示。
- **无 @提及的群聊**：消息文本开始流式时才显示。
- **心跳运行**：不显示。

## 模式

设置 `agents.defaults.typingMode`：

- `never` — 永不显示输入指示。
- `instant` — 模型循环开始后**立即**显示，即使运行最终只返回静默令牌。
- `thinking` — **首次推理增量**时显示（需要 `reasoningLevel: "stream"`）。
- `message` — **首次非静默文本增量**时显示（忽略 `NO_REPLY` 静默令牌）。

触发顺序（从早到晚）：`instant` → `thinking` → `message` → `never`。

## 配置

```json5
{
  agent: {
    typingMode: "thinking",
    typingIntervalSeconds: 6,
  },
}
```

Per-Session 覆盖：

```json5
{
  session: {
    typingMode: "message",
    typingIntervalSeconds: 4,
  },
}
```

## 注意事项

- `message` 模式不会为静默回复（`NO_REPLY`）显示输入指示。
- `thinking` 仅在运行流式推理（`reasoningLevel: "stream"`）时触发。
- 心跳运行永不显示输入指示，无论模式如何。
- `typingIntervalSeconds` 控制**刷新频率**，非开始时间。默认 6 秒。
