---
summary: "在插件中编写 agent 工具（schema、可选工具、允许列表）"
read_when:
  - 需要在插件中添加新的 agent 工具
  - 需要使工具通过允许列表实现可选启用
title: "插件 Agent 工具"
---

> **架构提示 — Rust CLI + Go Gateway**
> 插件系统完全使用 Go 实现（`backend/internal/plugins/`）。
> TS/JS 插件已不再支持。工具注册通过 `PluginAPI.RegisterTool` 完成。

# 插件 Agent 工具

OpenAcosmi 插件可注册 **agent 工具**（JSON Schema 函数），在 agent 运行期间暴露给 LLM。工具可以是**必需的**（始终可用）或**可选的**（需主动启用）。

Agent 工具在主配置的 `tools` 下配置，或在 `agents.list[].tools` 下按 agent 配置。允许列表/拒绝列表策略控制 agent 可调用哪些工具。

## 插件 API 概览

Go 插件通过 `PluginAPI` 结构体（`backend/internal/plugins/plugin_api.go`）注册功能：

```go
// PluginAPI 插件 API — 插件通过此接口注册功能
type PluginAPI struct {
    ID           string
    Name         string
    Version      string
    Description  string
    PluginConfig map[string]interface{}
    Runtime      PluginRuntime
    Logger       PluginLogger

    // 注册回调 — 由 Registry 在 CreateAPI 中注入
    RegisterTool          func(factory PluginToolFactory, opts *PluginToolOptions)
    RegisterHook          func(events []string, handler interface{}, opts *PluginHookOptions)
    RegisterHttpHandler   func(handler PluginHttpHandler)
    RegisterHttpRoute     func(path string, handler PluginHttpRouteHandler)
    RegisterChannel       func(plugin PluginChannelRegistration)
    RegisterProvider      func(provider ProviderPlugin)
    RegisterGatewayMethod func(method string, handler GatewayRequestHandler)
    RegisterCommand       func(command PluginCommandDefinition)
    RegisterService       func(service PluginService)
    // ...
}
```

## 基础工具注册

工具通过 `PluginToolFactory` 工厂函数注册，支持上下文感知（`PluginToolContext`）：

```go
// PluginToolFactory 工具工厂函数
type PluginToolFactory func(ctx PluginToolContext) interface{}

// PluginToolContext 工具注册上下文
type PluginToolContext struct {
    WorkspaceDir   string
    AgentDir       string
    AgentID        string
    SessionKey     string
    MessageChannel string
    AgentAccountID string
    Sandboxed      bool
}

// PluginToolOptions 工具注册选项
type PluginToolOptions struct {
    Name     string   // 单个工具名
    Names    []string // 多个工具名
    Optional bool     // 是否为可选工具
}
```

注册示例：

```go
api.RegisterTool(
    func(ctx plugins.PluginToolContext) interface{} {
        return &MyTool{workspace: ctx.WorkspaceDir}
    },
    &plugins.PluginToolOptions{
        Name:     "my_tool",
        Optional: false,
    },
)
```

工具注册由 `PluginRegistry.RegisterTool` 处理（`backend/internal/plugins/registry.go`），
它将工具工厂和元数据存入注册表，并检查名称冲突。

## 可选工具（需主动启用）

可选工具**永远不会**自动启用。用户必须将其添加到 agent 允许列表中。

通过 `Optional: true` 标记工具为可选：

```go
api.RegisterTool(
    func(ctx plugins.PluginToolContext) interface{} {
        return &WorkflowTool{agentID: ctx.AgentID}
    },
    &plugins.PluginToolOptions{
        Name:     "workflow_tool",
        Optional: true,  // 需主动启用
    },
)
```

在 `agents.list[].tools.allow`（或全局 `tools.allow`）中启用可选工具：

```json5
{
  agents: {
    list: [
      {
        id: "main",
        tools: {
          allow: [
            "workflow_tool", // 特定工具名称
            "workflow",      // 插件 ID（启用该插件的所有工具）
            "group:plugins", // 所有插件工具
          ],
        },
      },
    ],
  },
}
```

## 插件命令注册

除了 agent 工具，插件还可注册斜杠命令（`/command`）。命令通过 `RegisterCommand` 注册：

```go
api.RegisterCommand(plugins.PluginCommandDefinition{
    Name:        "my-cmd",
    Description: "执行自定义操作",
    AcceptsArgs: true,
    Handler: func(ctx plugins.PluginCommandContext) (plugins.PluginCommandResult, error) {
        return plugins.PluginCommandResult{
            Text: "处理完成: " + ctx.Args,
        }, nil
    },
})
```

命令名规则（`backend/internal/plugins/commands.go`）：

- 必须以字母开头，仅包含小写字母、数字、连字符和下划线
- 不得与保留命令名冲突（`help`、`status`、`config` 等）
- 已被其他插件注册的命令名会被拒绝

## 其他影响工具可用性的配置

- 仅包含插件工具的允许列表被视为插件启用；核心工具保持启用，除非你也将核心工具或组包含在允许列表中。
- `tools.profile` / `agents.list[].tools.profile`（基础允许列表）
- `tools.byProvider` / `agents.list[].tools.byProvider`（按 provider 的允许/拒绝）
- `tools.sandbox.tools.*`（沙箱环境下的工具策略）

## 插件钩子

插件可通过 `RegisterTypedHook` 注册生命周期钩子（`backend/internal/plugins/types.go`）：

- `before_agent_start` — agent 启动前
- `agent_end` — agent 结束后
- `message_received` — 收到消息时
- `message_sending` — 发送消息前（可修改或取消）
- `before_tool_call` / `after_tool_call` — 工具调用前/后
- `gateway_start` / `gateway_stop` — Gateway 启动/停止

## 规则与提示

- 工具名称**不得**与核心工具名称冲突；冲突的工具会被跳过。
- 允许列表中使用的插件 ID 不得与核心工具名称冲突。
- 对于触发副作用或需要额外二进制/凭证的工具，优先使用 `Optional: true`。
- 工厂函数在每个 agent 会话中被调用，`PluginToolContext` 提供当前上下文信息。
