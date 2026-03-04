---
summary: "macOS 应用内嵌 WebChat 的工作原理和调试方法"
read_when:
  - 调试 macOS WebChat 视图或回环端口
title: "WebChat"
---

# WebChat（macOS 应用）

macOS 菜单栏应用将 WebChat UI 嵌入为原生 SwiftUI 视图。它连接到 Go Gateway 并默认使用选定 agent 的**主会话**（支持会话切换器切换其他会话）。

- **本地模式**：直接连接到本地 Go Gateway WebSocket。
- **远程模式**：通过 SSH 转发 Gateway 控制端口，使用该隧道作为数据面。

## 启动和调试

- 手动：菜单栏 → "打开聊天"。
- 测试用自动打开：

  ```bash
  dist/OpenAcosmi.app/Contents/MacOS/OpenAcosmi --webchat
  ```

- 日志：`./scripts/clawlog.sh`（子系统 `bot.molt`，分类 `WebChatSwiftUI`）。

## 工作原理

- 数据面：Gateway WebSocket 方法 `chat.history`、`chat.send`、`chat.abort`、
  `chat.inject` 和事件 `chat`、`agent`、`presence`、`tick`、`health`。
- 会话：默认主会话（`main`，或作用域为全局时为 `global`）。UI 可切换会话。
- 引导使用专用会话以将首次运行设置与常规使用分开。

## 安全

- 远程模式仅通过 SSH 转发 Gateway WebSocket 控制端口。

## 已知限制

- UI 针对聊天会话优化（非完整浏览器沙箱）。
