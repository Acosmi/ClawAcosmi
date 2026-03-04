---
summary: "CLI reference for `openacosmi voicecall` (voice-call plugin command surface)"
read_when:
  - You use the voice-call plugin and want the CLI entry points
  - You want quick examples for `voicecall call|continue|status|tail|expose`
title: "voicecall"
status: active
arch: rust-cli
---

> [!NOTE]
> **架构状态：✅ 已适配** — Rust CLI crate (oa-cmd-voicecall) + Go Gateway stub 均已注册。
> 命令功能为 stub 实现，待后续补充完整业务逻辑。

# `openacosmi voicecall`

`voicecall` is a plugin-provided command. It only appears if the voice-call plugin is installed and enabled.

Primary doc:

- Voice-call plugin: [Voice Call](/plugins/voice-call)

## Common commands

```bash
openacosmi voicecall status --call-id <id>
openacosmi voicecall call --to "+15555550123" --message "Hello" --mode notify
openacosmi voicecall continue --call-id <id> --message "Any questions?"
openacosmi voicecall end --call-id <id>
```

## Exposing webhooks (Tailscale)

```bash
openacosmi voicecall expose --mode serve
openacosmi voicecall expose --mode funnel
openacosmi voicecall unexpose
```

Security note: only expose the webhook endpoint to networks you trust. Prefer Tailscale Serve over Funnel when possible.
