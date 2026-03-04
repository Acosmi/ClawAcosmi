---
summary: "Android 应用（节点）：连接手册 + Canvas/Chat/Camera"
read_when:
  - 配对或重新连接 Android 节点
  - 调试 Android 的 Gateway 发现或认证
  - 验证跨客户端的聊天历史一致性
title: "Android 应用"
---

> **架构提示 — Rust CLI + Go Gateway**
> Android 应用作为节点连接到 Go Gateway WebSocket，
> 配对管理通过 Rust CLI 的 `openacosmi nodes` 命令完成。

# Android 应用（节点）

## 支持概况

- 角色：伴侣节点应用（Android 不托管 Gateway）。
- 需要 Gateway：是（在 macOS、Linux 或 Windows WSL2 上运行 Go Gateway）。
- 安装：[快速开始](/start/getting-started) + [配对](/gateway/pairing)。
- Gateway：[运维手册](/gateway) + [配置](/gateway/configuration)。
  - 协议：[Gateway 协议](/gateway/protocol)（节点 + 控制面）。

## 系统控制

系统控制（launchd/systemd）位于 Gateway 主机上。参见 [Gateway](/gateway)。
守护进程管理由 Rust CLI 的 `oa-daemon` crate 实现。

## 连接手册

Android 节点应用 ⇄（mDNS/NSD + WebSocket）⇄ **Go Gateway**

Android 直接连接到 Go Gateway WebSocket（默认 `ws://<host>:18789`）并使用 Gateway 拥有的配对机制。

### 前置条件

- 可在"主"机器上运行 Go Gateway。
- Android 设备/模拟器可到达 Gateway WebSocket：
  - 同一局域网的 mDNS/NSD，**或**
  - 同一 Tailscale tailnet 使用广域 Bonjour / 单播 DNS-SD（见下文），**或**
  - 手动 Gateway 主机/端口（回退）
- 可在 Gateway 机器上运行 Rust CLI（`openacosmi`）（或通过 SSH）。

### 1）启动 Go Gateway

```bash
openacosmi gateway --port 18789 --verbose
```

确认日志中看到类似：

- `listening on ws://0.0.0.0:18789`

仅 tailnet 设置时（建议用于跨网络场景），将 Gateway 绑定到 tailnet IP：

- 在 Gateway 主机的 `~/.openacosmi/openacosmi.json` 中设置 `gateway.bind: "tailnet"`。
- 重启 Go Gateway / macOS 菜单栏应用。

### 2）验证发现（可选）

从 Gateway 机器：

```bash
dns-sd -B _openacosmi-gw._tcp local.
```

更多调试说明：[Bonjour](/gateway/bonjour)。

#### 跨网络 Tailnet 发现（通过单播 DNS-SD）

Android NSD/mDNS 发现不能跨网络。如果 Android 节点和 Gateway 在不同网络但通过 Tailscale 连接，使用广域 Bonjour / 单播 DNS-SD：

1. 在 Gateway 主机上设置 DNS-SD 区域（示例 `openacosmi.internal.`）并发布 `_openacosmi-gw._tcp` 记录。
2. 为选定域名配置 Tailscale 分裂 DNS 指向该 DNS 服务器。

详情和 CoreDNS 配置示例：[Bonjour](/gateway/bonjour)。

### 3）从 Android 连接

在 Android 应用中：

- 应用通过**前台服务**（持久通知）保持 Gateway 连接活跃。
- 打开**设置**。
- 在**已发现的 Gateway** 中选择你的 Gateway 并点击**连接**。
- 如果 mDNS 被阻止，使用**高级 → 手动 Gateway**（主机 + 端口）并点击**连接（手动）**。

首次成功配对后，Android 在启动时自动重连：

- 手动端点（如已启用），否则
- 上次发现的 Gateway（尽力而为）。

### 4）批准配对（CLI）

在 Gateway 机器上：

```bash
openacosmi nodes pending
openacosmi nodes approve <requestId>
```

配对详情：[Gateway 配对](/gateway/pairing)。

### 5）验证节点已连接

- 通过 nodes status：

  ```bash
  openacosmi nodes status
  ```

- 通过 Gateway：

  ```bash
  openacosmi gateway call node.list --params "{}"
  ```

### 6）聊天 + 历史

Android 节点的聊天页面使用 Go Gateway 的**主会话键**（`main`），因此历史和回复与 WebChat 及其他客户端共享：

- 历史：`chat.history`
- 发送：`chat.send`
- 推送更新（尽力而为）：`chat.subscribe` → `event:"chat"`

### 7）Canvas + 摄像头

#### Gateway Canvas 主机（推荐用于 Web 内容）

如果需要节点展示 agent 可在磁盘上编辑的真实 HTML/CSS/JS，将节点指向 Go Gateway 的 Canvas 主机。

说明：节点使用 `canvasHost.port`（默认 `18793`）上的独立 Canvas 主机。

1. 在 Gateway 主机上创建 `~/.openacosmi/workspace/canvas/index.html`。

2. 将节点导航到该位置（局域网）：

```bash
openacosmi nodes invoke --node "<Android Node>" --command canvas.navigate --params '{"url":"http://<gateway-hostname>.local:18793/__openacosmi__/canvas/"}'
```

Tailnet（可选）：如果两台设备都在 Tailscale 上，使用 MagicDNS 名称或 tailnet IP 代替 `.local`。

该服务器将实时重载客户端注入 HTML 并在文件更改时重新加载。
A2UI 主机位于 `http://<gateway-host>:18793/__openacosmi__/a2ui/`。

Canvas 命令（仅前台）：

- `canvas.eval`、`canvas.snapshot`、`canvas.navigate`（使用 `{"url":""}` 或 `{"url":"/"}` 返回默认脚手架）。`canvas.snapshot` 返回 `{ format, base64 }`（默认 `format="jpeg"`）。
- A2UI：`canvas.a2ui.push`、`canvas.a2ui.reset`

摄像头命令（仅前台；权限控制）：

- `camera.snap`（jpg）
- `camera.clip`（mp4）

参见 [摄像头节点](/nodes/camera) 获取参数和 CLI 辅助命令。
