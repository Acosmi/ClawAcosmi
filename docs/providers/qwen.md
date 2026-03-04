---
summary: "在 OpenAcosmi 中通过 OAuth 免费层使用通义千问 Qwen"
read_when:
  - 使用 Qwen 模型
  - 使用免费层 OAuth 访问 Qwen Coder
title: "通义千问 Qwen"
status: active
arch: rust-cli+go-gateway
---

# 通义千问（Qwen）

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - Qwen Portal 供应商默认配置：**Go Gateway**（`backend/internal/agents/models/providers.go` — `qwen-portal`，Base URL: `https://portal.qwen.ai/v1`）
> - API Key 环境变量：`DASHSCOPE_API_KEY`（回退：`QWEN_PORTAL_API_KEY`）
> - OAuth 登录由 **Rust CLI** 实现

Qwen 为 Qwen Coder 和 Qwen Vision 模型提供免费层 OAuth 流程（每天 2000 次请求，受 Qwen 速率限制约束）。

## 启用插件

```bash
openacosmi plugins enable qwen-portal-auth
```

启用后重启 Gateway。

## 认证

```bash
openacosmi models auth login --provider qwen-portal --set-default
```

这将运行 Qwen 设备码 OAuth 流程，并将供应商条目写入 `models.json`（加上 `qwen` 别名用于快速切换）。

## 模型 ID

- `qwen-portal/coder-model`
- `qwen-portal/vision-model`

切换模型：

```bash
openacosmi models set qwen-portal/coder-model
```

## 复用 Qwen Code CLI 登录

如果你已通过 Qwen Code CLI 登录，OpenAcosmi 会在加载认证存储时从 `~/.qwen/oauth_creds.json` 同步凭据。你仍需要一个 `models.providers.qwen-portal` 条目（使用上面的登录命令创建）。

## 注意事项

- Token 自动刷新；如果刷新失败或访问被撤销，请重新运行登录命令。
- 默认 Base URL：`https://portal.qwen.ai/v1`（如果 Qwen 提供了不同端点，可通过 `models.providers.qwen-portal.baseUrl` 覆盖）。
- 参见 [模型供应商](/concepts/model-providers) 了解供应商规则。
