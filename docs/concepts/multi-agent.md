---
summary: "多 Agent 路由：隔离 Agent、通道账户和绑定规则"
title: "多 Agent 路由"
read_when: "需要在一个 Gateway 进程中运行多个隔离 Agent（各有独立工作区 + 认证）。"
status: active
arch: go-gateway
---

# 多 Agent 路由

> [!IMPORTANT]
> **架构状态**：多 Agent 路由由 **Go Gateway** 实现（`backend/internal/agents/scope/`）。
> Rust CLI 通过 `openacosmi agents list --bindings` 查看路由配置，通过 `openacosmi agents add` 添加新 Agent。

目标：多个**隔离** Agent（各有独立工作区 + `agentDir` + 会话），加上多个通道账户（如两个 WhatsApp）在一个 Gateway 进程中运行。入站消息通过绑定（bindings）路由到 Agent。

## "一个 Agent" 是什么？

一个 **Agent** 是一个完全隔离的"大脑"，拥有自己的：

- **工作区**（文件、AGENTS.md/SOUL.md/USER.md、本地笔记、人格规则）。
- **状态目录**（`agentDir`）用于认证 Profile、模型注册表和按 Agent 配置。
- **会话存储**（聊天历史 + 路由状态）位于 `~/.openacosmi/agents/<agentId>/sessions`。

认证 Profile 是**按 Agent 隔离的**。每个 Agent 从自己的文件读取：

```
~/.openacosmi/agents/<agentId>/agent/auth-profiles.json
```

主 Agent 的凭证**不会**自动共享。不要跨 Agent 复用 `agentDir`（会导致认证/会话冲突）。如需共享凭证，将 `auth-profiles.json` 复制到另一个 Agent 的 `agentDir`。

Skills 通过每个工作区的 `skills/` 目录按 Agent 隔离，共享 Skills 可从 `~/.openacosmi/skills` 获取。

Gateway 可托管**一个 Agent**（默认）或**多个 Agent** 并行。

## 路径（快速映射）

- 配置：`~/.openacosmi/openacosmi.json`（或 `OPENACOSMI_CONFIG_PATH`）
- 状态目录：`~/.openacosmi`（或 `OPENACOSMI_STATE_DIR`）
- 工作区：`~/.openacosmi/workspace`（或 `~/.openacosmi/workspace-<agentId>`）
- Agent 目录：`~/.openacosmi/agents/<agentId>/agent`（或 `agents.list[].agentDir`）
- 会话：`~/.openacosmi/agents/<agentId>/sessions`

### 单 Agent 模式（默认）

如果不做任何配置，OpenAcosmi 运行单个 Agent：

- `agentId` 默认为 **`main`**。
- 会话键为 `agent:main:<mainKey>`。
- 工作区默认为 `~/.openacosmi/workspace`。

## Agent 辅助命令

使用 Agent 向导添加新的隔离 Agent：

```bash
openacosmi agents add work
```

然后添加 `bindings`（或让向导完成）以路由入站消息。

验证：

```bash
openacosmi agents list --bindings
```

## 多 Agent = 多用户、多人格

使用**多 Agent** 时，每个 `agentId` 成为**完全隔离的人格**：

- **不同的手机号/账户**（按通道 `accountId`）。
- **不同的个性**（按 Agent 的工作区文件如 `AGENTS.md` 和 `SOUL.md`）。
- **独立的认证 + 会话**（除非显式启用，否则无交叉通信）。

这允许**多人**共享一个 Gateway 服务器，同时保持 AI "大脑" 和数据隔离。

## 一个 WhatsApp 号码，多用户（私聊分割）

可将**不同的 WhatsApp 私聊**路由到不同 Agent，同时使用**一个 WhatsApp 账户**。通过 `peer.kind: "direct"` 按发送者 E.164 匹配。回复仍来自同一 WhatsApp 号码。

**重要细节**：私聊折叠到 Agent 的**主会话键**，因此真正的隔离需要**每人一个 Agent**。

示例：

```json5
{
  agents: {
    list: [
      { id: "alex", workspace: "~/.openacosmi/workspace-alex" },
      { id: "mia", workspace: "~/.openacosmi/workspace-mia" },
    ],
  },
  bindings: [
    {
      agentId: "alex",
      match: { channel: "whatsapp", peer: { kind: "direct", id: "+15551230001" } },
    },
    {
      agentId: "mia",
      match: { channel: "whatsapp", peer: { kind: "direct", id: "+15551230002" } },
    },
  ],
  channels: {
    whatsapp: {
      dmPolicy: "allowlist",
      allowFrom: ["+15551230001", "+15551230002"],
    },
  },
}
```

说明：

- 私聊访问控制是**全局的**（按 WhatsApp 账户的配对/白名单），不是按 Agent 的。
- 共享群组时，将群组绑定到一个 Agent 或使用[广播组](/channels/broadcast-groups)。

## 路由规则（消息如何选择 Agent）

绑定是**确定性的**，**最具体优先**：

1. `peer` 匹配（精确私聊/群组/频道 ID）
2. `guildId`（Discord）
3. `teamId`（Slack）
4. `accountId` 匹配
5. 通道级匹配（`accountId: "*"`）
6. 回退到默认 Agent（`agents.list[].default`，否则列表第一项，默认 `main`）

## Per-Agent 沙箱与工具配置

每个 Agent 可拥有独立的沙箱和工具限制：

```js
{
  agents: {
    list: [
      {
        id: "personal",
        workspace: "~/.openacosmi/workspace-personal",
        sandbox: { mode: "off" },
        // 无工具限制 - 所有工具可用
      },
      {
        id: "family",
        workspace: "~/.openacosmi/workspace-family",
        sandbox: {
          mode: "all",
          scope: "agent",
          docker: {
            setupCommand: "apt-get update && apt-get install -y git curl",
          },
        },
        tools: {
          allow: ["read"],
          deny: ["exec", "write", "edit", "apply_patch"],
        },
      },
    ],
  },
}
```

**优势：**

- **安全隔离**：限制不受信任 Agent 的工具
- **资源控制**：沙箱特定 Agent 同时保持其他 Agent 在主机上运行
- **灵活策略**：按 Agent 不同的权限

说明：`tools.elevated` 是**全局的**且基于发送者；不可按 Agent 配置。如需按 Agent 边界，使用 `agents.list[].tools` 拒绝 `exec`。

## 代码位置

| 组件 | 位置 |
|------|------|
| Agent 作用域解析 | `backend/internal/agents/scope/` |
| 路由绑定 | `backend/internal/routing/` |
| Rust CLI agents 命令 | `cli-rust/crates/oa-cmd-agents/` |
| Rust 路由库 | `cli-rust/crates/oa-routing/` |
