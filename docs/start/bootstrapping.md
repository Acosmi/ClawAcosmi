---
summary: "Agent 引导仪式：初始化工作区和身份文件"
read_when:
  - 了解 Agent 首次运行时发生的事情
  - 解释引导文件的存放位置
  - 调试引导身份设置
title: "Agent 引导"
sidebarTitle: "引导"
status: active
arch: rust-cli+go-gateway
---

# Agent 引导

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> 引导由 **Go Gateway** 在 Agent 首次启动时执行。

引导是**首次运行**时的初始化仪式，准备 Agent 工作区并收集身份信息。它在引导向导完成后、Agent 首次启动时发生。

## 引导做了什么

Agent 首次运行时，Go Gateway 引导工作区（默认 `~/.openacosmi/workspace`）：

- 初始化 `AGENTS.md`、`BOOTSTRAP.md`、`IDENTITY.md`、`USER.md`。
- 运行简短的问答仪式（逐个提问）。
- 将身份和偏好写入 `IDENTITY.md`、`USER.md`、`SOUL.md`。
- 完成后移除 `BOOTSTRAP.md`，确保仅运行一次。

## 在哪里运行

引导始终在 **Gateway 主机**上运行。如果 macOS 客户端连接到远程 Gateway，工作区和引导文件位于该远程机器上。

<Note>
当 Gateway 运行在另一台机器上时，请在 Gateway 主机上编辑工作区文件（例如 `user@gateway-host:~/.openacosmi/workspace`）。
</Note>

## 相关文档

- macOS 应用引导：[引导](/start/onboarding)
- 工作区布局：[Agent 工作区](/concepts/agent-workspace)
