---
summary: "Gateway 服务运行手册：生命周期与运维操作"
read_when:
  - 运行或调试 Gateway 进程
title: "Gateway 运行手册"
---

# Gateway 服务运行手册

> [!IMPORTANT]
> **架构状态**：Gateway 由 **Go** 实现（`backend/internal/gateway/`），
> 通过 `cmd/openacosmi/cmd_gateway.go` 提供 CLI 入口。Rust CLI 通过 RPC 与 Gateway 通信。

最后更新：2026-03-01

## 概述

- 常驻运行的 Go 进程，管理所有消息频道连接（WhatsApp/Telegram/飞书/钉钉/企微等）及控制/事件面。
- CLI 入口：`openacosmi gateway start`（通过 cobra 命令树）。
- 运行至停止；遇到致命错误时以非零退出码退出，由 supervisor 重启。

## 如何运行（本地）

```bash
# 通过 Makefile 启动开发模式（自动编译 + 运行）：
make gateway-dev

# 或直接运行编译后的二进制：
openacosmi gateway start --port 19001

# 指定 control UI 静态文件目录：
openacosmi gateway start --port 19001 --control-ui-dir ./ui/dist

# 强制启动（先终止占用端口的进程）：
openacosmi gateway start --force
```

- 配置热重载监听 `~/.openacosmi/openacosmi.json`（或 `OPENACOSMI_CONFIG_PATH`）。
  - 默认模式：`gateway.reload.mode="hybrid"`（安全变更热应用，关键变更重启）。
  - 热重载通过 Go 的 `reload.go` 实现进程内重启。
  - 禁用热重载：`gateway.reload.mode="off"`。
- 绑定 WebSocket 控制面到 `127.0.0.1:<port>`（默认 19001）。
- 同一端口复用 HTTP 服务（控制 UI、Hooks、A2UI）：
  - OpenAI Chat Completions (HTTP)：[`/v1/chat/completions`](/gateway/openai-http-api)。
  - OpenResponses (HTTP)：[`/v1/responses`](/gateway/openresponses-http-api)。
  - Tools Invoke (HTTP)：[`/tools/invoke`](/gateway/tools-invoke-http-api)。
- Canvas 文件服务器默认在 `canvasHost.port`（默认 `gateway.port+4`）启动。
- 日志输出到 stdout；使用 launchd/systemd 保持服务存活并轮转日志。
- `--force` 使用 `lsof` 查找占用端口的监听进程，发送 SIGTERM 后启动 Gateway。
- Gateway 认证默认启用：设置 `gateway.auth.token`（或 `OPENACOSMI_GATEWAY_TOKEN`）。
- 端口优先级：`--port` > `OPENACOSMI_GATEWAY_PORT` > `gateway.port` > 默认 `19001`。

## 远程访问

- 优先使用 Tailscale/VPN；否则使用 SSH 隧道：

  ```bash
  ssh -N -L 19001:127.0.0.1:19001 user@host
  ```

- 客户端通过隧道连接 `ws://127.0.0.1:19001`。
- 如配置了 token，客户端必须在 `connect.params.auth.token` 中包含它。

## 多 Gateway 实例（同主机）

通常不需要：一个 Gateway 可以服务多个消息频道和 Agent。仅在需要冗余或严格隔离时使用多实例。

隔离 state + config 并使用不同端口即可。完整指南：[多 Gateway](/gateway/multiple-gateways)。

服务名按 profile 区分：

- macOS：`bot.molt.<profile>`
- Linux：`openacosmi-gateway-<profile>.service`
- Windows：`OpenAcosmi Gateway (<profile>)`

安装元数据嵌入服务配置中：

- `OPENACOSMI_SERVICE_MARKER=openacosmi`
- `OPENACOSMI_SERVICE_KIND=gateway`
- `OPENACOSMI_SERVICE_VERSION=<version>`

救援 Bot 模式：保持第二个 Gateway 隔离运行，使用独立的 profile、state 目录、workspace 和端口。详见：[救援 Bot 指南](/gateway/multiple-gateways#rescue-bot-guide)。

### 开发 profile（`--dev`）

快速路径：运行完全隔离的开发实例，不影响主配置。

```bash
openacosmi --dev setup
openacosmi --dev gateway start --allow-unconfigured
# 对开发实例操作：
openacosmi --dev status
openacosmi --dev health
```

默认值（可通过 env/flags/config 覆盖）：

- `OPENACOSMI_STATE_DIR=~/.openacosmi-dev`
- `OPENACOSMI_CONFIG_PATH=~/.openacosmi-dev/openacosmi.json`
- `OPENACOSMI_GATEWAY_PORT=19001` (Gateway WS + HTTP)
- browser control service port = `19003` (derived: `gateway.port+2`, loopback only)
- `canvasHost.port=19005` (derived: `gateway.port+4`)
- `agents.defaults.workspace` default becomes `~/.openacosmi/workspace-dev` when you run `setup`/`onboard` under `--dev`.

衍生端口规则：

- 基础端口 = `gateway.port`（或 `OPENACOSMI_GATEWAY_PORT` / `--port`）
- 浏览器控制服务端口 = base + 2（仅 loopback）
- `canvasHost.port = base + 4`（或 `OPENACOSMI_CANVAS_HOST_PORT` / 配置覆盖）
- 浏览器 profile CDP 端口从 `browser.controlPort + 9 .. + 108` 自动分配。

每实例核查清单：

- 唯一的 `gateway.port`
- 唯一的 `OPENACOSMI_CONFIG_PATH`
- 唯一的 `OPENACOSMI_STATE_DIR`
- 唯一的 `agents.defaults.workspace`
- 独立的 WhatsApp 号码（如使用 WA）

按 profile 安装服务：

```bash
openacosmi --profile main gateway install
openacosmi --profile rescue gateway install
```

示例：

```bash
OPENACOSMI_CONFIG_PATH=~/.openacosmi/a.json OPENACOSMI_STATE_DIR=~/.openacosmi-a openacosmi gateway start --port 19001
OPENACOSMI_CONFIG_PATH=~/.openacosmi/b.json OPENACOSMI_STATE_DIR=~/.openacosmi-b openacosmi gateway start --port 19002
```

## 协议（运维视角）

- 完整文档：[Gateway 协议](/gateway/protocol) 和 [桥接协议（遗留）](/gateway/bridge-protocol)。
- 客户端首帧必须发送：`req {type:"req", id, method:"connect", params:{...}}`。
- Gateway 回复 `res {type:"res", id, ok:true, payload:hello-ok}`（或 `ok:false` + 错误后关闭连接）。
- 握手完成后：
  - 请求：`{type:"req", id, method, params}` → `{type:"res", id, ok, payload|error}`
  - 事件：`{type:"event", event, payload, seq?, stateVersion?}`
- 协议由 Go Gateway 的 `protocol.go` 定义和验证。
- `agent` 回复为两阶段：先返回 `res` 确认 `{runId,status:"accepted"}`，Agent 运行结束后返回最终 `res`；流式输出通过 `event:"agent"` 推送。

## RPC 方法

Go Gateway 在 `server_methods*.go` 中实现所有 RPC 方法：

- `health` — 完整健康快照（`server_methods_system.go`）。
- `status` — 简短状态摘要。
- `system-presence` — 当前在线列表（`system_presence.go`）。
- `system-event` — 发布 presence/系统通知。
- `send` — 通过活跃频道发送消息（`server_methods_send.go`）。
- `agent` — 运行 Agent 回合并流式推送事件（`server_methods_agent.go`）。
- `chat.*` — 聊天相关操作（`server_methods_chat.go`）。
- `node.list` — 列出已配对 + 已连接节点。
- `node.describe` — 描述节点能力。
- `node.invoke` — 调用节点命令。
- `node.pair.*` — 配对生命周期。

另见：[Presence](/concepts/presence)。

## 事件

事件通过 `Broadcaster`（`broadcast.go`）推送给所有已连接客户端：

- `agent` — Agent 运行的流式工具/输出事件（带 seq 标记）。
- `presence` — Presence 更新（增量 + stateVersion）。
- `tick` — 定期心跳保活。
- `shutdown` — Gateway 正在退出；payload 包含 `reason`。客户端应重连。
- `memory.compressed` — UHMS 记忆压缩事件。
- `coder.confirm.*` — 命令审批门控事件。
- `plan.confirm.*` — 方案确认事件。

## WebChat 集成

- WebChat（Web UI）通过 WebSocket 直连 Go Gateway，实现历史记录、发送、中止和事件推送。
- 远程使用同样通过 SSH/Tailscale 隧道；如配置了 token，客户端在 `connect` 时携带。
- Web UI 通过单一 WS 连接，从初始快照加载 presence 并监听 `presence` 事件更新界面。

## 类型与验证

- Go Gateway 在 `protocol.go` 中使用 Go 结构体和 `encoding/json` 验证每个入站帧。
- 协议类型定义在 `pkg/types/types_gateway.go` 中。
- 前端 UI（TypeScript）通过 WebSocket 消费 JSON 事件。

## 连接快照

- `hello-ok` 包含 `snapshot`（`presence`、`health`、`stateVersion`、`uptimeMs`）以及 `policy {maxPayload,maxBufferedBytes,tickIntervalMs}`，客户端可立即渲染无需额外请求。
- `health`/`system-presence` 仍可用于手动刷新，但连接时不是必需的。

## 错误码（res.error 结构）

- 错误格式：`{ code, message, details?, retryable?, retryAfterMs? }`。
- 标准错误码：
  - `NOT_LINKED` — 频道未认证。
  - `AGENT_TIMEOUT` — Agent 未在配置的截止时间内响应。
  - `INVALID_REQUEST` — 参数验证失败。
  - `UNAVAILABLE` — Gateway 正在关闭或依赖不可用。

## 保活行为

- `tick` 事件（或 WS ping/pong）周期性发出，让客户端在无流量时仍知 Gateway 存活。
- 发送/Agent 确认保持独立响应；不在 tick 中叠加发送确认。

## 重放 / 间隙

- 事件不会重放。客户端检测到 seq 间隙时应刷新（`health` + `system-presence`）后再继续。

## 服务监管（macOS 示例）

- 使用 launchd 保持服务存活：
  - Program：`openacosmi` 二进制路径
  - Arguments：`gateway start`
  - KeepAlive：true
  - StandardOut/Err：文件路径或 `syslog`
- 失败时 launchd 自动重启；致命配置错误应持续退出以引起运维注意。
- LaunchAgent 按用户运行，需要已登录的会话；无头部署使用自定义 LaunchDaemon。
  - `openacosmi gateway install` 写入 `~/Library/LaunchAgents/bot.molt.gateway.plist`。
  - `openacosmi doctor` 审计 LaunchAgent 配置并可更新到当前推荐默认值。

## Gateway 服务管理（CLI）

使用 Go CLI 管理 Gateway 服务：

```bash
openacosmi gateway status
openacosmi gateway install
openacosmi gateway stop
openacosmi gateway restart
openacosmi logs --follow
```

说明：

- `gateway status` 默认通过 RPC 探测 Gateway（可用 `--url` 覆盖）。
- `gateway status --json` 输出稳定的 JSON 格式，适合脚本使用。
- `logs` 通过 RPC 拉取 Gateway 日志（无需手动 `tail`/`grep`）。
- 建议**每台机器一个 Gateway**；使用隔离的 profile/端口实现冗余。详见 [多 Gateway](/gateway/multiple-gateways)。
- `gateway install` 已安装时为空操作；使用 `--force` 重新安装。

macOS 应用集成：

- OpenAcosmi.app 可安装 per-user LaunchAgent，标签为 `bot.molt.gateway`。
- 停止：`openacosmi gateway stop`（或 `launchctl bootout gui/$UID/bot.molt.gateway`）。
- 重启：`openacosmi gateway restart`（或 `launchctl kickstart -k gui/$UID/bot.molt.gateway`）。

## 服务监管（systemd 用户单元）

在 Linux/WSL2 上，OpenAcosmi 默认安装 **systemd 用户服务**。
单用户机器推荐用户服务；多用户或常驻服务器推荐**系统服务**。

`openacosmi gateway install` 写入用户单元。`openacosmi doctor` 审计并可更新到推荐默认值。

创建 `~/.config/systemd/user/openacosmi-gateway[-<profile>].service`：

```ini
[Unit]
Description=OpenAcosmi Gateway (profile: <profile>)
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/openacosmi gateway start --port 19001
Restart=always
RestartSec=5
Environment=OPENACOSMI_GATEWAY_TOKEN=
WorkingDirectory=/home/youruser

[Install]
WantedBy=default.target
```

启用 lingering（确保用户服务在注销后继续运行）：

```bash
sudo loginctl enable-linger youruser
```

然后启用服务：

```bash
systemctl --user enable --now openacosmi-gateway[-<profile>].service
```

**替代方案（系统服务）** — 常驻或多用户服务器，可安装 systemd **系统**单元：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now openacosmi-gateway[-<profile>].service
```

## Windows (WSL2)

Windows 安装应使用 **WSL2**，按照上述 Linux systemd 部分操作。

## 运维检查

- 存活性：打开 WS 发送 `req:connect` → 预期收到 `payload.type="hello-ok"`（含快照）。
- 就绪性：调用 `health` → 预期 `ok: true` 且频道已连接。
- 调试：订阅 `tick` 和 `presence` 事件；检查 `status` 显示认证状态。

## 安全保证

- 默认每主机一个 Gateway；多 profile 时隔离端口/状态并指向正确实例。
- Gateway 宕机时发送直接失败（fail-fast），不回退到直连频道。
- 非法首帧或格式错误的 JSON 被拒绝并关闭连接。
- 优雅关闭：发出 `shutdown` 事件后关闭；客户端必须处理关闭 + 重连。

## CLI 辅助工具

- `openacosmi gateway status` — 通过 RPC 请求 Gateway 状态。
- `openacosmi message send --target <num> --message "hi"` — 通过 Gateway 发送消息。
- `openacosmi agent --message "hi" --to <num>` — 运行 Agent 回合。
- `openacosmi gateway stop|restart` — 停止/重启 Gateway 服务。
- Gateway 子命令假定 Gateway 已在 `--url` 运行；不再自动启动。

## 迁移指南

- Gateway 已从 Node.js 完全迁移至 Go 实现（`backend/internal/gateway/`）。
- 所有 `pnpm` / TypeScript 相关命令已废弃，使用 `make gateway-dev` 或 Go 二进制替代。
- 客户端需使用 WS 协议并携带强制 connect 握手和结构化 presence。
