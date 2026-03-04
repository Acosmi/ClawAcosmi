---
summary: "插件清单 + JSON Schema 要求（严格配置验证）"
read_when:
  - 构建 OpenAcosmi 插件
  - 发布插件配置 Schema 或调试插件验证错误
title: "插件清单"
---

> **架构提示 — Rust CLI + Go Gateway**
> 插件系统由 Go Gateway 实现（`backend/internal/plugins/`），
> 清单加载和验证参见 `manifest.go`，插件发现参见 `discovery.go`。

# 插件清单（openacosmi.plugin.json）

每个插件**必须**在**插件根目录**中包含一个 `openacosmi.plugin.json` 文件。
Go Gateway 使用此清单来**无需执行插件代码即可验证配置**。缺失或无效的清单将被视为插件错误并阻止配置验证。

Go 实现：`backend/internal/plugins/manifest.go`（`LoadPluginManifest` 函数、`PluginManifest` 结构体）。

完整插件系统指南参见：[插件](/tools/plugin)。

## 必需字段

```json
{
  "id": "voice-call",
  "configSchema": {
    "type": "object",
    "additionalProperties": false,
    "properties": {}
  }
}
```

必需键：

- `id`（字符串）：标准插件 ID。
- `configSchema`（对象）：插件配置的 JSON Schema（内联）。

可选键：

- `kind`（字符串）：插件类型（示例：`"memory"`）。
- `channels`（数组）：此插件注册的渠道 ID（示例：`["matrix"]`）。
- `providers`（数组）：此插件注册的 provider ID。
- `skills`（数组）：要加载的技能目录（相对于插件根目录）。
- `name`（字符串）：插件显示名称。
- `description`（字符串）：插件简短描述。
- `uiHints`（对象）：配置字段的标签/占位符/敏感标志，用于 UI 渲染。
- `version`（字符串）：插件版本（信息性）。

Go 结构体映射（`backend/internal/plugins/manifest.go`）：

```go
type PluginManifest struct {
    ID           string                        `json:"id"`
    ConfigSchema map[string]interface{}        `json:"configSchema"`
    Kind         PluginKind                    `json:"kind,omitempty"`
    Channels     []string                      `json:"channels,omitempty"`
    Providers    []string                      `json:"providers,omitempty"`
    Skills       []string                      `json:"skills,omitempty"`
    Name         string                        `json:"name,omitempty"`
    Description  string                        `json:"description,omitempty"`
    Version      string                        `json:"version,omitempty"`
    UiHints      map[string]PluginConfigUiHint `json:"uiHints,omitempty"`
}
```

## JSON Schema 要求

- **每个插件必须包含 JSON Schema**，即使不接受任何配置。
- 空 Schema 是可接受的（例如 `{ "type": "object", "additionalProperties": false }`）。
- Schema 在配置读写时验证，而非运行时。

## 验证行为

- 未知的 `channels.*` 键是**错误**，除非渠道 ID 由插件清单声明。
- `plugins.entries.<id>`、`plugins.allow`、`plugins.deny` 和 `plugins.slots.*`
  必须引用**可发现的**插件 ID。未知 ID 是**错误**。
- 如果插件已安装但清单或 Schema 损坏或缺失，
  验证失败，Doctor 报告插件错误。
- 如果存在插件配置但插件已**禁用**，配置保留，
  在 Doctor + 日志中显示**警告**。

Go 实现：`backend/internal/plugins/registry.go`（插件注册和验证逻辑）。

## 说明

- 清单对**所有插件**都是必需的，包括本地文件系统加载的插件。
- 运行时仍会单独加载插件模块；清单仅用于发现 + 验证。
- 如果插件依赖原生模块，请文档化构建步骤和任何包管理器许可列表要求。

## 插件发现

Go Gateway 按以下顺序扫描插件（`backend/internal/plugins/discovery.go`）：

1. 配置中的额外路径
2. 工作区 `.openacosmi/extensions/` 目录
3. 全局 `extensions/` 目录
4. 内置（bundled）插件目录
