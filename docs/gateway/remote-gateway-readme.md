---
summary: "SSH 隧道设置：连接 OpenAcosmi.app 到远程 Gateway"
read_when: "通过 SSH 将 macOS 应用连接到远程 Gateway"
title: "远程 Gateway 设置"
---

# 通过远程 Gateway 运行 OpenAcosmi.app

> [!IMPORTANT]
> **架构状态**：远程 Gateway 由 **Go** 实现。SSH 隧道用于转发 WebSocket 端口。

OpenAcosmi.app 使用 SSH 隧道连接远程 Gateway。

## 架构图

```
┌─────────────────────────────────────────────────┐
│                  客户端机器                       │
│                                                  │
│  OpenAcosmi.app ──► ws://127.0.0.1:19001         │
│                     │                            │
│  SSH 隧道 ──────────┘                            │
└─────────────────────┬────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────┐
│                  远程机器                         │
│                                                  │
│  Go Gateway ──► ws://127.0.0.1:19001             │
└─────────────────────────────────────────────────┘
```

## 快速设置

### 第 1 步：配置 SSH

编辑 `~/.ssh/config`：

```ssh
Host remote-gateway
    HostName <REMOTE_IP>
    User <REMOTE_USER>
    LocalForward 19001 127.0.0.1:19001
    IdentityFile ~/.ssh/id_rsa
```

### 第 2 步：复制 SSH 密钥

```bash
ssh-copy-id -i ~/.ssh/id_rsa <REMOTE_USER>@<REMOTE_IP>
```

### 第 3 步：设置 Gateway Token

```bash
launchctl setenv OPENACOSMI_GATEWAY_TOKEN "<your-token>"
```

### 第 4 步：启动 SSH 隧道

```bash
ssh -N remote-gateway &
```

### 第 5 步：重启 OpenAcosmi.app

## 开机自启隧道

保存 `~/Library/LaunchAgents/bot.molt.ssh-tunnel.plist`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "...">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>bot.molt.ssh-tunnel</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/bin/ssh</string>
        <string>-N</string>
        <string>remote-gateway</string>
    </array>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
```

加载：

```bash
launchctl bootstrap gui/$UID ~/Library/LaunchAgents/bot.molt.ssh-tunnel.plist
```

## 故障排除

```bash
# 检查隧道是否运行
ps aux | grep "ssh -N remote-gateway" | grep -v grep
lsof -i :19001

# 重启隧道
launchctl kickstart -k gui/$UID/bot.molt.ssh-tunnel

# 停止隧道
launchctl bootout gui/$UID/bot.molt.ssh-tunnel
```

## 工作原理

| 组件 | 作用 |
| --- | --- |
| `LocalForward 19001 127.0.0.1:19001` | 将本地端口转发到远程端口 |
| `ssh -N` | SSH 仅端口转发（不执行远程命令） |
| `KeepAlive` | 隧道崩溃时自动重启 |
| `RunAtLoad` | 登录时自动启动 |
