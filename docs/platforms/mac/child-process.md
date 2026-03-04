---
summary: "macOS 上的 Gateway 生命周期（launchd）"
read_when:
  - 集成 macOS 应用与 Gateway 生命周期
title: "Gateway 生命周期"
---

> **架构提示 — Rust CLI + Go Gateway**
> macOS 应用通过 launchd 管理 Go Gateway，不将 Gateway 作为子进程。
> launchd 服务管理由 Rust CLI 的 `oa-daemon` crate 实现。

# macOS 上的 Gateway 生命周期

macOS 应用默认**通过 launchd 管理 Gateway**，不将 Gateway 作为子进程生成。
它首先尝试连接配置端口上已运行的 Gateway；如不可达，
通过外部 `openacosmi` Rust CLI 启用 launchd 服务（无内嵌运行时）。
这提供了可靠的登录自启动和崩溃重启。

子进程模式（Gateway 由应用直接生成）**目前未使用**。
如需更紧密的 UI 耦合，在终端中手动运行 Gateway。

## 默认行为（launchd）

- 应用安装标签为 `bot.molt.gateway` 的每用户 LaunchAgent
  （使用 `--profile`/`OPENACOSMI_PROFILE` 时为 `bot.molt.<profile>`；旧版 `com.openacosmi.*` 仍支持）。
- 本地模式启用时，应用确保 LaunchAgent 已加载并按需启动 Gateway。
- 日志写入 launchd Gateway 日志路径（在调试设置中可见）。

常用命令：

```bash
launchctl kickstart -k gui/$UID/bot.molt.gateway
launchctl bootout gui/$UID/bot.molt.gateway
```

使用命名 profile 时替换标签为 `bot.molt.<profile>`。

## 未签名开发构建

`scripts/restart-mac.sh --no-sign` 用于没有签名密钥时的快速本地构建。
为防止 launchd 指向未签名的 relay 二进制，它：

- 写入 `~/.openacosmi/disable-launchagent`。

`scripts/restart-mac.sh` 的签名运行会在标记存在时清除此覆盖。手动重置：

```bash
rm ~/.openacosmi/disable-launchagent
```

## 仅连接模式

要强制 macOS 应用**永不安装或管理 launchd**，使用 `--attach-only`（或 `--no-launchd`）
启动。这设置 `~/.openacosmi/disable-launchagent`，
应用仅连接到已运行的 Gateway。可在调试设置中切换相同行为。

## 远程模式

远程模式不启动本地 Gateway。应用使用 SSH 隧道连接远程主机并通过隧道通信。

## 为何选择 launchd

- 登录自启动。
- 内置重启/KeepAlive 语义。
- 可预测的日志和监管。
