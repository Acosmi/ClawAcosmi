---
summary: "符合当前 schema 的常见 OpenAcosmi 配置示例"
read_when:
  - 学习如何配置 OpenAcosmi
  - 寻找配置示例
  - 首次设置 OpenAcosmi
title: "配置示例"
---

# 配置示例

> [!IMPORTANT]
> **架构状态**：配置由 **Go Gateway** 加载（`backend/internal/gateway/config_loader.go`），
> 默认端口为 `19001`。

示例与当前配置 schema 对齐。详尽参考见 [配置](/gateway/configuration)。

## 快速开始

### 最小配置

```json5
{
  agent: { workspace: "~/.openacosmi/workspace" },
  channels: { whatsapp: { allowFrom: ["+15555550123"] } },
}
```

### 推荐起步配置

```json5
{
  identity: { name: "助手", theme: "helpful assistant", emoji: "🦜" },
  agent: {
    workspace: "~/.openacosmi/workspace",
    model: { primary: "anthropic/claude-sonnet-4-5" },
  },
  channels: {
    whatsapp: {
      allowFrom: ["+15555550123"],
      groups: { "*": { requireMention: true } },
    },
  },
}
```

## 常见模式

### 多平台设置

```json5
{
  agent: { workspace: "~/.openacosmi/workspace" },
  channels: {
    whatsapp: { allowFrom: ["+15555550123"] },
    telegram: { enabled: true, botToken: "YOUR_TOKEN", allowFrom: ["123456789"] },
    discord: { enabled: true, token: "YOUR_TOKEN", dm: { allowFrom: ["yourname"] } },
  },
}
```

### 安全 DM 模式（多用户 DM）

多人可 DM 时，使用 `dmScope: "per-channel-peer"` 隔离会话：

```json5
{
  session: { dmScope: "per-channel-peer" },
  channels: {
    whatsapp: { dmPolicy: "allowlist", allowFrom: ["+15555550123", "+15555550124"] },
  },
}
```

### OAuth + API Key 回退

```json5
{
  auth: {
    profiles: {
      "anthropic:subscription": { provider: "anthropic", mode: "oauth" },
      "anthropic:api": { provider: "anthropic", mode: "api_key" },
    },
    order: { anthropic: ["anthropic:subscription", "anthropic:api"] },
  },
}
```

### 工作 Bot（受限访问）

```json5
{
  identity: { name: "WorkBot", theme: "professional assistant" },
  agent: { workspace: "~/work-openacosmi", elevated: { enabled: false } },
  channels: {
    slack: {
      enabled: true,
      botToken: "xoxb-...",
      channels: { "#engineering": { allow: true, requireMention: true } },
    },
  },
}
```

### 仅本地模型

```json5
{
  agent: {
    workspace: "~/.openacosmi/workspace",
    model: { primary: "lmstudio/minimax-m2.1-gs32" },
  },
  models: {
    mode: "merge",
    providers: {
      lmstudio: {
        baseUrl: "http://127.0.0.1:1234/v1",
        apiKey: "lmstudio",
        api: "openai-responses",
        models: [{
          id: "minimax-m2.1-gs32", name: "MiniMax M2.1",
          cost: { input: 0, output: 0 }, contextWindow: 196608,
        }],
      },
    },
  },
}
```

### Gateway + 网络示例

```json5
{
  gateway: {
    mode: "local",
    port: 19001,
    bind: "loopback",
    controlUi: { enabled: true },
    auth: { mode: "token", token: "gateway-token" },
    tailscale: { mode: "serve" },
  },
}
```

## 提示

- `dmPolicy: "open"` 时 `allowFrom` 必须包含 `"*"`。
- 提供商 ID 格式不同（电话号码、用户 ID、频道 ID），参考各提供商文档。
- 更多可选配置：`web`、`browser`、`discovery`、`canvasHost`、`talk`。
