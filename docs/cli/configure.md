---
summary: "交互式配置向导（凭据、设备、agent 默认值）"
read_when:
  - You want to tweak credentials, devices, or agent defaults interactively
title: "configure"
status: active
arch: rust-cli
crate: oa-cmd-configure
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-configure`（`cli-rust/crates/oa-cmd-configure/`）。
> 入口二进制：`openacosmi`（Rust），本地配置操作。

# `openacosmi configure`

Interactive prompt to set up credentials, devices, and agent defaults.

Note: The **Model** section now includes a multi-select for the
`agents.defaults.models` allowlist (what shows up in `/model` and the model picker).

Tip: `openacosmi config` without a subcommand opens the same wizard. Use
`openacosmi config get|set|unset` for non-interactive edits.

Related:

- Gateway configuration reference: [Configuration](/gateway/configuration)
- Config CLI: [Config](/cli/config)

Notes:

- Choosing where the Gateway runs always updates `gateway.mode`. You can select "Continue" without other sections if that is all you need.
- Channel-oriented services (Slack/Discord/Matrix/Microsoft Teams) prompt for channel/room allowlists during setup. You can enter names or IDs; the wizard resolves names to IDs when possible.

## Examples

```bash
openacosmi configure
openacosmi configure --section models --section channels
```
