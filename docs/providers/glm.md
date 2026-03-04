---
summary: "GLM 模型系列概览及 OpenAcosmi 配置"
read_when:
  - 使用 GLM 模型
  - 需要 GLM 模型命名及设置
title: "GLM 模型"
status: active
arch: rust-cli+go-gateway
---

# GLM 模型

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - GLM 通过 `zai` 供应商访问，API Key 环境变量 `XAI_API_KEY` 由 **Go Gateway** 解析
> - Onboard 流程由 **Rust CLI** 实现（`cli-rust/crates/oa-cmd-onboard/`）

GLM 是一个**模型系列**（非公司），通过 Z.AI 平台提供。在 OpenAcosmi 中，GLM 模型通过 `zai` 供应商访问，模型 ID 如 `zai/glm-4.7`。

## CLI 设置

```bash
openacosmi onboard --auth-choice zai-api-key
```

## 配置示例

```json5
{
  env: { ZAI_API_KEY: "sk-..." },
  agents: { defaults: { model: { primary: "zai/glm-4.7" } } },
}
```

## 注意事项

- GLM 版本和可用性可能会变化；请查看 Z.AI 文档获取最新信息。
- 示例模型 ID 包括 `glm-4.7` 和 `glm-4.6`。
- 供应商详情参见 [Z.AI 供应商](/providers/zai)。
