---
summary: "出站消息的 Markdown 格式化管道"
read_when:
  - 修改 Markdown 渲染或分块行为
  - 添加新通道的格式化器
title: "Markdown 格式化"
status: active
arch: go-gateway
---

# Markdown 格式化

> [!NOTE]
> **架构状态**：Markdown 格式化管道由 **Go Gateway** 实现（`backend/internal/outbound/`）。

OpenAcosmi 将模型输出的 Markdown 转换为各通道支持的格式。

## 管道

```
模型输出（Markdown）
  → 解析为中间表示（IR）
  → 分块（按通道限制）
  → 通道特定渲染
  → 发送
```

## 中间表示（IR）

IR 是纯文本加样式/链接的 span 结构：

- 纯文本 + 样式 span（bold、italic、code、strikethrough）
- 链接 span（URL + 显示文本）
- 代码块（保持完整）

IR 使用 **UTF-16 偏移量**以兼容 Signal API。

## 通道渲染

| 通道 | 格式 |
|------|------|
| Slack | mrkdwn（Slack 标记语言） |
| Telegram | HTML |
| Signal | 纯文本 + style ranges |
| Discord | Markdown（原生支持） |
| WhatsApp | WhatsApp 格式化 |
| 飞书 | Markdown |
| 钉钉 | Markdown |

## 分块

- 在渲染前对 IR 文本分块。
- 内联格式不跨块断开。
- 代码围栏保护：不在代码块内断开。
- 断行时关闭并重新打开代码围栏以保持 Markdown 合法。

## 表格处理

三种模式：

- `code`（默认）：表格渲染为等宽代码块。
- `bullets`：表格渲染为项目符号列表。
- `off`：表格按原始 Markdown 传递。

配置：`agents.defaults.tableMode`

## 添加新通道格式化器

1. 在 `backend/internal/outbound/` 下实现通道特定的渲染器。
2. 在格式化管道中注册。
3. 处理 IR 类型（文本、样式、链接、代码块）。
