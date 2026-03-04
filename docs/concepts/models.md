---
summary: "Models CLI：列出、设置、别名、回退、扫描和状态"
read_when:
  - 使用 openacosmi models 命令
  - 排查"Model is not allowed"错误
title: "Models CLI"
status: active
arch: rust-cli
crate: oa-cmd-models
---

# Models CLI

> [!NOTE]
> **架构状态**：模型命令由 **Rust CLI** 实现（`cli-rust/crates/oa-cmd-models/`）。
> 模型选择和运行时解析由 **Go Gateway** 执行。

## 模型如何选择

1. **主模型**（`agents.defaults.model`）：如果可用（认证和未冷却），使用此模型。
2. **回退列表**（`agents.defaults.fallbacks`）：主模型不可用时逐一尝试。
3. **Provider 认证故障转移**：同一 Provider 内，在可用的认证 Profile 间轮换。

## 模型白名单

`agents.defaults.models`（数组）可限制允许的模型。**设置后即变为白名单**：未列出的模型将被拒绝。

```json5
{
  agent: {
    models: [
      "anthropic/claude-sonnet-4-20250514",
      "openai/gpt-4.1",
    ],
  },
}
```

## "Model is not allowed" 错误

原因：

1. `agents.defaults.models` 被设置但未包含你的模型 — 添加或留空。
2. 无匹配的认证 Profile — 运行 `openacosmi auth`。
3. Profile 在冷却中 — 等待或添加更多 Profile。

## 设置（首次）

```bash
openacosmi onboard     # 交互式首次设置
```

引导模型选择，包括设置白名单。

## 聊天中切换模型

- `/model` 打开交互式模型选择。
- `/model <model>` 为此会话设置模型。
- `/model list` 列出可用模型。
- `/model status` 显示当前模型 + 回退 + Profile 状态。

## CLI 命令

### 列出

```bash
openacosmi models list            # 列出配置的模型
openacosmi models status          # 详细状态（认证 + 冷却）
```

### 设置

```bash
openacosmi models set anthropic/claude-sonnet-4-20250514
openacosmi models set-image openai/gpt-4.1      # 设置图片模型
```

### 别名

```bash
openacosmi models aliases list
openacosmi models aliases add fast openai/gpt-4.1-mini
openacosmi models aliases remove fast
```

### 回退

```bash
openacosmi models fallbacks list
openacosmi models fallbacks add openai/gpt-4.1
openacosmi models fallbacks remove openai/gpt-4.1
openacosmi models fallbacks clear
```

### 扫描（OpenRouter 免费模型）

```bash
openacosmi models scan     # 扫描可用的 OpenRouter 免费模型
```
