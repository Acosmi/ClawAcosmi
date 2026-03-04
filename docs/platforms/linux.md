---
summary: "Linux 支持 + 伴侣应用状态"
read_when:
  - 查看 Linux 伴侣应用状态
  - 规划平台覆盖或贡献
title: "Linux"
---

> **架构提示 — Rust CLI + Go Gateway**
> Linux 上运行 Go Gateway（`backend/cmd/acosmi`）和 Rust CLI（`openacosmi`）。
> 守护进程管理由 Rust CLI 的 `oa-daemon` crate 实现（`systemd` 模块）。

# Linux

Go Gateway 在 Linux 上完全支持。**Go 是 Gateway 运行时**，**Rust** 是 CLI 工具。

原生 Linux 伴侣应用已在计划中。欢迎贡献力量帮助构建。

## 新手快速路径（VPS）

1. 构建并安装 Go Gateway 和 Rust CLI
2. `openacosmi onboard --install-daemon`
3. 从笔记本：`ssh -N -L 18789:127.0.0.1:18789 <user>@<host>`
4. 打开 `http://127.0.0.1:18789/` 并粘贴你的 token

VPS 分步指南：[exe.dev](/install/exe-dev)

## 安装

- [快速开始](/start/getting-started)
- [安装与更新](/install/updating)
- 可选流程：[Docker](/install/docker)

## Gateway

- [Gateway 运维手册](/gateway)
- [配置](/gateway/configuration)

## Gateway 服务安装（CLI）

使用以下方式之一：

```bash
openacosmi onboard --install-daemon
```

或：

```bash
openacosmi gateway install
```

或：

```bash
openacosmi configure
```

当提示时选择 **Gateway 服务**。

修复/迁移：

```bash
openacosmi doctor
```

Rust CLI 守护进程管理：`cli-rust/crates/oa-daemon/`（`systemd` 模块）。

## 系统控制（systemd 用户单元）

OpenAcosmi 默认安装 systemd **用户**服务。共享或始终在线的服务器使用**系统**服务。
完整单元示例和指南位于 [Gateway 运维手册](/gateway)。

最小设置：

创建 `~/.config/systemd/user/openacosmi-gateway[-<profile>].service`：

```ini
[Unit]
Description=OpenAcosmi Gateway (profile: <profile>)
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/openacosmi gateway --port 18789
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

启用：

```bash
systemctl --user enable --now openacosmi-gateway[-<profile>].service
```
