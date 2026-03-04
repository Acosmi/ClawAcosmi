---
summary: "Telegram 白名单加固：前缀 + 空白字符规范化"
read_when:
  - 查看历史 Telegram 白名单变更
title: "Telegram 白名单加固"
status: complete
arch: go-gateway
---

# Telegram 白名单加固

> [!NOTE]
> **架构状态**：白名单规范化逻辑由 **Go Gateway**（`backend/internal/channels/`）实现。

**日期**：2026-01-05
**状态**：已完成
**PR**：#216

## 摘要

Telegram 白名单现在不区分大小写地接受 `telegram:` 和 `tg:` 前缀，并容忍意外的空白字符。这使入站白名单检查与出站发送规范化保持一致。

## 变更内容

- 前缀 `telegram:` 和 `tg:` 被等同处理（不区分大小写）。
- 白名单条目会被修剪空白；空条目被忽略。

## 示例

以下所有格式都被接受为同一 ID：

- `telegram:123456`
- `TG:123456`
- `tg:123456`

## 为何重要

从日志或聊天 ID 复制粘贴时经常包含前缀和空白字符。规范化处理可以避免在判断是否在私聊或群组中回复时出现误判。

## 相关文档

- [群组](/channels/groups)
- [Telegram Provider](/channels/telegram)
