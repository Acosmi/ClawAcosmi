---
summary: "PeekabooBridge UI 自动化协议和使用"
read_when:
  - 实现或调试 UI 自动化
  - 集成 PeekabooBridge
title: "PeekabooBridge"
---

# PeekabooBridge（UI 自动化）

## 概览

PeekabooBridge 是通过 UNIX socket 和 JSON 协议实现的 UI 自动化桥接。
它允许 agent 通过 Go Gateway 的节点命令驱动 macOS UI 操作。

## 架构

```
Agent -> Go Gateway -> 节点主机 -> UNIX socket (bridge.sock) -> macOS 应用
```

## 主机优先级

客户端按以下顺序查找 PeekabooBridge 主机：

1. Peekaboo.app（如安装）
2. Claude.app（如安装）
3. OpenAcosmi.app
4. 本地直接执行

## 安全性

- Bridge 主机需要匹配的 TeamID。
- 调试模式：`PEEKABOO_ALLOW_UNSIGNED_SOCKET_CLIENTS=1`（仅开发用，允许同 UID 调用者）。
- 所有通信仅限本地；无网络 socket 暴露。

## Socket 路径

```
~/Library/Application Support/OpenAcosmi/bridge.sock
```

## 协议

JSON-over-UNIX-socket，支持的操作：

- 屏幕元素查询
- 点击和键入操作
- 窗口管理

详情参见 PeekabooBridge 协议文档。
