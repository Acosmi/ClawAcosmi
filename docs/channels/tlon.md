---
summary: "Tlon/Urbit 频道：能力、配置与运维"
read_when:
  - 处理 Tlon/Urbit 频道功能
title: "Tlon"
---

# Tlon

> [!IMPORTANT]
> **架构状态**：Tlon 频道由 **Go Gateway**（`backend/internal/channels/tlon/`）实现。
> CLI 命令通过 **Rust CLI** 提供。无需单独安装插件。

Tlon 是基于 Urbit 的去中心化通讯工具。支持 DM、群组提及、线程回复和纯文本媒体回退。

## Go Gateway 内置支持

Tlon 频道已在 Go Gateway 中原生实现，无需单独安装插件。确保 Go Gateway 已编译并运行即可。

## Setup

1. Install the Tlon plugin.
2. Gather your ship URL and login code.
3. Configure `channels.tlon`.
4. Restart the gateway.
5. DM the bot or mention it in a group channel.

Minimal config (single account):

```json5
{
  channels: {
    tlon: {
      enabled: true,
      ship: "~sampel-palnet",
      url: "https://your-ship-host",
      code: "lidlut-tabwed-pillex-ridrup",
    },
  },
}
```

## Group channels

Auto-discovery is enabled by default. You can also pin channels manually:

```json5
{
  channels: {
    tlon: {
      groupChannels: ["chat/~host-ship/general", "chat/~host-ship/support"],
    },
  },
}
```

Disable auto-discovery:

```json5
{
  channels: {
    tlon: {
      autoDiscoverChannels: false,
    },
  },
}
```

## Access control

DM allowlist (empty = allow all):

```json5
{
  channels: {
    tlon: {
      dmAllowlist: ["~zod", "~nec"],
    },
  },
}
```

Group authorization (restricted by default):

```json5
{
  channels: {
    tlon: {
      defaultAuthorizedShips: ["~zod"],
      authorization: {
        channelRules: {
          "chat/~host-ship/general": {
            mode: "restricted",
            allowedShips: ["~zod", "~nec"],
          },
          "chat/~host-ship/announcements": {
            mode: "open",
          },
        },
      },
    },
  },
}
```

## Delivery targets (CLI/cron)

Use these with `openacosmi message send` or cron delivery:

- DM: `~sampel-palnet` or `dm/~sampel-palnet`
- Group: `chat/~host-ship/channel` or `group:~host-ship/channel`

## Notes

- Group replies require a mention (e.g. `~your-bot-ship`) to respond.
- Thread replies: if the inbound message is in a thread, OpenAcosmi replies in-thread.
- Media: `sendMedia` falls back to text + URL (no native upload).
