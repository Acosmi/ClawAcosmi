---
summary: "出站 Provider 调用的重试策略"
read_when:
  - 修改重试行为或按通道默认值
title: "重试策略"
status: active
arch: go-gateway
---

# 重试策略

> [!NOTE]
> **架构状态**：重试逻辑由 **Go Gateway** 实现（`backend/internal/outbound/`）。

## 默认行为

- **尝试次数**：3（1 次初始 + 2 次重试）
- **最大延迟**：30 秒（指数退避上限）
- **抖动**：10%（防止重试风暴）
- 可按 Provider 配置

## 配置

```json5
{
  channels: {
    telegram: {
      retry: {
        maxAttempts: 5,
        maxDelayMs: 60000,
        jitterPercent: 15,
      },
    },
  },
}
```

## Provider 特定行为

### Discord

- 仅在 HTTP 429（速率限制）时重试。
- 使用 Discord 返回的 `retry_after` 值。

### Telegram

- 在 429、超时、连接重置等瞬态错误时重试。
- Markdown 解析错误**不重试**（回退纯文本后重新发送）。

## 重要说明

- 重试是**逐请求**的。
- 组合流程（如先上传附件再发送消息）不会重试已完成的步骤。
- 快速连续失败可能触发冷却（参见[模型故障转移](/concepts/model-failover)）。
