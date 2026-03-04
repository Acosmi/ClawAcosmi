---
summary: "管理聊天渠道账号及运行状态（WhatsApp/Telegram/Discord/Slack 等）"
read_when:
  - You want to add/remove channel accounts (WhatsApp/Telegram/Discord/Google Chat/Slack/Mattermost (plugin)/Signal/iMessage)
  - You want to check channel status or tail channel logs
title: "channels"
status: active
arch: rust-cli
crate: oa-cmd-channels
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-channels`（`cli-rust/crates/oa-cmd-channels/`）。
> 入口二进制：`openacosmi`（Rust），通过 WebSocket RPC 调用 Go Gateway 管理渠道。

# `openacosmi channels`

Manage chat channel accounts and their runtime status on the Gateway.

Related docs:

- Channel guides: [Channels](/channels/index)
- Gateway configuration: [Configuration](/gateway/configuration)

## Common commands

```bash
openacosmi channels list
openacosmi channels status
openacosmi channels capabilities
openacosmi channels capabilities --channel discord --target channel:123
openacosmi channels resolve --channel slack "#general" "@jane"
openacosmi channels logs --channel all
```

## Add / remove accounts

```bash
openacosmi channels add --channel telegram --token <bot-token>
openacosmi channels remove --channel telegram --delete
```

Tip: `openacosmi channels add --help` shows per-channel flags (token, app token, signal-cli paths, etc).

## Login / logout (interactive)

```bash
openacosmi channels login --channel whatsapp
openacosmi channels logout --channel whatsapp
```

## Troubleshooting

- Run `openacosmi status --deep` for a broad probe.
- Use `openacosmi doctor` for guided fixes.
- `openacosmi channels list` prints `Claude: HTTP 403 ... user:profile` → usage snapshot needs the `user:profile` scope. Use `--no-usage`, or provide a claude.ai session key (`CLAUDE_WEB_SESSION_KEY` / `CLAUDE_WEB_COOKIE`), or re-auth via Claude Code CLI.

## Capabilities probe

Fetch provider capability hints (intents/scopes where available) plus static feature support:

```bash
openacosmi channels capabilities
openacosmi channels capabilities --channel discord --target channel:123
```

Notes:

- `--channel` is optional; omit it to list every channel (including extensions).
- `--target` accepts `channel:<id>` or a raw numeric channel id and only applies to Discord.
- Probes are provider-specific: Discord intents + optional channel permissions; Slack bot + user scopes; Telegram bot flags + webhook; Signal daemon version; MS Teams app token + Graph roles/scopes (annotated where known). Channels without probes report `Probe: unavailable`.

## Resolve names to IDs

Resolve channel/user names to IDs using the provider directory:

```bash
openacosmi channels resolve --channel slack "#general" "@jane"
openacosmi channels resolve --channel discord "My Server/#support" "@someone"
openacosmi channels resolve --channel matrix "Project Room"
```

Notes:

- Use `--kind user|group|auto` to force the target type.
- Resolution prefers active matches when multiple entries share the same name.
