---
summary: "Zalo 个人插件：QR 登录 + 消息发送（通过 zca-cli）— 安装 + 渠道配置 + CLI + 工具"
read_when:
  - 需要 Zalo 个人（非官方）支持
  - 配置或开发 zalouser 插件
title: "Zalo 个人插件"
---

> **架构提示 — Rust CLI + Go Gateway**
> Zalo 个人插件在 Go Gateway 进程内运行，
> 渠道配置通过 Go Gateway 的渠道管理系统加载。

# Zalo 个人（插件）

通过插件实现 OpenAcosmi 的 Zalo 个人支持，使用 `zca-cli` 自动化普通 Zalo 用户账户。

> **警告：** 非官方自动化可能导致账户暂停/封禁。使用风险自负。

## 命名

渠道 ID 为 `zalouser`，明确表示这是自动化**个人 Zalo 用户账户**（非官方）。保留 `zalo` 用于未来可能的官方 Zalo API 集成。

## 运行位置

此插件在 **Go Gateway 进程内**运行。

如果使用远程 Gateway，在**运行 Gateway 的机器上**安装/配置，然后重启 Go Gateway。

## 安装

### 方式 A：从 npm 安装

```bash
openacosmi plugins install @openacosmi/zalouser
```

安装后重启 Go Gateway。

### 方式 B：从本地文件夹安装（开发用）

```bash
openacosmi plugins install ./extensions/zalouser
cd ./extensions/zalouser && pnpm install
```

安装后重启 Go Gateway。

## 前置条件：zca-cli

Gateway 机器必须在 `PATH` 上有 `zca`：

```bash
zca --version
```

## 配置

渠道配置位于 `channels.zalouser` 下（不是 `plugins.entries.*`）：

```json5
{
  channels: {
    zalouser: {
      enabled: true,
      dmPolicy: "pairing",
    },
  },
}
```

## CLI

```bash
openacosmi channels login --channel zalouser
openacosmi channels logout --channel zalouser
openacosmi channels status --probe
openacosmi message send --channel zalouser --target <threadId> --message "Hello from OpenAcosmi"
openacosmi directory peers list --channel zalouser --query "name"
```

## Agent 工具

工具名称：`zalouser`

操作：`send`、`image`、`link`、`friends`、`groups`、`me`、`status`
