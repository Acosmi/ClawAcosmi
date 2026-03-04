---
summary: "高级设置和开发工作流"
read_when:
  - 设置新开发机器
  - 想要最新功能但不破坏个人配置
title: "高级设置"
status: active
arch: rust-cli+go-gateway
---

# 高级设置

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - **Rust CLI**（`openacosmi`）：用户交互、命令解析、TUI、守护进程管理
> - **Go Gateway**（`acosmi`）：服务端逻辑、通道适配、Agent 执行
> - **前端 UI**：Vite + Lit，通过 `npm run dev` 启动

<Note>
如果是首次设置，请从 [快速开始](/start/getting-started) 开始。
向导详情见 [引导向导](/start/wizard)。
</Note>

最后更新：2026-03-01

## TL;DR

- **定制文件存放在仓库外：** `~/.openacosmi/workspace`（工作区）+ `~/.openacosmi/openacosmi.json`（配置）。
- **稳定工作流：** 安装 macOS 应用；让它运行内置的 Go Gateway。
- **前沿工作流：** 自己运行 Gateway（`make gateway-dev`），让 macOS 应用以 Local 模式连接。

## 前置条件（从源码）

- Go `>=1.22`
- Rust `>=1.75`（构建 CLI 和 Argus）
- Node.js（仅用于前端 UI 开发）
- Docker（可选；仅用于容器化部署 — 见 [Docker](/install/docker)）

## 定制策略（避免更新影响）

如果想要"100% 定制" _同时_ 方便更新，将自定义内容存放在：

- **配置：** `~/.openacosmi/openacosmi.json`（JSON/JSON5 格式）
- **工作区：** `~/.openacosmi/workspace`（技能、提示词、记忆文件；建议设为私有 git 仓库）

首次初始化：

```bash
openacosmi setup
```

## 从源码运行 Gateway

构建并运行 Go Gateway：

```bash
cd backend && make gateway
```

或使用开发模式（端口 19001，自动重载）：

```bash
make gateway-dev
```

生产环境运行（默认端口 18789）：

```bash
cd backend && ./build/acosmi --port 18789
```

## 稳定工作流（macOS 应用优先）

1. 安装并启动 **OpenAcosmi.app**（菜单栏）。
2. 完成引导/权限检查表（TCC 提示）。
3. 确保 Gateway 为 **Local** 模式且正在运行（应用管理 Go Gateway 进程）。
4. 连接通道（例如 WhatsApp）：

```bash
openacosmi channels login
```

1. 健康检查：

```bash
openacosmi health
```

如果引导流程不可用：

- 运行 `openacosmi setup`，然后 `openacosmi channels login`，最后通过 `openacosmi gateway` 手动启动。

## 前沿工作流（终端中运行 Gateway）

目标：开发 Go Gateway，获取实时日志，保持 macOS 应用 UI 连接。

### 0) （可选）从源码运行 macOS 应用

```bash
./scripts/restart-mac.sh
```

### 1) 启动开发 Gateway

```bash
make gateway-dev
```

开发模式在端口 19001 运行，支持热重载。

### 2) 将 macOS 应用指向运行中的 Gateway

在 **OpenAcosmi.app** 中：

- 连接模式：**Local**
  应用将连接到配置端口上运行的 Gateway。

### 3) 验证

- 应用内 Gateway 状态应显示 **"使用已有的 Gateway…"**
- 或通过 CLI：

```bash
openacosmi health
```

### 常见问题

- **端口不匹配：** Gateway WebSocket 默认 `ws://127.0.0.1:18789`（生产）或 `ws://127.0.0.1:19001`（开发模式）；保持应用和 CLI 使用相同端口。
- **状态文件位置：**
  - 凭证：`~/.openacosmi/credentials/`
  - 会话：`~/.openacosmi/agents/<agentId>/sessions/`
  - 日志：`/tmp/openacosmi/`

## 凭证存储映射

调试认证或决定备份内容时参考：

- **WhatsApp**：`~/.openacosmi/credentials/whatsapp/<accountId>/creds.json`
- **Telegram bot token**：配置文件/环境变量 或 `channels.telegram.tokenFile`
- **Discord bot token**：配置文件/环境变量（尚不支持 token 文件）
- **Slack tokens**：配置文件/环境变量（`channels.slack.*`）
- **飞书**：配置文件（`channels.feishu.*`）
- **钉钉**：配置文件（`channels.dingtalk.*`）
- **企业微信**：配置文件（`channels.wecom.*`）
- **配对允许列表**：`~/.openacosmi/credentials/<channel>-allowFrom.json`
- **模型认证配置**：`~/.openacosmi/agents/<agentId>/agent/auth-profiles.json`
- **Legacy OAuth 导入**：`~/.openacosmi/credentials/oauth.json`
  更多详情：[安全](/gateway/security#credential-storage-map)。

## 更新（不破坏配置）

- 将 `~/.openacosmi/workspace` 和 `~/.openacosmi/` 作为"你的数据"；不要将个人提示词/配置放入 `openacosmi` 仓库。
- 更新源码：`git pull` + `make build`（重新编译全部组件）。

## Linux（systemd 用户服务）

Linux 安装使用 systemd **用户**服务。默认情况下，systemd 在注销/空闲时停止用户服务，这会终止 Gateway。引导时会尝试为你启用 lingering（可能提示 sudo）。如果仍未启用，运行：

```bash
sudo loginctl enable-linger $USER
```

对于始终在线或多用户服务器，考虑使用**系统**服务替代用户服务（无需 lingering）。详见 [Gateway 运维手册](/gateway)。

## 相关文档

- [Gateway 运维手册](/gateway)（命令行参数、监管、端口）
- [Gateway 配置](/gateway/configuration)（配置 Schema + 示例）
- [Discord](/channels/discord) 和 [Telegram](/channels/telegram)（回复标签 + replyToMode 设置）
- [OpenAcosmi 助手设置](/start/openacosmi)
- [macOS 应用](/platforms/macos)（Gateway 生命周期）
