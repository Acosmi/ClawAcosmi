---
summary: "引导向导与配置 Schema 的 RPC 协议说明"
read_when: "修改引导向导步骤或配置 Schema 端点时"
title: "引导与配置协议"
status: active
arch: go-gateway + rust-cli
---

# 引导 + 配置协议

> [!NOTE]
> **架构状态**：Gateway RPC 由 **Go Gateway**（`backend/internal/gateway/`）实现。
> CLI 引导流程由 **Rust CLI**（`cli-rust/crates/oa-cmd-system/`）实现。

目的：在 CLI、macOS 应用和 Web UI 之间共享引导 + 配置界面。

## 组件

- 向导引擎（共享会话 + 提示 + 引导状态）。
- CLI 引导使用与 UI 客户端相同的向导流程。
- Gateway RPC 暴露向导 + 配置 Schema 端点。
- macOS 引导使用向导步骤模型。
- Web UI 从 JSON Schema + UI 提示渲染配置表单。

## Gateway RPC

- `wizard.start` 参数：`{ mode?: "local"|"remote", workspace?: string }`
- `wizard.next` 参数：`{ sessionId, answer?: { stepId, value? } }`
- `wizard.cancel` 参数：`{ sessionId }`
- `wizard.status` 参数：`{ sessionId }`
- `config.schema` 参数：`{}`

响应结构

- 向导：`{ sessionId, done, step?, status?, error? }`
- 配置 Schema：`{ schema, uiHints, version, generatedAt }`

## UI 提示

- `uiHints` 按路径键索引；可选元数据（label/help/group/order/advanced/sensitive/placeholder）。
- 敏感字段渲染为密码输入框；无额外脱敏层。
- 不支持的 Schema 节点回退到原始 JSON 编辑器。

## 备注

- 本文档是跟踪引导/配置协议重构的唯一位置。
