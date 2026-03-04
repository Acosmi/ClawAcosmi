---
summary: "macOS 上的 Gateway 运行时（外部 launchd 服务）"
read_when:
  - 打包 OpenAcosmi.app
  - 调试 macOS Gateway launchd 服务
  - 安装 macOS 的 Gateway CLI
title: "macOS 上的 Gateway"
---

> **架构提示 — Rust CLI + Go Gateway**
> macOS 应用不再内嵌运行时。需要外部安装 Rust CLI（`openacosmi`）和 Go Gateway。
> launchd 服务管理由 Rust CLI 的 `oa-daemon` crate 实现。

# macOS 上的 Gateway（外部 launchd）

OpenAcosmi.app 不再捆绑运行时。macOS 应用期望**外部**安装的 `openacosmi` Rust CLI，
不将 Gateway 作为子进程生成，而是管理每用户 launchd 服务以保持 Go Gateway 运行
（或连接到已运行的本地 Gateway）。

## 安装 CLI（本地模式必需）

Mac 上需要构建并安装 Rust CLI 和 Go Gateway：

```bash
# 构建 Go Gateway
cd backend && make build && cd ..

# 构建 Rust CLI
cd cli-rust && cargo build --release && cd ..

# 安装到 PATH
sudo cp cli-rust/target/release/openacosmi /usr/local/bin/
```

macOS 应用的**安装 CLI** 按钮执行相同的安装流程。

## Launchd（Gateway 作为 LaunchAgent）

标签：

- `bot.molt.gateway`（或使用 `--profile`/`OPENACOSMI_PROFILE` 时为 `bot.molt.<profile>`；旧版 `com.openacosmi.*` 可能仍存在）

Plist 位置（每用户）：

- `~/Library/LaunchAgents/bot.molt.gateway.plist`
  （或 `~/Library/LaunchAgents/bot.molt.<profile>.plist`）

管理器：

- macOS 应用在本地模式下拥有 LaunchAgent 的安装/更新。
- CLI 也可安装：`openacosmi gateway install`。

行为：

- "OpenAcosmi Active" 启用/禁用 LaunchAgent。
- 退出应用**不会**停止 Gateway（launchd 保持其运行）。
- 如果配置端口上已有 Gateway 运行，应用会连接到它而非启动新的。

日志：

- launchd stdout/err：`/tmp/openacosmi/openacosmi-gateway.log`

## 版本兼容性

macOS 应用检查 Gateway 版本是否与自身版本匹配。如不兼容，
更新 CLI 以匹配应用版本。

## 冒烟测试

```bash
openacosmi --version

OPENACOSMI_SKIP_CHANNELS=1 \
OPENACOSMI_SKIP_CANVAS_HOST=1 \
openacosmi gateway --port 18999 --bind loopback
```

然后：

```bash
openacosmi gateway call health --url ws://127.0.0.1:18999 --timeout 3000
```
