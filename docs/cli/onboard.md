---
summary: "交互式初始引导向导（本地/远程 Gateway 设置）"
read_when:
  - You want guided setup for gateway, workspace, auth, channels, and skills
title: "onboard"
status: active
arch: rust-cli
crate: oa-cmd-onboard
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-onboard`（73+ 测试）。

# `openacosmi onboard`

Interactive onboarding wizard (local or remote Gateway setup).

## Related guides

- CLI onboarding hub: [Onboarding Wizard (CLI)](/start/wizard)
- CLI onboarding reference: [CLI Onboarding Reference](/start/wizard-cli-reference)
- CLI automation: [CLI Automation](/start/wizard-cli-automation)
- macOS onboarding: [Onboarding (macOS App)](/start/onboarding)

## Examples

```bash
openacosmi onboard
openacosmi onboard --flow quickstart
openacosmi onboard --flow manual
openacosmi onboard --mode remote --remote-url ws://gateway-host:18789
```

Flow notes:

- `quickstart`: minimal prompts, auto-generates a gateway token.
- `manual`: full prompts for port/bind/auth (alias of `advanced`).
- Fastest first chat: `openacosmi dashboard` (Control UI, no channel setup).

## Common follow-up commands

```bash
openacosmi configure
openacosmi agents add <name>
```

<Note>
`--json` does not imply non-interactive mode. Use `--non-interactive` for scripts.
</Note>
