---
summary: "Hooks：命令和生命周期事件的事件驱动自动化"
read_when:
  - 需要为 /new、/reset、/stop 及 Agent 生命周期事件设置事件驱动自动化
  - 构建、安装或调试 hooks
title: "Hooks"
---

# Hooks（事件钩子）

> [!IMPORTANT]
> **架构状态**：内部事件 Hook 系统由 **Go Gateway** 实现。
> 核心代码：`backend/internal/hooks/`（类型定义、配置解析、发现、加载）、
> `backend/internal/gateway/`（Gateway 启动时加载 hooks）。
> Handler 类型：Go `func(event *InternalHookEvent) error`。

Hooks 提供了可扩展的事件驱动系统，用于在 Agent 命令和事件发生时自动执行操作。Hooks 从目录中自动发现，可通过 CLI 命令管理。

## 快速导航

- **Hooks**（本页）：在 Gateway 内部运行，响应 Agent 事件（`/new`、`/reset`、`/stop`、生命周期事件）。
- **Webhooks**：外部 HTTP 触发器。详见 [Webhooks](/automation/webhook)。

Hooks 也可以在插件中打包；详见 [Plugins](/tools/plugin#plugin-hooks)。

常见用途：

- 重置会话时保存记忆快照
- 保留命令审计日志
- 会话开始或结束时触发后续自动化
- 事件触发时写入文件或调用外部 API

## 内置 Hooks

OpenAcosmi 附带四个内置 hook，自动发现：

- **💾 session-memory**：`/new` 时保存会话上下文到工作区（默认 `~/.openacosmi/workspace/memory/`）
- **📝 command-logger**：记录所有命令事件到 `~/.openacosmi/logs/commands.log`
- **🚀 boot-md**：Gateway 启动时运行 `BOOT.md`（需启用 internal hooks）
- **😈 soul-evil**：在清洗窗口或随机概率下替换 `SOUL.md` 内容

管理命令：

```bash
# 列出可用 hooks
openacosmi hooks list

# 启用 hook
openacosmi hooks enable session-memory

# 检查 hook 状态
openacosmi hooks check

# 查看详细信息
openacosmi hooks info session-memory
```

## Hook 发现机制

Hooks 从三个目录自动发现（按优先级排列）：

1. **工作区 hooks**：`<workspace>/hooks/`（按 agent，最高优先级）
2. **托管 hooks**：`~/.openacosmi/hooks/`（用户安装，跨工作区共享）
3. **内置 hooks**：`<openacosmi>/dist/hooks/bundled/`（随 OpenAcosmi 一起发布）

每个 hook 是一个目录，包含：

```text
my-hook/
├── HOOK.md          # 元数据 + 文档
└── handler.go       # Handler 实现（Go）
```

## Hook Pack（包）

Hook pack 是标准包，通过 `package.json` 中的 `openacosmi.hooks` 导出一个或多个 hooks。安装方式：

```bash
openacosmi hooks install <path-or-spec>
```

## HOOK.md 格式

`HOOK.md` 文件在 YAML frontmatter 中包含元数据，正文为 Markdown 文档：

```markdown
---
name: my-hook
description: "此 hook 的简短描述"
metadata:
  { "openacosmi": { "emoji": "🔗", "events": ["command:new"], "requires": { "bins": ["node"] } } }
---

# My Hook

详细文档...
```

### 元数据字段

`metadata.openacosmi` 对象支持：

- **`emoji`**：CLI 显示用 emoji
- **`events`**：监听的事件数组（如 `["command:new", "command:reset"]`）
- **`export`**：使用的命名导出（默认 `"default"`）
- **`requires`**：可选需求
  - **`bins`**：PATH 上需要的二进制文件
  - **`anyBins`**：至少需要其中一个
  - **`env`**：必需的环境变量
  - **`config`**：必需的配置路径
  - **`os`**：必需的平台（如 `["darwin", "linux"]`）
- **`always`**：跳过资格检查（boolean）

## 事件类型

### 命令事件

Agent 命令触发时：

- **`command`**：所有命令事件（通用监听）
- **`command:new`**：执行 `/new` 命令时
- **`command:reset`**：执行 `/reset` 命令时
- **`command:stop`**：执行 `/stop` 命令时

### Agent 事件

- **`agent:bootstrap`**：工作区 bootstrap 文件注入前（hooks 可修改 `context.bootstrapFiles`）

### Gateway 事件

- **`gateway:startup`**：Gateway 启动后（渠道启动、hooks 加载完成）

### 工具结果 Hooks（插件 API）

- **`tool_result_persist`**：在工具结果写入会话 transcript 前进行转换。必须同步；返回更新后的 payload 或 `undefined` 保持原样。

## Handler 实现

Handler 是一个 Go 函数，签名为 `func(event *InternalHookEvent) error`：

```go
package myhook

import "github.com/openacosmi/claw-acismi/internal/hooks"

func Handler(event *hooks.InternalHookEvent) error {
    // 仅处理 'new' 命令
    if event.Type != hooks.HookEventCommand || event.Action != "new" {
        return nil
    }

    log.Printf("[my-hook] New 命令触发，会话: %s", event.SessionKey)

    // 自定义逻辑...

    // 可选：发送消息给用户
    event.Messages = append(event.Messages, "✨ My hook 已执行！")
    return nil
}
```

### 事件上下文

每个事件包含以下字段（Go struct `InternalHookEvent`，定义于 `backend/internal/hooks/hook_types.go`）：

```go
type InternalHookEvent struct {
    Type       InternalHookEventType  // "command" | "session" | "agent" | "gateway"
    Action     string                 // 如 "new", "reset", "stop"
    SessionKey string                 // 会话标识
    Context    map[string]interface{} // 附加上下文
    Timestamp  int64                  // Unix 毫秒
    Messages   []string               // 推送消息到此数组，将发送给用户
}
```

`Context` 可能包含：`sessionId`、`sessionFile`、`commandSource`（如 "whatsapp"、"telegram"）、`senderId`、`workspaceDir` 等。

## 创建自定义 Hook

### 1. 选择位置

- **工作区 hooks**（`<workspace>/hooks/`）：按 agent，最高优先级
- **托管 hooks**（`~/.openacosmi/hooks/`）：跨工作区共享

### 2. 创建目录结构

```bash
mkdir -p ~/.openacosmi/hooks/my-hook
cd ~/.openacosmi/hooks/my-hook
```

### 3. 创建 HOOK.md

```markdown
---
name: my-hook
description: "在 /new 时执行有用操作"
metadata: { "openacosmi": { "emoji": "🎯", "events": ["command:new"] } }
---

# My Custom Hook

此 hook 在执行 `/new` 时执行有用操作。
```

### 4. 创建 handler

编写 Go handler 函数（参见上方 Handler 实现示例）。

### 5. 启用和测试

```bash
# 验证 hook 已被发现
openacosmi hooks list

# 启用
openacosmi hooks enable my-hook

# 重启 Gateway 进程

# 通过消息渠道发送 /new 触发事件
```

## 配置

### 推荐格式

```json
{
  "hooks": {
    "internal": {
      "enabled": true,
      "entries": {
        "session-memory": { "enabled": true },
        "command-logger": { "enabled": false }
      }
    }
  }
}
```

### 按 Hook 配置

```json
{
  "hooks": {
    "internal": {
      "enabled": true,
      "entries": {
        "my-hook": {
          "enabled": true,
          "env": {
            "MY_CUSTOM_VAR": "value"
          }
        }
      }
    }
  }
}
```

### 额外目录

从其他目录加载 hooks：

```json
{
  "hooks": {
    "internal": {
      "enabled": true,
      "load": {
        "extraDirs": ["/path/to/more/hooks"]
      }
    }
  }
}
```

## CLI 命令

```bash
# 列出所有 hooks
openacosmi hooks list

# 仅显示合格的 hooks
openacosmi hooks list --eligible

# 详细输出（显示缺失需求）
openacosmi hooks list --verbose

# JSON 输出
openacosmi hooks list --json

# 查看详细信息
openacosmi hooks info session-memory

# 检查资格
openacosmi hooks check

# 启用/禁用
openacosmi hooks enable session-memory
openacosmi hooks disable command-logger
```

## 内置 Hook 详细参考

### session-memory

`/new` 时保存会话上下文到记忆文件。

- **事件**：`command:new`
- **需求**：`workspace.dir` 必须已配置
- **输出**：`<workspace>/memory/YYYY-MM-DD-slug.md`

工作流程：使用预重置会话条目定位 transcript → 提取最后 15 行对话 → LLM 生成描述性文件名 → 保存会话元数据。

### command-logger

记录所有命令事件到审计日志文件。

- **事件**：`command`
- **输出**：`~/.openacosmi/logs/commands.log`（JSONL 格式）

```bash
# 查看最近命令
tail -n 20 ~/.openacosmi/logs/commands.log

# 用 jq 美化
cat ~/.openacosmi/logs/commands.log | jq .
```

### boot-md

Gateway 启动时运行 `BOOT.md`。需启用 internal hooks。

- **事件**：`gateway:startup`
- **需求**：`workspace.dir` 必须已配置
- 读取工作区的 `BOOT.md` → 通过 agent runner 执行指令 → 通过 message tool 发送外发消息。

### soul-evil

在清洗窗口或随机概率下替换 `SOUL.md` 内容为 `SOUL_EVIL.md`。

- **事件**：`agent:bootstrap`
- **输出**：无文件写入；替换仅在内存中发生。

```json
{
  "hooks": {
    "internal": {
      "entries": {
        "soul-evil": {
          "enabled": true,
          "file": "SOUL_EVIL.md",
          "chance": 0.1,
          "purge": { "at": "21:00", "duration": "15m" }
        }
      }
}
}
}
```

## 最佳实践

### 保持 Handler 快速

Hooks 在命令处理期间运行，保持轻量：

```go
// ✓ 好 — 异步处理，立即返回
func Handler(event *hooks.InternalHookEvent) error {
    go processInBackground(event)
    return nil
}
```

### 优雅处理错误

```go
func Handler(event *hooks.InternalHookEvent) error {
    if err := riskyOperation(event); err != nil {
        log.Printf("[my-handler] 失败: %v", err)
        return nil // 不返回 error，允许其他 handler 继续
    }
    return nil
}
```

### 尽早过滤事件

```go
func Handler(event *hooks.InternalHookEvent) error {
    if event.Type != hooks.HookEventCommand || event.Action != "new" {
        return nil
    }
    // 处理逻辑...
    return nil
}
```

### 使用精确事件键

在元数据中尽量指定精确事件：

```yaml
metadata: { "openacosmi": { "events": ["command:new"] } }    # 精确 — 推荐
# 而非：
metadata: { "openacosmi": { "events": ["command"] } }        # 通用 — 开销更大
```

## 架构

### 核心组件（Go 实现）

- `backend/internal/hooks/hook_types.go`：类型定义（事件、元数据、快照）
- `backend/internal/hooks/hooks.go`：目录扫描与加载
- `backend/internal/hooks/hook_config.go`：资格检查与配置解析
- `backend/internal/hooks/hook_install.go`：Hook 安装逻辑
- `backend/internal/gateway/`：Gateway 启动时加载 hooks

### 发现流程

```text
Gateway 启动
    ↓
扫描目录（工作区 → 托管 → 内置）
    ↓
解析 HOOK.md 文件
    ↓
检查资格（bins、env、config、os）
    ↓
加载合格 hooks 的 handler
    ↓
注册 handler 到事件
```

### 事件流程

```text
用户发送 /new
    ↓
命令验证
    ↓
创建 hook 事件
    ↓
触发所有注册的 handler
    ↓
命令处理继续
    ↓
会话重置
```

## 故障排查

### Hook 未被发现

1. 检查目录结构：

```bash
ls -la ~/.openacosmi/hooks/my-hook/
# 应包含：HOOK.md, handler.go
```

1. 验证 HOOK.md 格式（需包含 YAML frontmatter）。
2. 运行 `openacosmi hooks list` 列出所有已发现 hooks。

### Hook 不合格

```bash
openacosmi hooks info my-hook
```

查看输出中的缺失需求。

### 调试日志

Gateway 在启动时记录 hook 加载日志：

```text
Registered hook: session-memory -> command:new
Registered hook: command-logger -> command
Registered hook: boot-md -> gateway:startup
```

监控 Gateway 日志：

```bash
# macOS
./scripts/clawlog.sh -f

# 其他平台
tail -f ~/.openacosmi/gateway.log
```
