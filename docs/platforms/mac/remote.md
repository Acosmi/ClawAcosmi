---
summary: "macOS 远程模式连接管道（SSH 隧道 + Tailscale）"
read_when:
  - 配置 macOS 应用远程连接
  - 调试远程 Gateway 连接
title: "macOS 远程访问"
---

# macOS 远程访问

## 远程模式

macOS 应用在**远程模式**下连接到运行在另一台机器上的 Go Gateway。
应用通过 SSH 隧道或 Tailscale 建立安全连接。

## SSH 隧道连接

远程模式下，macOS 应用打开 SSH 隧道将 Go Gateway 端口转发到本地：

```bash
# 控制隧道（Gateway WebSocket 端口）
ssh -N -L 18789:127.0.0.1:18789 user@gateway-host
```

### 隧道特性

- **本地端口：** Gateway 端口（默认 `18789`），始终稳定。
- **远程端口：** 远程主机上的相同 Gateway 端口。
- **行为：** 无随机本地端口；应用复用现有隧道或按需重启。
- **SSH 参数：** BatchMode + ExitOnForwardFailure + keepalive。

### IP 报告

SSH 隧道使用回环地址，Gateway 会将节点 IP 显示为 `127.0.0.1`。
如需显示真实客户端 IP，使用**直连**（ws/wss）传输。

## Tailscale 连接

如果两端都在 Tailscale tailnet 上，可直接连接：

```bash
openacosmi config set gateway.bind tailnet
```

macOS 应用会自动发现 tailnet 上的 Gateway。

## 节点主机服务

远程模式下，macOS 应用还启动本地**节点主机服务**（launchd），
使远程 Go Gateway 可到达该 Mac 的节点能力（canvas、camera、system.run 等）。

## 设置步骤

1. 在远程机器上运行 Go Gateway。
2. macOS 应用设置中选择 **远程** 模式。
3. 输入远程主机信息（或选择已发现的 tailnet Gateway）。
4. 连接并批准节点配对。

## 相关文档

- [Gateway 协议](/gateway/protocol)
- [节点](/nodes)
