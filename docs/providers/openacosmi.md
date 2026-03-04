---
summary: "在 OpenAcosmi 中使用 OpenAcosmi Zen（精选模型）"
read_when:
  - 使用 OpenAcosmi Zen 进行模型访问
  - 需要精选的编程友好模型列表
title: "OpenAcosmi Zen"
status: active
arch: rust-cli+go-gateway
---

# OpenAcosmi Zen

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - Zen 模型定义在 **Go Gateway**（`backend/internal/agents/models/openacosmi_zen_models.go`）
> - API Key 环境变量：`OPENACOSMI_API_KEY`（由 `backend/internal/agents/models/providers.go` 解析）

OpenAcosmi Zen 是 OpenAcosmi 团队推荐的**精选编程模型列表**。它是一个可选的托管模型访问路径，使用 API Key 和 `openacosmi` 供应商。Zen 目前处于 Beta 阶段。

## CLI 设置

```bash
openacosmi onboard --auth-choice openacosmi-zen
# 或非交互模式
openacosmi onboard --openacosmi-zen-api-key "$OPENACOSMI_API_KEY"
```

## 配置示例

```json5
{
  env: { OPENACOSMI_API_KEY: "sk-..." },
  agents: { defaults: { model: { primary: "openacosmi/claude-opus-4-6" } } },
}
```

## 注意事项

- `OPENACOSMI_ZEN_API_KEY` 也受支持。
- 登录 Zen，添加账单信息，然后复制 API Key。
- OpenAcosmi Zen 按请求计费；详情请查看 OpenAcosmi 管理面板。
