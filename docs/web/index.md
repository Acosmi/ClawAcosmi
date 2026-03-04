---
summary: "Gateway Web 界面：Control UI、绑定模式和安全"
read_when:
  - 通过 Tailscale 访问 Gateway
  - 使用浏览器 Control UI 和配置编辑
title: "Web 界面"
status: active
arch: rust-cli+go-gateway
---

# Web 界面(Gateway)

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
> Control UI 由 **Go Gateway** 的 HTTP 服务器托管（Vite + Lit 构建）。

Go Gateway 从与 WebSocket 相同的端口服务一个小型的**浏览器 Control UI**（Vite + Lit）：

- default: `http://<host>:18789/`
- optional prefix: set `gateway.controlUi.basePath` (e.g. `/openacosmi`)

Capabilities live in [Control UI](/web/control-ui).
This page focuses on bind modes, security, and web-facing surfaces.

## Webhooks

When `hooks.enabled=true`, the Gateway also exposes a small webhook endpoint on the same HTTP server.
See [Gateway configuration](/gateway/configuration) → `hooks` for auth + payloads.

## Config (default-on)

The Control UI is **enabled by default** when assets are present (`dist/control-ui`).
You can control it via config:

```json5
{
  gateway: {
    controlUi: { enabled: true, basePath: "/openacosmi" }, // basePath optional
  },
}
```

## Tailscale access

### Integrated Serve (recommended)

Keep the Gateway on loopback and let Tailscale Serve proxy it:

```json5
{
  gateway: {
    bind: "loopback",
    tailscale: { mode: "serve" },
  },
}
```

Then start the gateway:

```bash
openacosmi gateway
```

Open:

- `https://<magicdns>/` (or your configured `gateway.controlUi.basePath`)

### Tailnet bind + token

```json5
{
  gateway: {
    bind: "tailnet",
    controlUi: { enabled: true },
    auth: { mode: "token", token: "your-token" },
  },
}
```

Then start the gateway (token required for non-loopback binds):

```bash
openacosmi gateway
```

Open:

- `http://<tailscale-ip>:18789/` (or your configured `gateway.controlUi.basePath`)

### Public internet (Funnel)

```json5
{
  gateway: {
    bind: "loopback",
    tailscale: { mode: "funnel" },
    auth: { mode: "password" }, // or OPENACOSMI_GATEWAY_PASSWORD
  },
}
```

## Security notes

- Gateway auth is required by default (token/password or Tailscale identity headers).
- Non-loopback binds still **require** a shared token/password (`gateway.auth` or env).
- The wizard generates a gateway token by default (even on loopback).
- The UI sends `connect.params.auth.token` or `connect.params.auth.password`.
- The Control UI sends anti-clickjacking headers and only accepts same-origin browser
  websocket connections unless `gateway.controlUi.allowedOrigins` is set.
- With Serve, Tailscale identity headers can satisfy auth when
  `gateway.auth.allowTailscale` is `true` (no token/password required). Set
  `gateway.auth.allowTailscale: false` to require explicit credentials. See
  [Tailscale](/gateway/tailscale) and [Security](/gateway/security).
- `gateway.tailscale.mode: "funnel"` requires `gateway.auth.mode: "password"` (shared password).

## 构建 UI

Go Gateway 从 `dist/control-ui` 目录服务静态文件。通过以下命令构建：

```bash
cd ui && npm install && npm run build
```
