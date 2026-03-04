---
summary: "模型故障转移：认证 Profile 轮换与跨模型回退"
read_when:
  - 排查认证轮换或模型切换问题
  - 修改 cooldown、billing 或 failover 逻辑
title: "模型故障转移"
status: active
arch: go-gateway
---

# 模型故障转移

> [!NOTE]
> **架构状态**：模型故障转移由 **Go Gateway** 实现（`backend/internal/agents/`）。
> Rust CLI 通过 `openacosmi models status` 查看 Profile 状态和冷却信息。

OpenAcosmi 通过两级机制处理 Provider 故障：

1. **认证 Profile 轮换**：同一 Provider 内切换不同的认证凭证。
2. **模型回退**：所有 Profile 都失败后切换到 fallback 列表中的下一个模型。

## 认证 Profile 轮换

当 Provider 调用失败（429/500/认证错误）时，Gateway 尝试同一 Provider 的下一个 Profile。

### 认证存储

凭证存储在每个 Agent 的状态目录中：

```
~/.openacosmi/agents/<agentId>/agent/auth-profiles.json
```

每个 Provider 可有多个 Profile（不同的 API 密钥或 OAuth 令牌）。

### Profile ID 命名

- API 密钥：`<provider>:default`
- OAuth：`<provider>:<email>`（如 `anthropic:user@example.com`）

### 轮换顺序

1. 如设置了 `auth.order`：按显式列表顺序。
2. 否则：OAuth Profile 优先 → 同类型内按最旧令牌优先。

### 会话粘性

同一会话内倾向于复用相同 Profile 以保持 Provider 缓存一致。仅当当前 Profile 不可用时才切换。

### 冷却机制

- 失败的 Profile 进入冷却：`1min → 5min → 25min → 1h`（指数退避，上限 1h）。
- 成功调用后重置冷却计数器。
- 计费禁用（余额不足或配额耗尽）使用更长退避：`5h` 起步，最长 `24h`。

## 模型回退

当所有 Profile 均不可用时，Gateway 切换到 `agents.defaults.fallbacks` 中的下一个模型。

```json5
{
  agent: {
    model: "anthropic/claude-sonnet-4-20250514",
    fallbacks: [
      "openai/gpt-4.1",
      "google/gemini-2.5-pro",
    ],
  },
}
```

## 重要注意

- OAuth + API Key 共存时，轮换可能导致"看似丢失 OAuth"。使用 `auth.order` 固定顺序。
- 所有 Profile 均处于冷却中时，请求等待而非立即失败。
- 使用 `openacosmi models status` 查看实时 Profile 状态和剩余冷却。
