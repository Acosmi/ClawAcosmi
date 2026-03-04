---
summary: "CLI reference for `openacosmi devices` (device pairing + token rotation/revocation)"
read_when:
  - You are approving device pairing requests
  - You need to rotate or revoke device tokens
title: "devices"
status: active
arch: rust-cli
---

> [!NOTE]
> **架构状态：✅ 已适配** — Rust CLI crate (oa-cmd-devices) + Go Gateway stub 均已注册。
> 命令功能为 stub 实现，待后续补充完整业务逻辑。

# `openacosmi devices`

Manage device pairing requests and device-scoped tokens.

## Commands

### `openacosmi devices list`

List pending pairing requests and paired devices.

```
openacosmi devices list
openacosmi devices list --json
```

### `openacosmi devices approve <requestId>`

Approve a pending device pairing request.

```
openacosmi devices approve <requestId>
```

### `openacosmi devices reject <requestId>`

Reject a pending device pairing request.

```
openacosmi devices reject <requestId>
```

### `openacosmi devices rotate --device <id> --role <role> [--scope <scope...>]`

Rotate a device token for a specific role (optionally updating scopes).

```
openacosmi devices rotate --device <deviceId> --role operator --scope operator.read --scope operator.write
```

### `openacosmi devices revoke --device <id> --role <role>`

Revoke a device token for a specific role.

```
openacosmi devices revoke --device <deviceId> --role node
```

## Common options

- `--url <url>`: Gateway WebSocket URL (defaults to `gateway.remote.url` when configured).
- `--token <token>`: Gateway token (if required).
- `--password <password>`: Gateway password (password auth).
- `--timeout <ms>`: RPC timeout.
- `--json`: JSON output (recommended for scripting).

Note: when you set `--url`, the CLI does not fall back to config or environment credentials.
Pass `--token` or `--password` explicitly. Missing explicit credentials is an error.

## Notes

- Token rotation returns a new token (sensitive). Treat it like a secret.
- These commands require `operator.pairing` (or `operator.admin`) scope.
