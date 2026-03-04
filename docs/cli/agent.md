---
summary: "通过 Gateway 运行单次 agent 回合（可选投递回复）"
read_when:
  - You want to run one agent turn from scripts (optionally deliver reply)
title: "agent"
status: active
arch: rust-cli
crate: oa-cmd-agent
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-agent`（`cli-rust/crates/oa-cmd-agent/`）。
> 入口二进制：`openacosmi`（Rust），通过 WebSocket RPC 调用 Go Gateway。

# `openacosmi agent`

Run an agent turn via the Gateway (use `--local` for embedded).
Use `--agent <id>` to target a configured agent directly.

Related:

- Agent send tool: [Agent send](/tools/agent-send)

## Examples

```bash
openacosmi agent --to +15555550123 --message "status update" --deliver
openacosmi agent --agent ops --message "Summarize logs"
openacosmi agent --session-id 1234 --message "Summarize inbox" --thinking medium
openacosmi agent --agent ops --message "Generate report" --deliver --reply-channel slack --reply-to "#reports"
```
