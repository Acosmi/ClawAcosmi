---
summary: "OpenAcosmi macOS 伴侣应用（菜单栏 + Gateway 代理）"
read_when:
  - 实现 macOS 应用功能
  - 修改 macOS 上的 Gateway 生命周期或节点桥接
title: "macOS 应用"
---

> **架构提示 — Rust CLI + Go Gateway**
> macOS 应用管理/连接 Go Gateway（`backend/cmd/acosmi`），
> CLI 守护进程管理由 Rust 实现（`cli-rust/crates/oa-daemon/`），
> 节点主机逻辑参见 `backend/internal/nodehost/`。

# OpenAcosmi macOS 伴侣应用（菜单栏 + Gateway 代理）

macOS 应用是 OpenAcosmi 的**菜单栏伴侣**。它拥有权限管理，
本地管理/连接 Go Gateway（launchd 或手动），并将 macOS 功能作为节点暴露给 agent。

## 功能

- 在菜单栏中显示原生通知和状态。
- 拥有 TCC 提示（通知、辅助功能、屏幕录制、麦克风、语音识别、自动化/AppleScript）。
- 运行或连接到 Go Gateway（本地或远程）。
- 暴露 macOS 专属工具（Canvas、Camera、Screen Recording、`system.run`）。
- 在**远程**模式下启动本地节点主机服务（launchd），在**本地**模式下停止。
- 可选托管 **PeekabooBridge** 用于 UI 自动化。
- 支持通过 Rust CLI 安装全局 `openacosmi` 命令。

## 本地与远程模式

- **本地**（默认）：应用连接到运行中的本地 Go Gateway（如存在）；否则通过 `openacosmi gateway install` 启用 launchd 服务。
- **远程**：应用通过 SSH/Tailscale 连接远程 Go Gateway，不启动本地进程。
  应用启动本地**节点主机服务**以便远程 Gateway 可以到达该 Mac。
  应用不将 Gateway 作为子进程生成。

## Launchd 控制

应用管理按用户的 LaunchAgent，标签为 `bot.molt.gateway`
（使用 `--profile`/`OPENACOSMI_PROFILE` 时为 `bot.molt.<profile>`；旧版 `com.openacosmi.*` 仍会被卸载）。

```bash
launchctl kickstart -k gui/$UID/bot.molt.gateway
launchctl bootout gui/$UID/bot.molt.gateway
```

使用命名 profile 时替换标签为 `bot.molt.<profile>`。

如果 LaunchAgent 未安装，从应用启用或运行 `openacosmi gateway install`。

Rust CLI 守护进程管理：`cli-rust/crates/oa-daemon/`（`launchd` 模块）。

## 节点能力（Mac）

macOS 应用以节点身份呈现。常用命令：

- Canvas：`canvas.present`、`canvas.navigate`、`canvas.eval`、`canvas.snapshot`、`canvas.a2ui.*`
- Camera：`camera.snap`、`camera.clip`
- Screen：`screen.record`
- System：`system.run`、`system.notify`

节点通过 `permissions` 映射报告权限状态，agent 可据此决定允许的操作。

节点服务 + 应用 IPC：

- 远程模式下无头节点主机服务运行时，通过 Gateway WebSocket 作为节点连接。
- `system.run` 在 macOS 应用中执行（UI/TCC 上下文），通过本地 Unix socket 通信；提示和输出保留在应用内。

Go 实现：`backend/internal/nodehost/runner.go`（`NodeHostService`）。

架构图：

```
Go Gateway -> 节点服务 (WS)
                  |  IPC (UDS + token + HMAC + TTL)
                  v
              Mac 应用 (UI + TCC + system.run)
```

## 执行审批（system.run）

`system.run` 受 macOS 应用中的**执行审批**控制（设置 → 执行审批）。
安全模式 + ask + 允许列表存储在 Mac 本地：

```
~/.openacosmi/exec-approvals.json
```

示例：

```json
{
  "version": 1,
  "defaults": {
    "security": "deny",
    "ask": "on-miss"
  },
  "agents": {
    "main": {
      "security": "allowlist",
      "ask": "on-miss",
      "allowlist": [{ "pattern": "/opt/homebrew/bin/rg" }]
    }
  }
}
```

Go 审批评估：`backend/internal/nodehost/allowlist_eval.go`。

说明：

- `allowlist` 条目是解析后二进制路径的 glob 模式。
- 在提示中选择"始终允许"会将该命令添加到允许列表。
- `system.run` 环境覆盖会被过滤（丢弃 `PATH`、`DYLD_*`、`LD_*`、`NODE_OPTIONS`、`PYTHON*`、`PERL*`、`RUBYOPT`），然后与应用环境合并。

## 深层链接

应用注册 `openacosmi://` URL 方案用于本地操作。

### `openacosmi://agent`

触发 Go Gateway `agent` 请求。

```bash
open 'openacosmi://agent?message=Hello%20from%20deep%20link'
```

查询参数：

- `message`（必需）
- `sessionKey`（可选）
- `thinking`（可选）
- `deliver` / `to` / `channel`（可选）
- `timeoutSeconds`（可选）
- `key`（可选的无人值守模式密钥）

安全性：

- 不带 `key` 时，应用显示确认提示。
- 带有效 `key` 时，运行为无人值守（用于个人自动化）。

## 初次安装流程（典型）

1. 安装并启动 **OpenAcosmi.app**。
2. 完成权限检查列表（TCC 提示）。
3. 确保**本地**模式激活且 Go Gateway 运行中。
4. 如需终端访问，安装 Rust CLI。

## 构建与开发工作流（原生）

- `cd apps/macos && swift build`
- `swift run OpenAcosmi`（或 Xcode）
- 打包应用：`scripts/package-mac-app.sh`

## 调试 Gateway 连接（macOS CLI）

使用调试 CLI 执行与 macOS 应用相同的 Gateway WebSocket 握手和发现逻辑，无需启动应用。

```bash
cd apps/macos
swift run openacosmi-mac connect --json
swift run openacosmi-mac discover --timeout 3000 --json
```

连接选项：

- `--url <ws://host:port>`：覆盖配置
- `--mode <local|remote>`：从配置解析（默认：配置值或 local）
- `--probe`：强制新的健康探测
- `--timeout <ms>`：请求超时（默认：`15000`）
- `--json`：结构化输出用于对比

发现选项：

- `--include-local`：包含会被过滤为"本地"的 Gateway
- `--timeout <ms>`：整体发现窗口（默认：`2000`）
- `--json`：结构化输出用于对比

提示：与 `openacosmi gateway discover --json` 对比，查看 macOS 应用的发现管道（NWBrowser + tailnet DNS-SD 回退）是否与 Rust CLI 的发现结果不同。

## 远程连接管道（SSH 隧道）

macOS 应用在**远程**模式下运行时，打开 SSH 隧道以便本地 UI 组件像在 localhost 上一样与远程 Go Gateway 通信。

### 控制隧道（Gateway WebSocket 端口）

- **用途：** 健康检查、状态、Web Chat、配置和其他控制面调用。
- **本地端口：** Gateway 端口（默认 `18789`），始终稳定。
- **远程端口：** 远程主机上的相同 Gateway 端口。
- **行为：** 无随机本地端口；应用复用现有健康隧道或在需要时重启。
- **SSH 形状：** `ssh -N -L <local>:127.0.0.1:<remote>` 带 BatchMode + ExitOnForwardFailure + keepalive 选项。
- **IP 报告：** SSH 隧道使用回环地址，Gateway 会将节点 IP 显示为 `127.0.0.1`。如需显示真实客户端 IP，使用**直连（ws/wss）**传输（参见 [macOS 远程访问](/platforms/mac/remote)）。

设置步骤参见 [macOS 远程访问](/platforms/mac/remote)。协议详情参见 [Gateway 协议](/gateway/protocol)。

## 相关文档

- [Gateway 运维手册](/gateway)
- [Gateway（macOS）](/platforms/mac/bundled-gateway)
- [macOS 权限](/platforms/mac/permissions)
- [Canvas](/platforms/mac/canvas)
