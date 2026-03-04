---
summary: "会话剪枝：修剪旧工具结果以减少上下文膨胀"
read_when:
  - 需要减少工具输出导致的 LLM 上下文增长
  - 调整 agents.defaults.contextPruning
title: "会话剪枝"
status: active
arch: go-gateway
---

# 会话剪枝

> [!NOTE]
> **架构状态**：会话剪枝由 **Go Gateway** 在每次 LLM 调用前执行（`backend/internal/agents/runner/`）。

会话剪枝在每次 LLM 调用前，从内存中的上下文修剪**旧工具结果**。它**不会**重写磁盘上的会话历史（`*.jsonl`）。

## 何时运行

- 当 `mode: "cache-ttl"` 启用且该会话的最后一次 Anthropic 调用早于 `ttl` 时。
- 仅影响该请求发送给模型的消息。
- 仅对 Anthropic API 调用（及 OpenRouter Anthropic 模型）生效。
- 为获得最佳效果，将 `ttl` 与模型的 `cacheControlTtl` 匹配。
- 剪枝后 TTL 窗口重置，后续请求保持缓存直到 `ttl` 再次到期。

## 智能默认值（Anthropic）

- **OAuth 或 setup-token** Profile：启用 `cache-ttl` 剪枝，心跳设为 `1h`。
- **API Key** Profile：启用 `cache-ttl` 剪枝，心跳设为 `30m`，Anthropic 模型默认 `cacheControlTtl` 设为 `1h`。
- 如果显式设置了任何值，OpenAcosmi **不会**覆盖。

## 改善什么（成本 + 缓存行为）

- **为何剪枝**：Anthropic 提示词缓存仅在 TTL 内生效。会话空闲超过 TTL 后，下次请求除非先修剪，否则会重新缓存完整提示词。
- **什么变便宜**：剪枝减少 TTL 过期后首次请求的 **cacheWrite** 大小。
- **TTL 重置的意义**：剪枝运行后缓存窗口重置，后续请求可重用刚缓存的提示词。
- **不做什么**：剪枝不会增加 token 或"加倍"成本；它只改变 TTL 过期后首次请求时缓存什么。

## 可剪枝内容

- 仅 `toolResult` 消息。
- 用户 + 助手消息**永不**修改。
- 最后 `keepLastAssistants` 个助手消息受保护；其后的工具结果不被剪枝。
- 助手消息不足以建立截断点时跳过剪枝。
- 包含**图片块**的工具结果被跳过（永不修剪/清除）。

## 软修剪 vs 硬清除

- **软修剪**：仅针对超大工具结果。保留头尾，插入 `...`，并附加原始大小说明。跳过含图片块的结果。
- **硬清除**：用 `hardClear.placeholder` 替换整个工具结果。

## 工具选择

- `tools.allow` / `tools.deny` 支持 `*` 通配符。
- 拒绝优先。
- 匹配不区分大小写。
- 空 allow 列表 => 所有工具允许。

## 默认值（启用时）

- `ttl`: `"5m"`
- `keepLastAssistants`: `3`
- `softTrimRatio`: `0.3`
- `hardClearRatio`: `0.5`
- `minPrunableToolChars`: `50000`
- `softTrim`: `{ maxChars: 4000, headChars: 1500, tailChars: 1500 }`
- `hardClear`: `{ enabled: true, placeholder: "[旧工具结果内容已清除]" }`

## 示例

默认（关闭）：

```json5
{
  agent: {
    contextPruning: { mode: "off" },
  },
}
```

启用 TTL 感知剪枝：

```json5
{
  agent: {
    contextPruning: { mode: "cache-ttl", ttl: "5m" },
  },
}
```

限制剪枝到特定工具：

```json5
{
  agent: {
    contextPruning: {
      mode: "cache-ttl",
      tools: { allow: ["exec", "read"], deny: ["*image*"] },
    },
  },
}
```

参见配置参考：[Gateway 配置](/gateway/configuration)
