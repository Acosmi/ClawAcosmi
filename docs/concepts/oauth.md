---
summary: "OAuth 认证：令牌交换、存储和多账户模式"
read_when:
  - 修改 OAuth 流程或令牌存储
  - 排查认证/令牌刷新问题
title: "OAuth"
status: active
arch: go-gateway+rust-cli
---

# OAuth

> [!NOTE]
> **架构状态**：OAuth 令牌存储和刷新由 **Go Gateway** 管理。
> 初始认证流程通过 **Rust CLI** 的 `openacosmi auth` 命令触发（`oa-cmd-auth`）。

## 为什么有令牌池（Token Sink）

对于支持"仅一个活跃令牌"策略的 Provider（如 Anthropic），直接通过第三方 CLI 进行 OAuth 可能导致 OpenAcosmi 的令牌被吊销。令牌池存储允许 OpenAcosmi 管理轮换而不丢失凭证。

## 存储位置

OAuth 令牌存储在每个 Agent 的状态目录中：

```
~/.openacosmi/agents/<agentId>/agent/auth-profiles.json
```

格式（简化）：

```json
{
  "anthropic:user@example.com": {
    "provider": "anthropic",
    "type": "oauth",
    "accessToken": "...",
    "refreshToken": "...",
    "expiresAt": "2026-01-15T..."
  },
  "openai:default": {
    "provider": "openai",
    "type": "api-key",
    "apiKey": "sk-..."
  }
}
```

## Provider OAuth 流程

### Anthropic

```bash
openacosmi auth
# 或
openacosmi auth --provider anthropic
```

对于无浏览器的环境，使用 setup-token 流程：

```bash
openacosmi auth --provider anthropic --setup-token <token>
```

### OpenAI Codex（ChatGPT）

```bash
openacosmi auth --provider codex
```

使用 PKCE OAuth 流程。

### GitHub Copilot

```bash
openacosmi auth --provider copilot
```

### Google（Vertex / Gemini CLI）

```bash
openacosmi auth --provider vertex
```

## 令牌刷新

- 在令牌过期前自动刷新（由 Go Gateway 处理）。
- 刷新失败时 Profile 进入冷却。
- OAuth profile 的刷新令牌通常长期有效；访问令牌短期有效。

## 多账户模式

### 方式1：独立 Agent（推荐）

每个用户一个 Agent，各有独立的 `auth-profiles.json`：

```json5
{
  agents: {
    list: [
      { id: "alice", workspace: "~/.openacosmi/workspace-alice" },
      { id: "bob", workspace: "~/.openacosmi/workspace-bob" },
    ],
  },
}
```

### 方式2：同 Agent 多 Profile

同一 Agent 下多个 OAuth Profile，通过 `auth.order` 控制优先级：

```json5
{
  agent: {
    auth: {
      order: ["anthropic:alice@example.com", "anthropic:bob@example.com"],
    },
  },
}
```

**建议**：多人场景使用方式1以确保完全隔离。
