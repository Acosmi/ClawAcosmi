---
summary: "在 OpenAcosmi 中通过 API Key 或 Codex 订阅使用 OpenAI"
read_when:
  - 使用 OpenAI 模型
  - 使用 Codex 订阅认证
title: "OpenAI"
status: active
arch: rust-cli+go-gateway
---

# OpenAI

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - API Key 环境变量 `OPENAI_API_KEY` 由 **Go Gateway** 解析（`backend/internal/agents/models/providers.go`）
> - Codex OAuth 流程由 **Rust CLI** 实现（`cli-rust/crates/oa-cmd-auth/src/apply/`）
> - Onboard 流程：`cli-rust/crates/oa-cmd-onboard/`

OpenAI 提供 GPT 模型的开发者 API。Codex 支持 **ChatGPT 登录**（订阅访问）或 **API Key 登录**（按量计费）。Codex 云端需要 ChatGPT 登录。

## 方式 A：OpenAI API Key（OpenAI Platform）

**适用场景：** 直接 API 访问和按量计费。
从 OpenAI 管理面板获取 API Key。

### CLI 设置

```bash
openacosmi onboard --auth-choice openai-api-key
# 或非交互模式
openacosmi onboard --openai-api-key "$OPENAI_API_KEY"
```

### 配置示例

```json5
{
  env: { OPENAI_API_KEY: "sk-..." },
  agents: { defaults: { model: { primary: "openai/gpt-5.1-codex" } } },
}
```

## 方式 B：OpenAI Code（Codex）订阅

**适用场景：** 使用 ChatGPT/Codex 订阅访问而非 API Key。
Codex 云端需要 ChatGPT 登录，Codex CLI 支持 ChatGPT 或 API Key 登录。

### CLI 设置（Codex OAuth）

```bash
# 在向导中运行 Codex OAuth
openacosmi onboard --auth-choice openai-codex

# 或直接运行 OAuth
openacosmi models auth login --provider openai-codex
```

### 配置示例（Codex 订阅）

```json5
{
  agents: { defaults: { model: { primary: "openai-codex/gpt-5.3-codex" } } },
}
```

## 注意事项

- 模型引用始终使用 `provider/model` 格式（参见 [模型概念](/concepts/models)）。
- 认证详情与复用规则参见 [OAuth 概念](/concepts/oauth)。
