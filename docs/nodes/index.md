---
summary: "节点：配对、能力、权限以及 canvas/camera/screen/system 的 CLI 辅助命令"
read_when:
  - 将 iOS/Android 节点配对到 Gateway
  - 使用节点 canvas/camera 为 agent 提供上下文
  - 添加新的节点命令或 CLI 辅助功能
title: "节点"
---

> **架构提示 — Rust CLI + Go Gateway**
> CLI 命令由 Rust 二进制 `openacosmi` 处理（`cli-rust/crates/oa-cmd-node/`），
> 节点主机运行时逻辑由 Go Gateway 实现（`backend/internal/nodehost/`）。

# 节点

**节点（Node）** 是伴侣设备（macOS/iOS/Android/无头服务器），通过 **WebSocket**（与 operator 使用相同端口）以 `role: "node"` 身份连接到 Go Gateway，暴露命令接口（如 `canvas.*`、`camera.*`、`system.*`），由 `node.invoke` 调用。协议详情请参阅：[Gateway 协议](/gateway/protocol)。

旧版传输协议：[Bridge 协议](/gateway/bridge-protocol)（TCP JSONL；已弃用/移除）。

macOS 可以运行在**节点模式**下：菜单栏应用连接到 Go Gateway 的 WebSocket 服务器，将本地 canvas/camera 命令作为节点暴露（即 `openacosmi nodes …` 可对该 Mac 操作）。

说明：

- 节点是**外围设备**，不是 Gateway。它们不运行 Gateway 服务。
- Telegram/WhatsApp 等消息到达 **Gateway**，不经过节点。
- 故障排除手册：[节点故障排除](/nodes/troubleshooting)

## 配对与状态

**WebSocket 节点使用设备配对机制。** 节点在 `connect` 阶段呈现设备身份；Go Gateway
创建 `role: node` 的设备配对请求。通过 Rust CLI 的 devices 命令（或 UI）批准。

快速 CLI：

```bash
openacosmi devices list
openacosmi devices approve <requestId>
openacosmi devices reject <requestId>
openacosmi nodes status
openacosmi nodes describe --node <idOrNameOrIp>
```

说明：

- `nodes status` 在设备配对角色包含 `node` 时标记节点为**已配对**。
- `node.pair.*`（CLI：`openacosmi nodes pending/approve/reject`）是 Go Gateway 独立的
  节点配对存储；它**不**控制 WebSocket `connect` 握手。

## 远程节点主机（system.run）

当 Gateway 运行在一台机器上，而你希望命令在另一台机器上执行时，可以使用**节点主机（node host）**。模型仍然与 **Gateway** 通信；当选择 `host=node` 时，Go Gateway 将 `exec` 调用转发给**节点主机**。

### 运行位置

- **Gateway 主机**：接收消息、运行模型、路由工具调用（Go Gateway 进程）。
- **节点主机**：在节点机器上执行 `system.run`/`system.which`（Go `nodehost` 包）。
- **审批**：在节点主机上通过 `~/.openacosmi/exec-approvals.json` 执行。

### 启动节点主机（前台）

在节点机器上：

```bash
openacosmi node run --host <gateway-host> --port 18789 --display-name "Build Node"
```

### 通过 SSH 隧道连接远程 Gateway（回环绑定）

如果 Gateway 绑定到回环地址（`gateway.bind=loopback`，本地模式默认），
远程节点主机无法直接连接。创建 SSH 隧道并将节点主机指向隧道的本地端。

示例（节点主机 -> Gateway 主机）：

```bash
# 终端 A（保持运行）：将本地 18790 转发到 Gateway 127.0.0.1:18789
ssh -N -L 18790:127.0.0.1:18789 user@gateway-host

# 终端 B：导出 Gateway token 并通过隧道连接
export OPENACOSMI_GATEWAY_TOKEN="<gateway-token>"
openacosmi node run --host 127.0.0.1 --port 18790 --display-name "Build Node"
```

说明：

- token 是 Gateway 配置中的 `gateway.auth.token`（Gateway 主机的 `~/.openacosmi/openacosmi.json`）。
- `openacosmi node run` 读取 `OPENACOSMI_GATEWAY_TOKEN` 进行认证。

### 启动节点主机（后台服务）

```bash
openacosmi node install --host <gateway-host> --port 18789 --display-name "Build Node"
openacosmi node restart
```

### 配对与命名

在 Gateway 主机上：

```bash
openacosmi nodes pending
openacosmi nodes approve <requestId>
openacosmi nodes list
```

命名选项：

- `--display-name`：在 `openacosmi node run` / `openacosmi node install` 中设置（持久化到节点上的 `~/.openacosmi/node.json`）。
- `openacosmi nodes rename --node <id|name|ip> --name "Build Node"`：Gateway 端覆盖命名。

### 允许列表命令

执行审批是**按节点主机**配置的。从 Gateway 添加允许列表条目：

```bash
openacosmi approvals allowlist add --node <id|name|ip> "/usr/bin/uname"
openacosmi approvals allowlist add --node <id|name|ip> "/usr/bin/sw_vers"
```

审批规则存储在节点主机的 `~/.openacosmi/exec-approvals.json` 中。
Go 实现参见 `backend/internal/nodehost/allowlist_eval.go`。

### 将 exec 指向节点

配置默认值（Gateway 配置）：

```bash
openacosmi config set tools.exec.host node
openacosmi config set tools.exec.security allowlist
openacosmi config set tools.exec.node "<id-or-name>"
```

或按会话设置：

```
/exec host=node security=allowlist node=<id-or-name>
```

设置后，任何 `host=node` 的 `exec` 调用将在节点主机上运行（受节点允许列表/审批规则约束）。

相关文档：

- [节点主机 CLI](/cli/node)
- [Exec 工具](/tools/exec)
- [Exec 审批](/tools/exec-approvals)

## 调用命令

底层（原始 RPC）：

```bash
openacosmi nodes invoke --node <idOrNameOrIp> --command canvas.eval --params '{"javaScript":"location.href"}'
```

对于常见的"给 agent 附加 MEDIA 媒体文件"工作流，有更高层的辅助命令。

## 截图（canvas 快照）

如果节点正在显示 Canvas（WebView），`canvas.snapshot` 返回 `{ format, base64 }`。

CLI 辅助（写入临时文件并输出 `MEDIA:<path>`）：

```bash
openacosmi nodes canvas snapshot --node <idOrNameOrIp> --format png
openacosmi nodes canvas snapshot --node <idOrNameOrIp> --format jpg --max-width 1200 --quality 0.9
```

### Canvas 控制

```bash
openacosmi nodes canvas present --node <idOrNameOrIp> --target https://example.com
openacosmi nodes canvas hide --node <idOrNameOrIp>
openacosmi nodes canvas navigate https://example.com --node <idOrNameOrIp>
openacosmi nodes canvas eval --node <idOrNameOrIp> --js "document.title"
```

说明：

- `canvas present` 接受 URL 或本地文件路径（`--target`），以及可选的 `--x/--y/--width/--height` 用于定位。
- `canvas eval` 接受内联 JS（`--js`）或位置参数。

### A2UI（Canvas）

```bash
openacosmi nodes canvas a2ui push --node <idOrNameOrIp> --text "Hello"
openacosmi nodes canvas a2ui push --node <idOrNameOrIp> --jsonl ./payload.jsonl
openacosmi nodes canvas a2ui reset --node <idOrNameOrIp>
```

说明：

- 仅支持 A2UI v0.8 JSONL（v0.9/createSurface 会被拒绝）。

## 照片与视频（节点摄像头）

照片（`jpg`）：

```bash
openacosmi nodes camera list --node <idOrNameOrIp>
openacosmi nodes camera snap --node <idOrNameOrIp>            # 默认：前后摄像头（2 个 MEDIA 行）
openacosmi nodes camera snap --node <idOrNameOrIp> --facing front
```

视频片段（`mp4`）：

```bash
openacosmi nodes camera clip --node <idOrNameOrIp> --duration 10s
openacosmi nodes camera clip --node <idOrNameOrIp> --duration 3000 --no-audio
```

说明：

- 节点必须在**前台**才能使用 `canvas.*` 和 `camera.*`（后台调用返回 `NODE_BACKGROUND_UNAVAILABLE`）。
- 视频时长限制（当前 `<= 60s`）以避免 base64 载荷过大。
- Android 会在可能时提示 `CAMERA`/`RECORD_AUDIO` 权限；拒绝授权会返回 `*_PERMISSION_REQUIRED` 错误。

## 屏幕录制（节点）

节点暴露 `screen.record`（mp4）。示例：

```bash
openacosmi nodes screen record --node <idOrNameOrIp> --duration 10s --fps 10
openacosmi nodes screen record --node <idOrNameOrIp> --duration 10s --fps 10 --no-audio
```

说明：

- `screen.record` 要求节点应用在前台。
- Android 会在录制前显示系统屏幕捕获确认提示。
- 屏幕录制限制为 `<= 60s`。
- `--no-audio` 禁用麦克风捕获（iOS/Android 支持；macOS 使用系统捕获音频）。
- 使用 `--screen <index>` 在多显示器时选择屏幕。

## 位置（节点）

当设置中启用了位置功能时，节点暴露 `location.get`。

CLI 辅助：

```bash
openacosmi nodes location get --node <idOrNameOrIp>
openacosmi nodes location get --node <idOrNameOrIp> --accuracy precise --max-age 15000 --location-timeout 10000
```

说明：

- 位置功能**默认关闭**。
- "始终允许"需要系统权限；后台获取为尽力而为。
- 响应包含纬度/经度、精度（米）和时间戳。

## 短信（Android 节点）

当用户授予 **SMS** 权限且设备支持电话功能时，Android 节点可以暴露 `sms.send`。

底层调用：

```bash
openacosmi nodes invoke --node <idOrNameOrIp> --command sms.send --params '{"to":"+15555550123","message":"Hello from OpenAcosmi"}'
```

说明：

- 在可用能力中展示 `sms.send` 之前，必须在 Android 设备上接受权限提示。
- 无电话功能的纯 Wi-Fi 设备不会展示 `sms.send`。

## 系统命令（节点主机 / Mac 节点）

macOS 节点暴露 `system.run`、`system.notify` 和 `system.execApprovals.get/set`。
无头节点主机暴露 `system.run`、`system.which` 和 `system.execApprovals.get/set`。

Go 实现位于 `backend/internal/nodehost/runner.go`，命令处理器包括：

- `handleSystemRun` — 执行命令并返回 stdout/stderr/exit code
- `handleSystemWhich` — 查找命令路径
- `handleExecApprovalsGet/Set` — 管理执行审批规则

示例：

```bash
openacosmi nodes run --node <idOrNameOrIp> -- echo "Hello from mac node"
openacosmi nodes notify --node <idOrNameOrIp> --title "Ping" --body "Gateway ready"
```

说明：

- `system.run` 返回 stdout/stderr/exit code。
- `system.notify` 遵循 macOS 应用的通知权限状态。
- `system.run` 支持 `--cwd`、`--env KEY=VAL`、`--command-timeout` 和 `--needs-screen-recording`。
- `system.notify` 支持 `--priority <passive|active|timeSensitive>` 和 `--delivery <system|overlay|auto>`。
- macOS 节点会丢弃 `PATH` 覆盖；无头节点主机仅在 `PATH` 前置节点主机 PATH 时接受。
- 在 macOS 节点模式下，`system.run` 受 macOS 应用中的执行审批控制（设置 → 执行审批）。
  ask/allowlist/full 行为与无头节点主机相同；拒绝的提示返回 `SYSTEM_RUN_DENIED`。
- 在无头节点主机上，`system.run` 受 `~/.openacosmi/exec-approvals.json` 中的执行审批控制。
  Go 实现参见 `backend/internal/nodehost/allowlist_eval.go` 和 `allowlist_parse.go`。

## Exec 节点绑定

当有多个节点可用时，可以将 exec 绑定到特定节点。
这设置了 `exec host=node` 的默认节点（可按 agent 覆盖）。

全局默认：

```bash
openacosmi config set tools.exec.node "node-id-or-name"
```

按 agent 覆盖：

```bash
openacosmi config get agents.list
openacosmi config set agents.list[0].tools.exec.node "node-id-or-name"
```

取消设置以允许任意节点：

```bash
openacosmi config unset tools.exec.node
openacosmi config unset agents.list[0].tools.exec.node
```

## 权限映射

节点可在 `node.list` / `node.describe` 中包含 `permissions` 映射，按权限名称（如 `screenRecording`、`accessibility`）键值为布尔值（`true` = 已授予）。

## 无头节点主机（跨平台）

OpenAcosmi 可运行**无头节点主机**（无 UI），连接到 Go Gateway WebSocket 并暴露 `system.run` / `system.which`。这在 Linux/Windows 上或需要在服务器旁运行最小节点时很有用。

Go 实现位于 `backend/internal/nodehost/runner.go`（`NodeHostService` 结构体）。

启动：

```bash
openacosmi node run --host <gateway-host> --port 18789
```

说明：

- 仍需配对（Go Gateway 会显示节点审批提示）。
- 节点主机将节点 ID、token、显示名称和 Gateway 连接信息存储在 `~/.openacosmi/node.json` 中。
- 执行审批通过 `~/.openacosmi/exec-approvals.json` 在本地执行
  （参见 [Exec 审批](/tools/exec-approvals)）。
- 在 macOS 上，无头节点主机优先使用伴侣应用的执行主机（当可达时），
  不可用时回退到本地执行。设置 `OPENACOSMI_NODE_EXEC_HOST=app` 强制要求应用，
  或 `OPENACOSMI_NODE_EXEC_FALLBACK=0` 禁用回退。
- 当 Gateway WebSocket 使用 TLS 时，添加 `--tls` / `--tls-fingerprint`。

## Mac 节点模式

- macOS 菜单栏应用以节点身份连接到 Go Gateway WebSocket 服务器（启用 `openacosmi nodes …` 对该 Mac 操作）。
- 在远程模式下，应用为 Gateway 端口打开 SSH 隧道并连接到 `localhost`。
