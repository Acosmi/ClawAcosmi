---
summary: "Matrix 频道：能力、配置与运维"
read_when:
  - 处理 Matrix 频道功能
title: "Matrix"
---

# Matrix

> [!IMPORTANT]
> **架构状态**：Matrix 频道由 **Go Gateway**（`backend/internal/channels/matrix/`）实现。
> CLI 命令通过 **Rust CLI** 提供。无需单独安装插件。

Matrix 是开放、去中心化的消息协议。OpenAcosmi 以 Matrix **用户**身份连接到任意家服务器。
支持：DM、房间、线程、媒体、反应、投票、位置和 E2EE（加密支持）。

## Go Gateway 内置支持

Matrix 频道已在 Go Gateway 中原生实现，无需单独安装插件。确保 Go Gateway 已编译并运行即可。

## Setup

1. Install the Matrix plugin:
   - From npm: `openacosmi plugins install @openacosmi/matrix`
   - From a local checkout: `openacosmi plugins install ./extensions/matrix`
2. Create a Matrix account on a homeserver:
   - Browse hosting options at [https://matrix.org/ecosystem/hosting/](https://matrix.org/ecosystem/hosting/)
   - Or host it yourself.
3. Get an access token for the bot account:
   - Use the Matrix login API with `curl` at your home server:

   ```bash
   curl --request POST \
     --url https://matrix.example.org/_matrix/client/v3/login \
     --header 'Content-Type: application/json' \
     --data '{
     "type": "m.login.password",
     "identifier": {
       "type": "m.id.user",
       "user": "your-user-name"
     },
     "password": "your-password"
   }'
   ```

   - Replace `matrix.example.org` with your homeserver URL.
   - Or set `channels.matrix.userId` + `channels.matrix.password`: OpenAcosmi calls the same
     login endpoint, stores the access token in `~/.openacosmi/credentials/matrix/credentials.json`,
     and reuses it on next start.

4. Configure credentials:
   - Env: `MATRIX_HOMESERVER`, `MATRIX_ACCESS_TOKEN` (or `MATRIX_USER_ID` + `MATRIX_PASSWORD`)
   - Or config: `channels.matrix.*`
   - If both are set, config takes precedence.
   - With access token: user ID is fetched automatically via `/whoami`.
   - When set, `channels.matrix.userId` should be the full Matrix ID (example: `@bot:example.org`).
5. Restart the gateway (or finish onboarding).
6. Start a DM with the bot or invite it to a room from any Matrix client
   (Element, Beeper, etc.; see [https://matrix.org/ecosystem/clients/](https://matrix.org/ecosystem/clients/)). Beeper requires E2EE,
   so set `channels.matrix.encryption: true` and verify the device.

Minimal config (access token, user ID auto-fetched):

```json5
{
  channels: {
    matrix: {
      enabled: true,
      homeserver: "https://matrix.example.org",
      accessToken: "syt_***",
      dm: { policy: "pairing" },
    },
  },
}
```

E2EE config (end to end encryption enabled):

```json5
{
  channels: {
    matrix: {
      enabled: true,
      homeserver: "https://matrix.example.org",
      accessToken: "syt_***",
      encryption: true,
      dm: { policy: "pairing" },
    },
  },
}
```

## Encryption (E2EE)

End-to-end encryption is **supported** via the Rust crypto SDK.

Enable with `channels.matrix.encryption: true`:

- If the crypto module loads, encrypted rooms are decrypted automatically.
- Outbound media is encrypted when sending to encrypted rooms.
- On first connection, OpenAcosmi requests device verification from your other sessions.
- Verify the device in another Matrix client (Element, etc.) to enable key sharing.
- If the crypto module cannot be loaded, E2EE is disabled and encrypted rooms will not decrypt;
  OpenAcosmi logs a warning.
- If you see missing crypto module errors (for example, `@matrix-org/matrix-sdk-crypto-nodejs-*`),
  allow build scripts for `@matrix-org/matrix-sdk-crypto-nodejs` and run
  `pnpm rebuild @matrix-org/matrix-sdk-crypto-nodejs` or fetch the binary with
  `node node_modules/@matrix-org/matrix-sdk-crypto-nodejs/download-lib.js`.

Crypto state is stored per account + access token in
`~/.openacosmi/matrix/accounts/<account>/<homeserver>__<user>/<token-hash>/crypto/`
(SQLite database). Sync state lives alongside it in `bot-storage.json`.
If the access token (device) changes, a new store is created and the bot must be
re-verified for encrypted rooms.

**Device verification:**
When E2EE is enabled, the bot will request verification from your other sessions on startup.
Open Element (or another client) and approve the verification request to establish trust.
Once verified, the bot can decrypt messages in encrypted rooms.

## Routing model

- Replies always go back to Matrix.
- DMs share the agent's main session; rooms map to group sessions.

## Access control (DMs)

- Default: `channels.matrix.dm.policy = "pairing"`. Unknown senders get a pairing code.
- Approve via:
  - `openacosmi pairing list matrix`
  - `openacosmi pairing approve matrix <CODE>`
- Public DMs: `channels.matrix.dm.policy="open"` plus `channels.matrix.dm.allowFrom=["*"]`.
- `channels.matrix.dm.allowFrom` accepts full Matrix user IDs (example: `@user:server`). The wizard resolves display names to user IDs when directory search finds a single exact match.

## Rooms (groups)

- Default: `channels.matrix.groupPolicy = "allowlist"` (mention-gated). Use `channels.defaults.groupPolicy` to override the default when unset.
- Allowlist rooms with `channels.matrix.groups` (room IDs or aliases; names are resolved to IDs when directory search finds a single exact match):

```json5
{
  channels: {
    matrix: {
      groupPolicy: "allowlist",
      groups: {
        "!roomId:example.org": { allow: true },
        "#alias:example.org": { allow: true },
      },
      groupAllowFrom: ["@owner:example.org"],
    },
  },
}
```

- `requireMention: false` enables auto-reply in that room.
- `groups."*"` can set defaults for mention gating across rooms.
- `groupAllowFrom` restricts which senders can trigger the bot in rooms (full Matrix user IDs).
- Per-room `users` allowlists can further restrict senders inside a specific room (use full Matrix user IDs).
- The configure wizard prompts for room allowlists (room IDs, aliases, or names) and resolves names only on an exact, unique match.
- On startup, OpenAcosmi resolves room/user names in allowlists to IDs and logs the mapping; unresolved entries are ignored for allowlist matching.
- Invites are auto-joined by default; control with `channels.matrix.autoJoin` and `channels.matrix.autoJoinAllowlist`.
- To allow **no rooms**, set `channels.matrix.groupPolicy: "disabled"` (or keep an empty allowlist).
- Legacy key: `channels.matrix.rooms` (same shape as `groups`).

## Threads

- Reply threading is supported.
- `channels.matrix.threadReplies` controls whether replies stay in threads:
  - `off`, `inbound` (default), `always`
- `channels.matrix.replyToMode` controls reply-to metadata when not replying in a thread:
  - `off` (default), `first`, `all`

## Capabilities

| Feature         | Status                                                                                |
| --------------- | ------------------------------------------------------------------------------------- |
| Direct messages | ✅ Supported                                                                          |
| Rooms           | ✅ Supported                                                                          |
| Threads         | ✅ Supported                                                                          |
| Media           | ✅ Supported                                                                          |
| E2EE            | ✅ Supported (crypto module required)                                                 |
| Reactions       | ✅ Supported (send/read via tools)                                                    |
| Polls           | ✅ Send supported; inbound poll starts are converted to text (responses/ends ignored) |
| Location        | ✅ Supported (geo URI; altitude ignored)                                              |
| Native commands | ✅ Supported                                                                          |

## Troubleshooting

Run this ladder first:

```bash
openacosmi status
openacosmi gateway status
openacosmi logs --follow
openacosmi doctor
openacosmi channels status --probe
```

Then confirm DM pairing state if needed:

```bash
openacosmi pairing list matrix
```

Common failures:

- Logged in but room messages ignored: room blocked by `groupPolicy` or room allowlist.
- DMs ignored: sender pending approval when `channels.matrix.dm.policy="pairing"`.
- Encrypted rooms fail: crypto support or encryption settings mismatch.

For triage flow: [/channels/troubleshooting](/channels/troubleshooting).

## Configuration reference (Matrix)

Full configuration: [Configuration](/gateway/configuration)

Provider options:

- `channels.matrix.enabled`: enable/disable channel startup.
- `channels.matrix.homeserver`: homeserver URL.
- `channels.matrix.userId`: Matrix user ID (optional with access token).
- `channels.matrix.accessToken`: access token.
- `channels.matrix.password`: password for login (token stored).
- `channels.matrix.deviceName`: device display name.
- `channels.matrix.encryption`: enable E2EE (default: false).
- `channels.matrix.initialSyncLimit`: initial sync limit.
- `channels.matrix.threadReplies`: `off | inbound | always` (default: inbound).
- `channels.matrix.textChunkLimit`: outbound text chunk size (chars).
- `channels.matrix.chunkMode`: `length` (default) or `newline` to split on blank lines (paragraph boundaries) before length chunking.
- `channels.matrix.dm.policy`: `pairing | allowlist | open | disabled` (default: pairing).
- `channels.matrix.dm.allowFrom`: DM allowlist (full Matrix user IDs). `open` requires `"*"`. The wizard resolves names to IDs when possible.
- `channels.matrix.groupPolicy`: `allowlist | open | disabled` (default: allowlist).
- `channels.matrix.groupAllowFrom`: allowlisted senders for group messages (full Matrix user IDs).
- `channels.matrix.allowlistOnly`: force allowlist rules for DMs + rooms.
- `channels.matrix.groups`: group allowlist + per-room settings map.
- `channels.matrix.rooms`: legacy group allowlist/config.
- `channels.matrix.replyToMode`: reply-to mode for threads/tags.
- `channels.matrix.mediaMaxMb`: inbound/outbound media cap (MB).
- `channels.matrix.autoJoin`: invite handling (`always | allowlist | off`, default: always).
- `channels.matrix.autoJoinAllowlist`: allowed room IDs/aliases for auto-join.
- `channels.matrix.actions`: per-action tool gating (reactions/messages/pins/memberInfo/channelInfo).
