---
summary: "Bonjour/mDNS 发现 + 调试（Gateway 信标、客户端、常见故障模式）"
read_when:
  - 调试 macOS/iOS 上的 Bonjour 发现问题
  - 修改 mDNS 服务类型或 TXT 记录
title: "Bonjour 发现"
---

# Bonjour / mDNS 发现

> [!IMPORTANT]
> **架构状态**：Bonjour 广播由 **Go Gateway**（`backend/internal/gateway/server_discovery.go`）实现。

OpenAcosmi 使用 Bonjour（mDNS / DNS-SD）作为**仅限 LAN 的便利**功能来发现 Gateway WS 端点。不替代 SSH 或 Tailnet 连接。

## 跨网络 Bonjour（Unicast DNS-SD + Tailscale）

如果节点和 Gateway 在不同网络，multicast mDNS 不跨网络。可切换到 **unicast DNS-SD**（"Wide-Area Bonjour"）。

步骤：

1. 在 Gateway 宿主机运行 DNS 服务器（通过 Tailnet 可达）。
2. 为 `_openacosmi-gw._tcp` 发布 DNS-SD 记录。
3. 配置 Tailscale **split DNS**。

### Gateway 配置

```json5
{
  gateway: { bind: "tailnet" },
  discovery: { wideArea: { enabled: true } },
}
```

### DNS 服务器设置

```bash
openacosmi dns setup --apply
```

验证：

```bash
dns-sd -B _openacosmi-gw._tcp openacosmi.internal.
```

### Gateway 监听安全

Gateway WS 端口（默认 `19001`）默认绑定 loopback。Tailnet 设置使用 `gateway.bind: "tailnet"`。

## 服务类型

- `_openacosmi-gw._tcp` — Gateway 传输信标

## TXT 键（非机密）

- `role=gateway`
- `displayName=<friendly name>`
- `gatewayPort=<port>`
- `canvasPort=<port>`
- `transport=gateway`
- `tailnetDns=<magicdns>`（可选）

## macOS 调试

```bash
# 浏览实例
dns-sd -B _openacosmi-gw._tcp local.

# 解析实例
dns-sd -L "<instance>" _openacosmi-gw._tcp local.
```

## Gateway 日志调试

日志中查找 `bonjour:` 行：

- `bonjour: advertise failed ...`
- `bonjour: ... name conflict resolved`
- `bonjour: watchdog detected non-announced service ...`

## 常见故障

- **Bonjour 不跨网络**：使用 Tailnet 或 SSH。
- **Multicast 被阻止**：部分 Wi-Fi 网络禁用 mDNS。
- **休眠 / 接口变更**：macOS 可能暂时丢失 mDNS 结果。

## 禁用 / 配置

- `OPENACOSMI_DISABLE_BONJOUR=1` 禁用广播。
- `gateway.bind` 控制 Gateway 绑定模式。

## 相关文档

- 发现策略和传输选择：[发现](/gateway/discovery)
- 节点配对 + 审批：[Gateway 配对](/gateway/pairing)
