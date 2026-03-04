---
summary: "探索：模型配置、认证配置文件和回退行为"
read_when:
  - 探索未来模型选择 + 认证配置文件方案
title: "模型配置探索"
status: draft
arch: go-gateway
---

# 模型配置（探索）

> [!NOTE]
> **架构状态**：模型配置和回退逻辑由 **Go Gateway**（`backend/internal/agents/`）实现。
> 本文仅为探索性想法，非正式规范。

本文档记录未来模型配置的**想法**，并非发布规范。当前行为参见：

- [模型](/concepts/models)
- [模型故障转移](/concepts/model-failover)
- [OAuth + 配置文件](/concepts/oauth)

## 动机

运维人员需要：

- 每个 Provider 支持多个认证配置文件（个人 vs 工作）。
- 简单的 `/model` 选择和可预测的回退行为。
- 文本模型和图像模型之间清晰分离。

## 可能方向（概要）

- 保持模型选择简洁：`provider/model` 加可选别名。
- 允许 Provider 拥有多个认证配置文件，并有明确的优先顺序。
- 使用全局回退列表，使所有会话的故障转移行为一致。
- 仅在显式配置时覆盖图像路由。

## 待讨论问题

- 配置文件轮换应按 Provider 还是按模型？
- UI 如何为会话展示配置文件选择？
- 从旧版配置键迁移的最安全路径是什么？
