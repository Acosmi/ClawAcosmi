---
summary: "平台支持概览（Gateway + 伴侣应用）"
read_when:
  - 查看操作系统支持或安装路径
  - 决定在哪里运行 Gateway
title: "平台"
---

> **架构提示 — Rust CLI + Go Gateway**
> OpenAcosmi 核心由 Go 编写（Gateway 服务端），Rust 编写（CLI 工具）。
> 守护进程管理由 Rust CLI 的 `oa-daemon` crate 实现（macOS launchd、Linux systemd）。

# 平台

OpenAcosmi 核心使用 **Go** 编写 Gateway 服务端，**Rust** 编写 CLI 工具。
**Go Gateway** 是推荐的服务器运行时。

伴侣应用适用于 macOS（菜单栏应用）和移动节点（iOS/Android）。Windows 和
Linux 伴侣应用已计划中，但 Go Gateway 现已完全支持。
Windows 原生伴侣应用也在计划中；Gateway 推荐通过 WSL2 运行。

## 选择操作系统

- macOS：[macOS](/platforms/macos)
- iOS：[iOS](/platforms/ios)
- Android：[Android](/platforms/android)
- Windows：[Windows](/platforms/windows)
- Linux：[Linux](/platforms/linux)

## VPS 与托管

- VPS 中心：[VPS 托管](/vps)
- Fly.io：[Fly.io](/install/fly)
- Hetzner（Docker）：[Hetzner](/install/hetzner)
- GCP（Compute Engine）：[GCP](/install/gcp)
- exe.dev（VM + HTTPS 代理）：[exe.dev](/install/exe-dev)

## 常用链接

- 安装指南：[快速开始](/start/getting-started)
- Gateway 运维手册：[Gateway](/gateway)
- Gateway 配置：[配置](/gateway/configuration)
- 服务状态：`openacosmi gateway status`

## Gateway 服务安装（CLI）

使用以下方式之一（全部支持）：

- 向导（推荐）：`openacosmi onboard --install-daemon`
- 直接安装：`openacosmi gateway install`
- 配置流程：`openacosmi configure` → 选择 **Gateway 服务**
- 修复/迁移：`openacosmi doctor`（提供安装或修复服务的选项）

Rust CLI 守护进程管理实现：`cli-rust/crates/oa-daemon/`。

服务目标取决于操作系统：

- macOS：LaunchAgent（`bot.molt.gateway` 或 `bot.molt.<profile>`；旧版 `com.openacosmi.*`）
- Linux/WSL2：systemd 用户服务（`openacosmi-gateway[-<profile>].service`）
