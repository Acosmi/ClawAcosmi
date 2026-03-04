---
summary: "OpenAcosmi 功能特性总览"
read_when:
  - 快速了解系统支持的功能
title: "功能特性"
status: active
arch: rust-cli+go-gateway
---

# 功能特性

> [!NOTE]
> **架构状态**：功能由 **Go Gateway**（服务端）和 **Rust CLI**（客户端）共同提供。

## 通道

- WhatsApp（via Baileys）
- Telegram（via grammY）
- Discord
- Slack
- Signal
- iMessage
- 飞书（Feishu）
- 钉钉（DingTalk）
- 企业微信（WeCom）
- WebChat（内置 Web UI 通道）

## 应用

- **Web Control UI**：基于 Web 的管理面板（`ui/`）
- **macOS 菜单栏应用**：快速访问 Agent 状态
- **Rust CLI**：完整命令行工具（`openacosmi`）

## 路由

- 多 Agent 路由与隔离（bindings）
- 按通道/peer/群组绑定到不同 Agent
- 广播组（多 Agent 并行）

## 媒体

- 图片输入/输出（各通道）
- 音频转录（STT）
- 文档处理
- 图片生成工具

## Agent 能力

- 内置工具（read/exec/edit/write）
- Skills 系统（工作区/托管/捆绑）
- 记忆系统（文件记忆 + 向量搜索）
- 心跳运行（定时检查）
- Block streaming 和 Draft streaming

## 移动节点

- iOS/Android 节点连接
- Canvas 支持（交互式 HTML UI）
- 设备配对系统

## 其他

- 订阅认证
- 群聊 @提及激活
- 沙箱执行
- OAuth 多账户支持
- 模型故障转移和回退
