---
summary: "CLI reference for `openacosmi approvals` (exec approvals for gateway or node hosts)"
read_when:
  - You want to edit exec approvals from the CLI
  - You need to manage allowlists on gateway or node hosts
title: "approvals"
status: active
arch: rust-cli
---

> [!NOTE]
> **架构状态：✅ 已适配** — Rust CLI crate (oa-cmd-approvals) + Go Gateway stub 均已注册。
> 命令功能为 stub 实现，待后续补充完整业务逻辑。

# `openacosmi approvals`

Manage exec approvals for the **local host**, **gateway host**, or a **node host**.
By default, commands target the local approvals file on disk. Use `--gateway` to target the gateway, or `--node` to target a specific node.

Related:

- Exec approvals: [Exec approvals](/tools/exec-approvals)
- Nodes: [Nodes](/nodes)

## Common commands

```bash
openacosmi approvals get
openacosmi approvals get --node <id|name|ip>
openacosmi approvals get --gateway
```

## Replace approvals from a file

```bash
openacosmi approvals set --file ./exec-approvals.json
openacosmi approvals set --node <id|name|ip> --file ./exec-approvals.json
openacosmi approvals set --gateway --file ./exec-approvals.json
```

## Allowlist helpers

```bash
openacosmi approvals allowlist add "~/Projects/**/bin/rg"
openacosmi approvals allowlist add --agent main --node <id|name|ip> "/usr/bin/uptime"
openacosmi approvals allowlist add --agent "*" "/usr/bin/uname"

openacosmi approvals allowlist remove "~/Projects/**/bin/rg"
```

## Notes

- `--node` uses the same resolver as `openacosmi nodes` (id, name, ip, or id prefix).
- `--agent` defaults to `"*"`, which applies to all agents.
- The node host must advertise `system.execApprovals.get/set` (macOS app or headless node host).
- Approvals files are stored per host at `~/.openacosmi/exec-approvals.json`.
