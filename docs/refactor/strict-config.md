---
summary: "严格配置校验 + 仅通过 doctor 迁移"
read_when:
  - 设计或实现配置校验行为
  - 处理配置迁移或 doctor 工作流
  - 处理插件配置 schema 或插件加载门控
title: "严格配置校验"
status: active
arch: rust-cli+go-gateway
---

# 严格配置校验（仅 doctor 迁移）

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - 配置校验由 **Go Gateway** 实现（`backend/internal/config/validator.go`）
> - Doctor 命令由 **Rust CLI** 实现（`cli-rust/crates/oa-cmd-doctor/`）
> - 配置 Schema 定义在 `backend/internal/config/schema_hints_data.go`

## 目标

- **在任何层级拒绝未知配置键**（root + nested）。
- **拒绝没有 schema 的插件配置**；不加载该插件。
- **移除加载时的旧版自动迁移**；迁移仅通过 doctor 运行。
- **启动时自动运行 doctor（dry-run）**；如果无效，阻止非诊断命令。

## 非目标

- 加载时的向后兼容性（旧版键不自动迁移）。
- 静默丢弃未识别的键。

## 严格校验规则

- 配置必须在每个层级精确匹配 schema。
- 未知键是校验错误（root 或 nested 均不透传）。
- `plugins.entries.<id>.config` 必须通过插件的 schema 校验。
  - 如果插件缺少 schema，**拒绝加载插件**并显示明确错误。
- 未知的 `channels.<id>` 键是错误，除非插件 manifest 声明了该 channel id。
- 所有插件需要插件 manifest（`openacosmi.plugin.json`）。

## 插件 Schema 强制执行

- 每个插件在 manifest 中提供严格的 JSON Schema。
- 插件加载流程：
  1. 解析插件 manifest + schema（`openacosmi.plugin.json`）。
  2. 根据 schema 校验配置。
  3. 缺少 schema 或配置无效时：阻止插件加载，记录错误。
- 错误信息包含：
  - 插件 id
  - 原因（缺少 schema / 配置无效）
  - 校验失败的路径
- 禁用的插件保留配置，但 Doctor + 日志会显示警告。

## Doctor 流程

- Doctor 在**每次**加载配置时运行（默认 dry-run）。
- 如果配置无效：
  - 打印摘要 + 可操作的错误。
  - 提示：`openacosmi doctor --fix`。
- `openacosmi doctor --fix`：
  - 应用迁移。
  - 移除未知键。
  - 写入更新后的配置。

## 命令门控（配置无效时）

允许的命令（仅诊断）：

- `openacosmi doctor`
- `openacosmi logs`
- `openacosmi health`
- `openacosmi help`
- `openacosmi status`
- `openacosmi gateway status`

其他所有命令必须硬失败并提示："Config invalid. Run `openacosmi doctor --fix`."

## 错误 UX 格式

- 单个摘要标题。
- 分组部分：
  - 未知键（完整路径）
  - 旧版键 / 需要迁移
  - 插件加载失败（插件 id + 原因 + 路径）

## 实现触点

- `backend/internal/config/validator.go`：严格校验，拒绝未知键。
- `backend/internal/config/schema_hints_data.go`：确保严格通道 schema。
- `backend/internal/config/merge_patch.go`：配置合并与迁移。
- `backend/internal/config/defaults.go`：默认值管理。
- `backend/internal/gateway/plugin_registry.go`：插件 schema 注册与门控。
- CLI 命令门控在 `cli-rust/crates/oa-cmd-doctor/`。

## 测试

- 未知键拒绝（root + nested）。
- 插件缺少 schema → 插件加载被阻止并显示明确错误。
- 配置无效 → Gateway 启动被阻止（诊断命令除外）。
- Doctor dry-run 自动运行；`doctor --fix` 写入修正后的配置。
