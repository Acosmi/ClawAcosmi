---
read_when:
  - 你想通过 OpenAcosmi Zen 访问模型
  - 你想要一个适合编程的精选模型列表
summary: 在 OpenAcosmi 中使用 OpenAcosmi Zen（精选模型）
title: OpenAcosmi Zen
x-i18n:
  generated_at: "2026-02-01T21:35:16Z"
  model: claude-opus-4-5
  provider: pi
  source_hash: 1390f9803a3cac48cb40694dd69267e3ddccd203a4ce8babda3198b926b5f6a3
  source_path: providers/openacosmi.md
  workflow: 15
---

# OpenAcosmi Zen

OpenAcosmi Zen 是由 OpenAcosmi 团队推荐的一组**精选模型列表**，适用于编程智能体。它是一个可选的托管模型访问路径，使用 API 密钥和 `openacosmi` 提供商。Zen 目前处于测试阶段。

## CLI 设置

```bash
openacosmi onboard --auth-choice openacosmi-zen
# 或非交互式
openacosmi onboard --openacosmi-zen-api-key "$OPENACOSMI_API_KEY"
```

## 配置片段

```json5
{
  env: { OPENACOSMI_API_KEY: "sk-..." },
  agents: { defaults: { model: { primary: "openacosmi/claude-opus-4-6" } } },
}
```

## 注意事项

- 也支持 `OPENACOSMI_ZEN_API_KEY`。
- 你需要登录 Zen，添加账单信息，然后复制你的 API 密钥。
- OpenAcosmi Zen 按请求计费；详情请查看 OpenAcosmi 控制台。
