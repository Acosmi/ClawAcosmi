---
summary: "在 OpenAcosmi 中使用百度千帆统一 API 访问多种模型"
read_when:
  - 需要单个 API Key 访问多种 LLM
  - 需要百度千帆设置指南
title: "千帆 Qianfan"
status: active
arch: rust-cli+go-gateway
---

# 千帆（Qianfan）供应商指南

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - 千帆供应商默认配置：**Go Gateway**（`backend/internal/agents/models/providers.go` — `qianfan`，Base URL: `https://qianfan.baidubce.com/v2`，默认模型: `deepseek-v3.2`）
> - API Key 环境变量：`QIANFAN_API_KEY`
> - Onboard 流程由 **Rust CLI** 实现（`cli-rust/crates/oa-cmd-onboard/src/auth/models.rs` 中定义 `QIANFAN_BASE_URL`、`QIANFAN_DEFAULT_MODEL_ID`）

千帆是百度的 MaaS 平台，提供**统一 API**，通过单一端点和 API Key 将请求路由到多种模型。它兼容 OpenAI，大多数 OpenAI SDK 只需切换 Base URL 即可使用。

## 前置条件

1. 拥有千帆 API 访问权限的百度云账户
2. 从千帆控制台获取的 API Key
3. 已安装 OpenAcosmi

## 获取 API Key

1. 访问[千帆控制台](https://console.bce.baidu.com/qianfan/ais/console/apiKey)
2. 创建新应用或选择已有应用
3. 生成 API Key（格式：`bce-v3/ALTAK-...`）
4. 复制 API Key 用于 OpenAcosmi

## CLI 设置

```bash
openacosmi onboard --auth-choice qianfan-api-key
```

## 相关文档

- [OpenAcosmi 配置](/gateway/configuration)
- [模型供应商](/concepts/model-providers)
- [Agent 设置](/concepts/agent)
- [千帆 API 文档](https://cloud.baidu.com/doc/qianfan-api/s/3m7of64lb)
