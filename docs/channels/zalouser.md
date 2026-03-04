---
summary: "Zalo 个人账户支持：通过 zca-cli（二维码登录）"
read_when:
  - 设置 Zalo Personal
  - 调试 Zalo Personal 登录或消息流
title: "Zalo Personal"
---

# Zalo Personal（非官方）

> [!IMPORTANT]
> **架构状态**：Zalo Personal 频道由 **Go Gateway**（`backend/internal/channels/zalouser/`）实现。
> CLI 命令通过 **Rust CLI** 提供。无需单独安装插件。

状态：实验性。此集成通过 `zca-cli` 自动化操作 **个人 Zalo 账户**。

> **警告：** 这是非官方集成，可能导致账户被封禁。风隩自担。

## Go Gateway 内置支持

Zalo Personal 频道已在 Go Gateway 中原生实现，无需单独安装插件。确保 Go Gateway 已编译并运行即可。

## Prerequisite: zca-cli

The Gateway machine must have the `zca` binary available in `PATH`.

- Verify: `zca --version`
- If missing, install zca-cli (see `extensions/zalouser/README.md` or the upstream zca-cli docs).

## Quick setup (beginner)

1. Install the plugin (see above).
2. Login (QR, on the Gateway machine):
   - `openacosmi channels login --channel zalouser`
   - Scan the QR code in the terminal with the Zalo mobile app.
3. Enable the channel:

```json5
{
  channels: {
    zalouser: {
      enabled: true,
      dmPolicy: "pairing",
    },
  },
}
```

1. Restart the Gateway (or finish onboarding).
2. DM access defaults to pairing; approve the pairing code on first contact.

## What it is

- Uses `zca listen` to receive inbound messages.
- Uses `zca msg ...` to send replies (text/media/link).
- Designed for “personal account” use cases where Zalo Bot API is not available.

## Naming

Channel id is `zalouser` to make it explicit this automates a **personal Zalo user account** (unofficial). We keep `zalo` reserved for a potential future official Zalo API integration.

## Finding IDs (directory)

Use the directory CLI to discover peers/groups and their IDs:

```bash
openacosmi directory self --channel zalouser
openacosmi directory peers list --channel zalouser --query "name"
openacosmi directory groups list --channel zalouser --query "work"
```

## Limits

- Outbound text is chunked to ~2000 characters (Zalo client limits).
- Streaming is blocked by default.

## Access control (DMs)

`channels.zalouser.dmPolicy` supports: `pairing | allowlist | open | disabled` (default: `pairing`).
`channels.zalouser.allowFrom` accepts user IDs or names. The wizard resolves names to IDs via `zca friend find` when available.

Approve via:

- `openacosmi pairing list zalouser`
- `openacosmi pairing approve zalouser <code>`

## Group access (optional)

- Default: `channels.zalouser.groupPolicy = "open"` (groups allowed). Use `channels.defaults.groupPolicy` to override the default when unset.
- Restrict to an allowlist with:
  - `channels.zalouser.groupPolicy = "allowlist"`
  - `channels.zalouser.groups` (keys are group IDs or names)
- Block all groups: `channels.zalouser.groupPolicy = "disabled"`.
- The configure wizard can prompt for group allowlists.
- On startup, OpenAcosmi resolves group/user names in allowlists to IDs and logs the mapping; unresolved entries are kept as typed.

Example:

```json5
{
  channels: {
    zalouser: {
      groupPolicy: "allowlist",
      groups: {
        "123456789": { allow: true },
        "Work Chat": { allow: true },
      },
    },
  },
}
```

## Multi-account

Accounts map to zca profiles. Example:

```json5
{
  channels: {
    zalouser: {
      enabled: true,
      defaultAccount: "default",
      accounts: {
        work: { enabled: true, profile: "work" },
      },
    },
  },
}
```

## Troubleshooting

**`zca` not found:**

- Install zca-cli and ensure it’s on `PATH` for the Gateway process.

**Login doesn’t stick:**

- `openacosmi channels status --probe`
- Re-login: `openacosmi channels logout --channel zalouser && openacosmi channels login --channel zalouser`
