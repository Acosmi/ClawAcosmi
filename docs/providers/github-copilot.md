---
summary: "在 OpenAcosmi 中通过设备流程登录 GitHub Copilot"
read_when:
  - 使用 GitHub Copilot 作为模型供应商
  - 需要 openacosmi models auth login-github-copilot 流程
title: "GitHub Copilot"
status: active
arch: rust-cli+go-gateway
---

# GitHub Copilot

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - GitHub Copilot 模型定义位于 **Go Gateway**（`backend/internal/agents/models/github_copilot_models.go`）
> - 登录流程由 **Rust CLI** 实现
> - 模型命令通过 `oa-cmd-models` crate 处理

## 什么是 GitHub Copilot？

GitHub Copilot 是 GitHub 的 AI 编程助手。它为你的 GitHub 账户和计划提供 Copilot 模型访问。OpenAcosmi 支持两种方式使用 Copilot。

## 两种使用方式

### 1）内置 GitHub Copilot 供应商（`github-copilot`）

使用原生设备登录流程获取 GitHub token，然后在 OpenAcosmi 运行时交换为 Copilot API token。这是**默认**且最简单的方式，因为不需要 VS Code。

### 2）Copilot Proxy 插件（`copilot-proxy`）

使用 **Copilot Proxy** VS Code 扩展作为本地桥接。OpenAcosmi 与代理的 `/v1` 端点通信，使用你在其中配置的模型列表。当你已在 VS Code 中运行 Copilot Proxy 或需要通过其路由时，选择此方式。你必须启用插件并保持 VS Code 扩展运行。

## CLI 设置

```bash
openacosmi models auth login-github-copilot
```

系统会提示你访问一个 URL 并输入一次性代码。保持终端打开直到完成。

### 可选参数

```bash
openacosmi models auth login-github-copilot --profile-id github-copilot:work
openacosmi models auth login-github-copilot --yes
```

## 设置默认模型

```bash
openacosmi models set github-copilot/gpt-4o
```

### 配置示例

```json5
{
  agents: { defaults: { model: { primary: "github-copilot/gpt-4o" } } },
}
```

## 注意事项

- 需要交互式 TTY；请在终端中直接运行。
- Copilot 模型可用性取决于你的计划；如果模型被拒绝，请尝试其他 ID（例如 `github-copilot/gpt-4.1`）。
- 登录会将 GitHub token 存储在认证 profile 中，OpenAcosmi 运行时会交换为 Copilot API token。
