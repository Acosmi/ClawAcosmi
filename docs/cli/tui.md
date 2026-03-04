---
summary: "CLI reference for `openacosmi tui` (terminal UI connected to the Gateway)"
read_when:
  - You want a terminal UI for the Gateway (remote-friendly)
  - You want to pass url/token/session from scripts
title: "tui"
status: active
arch: rust-cli
---

> [!NOTE]
> **架构状态：✅ 已适配** — Rust CLI crate (oa-cmd-tui) + Go Gateway stub 均已注册。
> 命令功能为 stub 实现，待后续补充完整业务逻辑。

# `openacosmi tui`

Open the terminal UI connected to the Gateway.

Related:

- TUI guide: [TUI](/web/tui)

## Examples

```bash
openacosmi tui
openacosmi tui --url ws://127.0.0.1:18789 --token <token>
openacosmi tui --session main --deliver
```
