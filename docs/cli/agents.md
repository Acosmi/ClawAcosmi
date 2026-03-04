---
summary: "管理隔离的 agent（工作区 + 认证 + 路由）"
read_when:
  - You want multiple isolated agents (workspaces + routing + auth)
title: "agents"
status: active
arch: rust-cli
crate: oa-cmd-agents
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-agents`（`cli-rust/crates/oa-cmd-agents/`）。
> 入口二进制：`openacosmi`（Rust），本地操作 + RPC 调用 Go Gateway。

# `openacosmi agents`

Manage isolated agents (workspaces + auth + routing).

Related:

- Multi-agent routing: [Multi-Agent Routing](/concepts/multi-agent)
- Agent workspace: [Agent workspace](/concepts/agent-workspace)

## Examples

```bash
openacosmi agents list
openacosmi agents add work --workspace ~/.openacosmi/workspace-work
openacosmi agents set-identity --workspace ~/.openacosmi/workspace --from-identity
openacosmi agents set-identity --agent main --avatar avatars/openacosmi.png
openacosmi agents delete work
```

## Identity files

Each agent workspace can include an `IDENTITY.md` at the workspace root:

- Example path: `~/.openacosmi/workspace/IDENTITY.md`
- `set-identity --from-identity` reads from the workspace root (or an explicit `--identity-file`)

Avatar paths resolve relative to the workspace root.

## Set identity

`set-identity` writes fields into `agents.list[].identity`:

- `name`
- `theme`
- `emoji`
- `avatar` (workspace-relative path, http(s) URL, or data URI)

Load from `IDENTITY.md`:

```bash
openacosmi agents set-identity --workspace ~/.openacosmi/workspace --from-identity
```

Override fields explicitly:

```bash
openacosmi agents set-identity --agent main --name "OpenAcosmi" --emoji "🦜" --avatar avatars/openacosmi.png
```

Config sample:

```json5
{
  agents: {
    list: [
      {
        id: "main",
        identity: {
          name: "OpenAcosmi",
          theme: "space lobster",
          emoji: "🦜",
          avatar: "avatars/openacosmi.png",
        },
      },
    ],
  },
}
```
