---
summary: "Windows（WSL2）支持 + 伴侣应用状态"
read_when:
  - 在 Windows 上安装 OpenAcosmi
  - 查看 Windows 伴侣应用状态
title: "Windows（WSL2）"
---

> **架构提示 — Rust CLI + Go Gateway**
> Windows 上通过 WSL2 运行 Go Gateway（`backend/cmd/acosmi`）和 Rust CLI（`openacosmi`）。
> 守护进程管理使用 systemd 用户服务。

# Windows（WSL2）

在 Windows 上推荐**通过 WSL2**（推荐 Ubuntu）使用 OpenAcosmi。
Rust CLI + Go Gateway 在 Linux 环境内运行，保持运行时一致性，
工具兼容性更好（Linux 二进制、skills）。原生 Windows 可能更复杂。
WSL2 提供完整的 Linux 体验 — 一条命令安装：`wsl --install`。

原生 Windows 伴侣应用已在计划中。

## 安装（WSL2）

- [快速开始](/start/getting-started)（在 WSL 内使用）
- [安装与更新](/install/updating)
- 官方 WSL2 指南（Microsoft）：[https://learn.microsoft.com/windows/wsl/install](https://learn.microsoft.com/windows/wsl/install)

## Gateway

- [Gateway 运维手册](/gateway)
- [配置](/gateway/configuration)

## Gateway 服务安装（CLI）

在 WSL2 内：

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

## 高级：通过 LAN 暴露 WSL 服务（portproxy）

WSL 有自己的虚拟网络。如果另一台机器需要访问 **WSL 内**运行的服务（SSH、本地 TTS 服务器或 Go Gateway），
必须将 Windows 端口转发到当前 WSL IP。WSL IP 在重启后会变化，因此可能需要刷新转发规则。

示例（以**管理员**身份的 PowerShell）：

```powershell
$Distro = "Ubuntu-24.04"
$ListenPort = 2222
$TargetPort = 22

$WslIp = (wsl -d $Distro -- hostname -I).Trim().Split(" ")[0]
if (-not $WslIp) { throw "WSL IP not found." }

netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=$ListenPort `
  connectaddress=$WslIp connectport=$TargetPort
```

允许端口通过 Windows 防火墙（一次性）：

```powershell
New-NetFirewallRule -DisplayName "WSL SSH $ListenPort" -Direction Inbound `
  -Protocol TCP -LocalPort $ListenPort -Action Allow
```

WSL 重启后刷新 portproxy：

```powershell
netsh interface portproxy delete v4tov4 listenport=$ListenPort listenaddress=0.0.0.0 | Out-Null
netsh interface portproxy add v4tov4 listenport=$ListenPort listenaddress=0.0.0.0 `
  connectaddress=$WslIp connectport=$TargetPort | Out-Null
```

说明：

- 从另一台机器通过 SSH 连接时目标为 **Windows 主机 IP**（示例：`ssh user@windows-host -p 2222`）。
- 远程节点必须指向**可达的** Gateway URL（不是 `127.0.0.1`）；使用 `openacosmi status --all` 确认。
- 使用 `listenaddress=0.0.0.0` 实现 LAN 访问；`127.0.0.1` 仅保持本地访问。
- 如需自动化，注册计划任务在登录时运行刷新步骤。

## WSL2 分步安装

### 1）安装 WSL2 + Ubuntu

打开 PowerShell（管理员）：

```powershell
wsl --install
# 或明确选择发行版：
wsl --list --online
wsl --install -d Ubuntu-24.04
```

如 Windows 要求则重启。

### 2）启用 systemd（Gateway 安装所需）

在 WSL 终端中：

```bash
sudo tee /etc/wsl.conf >/dev/null <<'EOF'
[boot]
systemd=true
EOF
```

然后从 PowerShell：

```powershell
wsl --shutdown
```

重新打开 Ubuntu，验证：

```bash
systemctl --user status
```

### 3）安装 OpenAcosmi（WSL 内）

在 WSL 内按照 Linux 快速开始流程：

```bash
# 构建 Go Gateway
cd backend && make build

# 构建 Rust CLI
cd cli-rust && cargo build --release

# 运行安装向导
openacosmi onboard
```

完整指南：[快速开始](/start/getting-started)

## Windows 伴侣应用

目前没有 Windows 伴侣应用。欢迎贡献力量使其实现。
