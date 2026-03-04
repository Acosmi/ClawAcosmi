---
summary: "集成 Tailscale Serve/Funnel 暴露 Gateway 控制面"
read_when:
  - 将 Gateway 控制 UI 暴露到 localhost 之外
  - 自动化 Tailnet 或公网访问
title: "Tailscale"
---

# Tailscale（Gateway 控制面）

> [!IMPORTANT]
> **架构状态**：Tailscale 集成由 **Go Gateway**（`backend/internal/gateway/tailscale_integration.go`）实现。

OpenAcosmi 可自动配置 Tailscale **Serve**（tailnet）或 **Funnel**（公网）。
Gateway 保持绑定 loopback，Tailscale 提供 HTTPS、路由和（Serve 模式下）身份 header。

## 模式

- `serve`：Tailnet-only Serve（`tailscale serve`）。
- `funnel`：公网 HTTPS（`tailscale funnel`），必须设置共享密码。
- `off`：默认（无 Tailscale 自动化）。

## 认证

`gateway.auth.mode` 控制握手：

- `token`（默认，设置 `OPENACOSMI_GATEWAY_TOKEN` 时）
- `password`（`OPENACOSMI_GATEWAY_PASSWORD` 或配置）

`tailscale.mode = "serve"` + `gateway.auth.allowTailscale = true` 时，Serve 代理请求可通过 Tailscale 身份 header 认证。

## 配置示例

### Tailnet-only（Serve）

```json5
{
  gateway: {
    bind: "loopback",
    tailscale: { mode: "serve" },
  },
}
```

### 直接绑定 Tailnet IP

```json5
{
  gateway: {
    bind: "tailnet",
    auth: { mode: "token", token: "your-token" },
  },
}
```

连接：`ws://<tailscale-ip>:19001`

### 公网（Funnel + 共享密码）

```json5
{
  gateway: {
    bind: "loopback",
    tailscale: { mode: "funnel" },
    auth: { mode: "password", password: "replace-me" },
  },
}
```

## CLI 示例

```bash
openacosmi gateway start --tailscale serve
openacosmi gateway start --tailscale funnel --auth password
```

## 注意事项

- Tailscale Serve/Funnel 需要安装并登录 `tailscale` CLI。
- `funnel` 模式必须使用 `password` 认证，避免公网暴露。
- `gateway.bind: "tailnet"` 是直接 Tailnet 绑定（无 HTTPS）。
- Serve/Funnel 仅暴露 **Gateway 控制 UI + WS**。

## 前提条件

- Serve 需要 tailnet 启用 HTTPS。
- Funnel 需要 Tailscale v1.38.3+、MagicDNS、HTTPS 和 funnel 节点属性。

## 相关链接

- [Tailscale Serve](https://tailscale.com/kb/1312/serve)
- [Tailscale Funnel](https://tailscale.com/kb/1223/tailscale-funnel)
