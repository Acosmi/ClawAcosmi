---
summary: "健康检查与引导修复（Gateway + 渠道）"
read_when:
  - You have connectivity/auth issues and want guided fixes
  - You updated and want a sanity check
title: "doctor"
status: active
arch: rust-cli
crate: oa-cmd-doctor
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-doctor`（20 个模块，108+ 测试）。
> 入口二进制：`openacosmi`（Rust），本地检查 + RPC 探测 Go Gateway。

# `openacosmi doctor`

Health checks + quick fixes for the gateway and channels.

Related:

- Troubleshooting: [Troubleshooting](/gateway/troubleshooting)
- Security audit: [Security](/gateway/security)

## Examples

```bash
openacosmi doctor
openacosmi doctor --repair
openacosmi doctor --deep
```

Notes:

- Interactive prompts (like keychain/OAuth fixes) only run when stdin is a TTY and `--non-interactive` is **not** set. Headless runs (cron, Telegram, no terminal) will skip prompts.
- `--fix` (alias for `--repair`) writes a backup to `~/.openacosmi/openacosmi.json.bak` and drops unknown config keys, listing each removal.

## macOS: `launchctl` env overrides

If you previously ran `launchctl setenv OPENACOSMI_GATEWAY_TOKEN ...` (or `...PASSWORD`), that value overrides your config file and can cause persistent “unauthorized” errors.

```bash
launchctl getenv OPENACOSMI_GATEWAY_TOKEN
launchctl getenv OPENACOSMI_GATEWAY_PASSWORD

launchctl unsetenv OPENACOSMI_GATEWAY_TOKEN
launchctl unsetenv OPENACOSMI_GATEWAY_PASSWORD
```
